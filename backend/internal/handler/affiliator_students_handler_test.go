package handler_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/service"

	"github.com/google/uuid"
)

type affiliatorStudentsFixture struct {
	env         *promoActiveTestEnv
	token       string
	affiliator  string
	ownSchool   string
	otherSchool string
	ownStudent  string
	foreignStud string
}

func newAffiliatorStudentsFixture(t *testing.T) affiliatorStudentsFixture {
	t.Helper()
	env := newPromoActiveDBEnv(t)
	ctx := context.Background()
	suffix := time.Now().Format("150405.000000")

	insertSchool := func(name string) string {
		var id string
		if err := env.pool.QueryRow(ctx,
			`INSERT INTO school (name, code, status) VALUES ($1, $2, 'active') RETURNING id`,
			name, "AFF"+strings.ReplaceAll(suffix, ".", "")+name,
		).Scan(&id); err != nil {
			t.Fatalf("insert school: %v", err)
		}
		return id
	}
	own, other := insertSchool("own"), insertSchool("other")

	var aff string
	if err := env.pool.QueryRow(ctx,
		`INSERT INTO users (email, role, name, school_ids) VALUES ($1, 'affiliator', 'Aff', ARRAY[$2]::uuid[]) RETURNING id`,
		"aff-"+suffix+"@test.local", own,
	).Scan(&aff); err != nil {
		t.Fatalf("insert affiliator: %v", err)
	}
	insertStudent := func(school string) string {
		var id string
		if err := env.pool.QueryRow(ctx,
			`INSERT INTO users (email, role, name, school_id, jenjang) VALUES ($1, 'student', 'Stu', $2, 'SMA') RETURNING id`,
			"stu-"+uuid.NewString()+"@test.local", school,
		).Scan(&id); err != nil {
			t.Fatalf("insert student: %v", err)
		}
		return id
	}
	return affiliatorStudentsFixture{
		env: env, token: promoActiveToken(t, env, aff, service.RoleAffiliator), affiliator: aff,
		ownSchool: own, otherSchool: other, ownStudent: insertStudent(own), foreignStud: insertStudent(other),
	}
}

func (f affiliatorStudentsFixture) do(t *testing.T, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/v1"+path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+f.token)
	rec := httptest.NewRecorder()
	f.env.e.ServeHTTP(rec, req)
	return rec
}

func studentIDs(t *testing.T, rec *httptest.ResponseRecorder) map[string]bool {
	t.Helper()
	var resp struct {
		Data []struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	ids := map[string]bool{}
	for _, s := range resp.Data {
		ids[s.ID] = true
	}
	return ids
}

func TestAffiliatorStudents_ListIsLimitedToAssignedSchools(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)

	rec := f.do(t, http.MethodGet, "/admin/students", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	ids := studentIDs(t, rec)
	if !ids[f.ownStudent] || ids[f.foreignStud] {
		t.Errorf("list ids = %v, want own student and not the foreign one", ids)
	}

	rec = f.do(t, http.MethodGet, "/admin/students?school_id="+f.otherSchool, "")
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "school_not_assigned") {
		t.Errorf("foreign school filter: want 403 school_not_assigned, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAffiliatorStudents_RegisterNeedsAnAssignedSchool(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)
	body := `{"name":"Budi","jenjang":"SMA"}`

	if rec := f.do(t, http.MethodPost, "/admin/students", body); rec.Code != http.StatusBadRequest {
		t.Errorf("no school_id: want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodPost, "/admin/students?school_id="+f.otherSchool, body); rec.Code != http.StatusForbidden {
		t.Errorf("foreign school: want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodPost, "/admin/students?school_id="+f.ownSchool, `{"name":"Budi","jenjang":"SMA","password":"secret123"}`); rec.Code != http.StatusForbidden {
		t.Errorf("manual password: want 403, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodPost, "/admin/students?school_id="+f.ownSchool, `{"name":"Budi","jenjang":"SMA","unlisted_school_name":"X"}`); rec.Code != http.StatusForbidden {
		t.Errorf("unlisted school: want 403, got %d body=%s", rec.Code, rec.Body.String())
	}

	rec := f.do(t, http.MethodPost, "/admin/students?school_id="+f.ownSchool, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("own school: want 201, got %d body=%s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &created)

	var registeredBy, schoolID string
	if err := f.env.pool.QueryRow(context.Background(),
		`SELECT registered_by::text, school_id::text FROM users WHERE id = $1`, created.ID,
	).Scan(&registeredBy, &schoolID); err != nil {
		t.Fatalf("read back: %v", err)
	}
	if registeredBy != f.affiliator || schoolID != f.ownSchool {
		t.Errorf("registered_by=%s school_id=%s, want %s and %s", registeredBy, schoolID, f.affiliator, f.ownSchool)
	}

	rec = f.do(t, http.MethodGet, "/admin/students?registered_by_me=true", "")
	ids := studentIDs(t, rec)
	if !ids[created.ID] || ids[f.ownStudent] {
		t.Errorf("registered_by_me ids = %v, want only the student registered by this affiliator", ids)
	}
}

func TestAffiliatorStudents_AccountActionsAreForbidden(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)
	for name, req := range map[string][2]string{
		"status":           {http.MethodPatch, "/admin/students/" + f.ownStudent + "?school_id=" + f.ownSchool},
		"credentials":      {http.MethodGet, "/admin/students/" + f.ownStudent + "/credentials?school_id=" + f.ownSchool},
		"bulk credentials": {http.MethodPost, "/admin/students/bulk/credentials?school_id=" + f.ownSchool},
		"set password":     {http.MethodPatch, "/admin/students/" + f.ownStudent + "/password"},
	} {
		if rec := f.do(t, req[0], req[1], `{"status":"deactivated"}`); rec.Code != http.StatusForbidden {
			t.Errorf("%s: want 403, got %d body=%s", name, rec.Code, rec.Body.String())
		}
	}
}

func TestAffiliatorStudents_DeactivatedAffiliatorCannotRegister(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)
	if _, err := f.env.pool.Exec(context.Background(), `UPDATE users SET status = 'deactivated' WHERE id = $1`, f.affiliator); err != nil {
		t.Fatalf("deactivate: %v", err)
	}

	rec := f.do(t, http.MethodPost, "/admin/students?school_id="+f.ownSchool, `{"name":"Budi","jenjang":"SMA"}`)
	if rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "account_deactivated") {
		t.Errorf("write while deactivated: want 403 account_deactivated, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodGet, "/admin/students", ""); rec.Code != http.StatusOK {
		t.Errorf("read while deactivated: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAffiliatorOrders_RoutesAreScopedAndGuarded(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)

	if rec := f.do(t, http.MethodGet, "/affiliates/me/products", ""); rec.Code != http.StatusOK {
		t.Errorf("products: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodPost, "/affiliates/me/orders/preview", `{"product_id":"`+uuid.NewString()+`"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("preview without school_id: want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	foreign := `{"school_id":"` + f.otherSchool + `","product_id":"` + uuid.NewString() + `","all":true}`
	if rec := f.do(t, http.MethodPost, "/affiliates/me/orders/preview", foreign); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "school_not_assigned") {
		t.Errorf("preview for a foreign school: want 403 school_not_assigned, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodPost, "/affiliates/me/orders/"+uuid.NewString()+"/checkout", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("checkout without Idempotency-Key: want 400, got %d body=%s", rec.Code, rec.Body.String())
	}

	var studentID string
	if err := f.env.pool.QueryRow(context.Background(),
		`INSERT INTO users (email, role, name) VALUES ($1, 'student', 'Stu') RETURNING id`,
		"stu-route-"+uuid.NewString()+"@test.local",
	).Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/affiliates/me/products", nil)
	req.Header.Set("Authorization", "Bearer "+promoActiveToken(t, f.env, studentID, service.RoleStudent))
	rec := httptest.NewRecorder()
	f.env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Errorf("student on affiliator route: want 403, got %d", rec.Code)
	}

	if _, err := f.env.pool.Exec(context.Background(), `UPDATE users SET status = 'deactivated' WHERE id = $1`, f.affiliator); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	own := `{"school_id":"` + f.ownSchool + `","product_id":"` + uuid.NewString() + `","all":true}`
	if rec := f.do(t, http.MethodPost, "/affiliates/me/orders", own); rec.Code != http.StatusForbidden || !strings.Contains(rec.Body.String(), "account_deactivated") {
		t.Errorf("order while deactivated: want 403 account_deactivated, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodGet, "/affiliates/me/products", ""); rec.Code != http.StatusOK {
		t.Errorf("read while deactivated: want 200, got %d", rec.Code)
	}
}

func TestAffiliatorDashboard_RoutesAndMonitoringAccess(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)

	rec := f.do(t, http.MethodGet, "/affiliates/me/dashboard", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("dashboard: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var dash struct {
		Schools []struct {
			SchoolID     string `json:"school_id"`
			StudentCount int    `json:"student_count"`
		} `json:"schools"`
		UnpurchasedStudents int `json:"unpurchased_students"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &dash); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(dash.Schools) != 1 || dash.Schools[0].SchoolID != f.ownSchool || dash.Schools[0].StudentCount != 1 || dash.UnpurchasedStudents != 1 {
		t.Errorf("dashboard = %+v, want only the assigned school with its one student", dash)
	}
	if rec := f.do(t, http.MethodGet, "/affiliates/me/dashboard?from=nope", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("bad date: want 400, got %d", rec.Code)
	}
	if rec := f.do(t, http.MethodGet, "/affiliates/me/orders", ""); rec.Code != http.StatusOK {
		t.Errorf("orders: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}

	if rec := f.do(t, http.MethodGet, "/admin/system/affiliators/"+f.affiliator, ""); rec.Code != http.StatusForbidden {
		t.Errorf("affiliator reading the monitoring detail: want 403, got %d", rec.Code)
	}

	superToken := promoActiveToken(t, f.env, uuid.NewString(), service.RoleSuperAdmin)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/system/affiliators/"+f.affiliator, nil)
	req.Header.Set("Authorization", "Bearer "+superToken)
	rec = httptest.NewRecorder()
	f.env.e.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), f.ownSchool) {
		t.Errorf("super admin detail: want 200 with the school, got %d body=%s", rec.Code, rec.Body.String())
	}
}
