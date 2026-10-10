package repository

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMigration0069_ExamProductsBecomeUnlimited(t *testing.T) {
	ctx := context.Background()
	pool := newMigration0025Pool(t)

	applyMigrationsUpTo(t, pool, "0068_bound_legacy_pending_orders.up.sql")

	tests := []struct {
		productType string
		wantStock   *int
	}{
		{"exam", nil},
		{"book", intPtr(5)},
		{"merchandise", intPtr(5)},
		{"medal", intPtr(5)},
		{"course", intPtr(5)},
	}

	ids := make(map[string]uuid.UUID, len(tests))
	for _, tt := range tests {
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx,
			`INSERT INTO product (type, name, price, stock, status) VALUES ($1, 'p', 1000, 5, 'published') RETURNING id`, tt.productType,
		).Scan(&id))
		ids[tt.productType] = id
	}

	applyMigrationFile(t, pool, "0069_exam_product_stock.up.sql")

	for _, tt := range tests {
		t.Run(tt.productType, func(t *testing.T) {
			var stock *int
			require.NoError(t, pool.QueryRow(ctx, `SELECT stock FROM product WHERE id = $1`, ids[tt.productType]).Scan(&stock))
			require.Equal(t, tt.wantStock, stock)
		})
	}
}
