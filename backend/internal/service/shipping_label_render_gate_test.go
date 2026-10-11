//go:build gotenberg_integration

package service

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestShippingLabelRender_PageGeometryAndContent(t *testing.T) {
	short := labelTestOrderFull("GATESHORT123")
	long := labelTestOrderFull("GATELONG123")
	long.ShippingAddress, _ = json.Marshal(map[string]string{"penerima": "Recipient Test", "telepon": "081200000000", "alamat": strings.Repeat("Long test street district province ", 3), "kode_pos": "12345"})
	long.Items = nil
	for i := 1; i <= 12; i++ {
		long.Items = append(long.Items, model.OrderItem{Name: fmt.Sprintf("Gate book %d", i), ProductType: "book", Qty: 1, WeightGrams: 100})
	}
	orders := []model.Order{short, long, short}
	renderer := NewGotenbergPDFGenerator(renderGateGotenbergURL(), nil)
	for _, size := range []string{"100x150", "roll80"} {
		t.Run(size, func(t *testing.T) {
			var slips []shippingLabelHTMLData
			for _, order := range orders {
				data, err := shippingLabelData(order, labelTestSenderConfig())
				if err != nil {
					t.Fatal(err)
				}
				slips = append(slips, data)
			}
			html, err := buildShippingLabelsHTML(slips, size)
			if err != nil {
				t.Fatal(err)
			}
			pdf, err := renderer.RenderHTML(context.Background(), html)
			if err != nil {
				t.Fatal(err)
			}
			boxes := regexp.MustCompile(`/MediaBox\s*\[\s*0\s+0\s+([\d.]+)\s+([\d.]+)\s*\]`).FindAllSubmatch(pdf, -1)
			if len(boxes) != len(orders) {
				t.Fatalf("page boxes=%d want=%d", len(boxes), len(orders))
			}
			heights := make([]float64, len(boxes))
			for i, box := range boxes {
				w, err := strconv.ParseFloat(string(box[1]), 64)
				if err != nil {
					t.Fatal(err)
				}
				h, err := strconv.ParseFloat(string(box[2]), 64)
				if err != nil {
					t.Fatal(err)
				}
				w *= 25.4 / 72
				h *= 25.4 / 72
				heights[i] = h
				wantWidth := 100.0
				if size == "roll80" {
					wantWidth = 80
				}
				if math.Abs(w-wantWidth) > 0.5 || (size == "100x150" && math.Abs(h-150) > 0.5) {
					t.Fatalf("page %d = %.2fx%.2fmm", i+1, w, h)
				}
				t.Logf("page %d = %.2fx%.2fmm", i+1, w, h)
			}
			if size == "roll80" && heights[1] <= heights[0]+5 {
				t.Fatalf("long slip must have its own longer page: %v", heights)
			}
			dir := t.TempDir()
			pdfPath := filepath.Join(dir, "labels.pdf")
			if err := os.WriteFile(pdfPath, pdf, 0600); err != nil {
				t.Fatal(err)
			}
			out, err := exec.Command("pdftotext", "-bbox", pdfPath, "-").CombinedOutput()
			if err != nil {
				t.Fatalf("pdftotext: %v %s", err, out)
			}
			var doc struct {
				Body struct {
					Pages []struct {
						Width  float64 `xml:"width,attr"`
						Height float64 `xml:"height,attr"`
						Words  []struct {
							XMin float64 `xml:"xMin,attr"`
							YMin float64 `xml:"yMin,attr"`
							XMax float64 `xml:"xMax,attr"`
							YMax float64 `xml:"yMax,attr"`
							Text string  `xml:",chardata"`
						} `xml:"word"`
					} `xml:"doc>page"`
				} `xml:"body"`
			}
			if err := xml.Unmarshal(out, &doc); err != nil {
				t.Fatal(err)
			}
			if len(doc.Body.Pages) != len(orders) {
				t.Fatalf("text pages=%d", len(doc.Body.Pages))
			}
			prefix := filepath.Join(dir, "page")
			if out, err := exec.Command("pdftoppm", "-r", "100", "-png", pdfPath, prefix).CombinedOutput(); err != nil {
				t.Fatalf("pdftoppm: %v %s", err, out)
			}
			for i, page := range doc.Body.Pages {
				var words []string
				for _, word := range page.Words {
					if word.XMin < 0 || word.YMin < 0 || word.XMax > page.Width+0.5 || word.YMax > page.Height+0.5 {
						t.Fatalf("page %d clipped word %q", i+1, word.Text)
					}
					words = append(words, word.Text)
				}
				text := strings.Join(words, " ")
				for _, want := range []string{orders[i].TrackingNumber, orders[i].ID.String(), "TIKI", "Reguler", "hub.abakacademy.id", "081200000000", "Akademi Bimbel Test", "081299999999", "Jl. Kantor No. 2", "54321", "Jumlah:", "Berat:", "Jenis Barang", "Ongkos Kirim:", "Packing slip", "bukan label resmi kurir."} {
					if !strings.Contains(text, want) {
						t.Fatalf("page %d missing %q: %s", i+1, want, text)
					}
				}
				{
					dest, err := parseShipmentDestination(orders[i].ShippingAddress)
					if err != nil {
						t.Fatal(err)
					}
					if !strings.Contains(text, strings.Join(strings.Fields(dest.Alamat), " ")) {
						t.Fatalf("page %d lost address text", i+1)
					}
				}
				if i == 1 && !strings.Contains(text, "7 barang lainnya") {
					t.Fatal("capped slip lost omission text")
				}
				f, err := os.Open(fmt.Sprintf("%s-%d.png", prefix, i+1))
				if err != nil {
					t.Fatal(err)
				}
				img, err := png.Decode(f)
				_ = f.Close()
				if err != nil {
					t.Fatal(err)
				}
				width := 100.0
				if size == "roll80" {
					width = 80
				}
				for _, anchor := range []string{orders[i].TrackingNumber} {
					bottom := -1.0
					for _, word := range page.Words {
						if word.Text == anchor {
							bottom = word.YMax
						}
					}
					if bottom < 0 {
						t.Fatalf("page %d missing barcode anchor %q", i+1, anchor)
					}
					next := page.Height
					for _, word := range page.Words {
						if word.YMin > bottom && word.YMin < next {
							next = word.YMin
						}
					}
					if next-bottom < 5 {
						t.Fatalf("page %d no barcode band below %q", i+1, anchor)
					}
					topMm := (bottom + (next-bottom)*0.25) * 25.4 / 72
					bottomMm := (bottom + (next-bottom)*0.75) * 25.4 / 72
					if regionMinBrightness(img, width, heights[i], width*0.15, topMm, width*0.85, bottomMm) > 300 {
						t.Fatalf("page %d missing barcode ink below %q", i+1, anchor)
					}
				}
			}
		})
	}
}
