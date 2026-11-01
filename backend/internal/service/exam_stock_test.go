package service

import (
	"context"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/repository"
)

func TestTracksStock(t *testing.T) {
	tests := []struct {
		name    string
		ptype   string
		limited bool
		want    bool
	}{
		{"book always tracks", "book", false, true},
		{"merchandise always tracks", "merchandise", false, true},
		{"medal always tracks", "medal", false, true},
		{"unlimited exam does not track", "exam", false, false},
		{"limited exam tracks", "exam", true, true},
		{"course never tracks", "course", true, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tracksStock(tt.ptype, tt.limited); got != tt.want {
				t.Errorf("tracksStock(%q, %v) = %v, want %v", tt.ptype, tt.limited, got, tt.want)
			}
		})
	}
}

func TestAddItem_ExamStock(t *testing.T) {
	tests := []struct {
		name    string
		stock   *int
		wantErr error
	}{
		{"limited exam with zero stock is out of stock", intptr(0), ErrOutOfStock},
		{"unlimited exam is allowed", nil, nil},
		{"limited exam with available stock is allowed", intptr(3), nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			svc, repo := newRealDBService(t)

			studentID := seedMerchStudent(t, repo)
			productID := seedExamProduct(t, repo, tt.stock)

			order, _, err := svc.MintCart(ctx, studentID)
			require.NoError(t, err)

			err = svc.AddItem(ctx, studentID, order.ID.String(), productID, 1)
			if tt.wantErr != nil {
				require.ErrorIs(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tt.stock, examProductStock(t, repo, productID))
		})
	}
}

func examProductStock(t *testing.T, repo *repository.Repository, productID string) *int {
	t.Helper()
	var stock *int
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`SELECT stock FROM product WHERE id = $1`, productID).Scan(&stock))
	return stock
}

// seedExamProduct inserts an exam product; a nil stock stores NULL (unlimited).
func seedExamProduct(t *testing.T, repo *repository.Repository, stock *int) string {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, stock, status) VALUES ('exam', $1, 100000, $2, 'published') RETURNING id`,
		"Exam "+uniqueSuffix(), stock,
	).Scan(&id))
	return id.String()
}
