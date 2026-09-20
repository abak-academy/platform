package main

import (
	"crypto/sha256"
	"encoding/csv"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"reflect"
	"regexp"
	"sort"
	"strings"
	"unicode"

	"akademi-bimbel/internal/model"
)

type sourceRow struct {
	NPSN, Name, Type, City, Address string
	Rows                            []int
	Conflict                        bool
	fingerprint                     string
}

type source struct {
	SHA256  string
	Rows    int
	Schools []sourceRow
}

type change struct {
	NPSN   string        `json:"npsn"`
	Rows   []int         `json:"source_rows"`
	City   string        `json:"source_city"`
	Action string        `json:"action"`
	Reason string        `json:"reason,omitempty"`
	Before *model.School `json:"before,omitempty"`
	After  *model.School `json:"after,omitempty"`
}

func readSource(input io.Reader) (source, error) {
	h := sha256.New()
	r := csv.NewReader(io.TeeReader(input, h))
	headers, err := r.Read()
	if err != nil {
		return source{}, fmt.Errorf("CSV header: %w", err)
	}
	columns := map[string]int{}
	for i, name := range headers {
		name = strings.TrimSpace(strings.TrimPrefix(name, "\ufeff"))
		if _, ok := columns[name]; ok {
			return source{}, fmt.Errorf("duplicate CSV header %q", name)
		}
		columns[name] = i
	}
	for _, name := range []string{"NPSN", "Nama", "Bentuk", "Kabupaten", "Alamat"} {
		if _, ok := columns[name]; !ok {
			return source{}, fmt.Errorf("missing CSV header %q", name)
		}
	}
	s := source{}
	byNPSN := map[string]int{}
	for line := 2; ; line++ {
		cells, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return source{}, fmt.Errorf("CSV record %d: %w", line, err)
		}
		s.Rows++
		cell := func(name string) string { return strings.TrimSpace(cells[columns[name]]) }
		raw, _ := json.Marshal(cells)
		fingerprint := sha256.Sum256(raw)
		row := sourceRow{NPSN: strings.ToUpper(cell("NPSN")), Name: cell("Nama"), Type: strings.ToUpper(cell("Bentuk")), City: cell("Kabupaten"), Address: cell("Alamat"), Rows: []int{line}, fingerprint: hex.EncodeToString(fingerprint[:])}
		if i, ok := byNPSN[row.NPSN]; ok {
			previous := &s.Schools[i]
			previous.Rows = append(previous.Rows, line)
			previous.Conflict = previous.Conflict || previous.fingerprint != row.fingerprint
		} else {
			byNPSN[row.NPSN] = len(s.Schools)
			s.Schools = append(s.Schools, row)
		}
	}
	if s.Rows == 0 {
		return source{}, fmt.Errorf("source CSV contains no schools")
	}
	s.SHA256 = hex.EncodeToString(h.Sum(nil))
	sort.Slice(s.Schools, func(i, j int) bool { return s.Schools[i].NPSN < s.Schools[j].NPSN })
	return s, nil
}

func cityKey(value string) string {
	value = strings.Join(strings.Fields(strings.ToUpper(value)), " ")
	for _, prefix := range []string{"KAB. ", "KAB "} {
		if strings.HasPrefix(value, prefix) {
			return "KABUPATEN " + strings.TrimPrefix(value, prefix)
		}
	}
	return value
}

func readAliases(input io.Reader) (map[string]string, error) {
	rows, err := csv.NewReader(input).ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 || !reflect.DeepEqual(rows[0], []string{"kabupaten", "kota_id"}) {
		return nil, fmt.Errorf("city map requires kabupaten,kota_id header")
	}
	aliases := map[string]string{}
	for _, row := range rows[1:] {
		key, id := cityKey(row[0]), strings.TrimSpace(row[1])
		if key == "" || id == "" {
			return nil, fmt.Errorf("empty city mapping")
		}
		if _, ok := aliases[key]; ok {
			return nil, fmt.Errorf("duplicate city mapping %q", key)
		}
		aliases[key] = id
	}
	return aliases, nil
}

var npsnPattern = regexp.MustCompile(`^[A-Z0-9]{8}$`)

const schoolTypes = "SD|MI|SMP|MTS|SMA|MA|SMK|ADI WIDYALAYA|KB|KURSUS|MADYAMA WIDYALAYA|MAK|MULA DHAMMASEKHA|NAVA DHAMMASEKHA|PAUDQ|PDF ULA|PDF ULYA|PDF WUSTHA|PKBM|PONDOK PESANTREN|PRATAMA WIDYALAYA|RA|SDLB|SDTK|SKB|SLB|SMAG.K|SMAK|SMPTK|SMTK|SPK KB|SPK SD|SPK SMA|SPK SMP|SPK TK|SPM ULA|SPM ULYA|SPM WUSTHA|SPS|TAMAN SEMINARI|TK|TPA|UTAMA WIDYALAYA|UTAMA WIDYALAYA KEJURUAN|UTTAMA DHAMMASEKHA|LKP|D1|D2|D3|S1|S2"

func makePlan(s source, cities []model.City, schools []model.School, aliases map[string]string) []change {
	citiesByName := map[string][]model.City{}
	citiesByID := map[string]model.City{}
	for _, c := range cities {
		citiesByName[cityKey(c.Name)] = append(citiesByName[cityKey(c.Name)], c)
		citiesByID[c.ID] = c
	}
	owners := map[string][]model.School{}
	codes := map[string]bool{}
	for _, school := range schools {
		if school.NPSN != nil {
			key := strings.ToUpper(strings.TrimSpace(*school.NPSN))
			owners[key] = append(owners[key], school)
		}
		codes[strings.ToUpper(strings.TrimSpace(school.Code))] = true
	}
	plan := make([]change, 0, len(s.Schools))
	for _, row := range s.Schools {
		c := change{NPSN: row.NPSN, Rows: row.Rows, City: row.City, Action: "conflict"}
		candidates := citiesByName[cityKey(row.City)]
		if id, ok := aliases[cityKey(row.City)]; ok {
			candidates = nil
			if city, found := citiesByID[id]; found {
				candidates = []model.City{city}
			}
		}
		nameValid := strings.ContainsFunc(row.Name, func(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) })
		switch {
		case row.Conflict:
			c.Reason = "conflicting source rows for NPSN"
		case !npsnPattern.MatchString(row.NPSN):
			c.Reason = "invalid NPSN"
		case !nameValid:
			c.Reason = "invalid school name"
		case !strings.Contains("|"+schoolTypes+"|", "|"+row.Type+"|") || row.Type == "" || strings.Contains(row.Type, "|"):
			c.Reason = "unknown school type"
		case len(candidates) != 1:
			c.Reason = "unknown or ambiguous city; provide reviewed city map"
		case len(owners[row.NPSN]) > 1:
			c.Reason = "multiple NPSN owners"
		default:
			city := candidates[0]
			npsn := row.NPSN
			after := model.School{Name: row.Name, Code: "NPSN-" + npsn, NPSN: &npsn, SchoolTypes: []string{row.Type}, Status: "active", ProvinsiID: &city.ProvinceID, KotaID: &city.ID}
			if row.Address != "" {
				address := row.Address
				after.Alamat = &address
			}
			c.Action = "insert"
			if len(owners[npsn]) == 1 {
				before := owners[npsn][0]
				c.Before = &before
				after = before
				hasType := len(before.SchoolTypes) == 0
				for _, value := range before.SchoolTypes {
					hasType = hasType || strings.EqualFold(strings.TrimSpace(value), row.Type)
				}
				if !hasType {
					c.Action = "conflict"
					c.Reason = "existing school type conflicts with source"
				} else if !compatible(before.KotaID, city.ID) || !compatible(before.ProvinsiID, city.ProvinceID) {
					c.Action = "conflict"
					c.Reason = "existing school location conflicts with source"
				} else {
					if len(after.SchoolTypes) == 0 {
						after.SchoolTypes = []string{row.Type}
					}
					after.ProvinsiID = &city.ProvinceID
					after.KotaID = &city.ID
					if blank(after.Alamat) && row.Address != "" {
						address := row.Address
						after.Alamat = &address
					}
					c.Action = "update"
					if reflect.DeepEqual(before, after) {
						c.Action = "unchanged"
					}
				}
			} else if codes[after.Code] {
				c.Action = "conflict"
				c.Reason = "generated school code already owned"
			}
			if c.Action != "conflict" {
				c.After = &after
			}
		}
		plan = append(plan, c)
	}
	return plan
}

func blank(value *string) bool                   { return value == nil || strings.TrimSpace(*value) == "" }
func compatible(value *string, want string) bool { return blank(value) || *value == want }
