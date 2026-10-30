package repository

import (
	"fmt"
	"strings"
	"time"
)

// AffiliateListQuery is the filter every affiliator-facing list shares. The zero
// value filters nothing. Type's meaning depends on the list (source, entry type,
// category or sanction), and From/To are a half-open range on the list's own date.
type AffiliateListQuery struct {
	Q      string
	Type   string
	Status string
	From   *time.Time
	To     *time.Time
}

// escapeLike makes user text match literally inside an ILIKE pattern.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// affPredicate builds the " AND ..." tail of a list query with numbered args.
type affPredicate struct {
	sql  string
	args []interface{}
	next int
}

func newAffPredicate(startArg int) *affPredicate { return &affPredicate{next: startArg} }

func (p *affPredicate) eq(column, value string) {
	if value == "" {
		return
	}
	p.sql += fmt.Sprintf(" AND %s = $%d", column, p.next)
	p.args = append(p.args, value)
	p.next++
}

func (p *affPredicate) dates(column string, from, to *time.Time) {
	if from != nil {
		p.sql += fmt.Sprintf(" AND %s >= $%d", column, p.next)
		p.args = append(p.args, *from)
		p.next++
	}
	if to != nil {
		p.sql += fmt.Sprintf(" AND %s < $%d", column, p.next)
		p.args = append(p.args, *to)
		p.next++
	}
}

// search matches the text against any of the columns, case-insensitively.
func (p *affPredicate) search(q string, columns ...string) {
	q = strings.TrimSpace(q)
	if q == "" || len(columns) == 0 {
		return
	}
	parts := make([]string, len(columns))
	for i, column := range columns {
		parts[i] = fmt.Sprintf(`%s ILIKE $%d ESCAPE '\'`, column, p.next)
	}
	p.sql += " AND (" + strings.Join(parts, " OR ") + ")"
	p.args = append(p.args, "%"+escapeLike(q)+"%")
	p.next++
}
