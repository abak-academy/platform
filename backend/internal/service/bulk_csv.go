package service

import (
	"bytes"
	"encoding/csv"
	"strings"
)

var utf8BOM = []byte{0xEF, 0xBB, 0xBF}

func newBulkCSVReader(data []byte) *csv.Reader {
	data = bytes.TrimPrefix(data, utf8BOM)
	r := csv.NewReader(bytes.NewReader(data))
	r.FieldsPerRecord = -1
	return r
}

func normalizeCSVHeader(h string) string {
	h = strings.TrimPrefix(h, "\ufeff")
	return strings.ToLower(strings.TrimSpace(h))
}

func bulkCell(record []string, idx int) string {
	if idx < 0 || idx >= len(record) {
		return ""
	}
	return strings.TrimSpace(record[idx])
}

func bulkOptionalCell(record []string, idx int) *string {
	v := bulkCell(record, idx)
	if v == "" {
		return nil
	}
	return &v
}

func bulkRowIsBlank(record []string) bool {
	for _, c := range record {
		if strings.TrimSpace(c) != "" {
			return false
		}
	}
	return true
}
