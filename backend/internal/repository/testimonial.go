package repository

import (
	"context"
	"encoding/json"

	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

const testimonialSelect = `SELECT t.id, t.name, t.role_label, t.photo_key, t.content, t.has_consent, t.consent_note,
	t.consent_by, u.name, t.consent_at, t.is_visible, t.sort_order, t.created_at, t.updated_at
	FROM testimonial t LEFT JOIN users u ON u.id = t.consent_by`

func scanTestimonial(row interface{ Scan(dest ...any) error }, t *model.Testimonial) error {
	var content []byte
	err := row.Scan(&t.ID, &t.Name, &t.RoleLabel, &t.PhotoKey, &content, &t.HasConsent, &t.ConsentNote,
		&t.ConsentBy, &t.ConsentByName, &t.ConsentAt, &t.IsVisible, &t.SortOrder, &t.CreatedAt, &t.UpdatedAt)
	if err != nil {
		return err
	}
	t.Content = map[string]model.TestimonialContent{}
	return json.Unmarshal(content, &t.Content)
}

func (r *Repository) listTestimonials(ctx context.Context, where string) ([]model.Testimonial, error) {
	rows, err := r.pool.Query(ctx, testimonialSelect+where+` ORDER BY t.sort_order, t.created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.Testimonial{}
	for rows.Next() {
		t := model.Testimonial{}
		if err := scanTestimonial(rows, &t); err != nil {
			return nil, err
		}
		items = append(items, t)
	}
	return items, rows.Err()
}

func (r *Repository) ListTestimonials(ctx context.Context) ([]model.Testimonial, error) {
	return r.listTestimonials(ctx, ``)
}

func (r *Repository) ListVisibleTestimonials(ctx context.Context) ([]model.Testimonial, error) {
	return r.listTestimonials(ctx, ` WHERE t.is_visible`)
}

func (r *Repository) GetTestimonial(ctx context.Context, id string) (*model.Testimonial, error) {
	return r.getTestimonial(ctx, r.pool, ``, id)
}

// GetTestimonialForUpdateTx locks the row so concurrent PATCHes and deletes serialize behind it.
func (r *Repository) GetTestimonialForUpdateTx(ctx context.Context, tx pgx.Tx, id string) (*model.Testimonial, error) {
	return r.getTestimonial(ctx, tx, ` FOR UPDATE OF t`, id)
}

func (r *Repository) getTestimonial(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, lock, id string) (*model.Testimonial, error) {
	t := &model.Testimonial{}
	if err := scanTestimonial(q.QueryRow(ctx, testimonialSelect+` WHERE t.id = $1`+lock, id), t); err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return t, nil
}

func testimonialWriteArgs(t *model.Testimonial) ([]any, error) {
	content := t.Content
	if content == nil {
		content = map[string]model.TestimonialContent{}
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	return []any{t.Name, t.RoleLabel, t.PhotoKey, raw, t.HasConsent, t.ConsentNote, t.ConsentBy, t.ConsentAt,
		t.IsVisible, t.SortOrder}, nil
}

// CreateTestimonial inserts t and reloads it into *t (id, timestamps, consent_by_name).
func (r *Repository) CreateTestimonial(ctx context.Context, t *model.Testimonial) error {
	args, err := testimonialWriteArgs(t)
	if err != nil {
		return err
	}
	var id string
	if err := r.pool.QueryRow(ctx,
		`INSERT INTO testimonial (name, role_label, photo_key, content, has_consent, consent_note, consent_by, consent_at, is_visible, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id`, args...).Scan(&id); err != nil {
		return err
	}
	got, err := r.GetTestimonial(ctx, id)
	if err != nil {
		return err
	}
	*t = *got
	return nil
}

// UpdateTestimonialTx writes every editable column, so callers merge a patch into a row loaded with GetTestimonialForUpdateTx.
func (r *Repository) UpdateTestimonialTx(ctx context.Context, tx pgx.Tx, id string, t *model.Testimonial) error {
	args, err := testimonialWriteArgs(t)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx,
		`UPDATE testimonial SET name = $1, role_label = $2, photo_key = $3, content = $4, has_consent = $5, consent_note = $6,
		consent_by = $7, consent_at = $8, is_visible = $9, sort_order = $10, updated_at = now() WHERE id = $11`,
		append(args, id)...)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

func (r *Repository) DeleteTestimonialTx(ctx context.Context, tx pgx.Tx, id string) error {
	tag, err := tx.Exec(ctx, `DELETE FROM testimonial WHERE id = $1`, id)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}
