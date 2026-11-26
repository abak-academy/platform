package repository

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"akademi-bimbel/internal/model"
)

const productGroupingColumns = `id, name, description, status, display_order, slug, short_name, scope, subjects, tone, illustration, logo_key, official_url, content, is_visible, participant_prefix, created_at, updated_at`

// ErrGroupingSlugTaken and ErrGroupingPrefixTaken are the unique violations of
// product_grouping_slug_key and product_grouping_participant_prefix_key.
var (
	ErrGroupingSlugTaken   = errors.New("grouping slug taken")
	ErrGroupingPrefixTaken = errors.New("grouping participant prefix taken")
)

func mapGroupingUniqueViolation(err error) error {
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != pgerrcodeUniqueViolation {
		return err
	}
	switch pgErr.ConstraintName {
	case "product_grouping_slug_key":
		return ErrGroupingSlugTaken
	case "product_grouping_participant_prefix_key":
		return ErrGroupingPrefixTaken
	}
	return err
}

func scanProductGrouping(row interface{ Scan(dest ...any) error }, g *model.ProductGrouping) error {
	var description *string
	var content []byte
	err := row.Scan(&g.ID, &g.Name, &description, &g.Status, &g.DisplayOrder, &g.Slug, &g.ShortName, &g.Scope,
		&g.Subjects, &g.Tone, &g.Illustration, &g.LogoKey, &g.OfficialURL, &content, &g.IsVisible, &g.ParticipantPrefix,
		&g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return err
	}
	if description != nil {
		g.Description = *description
	}
	if g.Subjects == nil {
		g.Subjects = []string{}
	}
	g.Content = map[string]model.EventNarrative{}
	return json.Unmarshal(content, &g.Content)
}

// groupingWriteArgs are the editable columns shared by create and update.
func groupingWriteArgs(g *model.ProductGrouping) ([]any, error) {
	content := g.Content
	if content == nil {
		content = map[string]model.EventNarrative{}
	}
	raw, err := json.Marshal(content)
	if err != nil {
		return nil, err
	}
	subjects := g.Subjects
	if subjects == nil {
		subjects = []string{}
	}
	return []any{g.Name, g.Description, g.Status, g.Slug, g.ShortName, g.Scope, subjects, g.Tone, g.Illustration,
		g.LogoKey, g.OfficialURL, raw, g.IsVisible, g.ParticipantPrefix}, nil
}

func (r *Repository) CreateProductGrouping(ctx context.Context, g *model.ProductGrouping) error {
	args, err := groupingWriteArgs(g)
	if err != nil {
		return err
	}
	err = scanProductGrouping(r.pool.QueryRow(ctx,
		`INSERT INTO product_grouping (name, description, status, slug, short_name, scope, subjects, tone, illustration, logo_key, official_url, content, is_visible, participant_prefix)
		VALUES ($1, $2, $3, $4, $5, $6, $7, COALESCE(NULLIF($8, ''), 'brand'), COALESCE(NULLIF($9, ''), 'trophy'), $10, $11, $12, $13, $14)
		RETURNING `+productGroupingColumns,
		args...,
	), g)
	return mapGroupingUniqueViolation(err)
}

func (r *Repository) getProductGrouping(ctx context.Context, q queryer, where string, arg string) (*model.ProductGrouping, error) {
	g := &model.ProductGrouping{}
	err := scanProductGrouping(q.QueryRow(ctx,
		`SELECT `+productGroupingColumns+` FROM product_grouping WHERE `+where, arg,
	), g)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return g, nil
}

func (r *Repository) GetProductGroupingByID(ctx context.Context, id string) (*model.ProductGrouping, error) {
	return r.getProductGrouping(ctx, r.pool, `id = $1`, id)
}

// GetProductGroupingForUpdateTx locks the grouping row so concurrent edits, member replaces and deletes serialize behind it.
func (r *Repository) GetProductGroupingForUpdateTx(ctx context.Context, tx pgx.Tx, id string) (*model.ProductGrouping, error) {
	return r.getProductGrouping(ctx, tx, `id = $1 FOR UPDATE`, id)
}

func (r *Repository) GetProductGroupingBySlug(ctx context.Context, slug string) (*model.ProductGrouping, error) {
	return r.getProductGrouping(ctx, r.pool, `slug = $1`, slug)
}

// ListProductGroupings returns groupings ordered for display. publishedOnly
// restricts the list to the student-facing landing page's rows.
func (r *Repository) ListProductGroupings(ctx context.Context, publishedOnly bool) ([]model.ProductGrouping, error) {
	query := `SELECT ` + productGroupingColumns + ` FROM product_grouping`
	if publishedOnly {
		query += ` WHERE status = 'published'`
	}
	query += ` ORDER BY display_order, created_at`

	rows, err := r.pool.Query(ctx, query)
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

const updateProductGroupingSQL = `UPDATE product_grouping SET name = $1, description = $2, status = $3, slug = $4, short_name = $5, scope = $6, subjects = $7,
	tone = COALESCE(NULLIF($8, ''), 'brand'), illustration = COALESCE(NULLIF($9, ''), 'trophy'),
	logo_key = $10, official_url = $11, content = $12, is_visible = $13, participant_prefix = $14, updated_at = now()
	WHERE id = $15`

// UpdateProductGrouping writes every editable column, so callers merge a patch
// into the loaded grouping first.
func (r *Repository) UpdateProductGrouping(ctx context.Context, id string, g *model.ProductGrouping) error {
	args, err := groupingWriteArgs(g)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx, updateProductGroupingSQL, append(args, id)...)
	return mapGroupingUniqueViolation(err)
}

func (r *Repository) UpdateProductGroupingTx(ctx context.Context, tx pgx.Tx, id string, g *model.ProductGrouping) error {
	args, err := groupingWriteArgs(g)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, updateProductGroupingSQL, append(args, id)...)
	return mapGroupingUniqueViolation(err)
}

// ListGroupingExamIDsTx returns the exams sold by the grouping's member products.
func (r *Repository) ListGroupingExamIDsTx(ctx context.Context, tx pgx.Tx, groupingID string) ([]uuid.UUID, error) {
	return queryUUIDs(ctx, tx,
		`SELECT DISTINCT pe.exam_id FROM product_exam pe JOIN product p ON p.id = pe.product_id WHERE p.grouping_id = $1`, groupingID)
}

// ListProductExamIDsTx returns the exams sold by the given products.
func (r *Repository) ListProductExamIDsTx(ctx context.Context, tx pgx.Tx, productIDs []uuid.UUID) ([]uuid.UUID, error) {
	return queryUUIDs(ctx, tx, `SELECT DISTINCT exam_id FROM product_exam WHERE product_id = ANY($1)`, productIDs)
}

// ListGroupingProductIDsTx returns the grouping's current member product ids.
func (r *Repository) ListGroupingProductIDsTx(ctx context.Context, tx pgx.Tx, groupingID uuid.UUID) ([]uuid.UUID, error) {
	return queryUUIDs(ctx, tx, `SELECT id FROM product WHERE grouping_id = $1`, groupingID)
}

func queryUUIDs(ctx context.Context, tx pgx.Tx, query string, args ...any) ([]uuid.UUID, error) {
	rows, err := tx.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []uuid.UUID{}
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (r *Repository) CountGroupingProducts(ctx context.Context, id string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx, `SELECT count(*) FROM product WHERE grouping_id = $1`, id).Scan(&n)
	return n, err
}

func (r *Repository) ListGroupingSchedule(ctx context.Context, groupingID string) ([]model.ProductGroupingScheduleItem, error) {
	return r.listGroupingSchedule(ctx, r.pool, groupingID)
}

func (r *Repository) ListGroupingScheduleTx(ctx context.Context, tx pgx.Tx, groupingID string) ([]model.ProductGroupingScheduleItem, error) {
	return r.listGroupingSchedule(ctx, tx, groupingID)
}

func (r *Repository) listGroupingSchedule(ctx context.Context, q queryer, groupingID string) ([]model.ProductGroupingScheduleItem, error) {
	rows, err := q.Query(ctx,
		`SELECT id, label, label_en, starts_on::text, ends_on::text FROM product_grouping_schedule
		WHERE grouping_id = $1 ORDER BY starts_on, ends_on NULLS FIRST, id`, groupingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []model.ProductGroupingScheduleItem{}
	for rows.Next() {
		var it model.ProductGroupingScheduleItem
		if err := rows.Scan(&it.ID, &it.Label, &it.LabelEN, &it.StartsOn, &it.EndsOn); err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

// ReplaceGroupingScheduleTx replaces the whole schedule inside tx; item IDs are regenerated.
func (r *Repository) ReplaceGroupingScheduleTx(ctx context.Context, tx pgx.Tx, groupingID string, items []model.ProductGroupingScheduleItem) error {
	if _, err := tx.Exec(ctx, `DELETE FROM product_grouping_schedule WHERE grouping_id = $1`, groupingID); err != nil {
		return err
	}
	for _, it := range items {
		if _, err := tx.Exec(ctx,
			`INSERT INTO product_grouping_schedule (grouping_id, label, label_en, starts_on, ends_on)
			VALUES ($1, $2, $3, $4::text::date, $5::text::date)`,
			groupingID, it.Label, it.LabelEN, it.StartsOn, it.EndsOn,
		); err != nil {
			return err
		}
	}
	return nil
}

// DeleteEmptyProductGroupingTx reports whether a row was deleted; false means the grouping has members.
func (r *Repository) DeleteEmptyProductGroupingTx(ctx context.Context, tx pgx.Tx, id string) (bool, error) {
	tag, err := tx.Exec(ctx,
		`DELETE FROM product_grouping WHERE id = $1 AND NOT EXISTS (SELECT 1 FROM product WHERE grouping_id = $1)`, id)
	return tag.RowsAffected() > 0, err
}

// ReorderProductGroupings sets display_order to each grouping's position in
// the given slice. Groupings not named keep their existing display_order.
func (r *Repository) ReorderProductGroupings(ctx context.Context, orderedIDs []uuid.UUID) error {
	for i, id := range orderedIDs {
		if _, err := r.pool.Exec(ctx, `UPDATE product_grouping SET display_order = $1, updated_at = now() WHERE id = $2`, i, id); err != nil {
			return err
		}
	}
	return nil
}

// ReplaceGroupingProducts atomically replaces a grouping's member products:
// every product currently in the grouping but absent from productIDs is
// unassigned, and every product in productIDs is assigned to it with
// grouping_order set to its position in the slice. Mirrors
// ReplaceProductCourses's replace-all-in-a-transaction idiom.
func (r *Repository) ReplaceGroupingProducts(ctx context.Context, tx pgx.Tx, groupingID uuid.UUID, productIDs []uuid.UUID) error {
	_, err := tx.Exec(ctx,
		`UPDATE product SET grouping_id = NULL, grouping_order = 0, updated_at = now()
		WHERE grouping_id = $1 AND NOT (id = ANY($2))`,
		groupingID, productIDs,
	)
	if err != nil {
		return err
	}
	for i, productID := range productIDs {
		if _, err := tx.Exec(ctx,
			`UPDATE product SET grouping_id = $1, grouping_order = $2, updated_at = now() WHERE id = $3`,
			groupingID, i, productID,
		); err != nil {
			return err
		}
	}
	return nil
}

// ListProductsByGrouping returns every one of a grouping's member products,
// regardless of status, ordered for the admin management page. Student-facing
// browsing goes through ListProducts with GroupingID set instead, which
// already applies the public catalog's visibility rules.
func (r *Repository) ListProductsByGrouping(ctx context.Context, groupingID string) ([]model.Product, error) {
	query := `SELECT p.id, p.type, p.name, p.description, p.price, COALESCE(p.stock, 0), (p.stock IS NOT NULL), p.status, p.weight_grams, p.image_url, p.specs, p.available_from, p.available_until, p.grouping_id, p.grouping_order, p.stages, p.grade_min, p.grade_max, p.created_at, p.updated_at
	FROM product p WHERE p.grouping_id = $1
	ORDER BY p.grouping_order, p.created_at`

	rows, err := r.pool.Query(ctx, query, groupingID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	products := []model.Product{}
	for rows.Next() {
		p := model.Product{}
		if err := scanProduct(rows, &p); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, rows.Err()
}
