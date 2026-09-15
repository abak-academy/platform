package handler_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/handler"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

func TestListSchools_SearchEnvelope(t *testing.T) {
	env := newAdminStuDBEnv(t)
	ctx := context.Background()
	e := echo.New()
	h := handler.New(env.svc)
	e.GET("/api/v1/schools", h.ListSchools)

	var cityID, provinceID string
	if err := env.pool.QueryRow(ctx,
		`SELECT id, province_id FROM city ORDER BY id LIMIT 1`,
	).Scan(&cityID, &provinceID); err != nil {
		t.Fatalf("load city: %v", err)
	}

	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")[:8]
	firstNPSN := "2" + strings.ToUpper(suffix[:7])
	secondNPSN := "3" + strings.ToUpper(suffix[:7])
	insertSearchSchool(t, env, "SMA Search Alpha "+suffix, "SMA", firstNPSN, provinceID, cityID)
	insertSearchSchool(t, env, "SMA Search Beta "+suffix, "SMA", secondNPSN, provinceID, cityID)
	insertSearchSchool(t, env, "SMK Search Hidden "+suffix, "SMK", "4"+strings.ToUpper(suffix[:7]), provinceID, cityID)
	for i := 0; i < 55; i++ {
		insertSearchSchool(t, env, fmt.Sprintf("SMA Search Bulk %02d %s", i, suffix), "SMA", fmt.Sprintf("7%s%02d", strings.ToUpper(suffix[:5]), i), provinceID, cityID)
	}

	t.Run("no filters returns bounded empty envelope", func(t *testing.T) {
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/schools", nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp schoolOptionsEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode envelope: %v body=%s", err, rec.Body.String())
		}
		if len(resp.Data) != 0 {
			t.Fatalf("no-filter response: want empty envelope, got %+v", resp)
		}
	})

	t.Run("province city category search is bounded to requested limit", func(t *testing.T) {
		path := "/api/v1/schools?province_id=" + provinceID + "&city_id=" + cityID + "&category=SMA&limit=1"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp schoolOptionsEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.Data) != 1 || resp.Data[0].Category == nil || *resp.Data[0].Category != "SMA" {
			t.Fatalf("unexpected response %+v", resp)
		}
	})

	t.Run("province city category browse works without name query", func(t *testing.T) {
		path := "/api/v1/schools?province_id=" + provinceID + "&city_id=" + cityID + "&category=SMA&limit=1000"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp schoolOptionsEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode browse response: %v", err)
		}
		if len(resp.Data) < 57 || len(resp.Data) > 1000 {
			t.Fatalf("browse response: want bounded scoped SMA schools including seeded rows, got %d", len(resp.Data))
		}
		for _, school := range resp.Data {
			if school.KotaID == nil || *school.KotaID != cityID || school.Category == nil || *school.Category != "SMA" {
				t.Fatalf("browse school not scoped to city/category: %+v", school)
			}
		}
	})

	t.Run("npsn search ignores stale filters", func(t *testing.T) {
		path := "/api/v1/schools?npsn=" + strings.ToLower(firstNPSN) + "&province_id=bad-province&category=SMK"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp schoolOptionsEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode npsn response: %v", err)
		}
		if len(resp.Data) != 1 || resp.Data[0].NPSN == nil || *resp.Data[0].NPSN != firstNPSN {
			t.Fatalf("npsn response: unexpected %+v", resp)
		}
	})
}

func TestGetSchool_HydratesLegacyInactiveSchool(t *testing.T) {
	env := newAdminStuDBEnv(t)
	ctx := context.Background()
	e := echo.New()
	h := handler.New(env.svc)
	e.GET("/api/v1/schools/:id", h.GetSchool)

	var schoolID string
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO school (name, code, status) VALUES ($1, $2, 'deactivated') RETURNING id`,
		"Legacy Inactive Hydration", "legacy_"+uuid.NewString()[:8],
	).Scan(&schoolID); err != nil {
		t.Fatalf("insert legacy school: %v", err)
	}

	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/schools/"+schoolID, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var option schoolOptionResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &option); err != nil {
		t.Fatalf("decode option: %v", err)
	}
	if option.ID != schoolID || option.Name != "Legacy Inactive Hydration" || option.Status != "deactivated" {
		t.Fatalf("unexpected hydrated option: %+v", option)
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/schools/not-a-uuid", nil))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), `"code":"invalid_request"`) {
		t.Fatalf("invalid id: want 400 invalid_request, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec = httptest.NewRecorder()
	e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/schools/"+uuid.NewString(), nil))
	if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), `"code":"school_not_found"`) {
		t.Fatalf("missing id: want 404 school_not_found, got %d body=%s", rec.Code, rec.Body.String())
	}
}

type schoolOptionsEnvelope struct {
	Data []schoolOptionResponse `json:"data"`
}

type schoolOptionResponse struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	Code       string  `json:"code"`
	NPSN       *string `json:"npsn"`
	Status     string  `json:"status"`
	Category   *string `json:"category"`
	ProvinsiID *string `json:"provinsi_id"`
	KotaID     *string `json:"kota_id"`
}

func insertSearchSchool(t *testing.T, env *adminStuDBTestEnv, name, category, npsn, provinceID, kotaID string) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(),
		`INSERT INTO school (name, code, npsn, school_types, alamat, status, category, provinsi_id, kota_id)
		VALUES ($1, $2, $3, ARRAY[$4], $5, 'active', $4, $6, $7)`,
		name, "search_"+uuid.NewString()[:8], npsn, category, "Jl. Search", provinceID, kotaID,
	); err != nil {
		t.Fatalf("insert search school: %v", err)
	}
}
