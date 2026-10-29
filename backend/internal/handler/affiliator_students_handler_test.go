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
	if rec := f.do(t, http.MethodPost, "/affiliates/me/orders/preview", `{"student_ids":["`+uuid.NewString()+`"]}`); rec.Code != http.StatusBadRequest {
		t.Errorf("preview without product_id: want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	var foreignStudent string
	if err := f.env.pool.QueryRow(context.Background(),
		`INSERT INTO users (email, role, name, school_id) VALUES ($1, 'student', 'Foreign', $2) RETURNING id`,
		"stu-foreign-"+uuid.NewString()+"@test.local", f.otherSchool,
	).Scan(&foreignStudent); err != nil {
		t.Fatalf("insert foreign student: %v", err)
	}
	foreign := `{"product_id":"` + uuid.NewString() + `","student_ids":["` + foreignStudent + `"]}`
	if rec := f.do(t, http.MethodPost, "/affiliates/me/orders/preview", foreign); rec.Code != http.StatusForbidden {
		t.Errorf("preview for a student of a foreign school: want 403, got %d body=%s", rec.Code, rec.Body.String())
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
	own := `{"product_id":"` + uuid.NewString() + `","student_ids":["` + uuid.NewString() + `"]}`
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
	if rec := f.do(t, http.MethodGet, "/affiliates/me/orders/students?q=a&limit=5", ""); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total":0`) {
		t.Errorf("purchase history: want 200 with total 0, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := f.do(t, http.MethodGet, "/affiliates/me/orders/students?school_id=nope", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("purchase history bad school id: want 400, got %d body=%s", rec.Code, rec.Body.String())
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

func TestAffiliatorListing_RoutesAreSuperAdminOnlyAndPaginated(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)
	super := promoActiveToken(t, f.env, uuid.NewString(), service.RoleSuperAdmin)
	getAs := func(token, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.env.e.ServeHTTP(rec, req)
		return rec
	}

	for _, path := range []string{"/admin/system/affiliators", "/admin/system/affiliators/promotable"} {
		if rec := getAs(f.token, path); rec.Code != http.StatusForbidden {
			t.Errorf("affiliator on %s: want 403, got %d", path, rec.Code)
		}
	}

	rec := getAs(super, "/admin/system/affiliators?limit=1&q=Aff")
	if rec.Code != http.StatusOK {
		t.Fatalf("list: want 200, got %d body=%s", rec.Code, rec.Body.String())
	}
	var list struct {
		Data       []struct{ ID string } `json:"data"`
		NextCursor string                `json:"next_cursor"`
		Total      int                   `json:"total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(list.Data) != 1 || list.Total < 1 {
		t.Errorf("list = %+v, want one row and a total", list)
	}

	if rec := getAs(super, "/admin/system/affiliators/promotable?limit=5"); rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), `"total"`) {
		t.Errorf("promotable: want 200 with a total, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAffiliatorBulkRoutes_AreSuperAdminOnlyAndValidateInput(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)
	super := promoActiveToken(t, f.env, uuid.NewString(), service.RoleSuperAdmin)
	post := func(token, path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/api/v1"+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.env.e.ServeHTTP(rec, req)
		return rec
	}

	for _, path := range []string{"/admin/system/affiliators/bulk/presign?filename=a.csv", "/admin/system/affiliators/bulk"} {
		if rec := post(f.token, path, `{"file_key":"affiliator-bulk/x.csv"}`); rec.Code != http.StatusForbidden {
			t.Errorf("affiliator on %s: want 403, got %d", path, rec.Code)
		}
	}
	if rec := post(super, "/admin/system/affiliators/bulk/presign", ""); rec.Code != http.StatusBadRequest {
		t.Errorf("presign without filename: want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := post(super, "/admin/system/affiliators/bulk", `{}`); rec.Code != http.StatusBadRequest {
		t.Errorf("enqueue without file_key: want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestAffiliateReportRoutes_AreSuperAdminOnlyAndValidateInput(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)
	super := promoActiveToken(t, f.env, uuid.NewString(), service.RoleSuperAdmin)
	get := func(token, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.env.e.ServeHTTP(rec, req)
		return rec
	}

	for _, name := range service.AffiliateReportNames {
		if rec := get(f.token, "/admin/system/affiliate-reports/"+name); rec.Code != http.StatusForbidden {
			t.Errorf("affiliator on report %s: want 403, got %d", name, rec.Code)
		}
		if rec := get(f.token, "/admin/system/affiliate-reports/"+name+"/export"); rec.Code != http.StatusForbidden {
			t.Errorf("affiliator on export %s: want 403, got %d", name, rec.Code)
		}
		if rec := get(super, "/admin/system/affiliate-reports/"+name+"?from=2019-03-01&to=2019-03-31"); rec.Code != http.StatusOK {
			t.Errorf("super admin on report %s: want 200, got %d body=%s", name, rec.Code, rec.Body.String())
		}
	}

	rec := get(super, "/admin/system/affiliate-reports/affiliators?limit=5&offset=0")
	var page struct {
		Data   []any `json:"data"`
		Total  int   `json:"total"`
		Limit  int   `json:"limit"`
		Offset int   `json:"offset"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || page.Limit != 5 || page.Total < 1 {
		t.Errorf("page = %+v err=%v, want the paging echoed and a total", page, err)
	}

	if rec := get(super, "/admin/system/affiliate-reports/schools?from=nope"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad date: want 400, got %d", rec.Code)
	}
	if rec := get(super, "/admin/system/affiliate-reports/schools?from=2019-04-02&to=2019-04-01"); rec.Code != http.StatusBadRequest {
		t.Errorf("reversed range: want 400, got %d", rec.Code)
	}
	if rec := get(super, "/admin/system/affiliate-reports/nope/export"); rec.Code != http.StatusBadRequest {
		t.Errorf("unknown export: want 400, got %d", rec.Code)
	}

	rec = get(super, "/admin/system/affiliate-reports/promos/export?from=2019-03-01&to=2019-03-31")
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), `laporan-affiliator-promos-2019-03-01_2019-03-31.csv`) {
		t.Errorf("export = %d %q %q, want a named CSV attachment", rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Content-Disposition"))
	}
}

func TestAffiliatorSchoolListRoutes_AreSuperAdminOnlyAndValidateInput(t *testing.T) {
	f := newAffiliatorStudentsFixture(t)
	super := promoActiveToken(t, f.env, uuid.NewString(), service.RoleSuperAdmin)
	get := func(token, path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, "/api/v1"+path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		rec := httptest.NewRecorder()
		f.env.e.ServeHTTP(rec, req)
		return rec
	}

	for _, path := range []string{"/admin/system/affiliators/schools", "/admin/system/affiliators/schools/export"} {
		if rec := get(f.token, path); rec.Code != http.StatusForbidden {
			t.Errorf("affiliator on %s: want 403, got %d", path, rec.Code)
		}
	}

	rec := get(super, "/admin/system/affiliators/schools?limit=5&affiliator_id="+f.affiliator)
	var page struct {
		Data []struct {
			SchoolID string `json:"school_id"`
		} `json:"data"`
		Total int `json:"total"`
		Limit int `json:"limit"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("list: %d %v body=%s", rec.Code, err, rec.Body.String())
	}
	if page.Limit != 5 || page.Total != 1 || len(page.Data) != 1 || page.Data[0].SchoolID != f.ownSchool {
		t.Errorf("page = %+v, want the one assigned school of the fixture affiliator", page)
	}

	if rec := get(super, "/admin/system/affiliators/schools?school_id=nope"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad school id: want 400, got %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := get(super, "/admin/system/affiliators/schools/export?affiliator_id=nope"); rec.Code != http.StatusBadRequest {
		t.Errorf("bad affiliator id on export: want 400, got %d", rec.Code)
	}

	rec = get(super, "/admin/system/affiliators/schools/export?affiliator_id="+f.affiliator)
	if rec.Code != http.StatusOK || !strings.HasPrefix(rec.Header().Get("Content-Type"), "text/csv") ||
		!strings.Contains(rec.Header().Get("Content-Disposition"), "sekolah-affiliator.csv") || !strings.Contains(rec.Body.String(), "kode sekolah") {
		t.Errorf("export = %d %q %q, want a CSV attachment", rec.Code, rec.Header().Get("Content-Type"), rec.Header().Get("Content-Disposition"))
	}
}
