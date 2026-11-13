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
		rec := postJSONWithToken(t, env.e, order+"/preview", token, `{"product_ids":["`+product+`"],"student_ids":["`+mine+`"]}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		items, _ := body["items"].([]any)
		if len(items) != 1 {
			t.Fatalf("preview items = %v, want 1", body["items"])
		}
		item, _ := items[0].(map[string]any)
		if item["net_new_count"] != float64(1) || body["total"] != float64(100000) {
			t.Errorf("preview = %v, want 1 student and 100000", body)
		}
	})

	t.Run("a student outside the Mitra's scope refuses the request", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, order+"/preview", token, `{"product_ids":["`+product+`"],"student_ids":["`+mine+`","`+other+`"]}`)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "student_not_allowed" {
			t.Errorf("want 403 student_not_allowed, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a product that is not open is refused", func(t *testing.T) {
		closed := seedOpenCourseProduct(t, env, 50000)
		if _, err := env.pool.Exec(context.Background(), `UPDATE product SET partner_access = NULL WHERE id = $1`, closed); err != nil {
			t.Fatalf("close product: %v", err)
		}
		rec := postJSONWithToken(t, env.e, order+"/preview", token, `{"product_ids":["`+closed+`"],"student_ids":["`+mine+`"]}`)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "product_not_allowed" {
			t.Errorf("want 403 product_not_allowed, got %d %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a request with no students is a bad request", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, order, token, `{"product_ids":["`+product+`"],"student_ids":[]}`)
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

		rec := postJSONWithToken(t, env.e, order, token, `{"product_ids":["`+product+`"],"student_ids":["`+mine+`"],"promo_code":"`+code+`"}`)
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

	t.Run("the dashboard counts what the Mitra did", func(t *testing.T) {
		rec := getWithToken(t, env.e, "/api/v1/partners/me/dashboard", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		if body["students_registered"] != float64(1) {
			t.Errorf("students_registered = %v, want 1 (only the student this Mitra registered)", body["students_registered"])
		}
		if body["orders_paid"] != float64(1) {
			t.Errorf("orders_paid = %v, want the one paid above", body["orders_paid"])
		}
		for _, key := range []string{"orders_unpaid", "vouchers_available", "products_open"} {
			if _, ok := body[key]; !ok {
				t.Errorf("%s is missing: %s", key, rec.Body.String())
			}
		}
	})

	t.Run("a suspended Mitra cannot start an order", func(t *testing.T) {
		suspended := seedProgramAccount(t, env.pool, "deactivated", "suspended", "", partnerOnlyJSON, "")
		rec := postJSONWithToken(t, env.e, order, env.tokenForUser(t, suspended, service.RoleAffiliator),
			`{"product_ids":["`+product+`"],"student_ids":["`+uuid.NewString()+`"]}`)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "account_deactivated" {
			t.Errorf("want 403 account_deactivated, got %d %s", rec.Code, rec.Body.String())
		}
	})
}

func TestPartnerCatalogRoutes(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")
	school, otherSchool := seedSchool(t, env.pool), seedSchool(t, env.pool)
	programs := `{"affiliate": false, "partner": {"school_ids": ["` + school + `"]}}`
	partner := seedProgramAccount(t, env.pool, "active", "active", "", programs, "")
	token := env.tokenForUser(t, partner, service.RoleAffiliator)
	base := "/api/v1/partners/me"

	voucher := "SCH" + strings.ToUpper(uniqueHandlerSuffix()[:6])
	if _, err := env.pool.Exec(context.Background(),
		`INSERT INTO promo_code (code, discount_percent, partner) VALUES ($1, 10, '{"ids":[],"quota_unit":"order"}')`, voucher); err != nil {
		t.Fatalf("seed voucher: %v", err)
	}
	product := seedOpenCourseProduct(t, env, 100000)

	t.Run("schools lists only the allowed ones", func(t *testing.T) {
		rec := getWithToken(t, env.e, base+"/schools", token)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), school) || strings.Contains(rec.Body.String(), otherSchool) {
			t.Errorf("want only the allowed school in a 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("schools are searched and paged with a total", func(t *testing.T) {
		rec := getWithToken(t, env.e, base+"/schools?page=1&per_page=1&q=zzz-no-such", token)
		body := decodeBody(t, rec)
		if rec.Code != http.StatusOK || body["total"] != float64(0) || body["page"] != float64(1) || body["per_page"] != float64(1) {
			t.Errorf("want an empty page with total 0, got %d %v", rec.Code, body)
		}
		if rec := getWithToken(t, env.e, base+"/schools?page=0", token); rec.Code != http.StatusBadRequest {
			t.Errorf("page 0: want 400, got %d", rec.Code)
		}
	})

	t.Run("students answer by page number when asked, with a total", func(t *testing.T) {
		rec := getWithToken(t, env.e, base+"/students?page=1&per_page=5", token)
		body := decodeBody(t, rec)
		if rec.Code != http.StatusOK || body["page"] != float64(1) || body["per_page"] != float64(5) {
			t.Errorf("want a numbered page, got %d %v", rec.Code, body)
		}
		if _, ok := body["total"]; !ok {
			t.Errorf("want a total, got %v", body)
		}
	})

	t.Run("orders are searched and filtered by date", func(t *testing.T) {
		if rec := getWithToken(t, env.e, base+"/orders?from=nope", token); rec.Code != http.StatusBadRequest {
			t.Errorf("bad date: want 400, got %d", rec.Code)
		}
		rec := getWithToken(t, env.e, base+"/orders?q=anything&from=2026-01-01&to=2026-12-31&page=1&per_page=5", token)
		if body := decodeBody(t, rec); rec.Code != http.StatusOK || body["total"] != float64(0) {
			t.Errorf("want an empty page, got %d %v", rec.Code, body)
		}
	})

	t.Run("an unknown order status is a bad request, a known one filters", func(t *testing.T) {
		if rec := getWithToken(t, env.e, base+"/orders?status=nope", token); rec.Code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", rec.Code)
		}
		if rec := getWithToken(t, env.e, base+"/orders?status=paid", token); rec.Code != http.StatusOK {
			t.Errorf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("an active Mitra sees the voucher and the product", func(t *testing.T) {
		if rec := getWithToken(t, env.e, base+"/vouchers", token); !strings.Contains(rec.Body.String(), voucher) {
			t.Errorf("voucher missing: %s", rec.Body.String())
		}
		if rec := getWithToken(t, env.e, base+"/products", token); !strings.Contains(rec.Body.String(), product) {
			t.Errorf("product missing: %s", rec.Body.String())
		}
	})

	t.Run("a suspended Mitra sees neither, and the dashboard counts them as zero", func(t *testing.T) {
		suspended := seedProgramAccount(t, env.pool, "deactivated", "suspended", "", programs, "")
		sToken := env.tokenForUser(t, suspended, service.RoleAffiliator)
		for _, path := range []string{"/vouchers", "/products"} {
			rec := getWithToken(t, env.e, base+path, sToken)
			if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), voucher) || strings.Contains(rec.Body.String(), product) {
				t.Errorf("%s: want an empty 200, got %d: %s", path, rec.Code, rec.Body.String())
			}
		}
		body := decodeBody(t, getWithToken(t, env.e, base+"/dashboard", sToken))
		if body["vouchers_available"] != float64(0) || body["products_open"] != float64(0) {
			t.Errorf("dashboard = %v, want zero vouchers and products", body)
		}
	})

	t.Run("nothing is listed while the Mitra program is off", func(t *testing.T) {
		env.setPartnerFlag(t, "false")
		defer env.setPartnerFlag(t, "true")
		rec := getWithToken(t, env.e, base+"/vouchers", token)
		if rec.Code != http.StatusOK || strings.Contains(rec.Body.String(), voucher) {
			t.Errorf("want an empty 200, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestAdminPartnerReport(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")
	partner := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	token := env.tokenForUser(t, partner, service.RoleAffiliator)
	admin := env.tokenFor(t, service.RoleSuperAdmin)
	product := seedOpenCourseProduct(t, env, 100000)
	student := seedStudentRegisteredBy(t, env, partner)
	code := "RPT" + strings.ToUpper(uniqueHandlerSuffix()[:6])
	if rec := postJSONWithToken(t, env.e, "/api/v1/admin/promo-codes", admin,
		`{"code":"`+code+`","discount_percent":100,"partner_voucher":{"enabled":true,"quota_unit":"student"}}`); rec.Code != http.StatusCreated {
		t.Fatalf("create voucher: %d %s", rec.Code, rec.Body.String())
	}
	rec := postJSONWithToken(t, env.e, "/api/v1/partners/me/orders", token, `{"product_ids":["`+product+`"],"student_ids":["`+student+`"],"promo_code":"`+code+`"}`)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create order: %d %s", rec.Code, rec.Body.String())
	}
	orderID, _ := decodeBody(t, rec)["id"].(string)
	if rec := postJSONWithKey(t, env, "/api/v1/partners/me/orders/"+orderID+"/checkout", token, `{}`, "key-"+uniqueHandlerSuffix()); rec.Code != http.StatusOK {
		t.Fatalf("checkout: %d %s", rec.Code, rec.Body.String())
	}
	base := "/api/v1/admin/partner-reports/orders"

	t.Run("a purchase shows its Mitra, voucher and students, with totals", func(t *testing.T) {
		rec := getWithToken(t, env.e, base+"?partner_id="+partner, admin)
		body := decodeBody(t, rec)
		if rec.Code != http.StatusOK || body["total"] != float64(1) {
			t.Fatalf("want one order, got %d %v", rec.Code, body)
		}
		if !strings.Contains(rec.Body.String(), code) || !strings.Contains(rec.Body.String(), orderID) {
			t.Errorf("want the voucher and order in the row: %s", rec.Body.String())
		}
		rows, _ := body["data"].([]any)
		row, _ := rows[0].(map[string]any)
		if names, _ := row["student_list"].([]any); len(names) != 1 {
			t.Errorf("student_list = %v, want the one student named", row["student_list"])
		}
		summary, _ := body["summary"].(map[string]any)
		if summary["orders"] != float64(1) || summary["paid_orders"] != float64(1) || summary["seats"] != float64(1) {
			t.Errorf("summary = %v, want 1 paid order of 1 seat", summary)
		}
	})

	t.Run("search, status and another Mitra narrow it", func(t *testing.T) {
		for path, want := range map[string]float64{
			base + "?partner_id=" + partner + "&q=" + code:        1,
			base + "?partner_id=" + partner + "&status=cancelled": 0,
			base + "?partner_id=" + uuid.NewString():              0,
		} {
			if body := decodeBody(t, getWithToken(t, env.e, path, admin)); body["total"] != want {
				t.Errorf("%s: total = %v, want %v", path, body["total"], want)
			}
		}
	})

	t.Run("the export carries the same purchase as CSV", func(t *testing.T) {
		rec := getWithToken(t, env.e, base+"/export?partner_id="+partner, admin)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Header().Get("Content-Type"), "text/csv") {
			t.Fatalf("want a CSV, got %d %s", rec.Code, rec.Header().Get("Content-Type"))
		}
		if !strings.Contains(rec.Body.String(), "voucher_code") || !strings.Contains(rec.Body.String(), code) {
			t.Errorf("csv = %s", rec.Body.String())
		}
	})

	t.Run("only an admin with the affiliates capability may read it", func(t *testing.T) {
		if rec := getWithToken(t, env.e, base, token); rec.Code != http.StatusForbidden && rec.Code != http.StatusUnauthorized {
			t.Errorf("a Mitra: want 403, got %d", rec.Code)
		}
		if rec := getWithToken(t, env.e, base+"?from=nope", admin); rec.Code != http.StatusBadRequest {
			t.Errorf("bad date: want 400, got %d", rec.Code)
		}
	})
}
