package integration_test

import (
	"context"
	"net/http"
	"testing"

	"akademi-bimbel/internal/model"

	"github.com/stretchr/testify/require"
)

func TestExamProductStock_CreateAndEditStoresQuota(t *testing.T) {
	env := newTestEnv(t)
	ctx := context.Background()
	token := authToken(t, env, seedUser(t, env, "super_admin", "active", false), "super_admin")

	exam, err := env.svc.CreateExam(ctx, model.Exam{Title: "Stock Exam", Mode: "standard"})
	require.NoError(t, err)

	tests := []struct {
		name        string
		createStock any
		patchBody   map[string]any
		wantStock   *int
		wantLimited bool
	}{
		{"create unlimited stores NULL", nil, nil, nil, false},
		{"create limited stores quota", 5, nil, intPtr(5), true},
		{"patch unlimited to limited", nil, map[string]any{"name": "Patch Unlimited", "stock": 3}, intPtr(3), true},
		{"patch limited to unlimited", 5, map[string]any{"name": "Patch Limited", "stock": nil}, nil, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			productID, limited := createExamProduct(t, env, token, exam.ID.String(), tt.name, tt.createStock)

			if tt.patchBody != nil {
				resp, out := doJSONBody(t, env, http.MethodPatch, "/api/v1/admin/products/"+productID, tt.patchBody, token)
				require.Equal(t, http.StatusOK, resp.StatusCode, "body=%v", out)
				limited, _ = out["stock_limited"].(bool)
			}

			require.Equal(t, tt.wantLimited, limited)
			require.Equal(t, tt.wantStock, productStock(t, env, ctx, productID))
		})
	}
}

func createExamProduct(t *testing.T, env *testEnv, token, examID, name string, stock any) (string, bool) {
	t.Helper()
	resp, out := doJSONBody(t, env, http.MethodPost, "/api/v1/admin/products",
		map[string]any{"type": "exam", "name": name, "price": 1000, "stock": stock, "exam_ids": []string{examID}},
		token)
	require.Equal(t, http.StatusCreated, resp.StatusCode, "body=%v", out)
	id, _ := out["id"].(string)
	require.NotEmpty(t, id)
	limited, _ := out["stock_limited"].(bool)
	return id, limited
}

func productStock(t *testing.T, env *testEnv, ctx context.Context, productID string) *int {
	t.Helper()
	var stock *int
	require.NoError(t, env.pool.QueryRow(ctx, `SELECT stock FROM product WHERE id = $1`, productID).Scan(&stock))
	return stock
}

func intPtr(n int) *int { return &n }
