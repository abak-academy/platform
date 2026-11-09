package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
)

func postJSONWithKey(t *testing.T, env *affiliateJobsEnv, path, token, body, key string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	rec := httptest.NewRecorder()
	env.e.ServeHTTP(rec, req)
	return rec
}

func seedOpenCourseProduct(t *testing.T, env *affiliateJobsEnv, price int) string {
	t.Helper()
	ctx := context.Background()
	var courseID, productID string
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO course (title, level, subject, instructor_name) VALUES ($1, 'sma', 'math', 'Guru') RETURNING id::text`, "Kelas "+uniqueHandlerSuffix(),
	).Scan(&courseID); err != nil {
		t.Fatalf("seed course: %v", err)
	}
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO product (type, name, price, status, partner_access) VALUES ('course', $1, $2, 'published', '{"scope":"all","ids":[]}') RETURNING id::text`,
		"Produk "+uniqueHandlerSuffix(), price,
	).Scan(&productID); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	if _, err := env.pool.Exec(ctx, `INSERT INTO product_course (product_id, course_id) VALUES ($1, $2)`, productID, courseID); err != nil {
		t.Fatalf("link course: %v", err)
	}
	return productID
}

func seedStudentRegisteredBy(t *testing.T, env *affiliateJobsEnv, registeredBy string) string {
	t.Helper()
	var id string
	suffix := uniqueHandlerSuffix()
	var registrar any
	if registeredBy != "" {
		registrar = registeredBy
	}
	if err := env.pool.QueryRow(context.Background(),
		`INSERT INTO users (email, username, role, name, status, registered_by, jenjang) VALUES ($1, $2, 'student', $3, 'active', $4, 'sma') RETURNING id::text`,
		"stu-"+suffix+"@test.local", "stu"+suffix, "Siswa "+suffix, registrar,
	).Scan(&id); err != nil {
		t.Fatalf("seed student: %v", err)
	}
	return id
}

func TestPartnerOrderRoutes(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")
	partner := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	token := env.tokenForUser(t, partner, service.RoleAffiliator)
	product := seedOpenCourseProduct(t, env, 100000)
	mine, other := seedStudentRegisteredBy(t, env, partner), seedStudentRegisteredBy(t, env, "")
	order := "/api/v1/partners/me/orders"

	t.Run("a preview shows the charge", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, order+"/preview", token, `{"product_id":"`+product+`","student_ids":["`+mine+`"]}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		if body["net_new_count"] != float64(1) || body["total"] != float64(100000) {
			t.Errorf("preview = %v, want 1 student and 100000", body)
		}
	})

	t.Run("a student outside the Mitra's scope refuses the request", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, order+"/preview", token, `{"product_id":"`+product+`","student_ids":["`+mine+`","`+other+`"]}`)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "student_not_allowed" {
			t.Errorf("want 403 student_not_allowed, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a product that is not open is refused", func(t *testing.T) {
		closed := seedOpenCourseProduct(t, env, 50000)
		if _, err := env.pool.Exec(context.Background(), `UPDATE product SET partner_access = NULL WHERE id = $1`, closed); err != nil {
			t.Fatalf("close product: %v", err)
		}
		rec := postJSONWithToken(t, env.e, order+"/preview", token, `{"product_id":"`+closed+`","student_ids":["`+mine+`"]}`)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "product_not_allowed" {
			t.Errorf("want 403 product_not_allowed, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a request with no students is a bad request", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, order, token, `{"product_id":"`+product+`","student_ids":[]}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("create, list, read and pay a fully discounted order", func(t *testing.T) {
		admin := env.tokenFor(t, service.RoleSuperAdmin)
		code := "FULL" + strings.ToUpper(uniqueHandlerSuffix()[:6])
		if rec := postJSONWithToken(t, env.e, "/api/v1/admin/promo-codes", admin,
			`{"code":"`+code+`","discount_percent":100,"partner_voucher":{"enabled":true,"quota_unit":"student"}}`); rec.Code != http.StatusCreated {
			t.Fatalf("create voucher: %d %s", rec.Code, rec.Body.String())
		}

		rec := postJSONWithToken(t, env.e, order, token, `{"product_id":"`+product+`","student_ids":["`+mine+`"],"promo_code":"`+code+`"}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("create: want 201, got %d: %s", rec.Code, rec.Body.String())
		}
		orderID, _ := decodeBody(t, rec)["id"].(string)

		if rec := getWithToken(t, env.e, order, token); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), orderID) {
			t.Errorf("list: want the order in a 200, got %d: %s", rec.Code, rec.Body.String())
		}
		rec = getWithToken(t, env.e, order+"/"+orderID, token)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), mine) {
			t.Errorf("detail: want the student named, got %d: %s", rec.Code, rec.Body.String())
		}
		if rec := getWithToken(t, env.e, order+"/"+orderID, env.tokenForUser(t, seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, ""), service.RoleAffiliator)); rec.Code != http.StatusNotFound {
			t.Errorf("another Mitra reading it: want 404, got %d", rec.Code)
		}

		if rec := postJSONWithKey(t, env, order+"/"+orderID+"/checkout", token, `{}`, ""); rec.Code != http.StatusBadRequest {
			t.Errorf("checkout without a key: want 400, got %d", rec.Code)
		}
		rec = postJSONWithKey(t, env, order+"/"+orderID+"/checkout", token, `{}`, "key-"+uniqueHandlerSuffix())
		if rec.Code != http.StatusOK || decodeBody(t, rec)["free"] != true {
			t.Errorf("checkout: want 200 free, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a suspended Mitra cannot start an order", func(t *testing.T) {
		suspended := seedProgramAccount(t, env.pool, "deactivated", "suspended", "", partnerOnlyJSON, "")
		rec := postJSONWithToken(t, env.e, order, env.tokenForUser(t, suspended, service.RoleAffiliator),
			`{"product_id":"`+product+`","student_ids":["`+uuid.NewString()+`"]}`)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "account_deactivated" {
			t.Errorf("want 403 account_deactivated, got %d %s", rec.Code, rec.Body.String())
		}
	})
}
