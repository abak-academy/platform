package repository

import "testing"

func TestTestFilterSQL_Used(t *testing.T) {
	yes, no := true, false
	const sub = ` (SELECT 1 FROM exam_test et WHERE et.test_id = t.id)`
	tests := []struct {
		name   string
		filter TestFilter
		want   string
		nArgs  int
		next   int
	}{
		{"nil adds nothing", TestFilter{}, "", 0, 1},
		{"true", TestFilter{Used: &yes}, ` AND EXISTS` + sub, 0, 1},
		{"false", TestFilter{Used: &no}, ` AND NOT EXISTS` + sub, 0, 1},
		{"keeps arg indexes", TestFilter{Subject: "math", Used: &no}, ` AND t.subject = $1 AND NOT EXISTS` + sub, 1, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, args, next := testFilterSQL(tt.filter, 1)
			if got != tt.want || len(args) != tt.nArgs || next != tt.next {
				t.Fatalf("got %q args=%d next=%d", got, len(args), next)
			}
		})
	}
}
