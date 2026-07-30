package service

// csvSafeField neutralises a spreadsheet formula lead so an exported field is
// read as literal text. Student names are attacker-supplied at registration, so
// a name like `=HYPERLINK(...)` would otherwise execute when an admin opens the
// export in Excel or Sheets. Mirrors the frontend roster export (C-08).
//
// Quoting is deliberately left to encoding/csv, which already applies RFC 4180
// to embedded quotes, commas and newlines — unlike the hand-rolled frontend
// writer, which has to force-quote itself.
func csvSafeField(v string) string {
	if v == "" {
		return v
	}
	// Indexing the first byte is safe for these six sentinels: all are ASCII,
	// and every byte of a multi-byte UTF-8 rune is >= 0x80.
	switch v[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + v
	}
	return v
}

// csvSafeRow applies csvSafeField to every field. Use it where every column is
// user-supplied text; pass fields individually where a column is machine-
// generated and must stay typed (a negative score would become text).
func csvSafeRow(fields ...string) []string {
	out := make([]string, len(fields))
	for i, f := range fields {
		out[i] = csvSafeField(f)
	}
	return out
}
