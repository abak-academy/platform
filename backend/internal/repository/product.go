package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
)

var ErrNotFound = errors.New("not found")

// scanProduct scans a product row, handling nullable TEXT/INT columns that pgx v5 cannot
// scan directly into non-pointer Go types.
func scanProduct(row interface{ Scan(dest ...any) error }, p *model.Product) error {
	var description, imageURL *string
	var weightGrams *int
	var specs []byte
	err := row.Scan(
		&p.ID, &p.Type, &p.Name, &description, &p.Price, &p.Stock, &p.Status,
		&weightGrams, &imageURL, &specs, &p.AvailableFrom, &p.AvailableUntil, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return err
	}
	if description != nil {
		p.Description = *description
	}
	if imageURL != nil {
		p.ImageURL = *imageURL
	}
	if weightGrams != nil {
		p.WeightGrams = *weightGrams
	}
	p.Specs = []model.ProductSpec{}
	if len(specs) > 0 {
		if err := json.Unmarshal(specs, &p.Specs); err != nil {
			return err
		}
	}
	return nil
}

// productAvailabilityFilter is the SQL predicate that restricts the public
// catalog to products currently inside their availability window (P-A).
const productAvailabilityFilter = ` AND (available_from IS NULL OR available_from <= now())` +
	` AND (available_until IS NULL OR available_until >= now())`

type ProductFilter struct {
	Type       string
	Status     string
	VisibleOnly bool // true = only published + not hidden
	Cursor     string
	Limit      int
}

func (r *Repository) CreateProduct(ctx context.Context, p *model.Product) error {
	specs, err := marshalSpecs(p.Specs)
	if err != nil {
		return err
	}
	err = r.pool.QueryRow(ctx,
		`INSERT INTO product (type, name, description, price, stock, status, weight_grams, image_url, specs, available_from, available_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`,
		p.Type, p.Name, p.Description, p.Price, p.Stock, p.Status, p.WeightGrams, p.ImageURL, specs, p.AvailableFrom, p.AvailableUntil,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	return err
}

// marshalSpecs renders specs as a JSON array, never SQL NULL — the column is
// NOT NULL and a nil slice would marshal to "null".
func marshalSpecs(specs []model.ProductSpec) ([]byte, error) {
	if specs == nil {
		specs = []model.ProductSpec{}
	}
	return json.Marshal(specs)
}

func (r *Repository) GetProductByID(ctx context.Context, id string) (*model.Product, error) {
	p := &model.Product{}
	err := scanProduct(r.pool.QueryRow(ctx,
		`SELECT id, type, name, description, price, stock, status, weight_grams, image_url, specs, available_from, available_until, created_at, updated_at
		FROM product
		WHERE id = $1`,
		id,
	), p)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

// GetProductByExamID returns the exam-type product linked to the given exam
// via product_exam. Returns ErrNotFound when no product is linked or the
// linked product is not of type "exam" or is not published.
func (r *Repository) GetProductByExamID(ctx context.Context, examID uuid.UUID) (*model.Product, error) {
	p := &model.Product{}
	err := scanProduct(r.pool.QueryRow(ctx,
		`SELECT p.id, p.type, p.name, p.description, p.price, p.stock, p.status,
		        p.weight_grams, p.image_url, p.specs, p.available_from, p.available_until, p.created_at, p.updated_at
		 FROM product p
		 JOIN product_exam pe ON pe.product_id = p.id
		 WHERE pe.exam_id = $1 AND p.type = 'exam' AND p.status = 'published'
		   AND (p.available_from IS NULL OR p.available_from <= now())
		   AND (p.available_until IS NULL OR p.available_until >= now())`,
		examID,
	), p)
	if err != nil {
		if isNotFound(err) {
			return nil, ErrNotFound
		}
		return nil, err
	}
	return p, nil
}

func (r *Repository) ListProducts(ctx context.Context, filter ProductFilter) ([]model.Product, string, error) {
	if filter.Limit == 0 {
		filter.Limit = 20
	}

	products := []model.Product{}
	query := `SELECT id, type, name, description, price, stock, status, weight_grams, image_url, specs, available_from, available_until, created_at, updated_at
	FROM product WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.Type != "" {
		query += fmt.Sprintf(` AND type = $%d`, argIdx)
		args = append(args, filter.Type)
		argIdx++
	}
	if filter.Status != "" {
		query += fmt.Sprintf(` AND status = $%d`, argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.VisibleOnly {
		// public catalog: published and within the availability window
		query += ` AND status = 'published'`
		query += productAvailabilityFilter
	}
	if filter.Cursor != "" {
		if _, err := uuid.Parse(filter.Cursor); err != nil {
			return nil, "", ErrInvalidCursor
		}
		query += fmt.Sprintf(` AND id > $%d`, argIdx)
		args = append(args, filter.Cursor)
		argIdx++
	}

	query += ` ORDER BY id LIMIT $` + fmt.Sprintf("%d", argIdx)
	args = append(args, filter.Limit+1)

	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	for rows.Next() {
		p := model.Product{}
		if err := scanProduct(rows, &p); err != nil {
			return nil, "", err
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, "", err
	}

	var nextCursor string
	if len(products) > filter.Limit {
		products = products[:filter.Limit]
		// cursor is the last row actually returned — `id > $n` on the next
		// page must exclude it, not the unreturned peek row.
		nextCursor = products[filter.Limit-1].ID
	}

	return products, nextCursor, nil
}

func (r *Repository) UpdateProduct(ctx context.Context, id string, p *model.Product) error {
	specs, err := marshalSpecs(p.Specs)
	if err != nil {
		return err
	}
	_, err = r.pool.Exec(ctx,
		`UPDATE product
		SET type = $1, name = $2, description = $3, price = $4, stock = $5, status = $6, weight_grams = $7, image_url = $8, specs = $9, available_from = $10, available_until = $11, updated_at = now()
		WHERE id = $12`,
		p.Type, p.Name, p.Description, p.Price, p.Stock, p.Status, p.WeightGrams, p.ImageURL, specs, p.AvailableFrom, p.AvailableUntil, id,
	)
	return err
}

func (r *Repository) UpdateProductTx(ctx context.Context, tx pgx.Tx, id string, p *model.Product) error {
	specs, err := marshalSpecs(p.Specs)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`UPDATE product
		SET type = $1, name = $2, description = $3, price = $4, stock = $5, status = $6, weight_grams = $7, image_url = $8, specs = $9, available_from = $10, available_until = $11, updated_at = now()
		WHERE id = $12`,
		p.Type, p.Name, p.Description, p.Price, p.Stock, p.Status, p.WeightGrams, p.ImageURL, specs, p.AvailableFrom, p.AvailableUntil, id,
	)
	return err
}

func (r *Repository) PublishProduct(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE product SET status = 'published', updated_at = now() WHERE id = $1 AND status = 'draft'`,
		id,
	)
	return err
}

func (r *Repository) DeleteProduct(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM product WHERE id = $1`,
		id,
	)
	return err
}

func (r *Repository) ArchiveProduct(ctx context.Context, id string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE product SET status = 'archived', updated_at = now() WHERE id = $1`,
		id,
	)
	return err
}

// ReplaceProductCourses atomically replaces all product_course links for a product.
func (r *Repository) ReplaceProductCourses(ctx context.Context, tx pgx.Tx, productID uuid.UUID, courseIDs []uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM product_course WHERE product_id = $1`, productID)
	if err != nil {
		return err
	}
	for _, courseID := range courseIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO product_course (product_id, course_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			productID, courseID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateProductWithCourses inserts a product and its product_course links in one transaction.
func (r *Repository) CreateProductWithCourses(ctx context.Context, tx pgx.Tx, p *model.Product, courseIDs []uuid.UUID) error {
	specs, err := marshalSpecs(p.Specs)
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO product (type, name, description, price, stock, status, weight_grams, image_url, specs, available_from, available_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`,
		p.Type, p.Name, p.Description, p.Price, p.Stock, p.Status, p.WeightGrams, p.ImageURL, specs, p.AvailableFrom, p.AvailableUntil,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return err
	}

	for _, courseID := range courseIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO product_course (product_id, course_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`,
			p.ID, courseID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReplaceProductExams atomically replaces all product_exam links for a product,
// mirroring ReplaceProductCourses.
func (r *Repository) ReplaceProductExams(ctx context.Context, tx pgx.Tx, productID uuid.UUID, examIDs []uuid.UUID) error {
	_, err := tx.Exec(ctx, `DELETE FROM product_exam WHERE product_id = $1`, productID)
	if err != nil {
		return err
	}
	for _, examID := range examIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2) ON CONFLICT DO NOTHING`,
			productID, examID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}

// CreateProductWithExams inserts a product and its product_exam links in one transaction,
// mirroring CreateProductWithCourses.
func (r *Repository) CreateProductWithExams(ctx context.Context, tx pgx.Tx, p *model.Product, examIDs []uuid.UUID) error {
	specs, err := marshalSpecs(p.Specs)
	if err != nil {
		return err
	}
	err = tx.QueryRow(ctx,
		`INSERT INTO product (type, name, description, price, stock, status, weight_grams, image_url, specs, available_from, available_until)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at`,
		p.Type, p.Name, p.Description, p.Price, p.Stock, p.Status, p.WeightGrams, p.ImageURL, specs, p.AvailableFrom, p.AvailableUntil,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return err
	}

	for _, examID := range examIDs {
		_, err := tx.Exec(ctx,
			`INSERT INTO product_exam (product_id, exam_id)
			VALUES ($1, $2)
			ON CONFLICT DO NOTHING`,
			p.ID, examID,
		)
		if err != nil {
			return err
		}
	}
	return nil
}
