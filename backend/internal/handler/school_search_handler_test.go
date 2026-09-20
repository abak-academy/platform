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

	t.Run("province city school_type search is bounded to requested limit", func(t *testing.T) {
		path := "/api/v1/schools?province_id=" + provinceID + "&city_id=" + cityID + "&school_type=SMA&limit=1"
		rec := httptest.NewRecorder()
		e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d body=%s", rec.Code, rec.Body.String())
		}
		var resp schoolOptionsEnvelope
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}
		if len(resp.Data) != 1 || len(resp.Data[0].SchoolTypes) != 1 || resp.Data[0].SchoolTypes[0] != "SMA" {
			t.Fatalf("unexpected response %+v", resp)
		}
	})

	t.Run("province city school_type browse works without name query", func(t *testing.T) {
		path := "/api/v1/schools?province_id=" + provinceID + "&city_id=" + cityID + "&school_type=SMA&limit=1000"
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
			if school.KotaID == nil || *school.KotaID != cityID || len(school.SchoolTypes) != 1 || school.SchoolTypes[0] != "SMA" {
				t.Fatalf("browse school not scoped to city/school_type: %+v", school)
			}
		}
	})

	t.Run("npsn search ignores stale filters", func(t *testing.T) {
		path := "/api/v1/schools?npsn=" + strings.ToLower(firstNPSN) + "&province_id=bad-province&school_type=SMK"
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

	t.Run("array membership matches every type once and preserves stored values", func(t *testing.T) {
		var schoolID string
		types := []string{"SD", " smp ", "SMP", "TK", "SLB", "PONDOK PESANTREN"}
		if err := env.pool.QueryRow(ctx,
			`INSERT INTO school (name, code, school_types, provinsi_id, kota_id)
			VALUES ($1, $2, $3, $4, $5) RETURNING id`,
			"Array School "+suffix, "array_"+suffix, types, provinceID, cityID,
		).Scan(&schoolID); err != nil {
			t.Fatal(err)
		}
		for _, schoolType := range []string{"SD", "smp", "TK", "SLB", "PONDOK%20PESANTREN", "SMK"} {
			path := "/api/v1/schools?province_id=" + provinceID + "&city_id=" + cityID + "&school_type=" + schoolType + "&limit=1000"
			rec := httptest.NewRecorder()
			e.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("%s: status=%d body=%s", schoolType, rec.Code, rec.Body.String())
			}
			if strings.Contains(rec.Body.String(), `"category"`) {
				t.Fatal("school response must not expose category")
			}
			var resp schoolOptionsEnvelope
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatal(err)
			}
			matches := 0
			for _, school := range resp.Data {
				if school.ID == schoolID {
					matches++
					if strings.Join(school.SchoolTypes, "|") != strings.Join(types, "|") {
						t.Fatalf("stored types changed: %+v", school.SchoolTypes)
					}
				}
			}
			want := 1
			if schoolType == "SMK" {
				want = 0
			}
			if matches != want {
				t.Fatalf("%s: want %d school result, got %d", schoolType, want, matches)
			}
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
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Code        string   `json:"code"`
	NPSN        *string  `json:"npsn"`
	Status      string   `json:"status"`
	SchoolTypes []string `json:"school_types"`
	ProvinsiID  *string  `json:"provinsi_id"`
	KotaID      *string  `json:"kota_id"`
}

func insertSearchSchool(t *testing.T, env *adminStuDBTestEnv, name, school_type, npsn, provinceID, kotaID string) {
	t.Helper()
	if _, err := env.pool.Exec(context.Background(),
		`INSERT INTO school (name, code, npsn, school_types, alamat, status, provinsi_id, kota_id)
		VALUES ($1, $2, $3, ARRAY[$4], $5, 'active', $6, $7)`,
		name, "search_"+uuid.NewString()[:8], npsn, school_type, "Jl. Search", provinceID, kotaID,
	); err != nil {
		t.Fatalf("insert search school: %v", err)
	}
}
