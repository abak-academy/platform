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

func TestPartnerVoucherRoutes(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")
	admin := env.tokenFor(t, service.RoleSuperAdmin)
	mitra := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	mitraToken := env.tokenForUser(t, mitra, service.RoleAffiliator)
	code := "MITRAH" + strings.ToUpper(uniqueHandlerSuffix()[:6])

	create := func(body string) *httptest.ResponseRecorder {
		return postJSONWithToken(t, env.e, "/api/v1/admin/promo-codes", admin, body)
	}

	t.Run("the promo admin creates a Mitra voucher with a quota unit", func(t *testing.T) {
		rec := create(`{"code":"` + code + `","discount_percent":20,"is_public":true,"partner_voucher":{"enabled":true,"ids":["` + mitra + `"],"quota_unit":"student"}}`)
		if rec.Code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %s", rec.Code, rec.Body.String())
		}
		body := decodeBody(t, rec)
		partner, _ := body["partner"].(map[string]any)
		if partner["quota_unit"] != "student" {
			t.Errorf("partner = %v, want quota_unit student", partner)
		}
		if body["is_public"] != false {
			t.Errorf("is_public = %v, want false for a Mitra voucher", body["is_public"])
		}
	})

	t.Run("a quota unit is required", func(t *testing.T) {
		rec := create(`{"code":"NOUNIT` + uniqueHandlerSuffix()[:6] + `","discount_percent":5,"partner_voucher":{"enabled":true}}`)
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("want a validation error, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("the Mitra sees it, a student cannot use it, the public list hides it", func(t *testing.T) {
		rec := getWithToken(t, env.e, "/api/v1/partners/me/vouchers", mitraToken)
		if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), code) {
			t.Fatalf("GET /partners/me/vouchers: want 200 with %s, got %d: %s", code, rec.Code, rec.Body.String())
		}

		rec = postJSONWithToken(t, env.e, "/api/v1/promo-codes/validate", "", `{"code":"`+code+`","subtotal":100000}`)
		if rec.Code == http.StatusOK {
			t.Errorf("a student validated a Mitra voucher: %s", rec.Body.String())
		}

		rec = getWithToken(t, env.e, "/api/v1/promo-codes/active", env.tokenFor(t, service.RoleStudent))
		if strings.Contains(rec.Body.String(), code) {
			t.Errorf("the public list shows a Mitra voucher: %s", rec.Body.String())
		}
	})

	t.Run("an update without the setting keeps the voucher a Mitra voucher", func(t *testing.T) {
		var id string
		if err := env.pool.QueryRow(context.Background(), `SELECT id::text FROM promo_code WHERE code = $1`, code).Scan(&id); err != nil {
			t.Fatalf("find promo: %v", err)
		}
		rec := putJSONWithToken(t, env.e, "/api/v1/admin/promo-codes/"+id, admin, `{"max_uses":9,"is_public":true}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		var stillPartner bool
		if err := env.pool.QueryRow(context.Background(), `SELECT partner IS NOT NULL FROM promo_code WHERE id = $1`, id).Scan(&stillPartner); err != nil {
			t.Fatalf("read promo: %v", err)
		}
		if !stillPartner {
			t.Error("a PUT from a client that does not know the setting turned the voucher into a public promo")
		}
	})

	t.Run("an account outside the Mitra program gets nothing", func(t *testing.T) {
		plain := seedProgramAccount(t, env.pool, "active", "active", "AFX"+uniqueHandlerSuffix()[:5], "", "")
		rec := getWithToken(t, env.e, "/api/v1/partners/me/vouchers", env.tokenForUser(t, plain, service.RoleAffiliator))
		if rec.Code != http.StatusForbidden {
			t.Errorf("want 403, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}

func TestProductPartnerAccessRoutes(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")
	admin := env.tokenFor(t, service.RoleSuperAdmin)
	mitra := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	other := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	var productID string
	if err := env.pool.QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, status) VALUES ('course', $1, 100000, 'published') RETURNING id::text`,
		"Kelas Mitra "+uniqueHandlerSuffix()).Scan(&productID); err != nil {
		t.Fatalf("seed product: %v", err)
	}
	path := "/api/v1/admin/products/" + productID + "/partner-access"

	t.Run("only an admin with the affiliate permission can read or set it", func(t *testing.T) {
		store := env.tokenFor(t, service.RoleAdminStore)
		if rec := getWithToken(t, env.e, path, store); rec.Code != http.StatusForbidden {
			t.Errorf("GET as admin_store: want 403, got %d", rec.Code)
		}
		if rec := putJSONWithToken(t, env.e, path, store, `{"scope":"all"}`); rec.Code != http.StatusForbidden {
			t.Errorf("PUT as admin_store: want 403, got %d", rec.Code)
		}
	})

	t.Run("a new product is closed", func(t *testing.T) {
		rec := getWithToken(t, env.e, path, admin)
		if rec.Code != http.StatusOK || decodeBody(t, rec)["scope"] != "none" {
			t.Errorf("want scope none, got %d %s", rec.Code, rec.Body.String())
		}
		if rec := getWithToken(t, env.e, "/api/v1/partners/me/products", env.tokenForUser(t, mitra, service.RoleAffiliator)); strings.Contains(rec.Body.String(), productID) {
			t.Errorf("a closed product is listed to a Mitra")
		}
	})

	t.Run("selected shows it to the chosen Mitra only", func(t *testing.T) {
		rec := putJSONWithToken(t, env.e, path, admin, `{"scope":"selected","ids":["`+mitra+`"]}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		chosen := getWithToken(t, env.e, "/api/v1/partners/me/products", env.tokenForUser(t, mitra, service.RoleAffiliator))
		if !strings.Contains(chosen.Body.String(), productID) {
			t.Errorf("the chosen Mitra does not see it: %s", chosen.Body.String())
		}
		notChosen := getWithToken(t, env.e, "/api/v1/partners/me/products", env.tokenForUser(t, other, service.RoleAffiliator))
		if strings.Contains(notChosen.Body.String(), productID) {
			t.Errorf("another Mitra sees it")
		}
	})

	t.Run("bad input", func(t *testing.T) {
		if rec := putJSONWithToken(t, env.e, path, admin, `{"scope":"selected"}`); rec.Code == http.StatusOK {
			t.Errorf("selected without a Mitra was accepted")
		}
		unknown := "/api/v1/admin/products/" + uuid.NewString() + "/partner-access"
		if rec := putJSONWithToken(t, env.e, unknown, admin, `{"scope":"all"}`); rec.Code != http.StatusNotFound {
			t.Errorf("an unknown product: want 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})
}
