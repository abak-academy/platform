package repository

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

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
const productAvailabilityFilter = ` AND (p.available_from IS NULL OR p.available_from <= now())` +
	` AND (p.available_until IS NULL OR p.available_until >= now())`

// examOrderabilityFilter closes ordering when the exam is no longer startable:
// latestStart (scheduled_end_at when flexible, scheduled_at otherwise) plus the
// exam runtime and grace window. This mirrors the Session Monitor's live window;
// stopping at scheduled_at would reject late purchases while students can still
// legitimately start.
const examOrderabilityFilter = ` AND (
			e.scheduled_at IS NULL OR now() <= COALESCE(e.scheduled_end_at, e.scheduled_at) +
			((COALESCE(e.duration_minutes, COALESCE((
				SELECT SUM(t.duration_minutes)
				FROM exam_test et
				JOIN test t ON t.id = et.test_id
				WHERE et.exam_id = e.id
			), 0)) + COALESCE(e.grace_window_minutes, 0)) * INTERVAL '1 minute')
		)`

// linkedOrderableExamProductFilter keeps exam products out of student-facing
// catalog/detail reads after every linked exam has passed its startable window.
// Non-exam products use only the product availability window.
const linkedOrderableExamProductFilter = ` AND (p.type <> 'exam' OR EXISTS (
			SELECT 1 FROM product_exam pe
			JOIN exam e ON e.id = pe.exam_id
			WHERE pe.product_id = p.id` + examOrderabilityFilter + `
		))`

type ProductFilter struct {
	Type string
	// Types is a role-scoped allowlist applied in SQL. nil means unrestricted;
	// a non-nil empty slice means the role may see no type at all.
	Types             []string
	Status            string
	VisibleOnly       bool // true = student-visible catalog rules
	OrderableExamOnly bool // true = linked to at least one exam whose startable window is open
	Cursor            string
	Limit             int
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
// linked product is not currently orderable for that exam.
func (r *Repository) GetProductByExamID(ctx context.Context, examID uuid.UUID) (*model.Product, error) {
	p := &model.Product{}
	err := scanProduct(r.pool.QueryRow(ctx,
		`SELECT p.id, p.type, p.name, p.description, p.price, p.stock, p.status,
		        p.weight_grams, p.image_url, p.specs, p.available_from, p.available_until, p.created_at, p.updated_at
		 FROM product p
		 JOIN product_exam pe ON pe.product_id = p.id
		 JOIN exam e ON e.id = pe.exam_id
		 WHERE pe.exam_id = $1 AND p.type = 'exam' AND p.status = 'published'
		   AND (p.available_from IS NULL OR p.available_from <= now())
		   AND (p.available_until IS NULL OR p.available_until >= now())`+examOrderabilityFilter,
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
	query := `SELECT p.id, p.type, p.name, p.description, p.price, p.stock, p.status, p.weight_grams, p.image_url, p.specs, p.available_from, p.available_until, p.created_at, p.updated_at
	FROM product p WHERE 1=1`
	args := []interface{}{}
	argIdx := 1

	if filter.Type != "" {
		query += fmt.Sprintf(` AND p.type = $%d`, argIdx)
		args = append(args, filter.Type)
		argIdx++
	}
	// Applied here, in the WHERE clause, so the role's type boundary is enforced
	// before LIMIT picks a page — filtering after the fact would let a page of
	// physical rows crowd out the course/exam rows the role is entitled to.
	if filter.Types != nil {
		if len(filter.Types) == 0 {
			return []model.Product{}, "", nil
		}
		placeholders := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			placeholders[i] = fmt.Sprintf("$%d", argIdx)
			args = append(args, t)
			argIdx++
		}
		query += ` AND p.type IN (` + strings.Join(placeholders, ", ") + `)`
	}
	if filter.Status != "" {
		query += fmt.Sprintf(` AND p.status = $%d`, argIdx)
		args = append(args, filter.Status)
		argIdx++
	}
	if filter.VisibleOnly {
		// public catalog: published, within the product availability window, and
		// for exam products linked to at least one exam whose startable window is
		// still open.
		query += ` AND p.status = 'published'`
		query += productAvailabilityFilter
		query += linkedOrderableExamProductFilter
	}
	if filter.OrderableExamOnly {
		query += ` AND EXISTS (
			SELECT 1 FROM product_exam pe
			JOIN exam e ON e.id = pe.exam_id
			WHERE pe.product_id = p.id` + examOrderabilityFilter + `
		)`
	}
	if filter.Cursor != "" {
		if _, err := uuid.Parse(filter.Cursor); err != nil {
			return nil, "", ErrInvalidCursor
		}
		query += fmt.Sprintf(` AND p.id > $%d`, argIdx)
		args = append(args, filter.Cursor)
		argIdx++
	}

	query += ` ORDER BY p.id LIMIT $` + fmt.Sprintf("%d", argIdx)
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

func (r *Repository) ProductHasOrderableExam(ctx context.Context, productID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS (
			SELECT 1 FROM product_exam pe
			JOIN exam e ON e.id = pe.exam_id
			WHERE pe.product_id = $1`+examOrderabilityFilter+`
		)`,
		productID,
	).Scan(&ok)
	return ok, err
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
