package service

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"html/template"
	"sort"
	"strconv"
)

// certificateHTMLTemplate emits a single self-contained HTML document (FR-2,
// FR-3, FR-4): geometry/font/color values are pre-sanitized in Go and passed
// as template.CSS/template.JS (trusted, not re-escaped), while field .Text
// values stay plain strings so html/template HTML-escapes them normally.
var certificateHTMLTemplate = template.Must(template.New("certificate").Parse(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<style>{{.StyleBlock}}</style>
</head>
<body>
<img class="certificate-bg" src="data:{{.BackgroundMime}};base64,{{.BackgroundBase64}}" alt="">
{{range .Fields}}{{if .IsImage}}<img class="field" style="{{.Style}}" src="data:{{.ImageMime}};base64,{{.ImageBase64}}" alt="">
{{else}}<div class="field" style="{{.Style}}">{{.Text}}</div>
{{end}}{{end}}<script>{{.Script}}</script>
</body>
</html>`))

// certificateFitScript shrinks any field whose rendered text overflows its
// box (FR-8), mirroring renderCertificateWithImages's shrinkToFitSafetyMargin/
// minShrinkToFitSizePt so PDF and HTML degrade the same way for a long value.
// It sets window.__certificateReady so a Gotenberg caller can waitForExpression
// on it instead of a fixed waitDelay.
const certificateFitScript = template.JS(`(function(){
var minPt = 6;
var margin = 0.97;
document.querySelectorAll('.field').forEach(function(el){
  var size = parseFloat(el.style.fontSize) || 0;
  if (!size) return;
  while (el.scrollWidth > el.clientWidth && size > minPt) {
    size = Math.max(minPt, size * margin);
    el.style.fontSize = size + 'pt';
  }
});
window.__certificateReady = true;
})();`)

type certificateFontFace struct {
	Family string
	Weight string
	Base64 string
}

type certificateFieldView struct {
	IsImage     bool
	Style       template.CSS
	Text        string
	ImageBase64 string
	ImageMime   string
}

type certificateHTMLData struct {
	StyleBlock       template.CSS
	BackgroundBase64 string
	BackgroundMime   string
	Fields           []certificateFieldView
	Script           template.JS
}

// fallbackImageMime is what an undecodable image is embedded as. Every built-in
// background is a PNG, so this only ever applies to a corrupt upload — which
// would not render under any mime type.
const fallbackImageMime = "image/png"

// imageMimeOrFallback resolves the real mime of embedded image bytes. Uploads
// are accepted as any image type (the picker is not restricted to PNG), so
// hardcoding image/png here would mislabel every JPEG background.
func imageMimeOrFallback(data []byte) string {
	if mime, ok := decodeImageMime(data); ok {
		return mime
	}
	return fallbackImageMime
}

// buildCertificateHTML renders layout+vals+bg+images into self-contained
// certificate HTML (FR-2..FR-5, FR-9): no DB/network access, no PDF library.
func buildCertificateHTML(layout Layout, vals map[FieldID]string, bg []byte, images map[FieldID][]byte) ([]byte, error) {
	faces, err := certificateFontFaces()
	if err != nil {
		return nil, fmt.Errorf("build certificate font faces: %w", err)
	}

	var fields []certificateFieldView
	for _, f := range layout.Fields {
		if !f.Visible {
			continue
		}
		if imageFieldIDs[f.ID] {
			img := images[f.ID]
			if len(img) == 0 {
				continue
			}
			fields = append(fields, certificateFieldView{
				IsImage:     true,
				Style:       imageFieldStyle(f),
				ImageBase64: base64.StdEncoding.EncodeToString(img),
				ImageMime:   imageMimeOrFallback(img),
			})
			continue
		}
		text := vals[f.ID]
		if text == "" {
			continue
		}
		fields = append(fields, certificateFieldView{
			Style: textFieldStyle(f),
			Text:  text,
		})
	}

	data := certificateHTMLData{
		StyleBlock:       buildCertificateStyleBlock(layout, faces),
		BackgroundBase64: base64.StdEncoding.EncodeToString(bg),
		BackgroundMime:   imageMimeOrFallback(bg),
		Fields:           fields,
		Script:           certificateFitScript,
	}

	var buf bytes.Buffer
	if err := certificateHTMLTemplate.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute certificate html template: %w", err)
	}
	return buf.Bytes(), nil
}

// certificateFontFaces reads every bundled OFL family from fontFS (FR-3),
// reusing pdffonts.go's fontFiles map. It emits one normal-weight face per
// family, plus a bold face only when a genuinely distinct bold TTF is
// bundled (source_serif_4, public_sans) — the other four reuse their single
// file for both style keys, so a second face would be redundant.
func certificateFontFaces() ([]certificateFontFace, error) {
	families := make([]string, 0, len(fontFiles))
	for family := range fontFiles {
		families = append(families, family)
	}
	sort.Strings(families)

	var faces []certificateFontFace
	for _, family := range families {
		styles := fontFiles[family]
		regularPath := styles[""]
		boldPath := styles["B"]

		regularBytes, err := fontFS.ReadFile(regularPath)
		if err != nil {
			return nil, fmt.Errorf("read font %s: %w", regularPath, err)
		}
		faces = append(faces, certificateFontFace{
			Family: family,
			Weight: "normal",
			Base64: base64.StdEncoding.EncodeToString(regularBytes),
		})

		if boldPath != regularPath {
			boldBytes, err := fontFS.ReadFile(boldPath)
			if err != nil {
				return nil, fmt.Errorf("read font %s: %w", boldPath, err)
			}
			faces = append(faces, certificateFontFace{
				Family: family,
				Weight: "bold",
				Base64: base64.StdEncoding.EncodeToString(boldBytes),
			})
		}
	}
	return faces, nil
}

// buildCertificateStyleBlock emits the @page rule (FR-2) and every @font-face
// (FR-3) as a single trusted CSS blob.
func buildCertificateStyleBlock(layout Layout, faces []certificateFontFace) template.CSS {
	var b bytes.Buffer
	fmt.Fprintf(&b, "@page{size:%smm %smm;margin:0;}", formatMm(layout.Page.WidthMm), formatMm(layout.Page.HeightMm))
	b.WriteString("*{margin:0;padding:0;box-sizing:border-box;}")
	fmt.Fprintf(&b, "html,body{width:%smm;height:%smm;}", formatMm(layout.Page.WidthMm), formatMm(layout.Page.HeightMm))
	b.WriteString(".certificate-bg{position:absolute;left:0;top:0;width:100%;height:100%;}")
	b.WriteString(".field{position:absolute;white-space:nowrap;}")
	for _, face := range faces {
		fmt.Fprintf(&b, "@font-face{font-family:%s;font-weight:%s;src:url(data:font/ttf;base64,%s) format(\"truetype\");}", face.Family, face.Weight, face.Base64)
	}
	return template.CSS(b.String())
}

// textFieldStyle positions a text field per FR-2: mm 1:1, top-left origin, no
// Y-flip. Font falls back to source_serif_4 and color to black on bad input
// (FR-9) so a stale/corrupt design still renders.
func textFieldStyle(f LayoutField) template.CSS {
	weight := "normal"
	if f.Weight == "bold" {
		weight = "bold"
	}
	return template.CSS(fmt.Sprintf(
		"position:absolute;left:%smm;top:%smm;width:%smm;text-align:%s;color:%s;font-size:%spt;font-family:%s;font-weight:%s;",
		formatMm(f.XMm), formatMm(f.YMm), formatMm(f.WMm), cssAlign(f.Align), safeCSSColor(f.Color), formatMm(f.SizePt), ResolveFontFamily(f.Font), weight,
	))
}

// imageFieldStyle positions an image field (logo/signature) per FR-2/FR-4,
// scaling to fit its box without cropping or stretching (contain).
func imageFieldStyle(f LayoutField) template.CSS {
	return template.CSS(fmt.Sprintf(
		"position:absolute;left:%smm;top:%smm;width:%smm;height:%smm;object-fit:contain;",
		formatMm(f.XMm), formatMm(f.YMm), formatMm(f.WMm), formatMm(f.HMm),
	))
}

// cssAlign maps the layout schema's align values to CSS text-align; an
// unrecognised value centers, matching the field default.
func cssAlign(align string) string {
	switch align {
	case "left", "right":
		return align
	default:
		return "center"
	}
}

// safeCSSColor validates a "#RRGGBB" color, falling back to black on anything
// malformed (FR-9) — mirrors hexToRGB's fallback but keeps the CSS string form.
func safeCSSColor(color string) string {
	if len(color) == 7 && color[0] == '#' {
		for _, c := range color[1:] {
			if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')) {
				return "#000000"
			}
		}
		return color
	}
	return "#000000"
}

// formatMm renders a float64 without exponential notation or trailing zeros,
// so generated CSS numbers stay predictable and diffable.
func formatMm(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}
