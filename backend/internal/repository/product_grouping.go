package repository

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

const productGroupingColumns = `id, name, description, status, display_order, created_at, updated_at`

func scanProductGrouping(row interface{ Scan(dest ...any) error }, g *model.ProductGrouping) error {
	var description *string
	err := row.Scan(&g.ID, &g.Name, &description, &g.Status, &g.DisplayOrder, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return err
	}
	if description != nil {
		g.Description = *description
	}
	return nil
}

func (r *Repository) CreateProductGrouping(ctx context.Context, g *model.ProductGrouping) error {
	err := scanProductGrouping(r.pool.QueryRow(ctx,
		`INSERT INTO product_grouping (name, description, status)
		VALUES ($1, $2, $3)
		RETURNING `+productGroupingColumns,
		g.Name, g.Description, g.Status,
	), g)
	return err
}

func (r *Repository) GetProductGroupingByID(ctx context.Context, id string) (*model.ProductGrouping, error) {
	g := &model.ProductGrouping{}
	err := scanProductGrouping(r.pool.QueryRow(ctx,
		`SELECT `+productGroupingColumns+` FROM product_grouping WHERE id = $1`,
		id,
	), g)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return g, nil
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

func (r *Repository) UpdateProductGrouping(ctx context.Context, id string, g *model.ProductGrouping) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE product_grouping SET name = $1, description = $2, status = $3, updated_at = now() WHERE id = $4`,
		g.Name, g.Description, g.Status, id,
	)
	return err
}

func (r *Repository) DeleteProductGrouping(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM product_grouping WHERE id = $1`, id)
	return err
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

// ListProductsByGrouping returns a grouping's member products ordered for
// display. publishedOnly applies the same student-visible rules as the
// public catalog (status, availability window, linked-exam orderability).
func (r *Repository) ListProductsByGrouping(ctx context.Context, groupingID string, publishedOnly bool) ([]model.Product, error) {
	query := `SELECT p.id, p.type, p.name, p.description, p.price, COALESCE(p.stock, 0), (p.stock IS NOT NULL), p.status, p.weight_grams, p.image_url, p.specs, p.available_from, p.available_until, p.grouping_id, p.grouping_order, p.created_at, p.updated_at
	FROM product p WHERE p.grouping_id = $1`
	if publishedOnly {
		query += ` AND p.status = 'published'`
		query += productAvailabilityFilter
		query += linkedOrderableExamProductFilter
	}
	query += ` ORDER BY p.grouping_order, p.created_at`

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
