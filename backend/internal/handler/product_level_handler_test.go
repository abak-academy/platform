package handler_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/service"
)

func patchProduct(t *testing.T, env *adminProductDBTestEnv, id, token string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPatch, "/api/v1/admin/products/"+id, strings.NewReader(mustJSON(t, body)))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	return rec
}

func createExamProduct(t *testing.T, env *adminProductDBTestEnv, token string) string {
	t.Helper()
	rec := postJSONWithToken(t, env.e, "/api/v1/admin/products", token, mustJSON(t, map[string]any{"type": "book", "name": "Edisi", "price": 1000, "stock": 7}))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create product: %d %s", rec.Code, rec.Body.String())
	}
	return decodeMap(t, rec)["id"].(string)
}

func TestAdminUpdateProduct_LevelFields(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000b1", service.RoleSuperAdmin)
	id := createExamProduct(t, env, token)

	rec := patchProduct(t, env, id, token, map[string]any{"name": "Edisi", "price": 1000, "stock": 7, "stages": []string{"SD", "SMP"}, "grade_min": 4, "grade_max": 8})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch level = %d %s", rec.Code, rec.Body.String())
	}
	got := decodeMap(t, rec)
	if s, _ := got["stages"].([]any); len(s) != 2 || s[0] != "SD" || s[1] != "SMP" || got["grade_min"] != float64(4) || got["grade_max"] != float64(8) {
		t.Fatalf("response level = %v %v %v", got["stages"], got["grade_min"], got["grade_max"])
	}
	if got["stock"] != float64(7) {
		t.Fatalf("stock = %v, want 7 untouched by a level patch", got["stock"])
	}

	var stages []string
	var gmin, gmax *int
	if err := env.pool.QueryRow(t.Context(), `SELECT stages, grade_min, grade_max FROM product WHERE id = $1`, id).Scan(&stages, &gmin, &gmax); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(stages) != 2 || gmin == nil || *gmin != 4 || gmax == nil || *gmax != 8 {
		t.Fatalf("persisted = %v %v %v", stages, gmin, gmax)
	}

	// Fields absent from the body preserve the stored level.
	if rec := patchProduct(t, env, id, token, map[string]any{"name": "Renamed", "price": 1000, "stock": 7}); rec.Code != http.StatusOK {
		t.Fatalf("patch without level = %d %s", rec.Code, rec.Body.String())
	}
	if err := env.pool.QueryRow(t.Context(), `SELECT stages, grade_min, grade_max FROM product WHERE id = $1`, id).Scan(&stages, &gmin, &gmax); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if len(stages) != 2 || gmin == nil || *gmin != 4 || gmax == nil || *gmax != 8 {
		t.Fatalf("absent level fields changed the stored level: %v %v %v", stages, gmin, gmax)
	}

	// null clears a grade, [] clears stages.
	rec = patchProduct(t, env, id, token, map[string]any{"name": "Renamed", "price": 1000, "stock": 7, "stages": []string{}, "grade_min": nil, "grade_max": nil})
	if rec.Code != http.StatusOK {
		t.Fatalf("clear level = %d %s", rec.Code, rec.Body.String())
	}
	if err := env.pool.QueryRow(t.Context(), `SELECT stages, grade_min, grade_max FROM product WHERE id = $1`, id).Scan(&stages, &gmin, &gmax); err != nil || len(stages) != 0 || gmin != nil || gmax != nil {
		t.Fatalf("cleared level = %v %v %v err=%v", stages, gmin, gmax, err)
	}
}

func TestAdminUpdateProduct_LevelValidation422(t *testing.T) {
	env := newAdminProductDBEnv(t)
	token := mintProductToken(t, env, "00000000-0000-0000-0000-0000000000b2", service.RoleSuperAdmin)
	id := createExamProduct(t, env, token)
	base := func(extra map[string]any) map[string]any {
		m := map[string]any{"name": "Edisi", "price": 1000, "stock": 7}
		for k, v := range extra {
			m[k] = v
		}
		return m
	}
	if rec := patchProduct(t, env, id, token, base(map[string]any{"grade_min": 5, "grade_max": 9})); rec.Code != http.StatusOK {
		t.Fatalf("seed level = %d", rec.Code)
	}
	cases := map[string]map[string]any{
		"unknown stage":                  {"stages": []string{"TK"}},
		"grade_max below grade_min":      {"grade_min": 6, "grade_max": 4},
		"grade 0":                        {"grade_min": 0},
		"grade 13":                       {"grade_max": 13},
		"max below the stored grade_min": {"grade_max": 3},
	}
	for name, extra := range cases {
		rec := patchProduct(t, env, id, token, base(extra))
		if rec.Code != http.StatusUnprocessableEntity || decodeMap(t, rec)["code"] != "validation_failed" {
			t.Errorf("%s = %d %s, want 422 validation_failed", name, rec.Code, rec.Body.String())
		}
	}
	var gmin, gmax int
	if err := env.pool.QueryRow(t.Context(), `SELECT grade_min, grade_max FROM product WHERE id = $1`, id).Scan(&gmin, &gmax); err != nil || gmin != 5 || gmax != 9 {
		t.Fatalf("rejected patches changed the level: %d %d err=%v", gmin, gmax, err)
	}
}
