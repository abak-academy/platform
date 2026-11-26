package repository

import (
	"context"
	"time"

	"akademi-bimbel/internal/model"
)

// EditionExam is one exam linked to a member product through product_exam.
type EditionExam struct {
	ProductID      string
	ExamID         string
	ScheduledAt    *time.Time
	ScheduledEndAt *time.Time
}

// ListVisibleProductGroupings returns the events shown on the public landing, in display order.
func (r *Repository) ListVisibleProductGroupings(ctx context.Context) ([]model.ProductGrouping, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+productGroupingColumns+` FROM product_grouping
		WHERE is_visible AND status = 'published' ORDER BY display_order, created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groupings := []model.ProductGrouping{}
	for rows.Next() {
		g := model.ProductGrouping{}
		if err := scanProductGrouping(rows, &g); err != nil {
			return nil, err
		}
		groupings = append(groupings, g)
	}
	return groupings, rows.Err()
}

// ListGroupingEditionExams returns the exams behind a grouping's exam-type member products.
func (r *Repository) ListGroupingEditionExams(ctx context.Context, groupingID string) ([]EditionExam, error) {
	rows, err := r.pool.Query(ctx, `SELECT pe.product_id, pe.exam_id, e.scheduled_at, e.scheduled_end_at
		FROM product p
		JOIN product_exam pe ON pe.product_id = p.id
		JOIN exam e ON e.id = pe.exam_id
		WHERE p.grouping_id = $1 AND p.type = 'exam'
		ORDER BY pe.product_id, e.scheduled_at NULLS LAST, pe.exam_id`, groupingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	exams := []EditionExam{}
	for rows.Next() {
		var x EditionExam
		if err := rows.Scan(&x.ProductID, &x.ExamID, &x.ScheduledAt, &x.ScheduledEndAt); err != nil {
			return nil, err
		}
		exams = append(exams, x)
	}
	return exams, rows.Err()
}

// ListMultiEventProductIDs returns the grouping's member products that share an
// exam with a product in a different grouping.
func (r *Repository) ListMultiEventProductIDs(ctx context.Context, groupingID string) (map[string]bool, error) {
	rows, err := r.pool.Query(ctx, `SELECT DISTINCT p.id
		FROM product p
		JOIN product_exam pe ON pe.product_id = p.id
		JOIN product_exam pe2 ON pe2.exam_id = pe.exam_id
		JOIN product p2 ON p2.id = pe2.product_id
		WHERE p.grouping_id = $1 AND p2.grouping_id IS NOT NULL AND p2.grouping_id <> $1`, groupingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = true
	}
	return ids, rows.Err()
}
