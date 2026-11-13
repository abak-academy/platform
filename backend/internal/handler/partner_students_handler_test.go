package handler_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"akademi-bimbel/internal/service"
)

func TestPartnerStudentRoutes(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")
	partner := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
	token := env.tokenForUser(t, partner, service.RoleAffiliator)

	register := func(body string) (int, map[string]any) {
		rec := postJSONWithToken(t, env.e, "/api/v1/partners/me/students", token, body)
		var decoded map[string]any
		if rec.Body.Len() > 0 {
			decoded = decodeBody(t, rec)
		}
		return rec.Code, decoded
	}

	t.Run("a registered student comes back with a temporary password and the Mitra as registrar", func(t *testing.T) {
		code, body := register(`{"name":"Rina Handler","jenjang":"sma"}`)
		if code != http.StatusCreated {
			t.Fatalf("want 201, got %d: %v", code, body)
		}
		if body["temp_password"] == nil || body["temp_password"] == "" {
			t.Errorf("temp_password missing: %v", body)
		}
		var registeredBy string
		if err := env.pool.QueryRow(context.Background(),
			`SELECT registered_by::text FROM users WHERE id = $1`, body["id"]).Scan(&registeredBy); err != nil {
			t.Fatalf("read registered_by: %v", err)
		}
		if registeredBy != partner {
			t.Errorf("registered_by = %s, want %s", registeredBy, partner)
		}
	})

	t.Run("a Mitra cannot choose the password", func(t *testing.T) {
		code, _ := register(`{"name":"Sari Handler","jenjang":"sma","password":"mine12345"}`)
		if code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", code)
		}
	})

	t.Run("name and jenjang are required", func(t *testing.T) {
		code, _ := register(`{"name":"Tanpa Jenjang"}`)
		if code != http.StatusBadRequest {
			t.Errorf("want 400, got %d", code)
		}
	})

	t.Run("the list shows only the safe columns", func(t *testing.T) {
		rec := getWithToken(t, env.e, "/api/v1/partners/me/students?registered_by_me=true", token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		body := rec.Body.String()
		if !strings.Contains(body, "Rina Handler") {
			t.Fatalf("the student just registered is missing: %s", body)
		}
		for _, leaked := range []string{`"email"`, `"phone"`, `"dob"`, `"alamat_domisili"`, `"password`, `"temp_password"`} {
			if strings.Contains(body, leaked) {
				t.Errorf("the list leaks %s: %s", leaked, body)
			}
		}
		for _, want := range []string{`"activated":false`, `"registered_by_me":true`, `"school_name"`, `"username"`} {
			if !strings.Contains(body, want) {
				t.Errorf("the list lacks %s: %s", want, body)
			}
		}
	})

	t.Run("the list flags a student who already owns a product named in product_ids", func(t *testing.T) {
		productID := seedOpenCourseProduct(t, env, 100000)
		var courseID string
		if err := env.pool.QueryRow(context.Background(), `SELECT course_id::text FROM product_course WHERE product_id = $1`, productID).Scan(&courseID); err != nil {
			t.Fatalf("read course id: %v", err)
		}
		_, body := register(`{"name":"Wati Handler","jenjang":"sma"}`)
		studentID := body["id"].(string)
		if _, err := env.pool.Exec(context.Background(),
			`INSERT INTO course_session (student_id, course_id, status, source) VALUES ($1, $2, 'active', 'order')`, studentID, courseID); err != nil {
			t.Fatalf("enroll student: %v", err)
		}

		rec := getWithToken(t, env.e, "/api/v1/partners/me/students?registered_by_me=true&product_ids="+productID, token)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		resp := decodeBody(t, rec)
		rows, _ := resp["data"].([]any)
		var found bool
		for _, raw := range rows {
			row, _ := raw.(map[string]any)
			if row["id"] != studentID {
				continue
			}
			found = true
			owned, _ := row["owned_product_ids"].([]any)
			if len(owned) != 1 || owned[0] != productID {
				t.Errorf("owned_product_ids = %v, want [%s]", row["owned_product_ids"], productID)
			}
		}
		if !found {
			t.Fatalf("the enrolled student is missing from the list: %s", rec.Body.String())
		}

		// Without product_ids, the field is absent (omitempty), not an empty list.
		rec = getWithToken(t, env.e, "/api/v1/partners/me/students?registered_by_me=true", token)
		if strings.Contains(rec.Body.String(), "owned_product_ids") {
			t.Errorf("owned_product_ids must not appear when product_ids was not asked for: %s", rec.Body.String())
		}
	})

	t.Run("bad filters are a bad request", func(t *testing.T) {
		for _, path := range []string{
			"/api/v1/partners/me/students?status=deleted",
			"/api/v1/partners/me/students?registered_by_me=maybe",
		} {
			if rec := getWithToken(t, env.e, path, token); rec.Code != http.StatusBadRequest {
				t.Errorf("GET %s: want 400, got %d", path, rec.Code)
			}
		}
	})

	t.Run("credentials can be reissued only until the student activates", func(t *testing.T) {
		_, body := register(`{"name":"Tono Handler","jenjang":"sma"}`)
		id := body["id"].(string)
		path := "/api/v1/partners/me/students/" + id + "/credentials"

		rec := postJSONWithToken(t, env.e, path, token, `{}`)
		if rec.Code != http.StatusOK {
			t.Fatalf("want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		if decodeBody(t, rec)["temp_password"] == nil {
			t.Errorf("temp_password missing: %s", rec.Body.String())
		}

		if _, err := env.pool.Exec(context.Background(), `UPDATE users SET password_temporary = false WHERE id = $1`, id); err != nil {
			t.Fatalf("activate student: %v", err)
		}
		rec = postJSONWithToken(t, env.e, path, token, `{}`)
		if rec.Code != http.StatusConflict || decodeBody(t, rec)["code"] != "student_already_activated" {
			t.Errorf("after activation: want 409 student_already_activated, got %d %s", rec.Code, rec.Body.String())
		}

		rec = postJSONWithToken(t, env.e, "/api/v1/partners/me/students/not-a-uuid/credentials", token, `{}`)
		if rec.Code == http.StatusOK {
			t.Errorf("a bad id must not succeed")
		}
	})

	t.Run("another Mitra's student is not found", func(t *testing.T) {
		_, body := register(`{"name":"Umar Handler","jenjang":"sma"}`)
		otherMitra := seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "")
		otherToken := env.tokenForUser(t, otherMitra, service.RoleAffiliator)
		rec := postJSONWithToken(t, env.e, "/api/v1/partners/me/students/"+body["id"].(string)+"/credentials", otherToken, `{}`)
		if rec.Code != http.StatusNotFound {
			t.Errorf("want 404, got %d: %s", rec.Code, rec.Body.String())
		}
	})

	t.Run("a bulk import can only start from the Mitra's own upload folder", func(t *testing.T) {
		rec := postJSONWithToken(t, env.e, "/api/v1/partners/me/students/bulk", token, `{"file_key":""}`)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("an empty file_key: want 400, got %d", rec.Code)
		}
		foreign := `{"file_key":"student-bulk/` + seedProgramAccount(t, env.pool, "active", "active", "", partnerOnlyJSON, "") + `/stolen.csv"}`
		rec = postJSONWithToken(t, env.e, "/api/v1/partners/me/students/bulk", token, foreign)
		if rec.Code == http.StatusAccepted {
			t.Errorf("a file from another folder must not be accepted: %s", rec.Body.String())
		}
	})
}

func TestPartnerStudentRoutes_followTheGate(t *testing.T) {
	env := newAffiliateJobsEnv(t)
	env.setPartnerFlag(t, "true")

	t.Run("an account outside the Mitra program is refused", func(t *testing.T) {
		plain := seedProgramAccount(t, env.pool, "active", "active", "AFS"+uniqueHandlerSuffix()[:5], "", "")
		token := env.tokenForUser(t, plain, service.RoleAffiliator)
		rec := getWithToken(t, env.e, "/api/v1/partners/me/students", token)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "program_not_enrolled" {
			t.Errorf("want 403 program_not_enrolled, got %d %s", rec.Code, rec.Body.String())
		}
		rec = postJSONWithToken(t, env.e, "/api/v1/partners/me/students", token, `{"name":"X","jenjang":"sma"}`)
		if rec.Code != http.StatusForbidden {
			t.Errorf("registering: want 403, got %d", rec.Code)
		}
	})

	t.Run("a suspended Mitra can read the list but not register", func(t *testing.T) {
		suspended := seedProgramAccount(t, env.pool, "deactivated", "suspended", "", partnerOnlyJSON, "")
		token := env.tokenForUser(t, suspended, service.RoleAffiliator)
		if rec := getWithToken(t, env.e, "/api/v1/partners/me/students", token); rec.Code != http.StatusOK {
			t.Errorf("list: want 200, got %d: %s", rec.Code, rec.Body.String())
		}
		rec := postJSONWithToken(t, env.e, "/api/v1/partners/me/students", token, `{"name":"X","jenjang":"sma"}`)
		if rec.Code != http.StatusForbidden || decodeBody(t, rec)["code"] != "account_deactivated" {
			t.Errorf("register: want 403 account_deactivated, got %d %s", rec.Code, rec.Body.String())
		}
	})
}
