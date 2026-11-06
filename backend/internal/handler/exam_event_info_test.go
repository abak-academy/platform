package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"

	"akademi-bimbel/internal/service"
)

type eventInfoBody struct {
	ID                  string  `json:"id"`
	WhatsAppGroupURL    *string `json:"whatsapp_group_url"`
	AdditionalRulesNote *string `json:"additional_rules_note"`
	AdditionalRulesURL  *string `json:"additional_rules_url"`
}

func decodeEventInfo(t *testing.T, rec *httptest.ResponseRecorder) eventInfoBody {
	t.Helper()
	var out eventInfoBody
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v body=%s", err, rec.Body.String())
	}
	return out
}

func postExamJSON(t *testing.T, e *echo.Echo, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	b, _ := json.Marshal(body)
	return postJSONWithToken(t, e, path, token, string(b))
}

func eqStr(a *string, b string) bool { return a != nil && *a == b }

func TestExamEventInfo_createAndPatchSemantics(t *testing.T) {
	env := newStudentFlowEnv(t)
	schoolID := seedSchool(t, env.pool)
	adminID := seedUserWithSchool(t, env.pool, service.RoleSuperAdmin, "Event Admin", schoolID)
	token := env.token(t, adminID, service.RoleSuperAdmin)

	created := postExamJSON(t, env.e, "/api/v1/admin/exams", token, map[string]any{
		"title": "Event Info", "whatsapp_group_url": "  ", "additional_rules_note": "",
	})
	if created.Code != http.StatusCreated {
		t.Fatalf("create: want 201, got %d body=%s", created.Code, created.Body.String())
	}
	got := decodeEventInfo(t, created)
	if got.WhatsAppGroupURL != nil || got.AdditionalRulesNote != nil || got.AdditionalRulesURL != nil {
		t.Fatalf("blank/absent create = %+v, want all null", got)
	}
	path := "/api/v1/admin/exams/" + got.ID

	patch := func(body map[string]any) eventInfoBody {
		t.Helper()
		rec := patchJSONRequest(t, env.e, path, token, body)
		if rec.Code != http.StatusOK {
			t.Fatalf("patch %v: want 200, got %d body=%s", body, rec.Code, rec.Body.String())
		}
		return decodeEventInfo(t, rec)
	}

	got = patch(map[string]any{
		"whatsapp_group_url": "https://chat.whatsapp.com/Abc", "additional_rules_note": "Baca.\nSelesai.", "additional_rules_url": "https://example.com/p",
	})
	if !eqStr(got.WhatsAppGroupURL, "https://chat.whatsapp.com/Abc") || !eqStr(got.AdditionalRulesNote, "Baca.\nSelesai.") || !eqStr(got.AdditionalRulesURL, "https://example.com/p") {
		t.Fatalf("set = %+v", got)
	}

	got = patch(map[string]any{"title": "Renamed"})
	if !eqStr(got.WhatsAppGroupURL, "https://chat.whatsapp.com/Abc") || !eqStr(got.AdditionalRulesNote, "Baca.\nSelesai.") || !eqStr(got.AdditionalRulesURL, "https://example.com/p") {
		t.Fatalf("patching title changed event info: %+v", got)
	}

	got = patch(map[string]any{"additional_rules_note": nil, "additional_rules_url": ""})
	if got.AdditionalRulesNote != nil || got.AdditionalRulesURL != nil || !eqStr(got.WhatsAppGroupURL, "https://chat.whatsapp.com/Abc") {
		t.Fatalf("null/blank clear = %+v, want rules cleared and whatsapp kept", got)
	}

	got = patch(map[string]any{"whatsapp_group_url": ""})
	if got.WhatsAppGroupURL != nil {
		t.Fatalf("blank whatsapp should clear: %+v", got)
	}

	detail := getRequest(t, env.e, path, token)
	if detail.Code != http.StatusOK {
		t.Fatalf("detail: %d %s", detail.Code, detail.Body.String())
	}
	if d := decodeEventInfo(t, detail); d.WhatsAppGroupURL != nil {
		t.Fatalf("detail = %+v", d)
	}

	for _, bad := range []map[string]any{
		{"whatsapp_group_url": "https://example.com/x"},
		{"additional_rules_url": "javascript:alert(1)"},
	} {
		if rec := patchJSONRequest(t, env.e, path, token, bad); rec.Code != http.StatusUnprocessableEntity {
			t.Errorf("patch %v: want 422, got %d body=%s", bad, rec.Code, rec.Body.String())
		}
	}
	if rec := postExamJSON(t, env.e, "/api/v1/admin/exams", token, map[string]any{"title": "Bad", "whatsapp_group_url": "http://chat.whatsapp.com/x"}); rec.Code != http.StatusUnprocessableEntity {
		t.Errorf("create invalid: want 422, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestExamEventInfo_projections(t *testing.T) {
	env := newStudentFlowEnv(t)
	schoolID := seedSchool(t, env.pool)
	adminID := seedUserWithSchool(t, env.pool, service.RoleSuperAdmin, "Projection Admin", schoolID)
	studentID := seedUserWithSchool(t, env.pool, "student", "Projection Student", schoolID)
	other := seedUserWithSchool(t, env.pool, "student", "Projection Other", schoolID)
	examID := seedStudentFlowExam(t, env.pool)
	if _, err := env.pool.Exec(t.Context(),
		`UPDATE exam SET whatsapp_group_url = 'https://chat.whatsapp.com/Abc', additional_rules_note = 'Catatan', additional_rules_url = 'https://example.com/p' WHERE id = $1`, examID,
	); err != nil {
		t.Fatal(err)
	}
	regID := seedStudentFlowRegistration(t, env.pool, studentID, examID)
	adminToken := env.token(t, adminID, service.RoleSuperAdmin)

	check := func(label string, body []byte, root string) {
		t.Helper()
		var doc map[string]json.RawMessage
		if err := json.Unmarshal(body, &doc); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
		if root != "" {
			if err := json.Unmarshal(doc[root], &doc); err != nil {
				t.Fatalf("%s: %v", label, err)
			}
		}
		for key, want := range map[string]string{
			"whatsapp_group_url": "https://chat.whatsapp.com/Abc", "additional_rules_note": "Catatan", "additional_rules_url": "https://example.com/p",
		} {
			var got *string
			_ = json.Unmarshal(doc[key], &got)
			if !eqStr(got, want) {
				t.Errorf("%s: %s = %v, want %q", label, key, got, want)
			}
		}
		assertNoForbiddenKeys(t, label, body)
	}

	rec := getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String(), adminToken)
	check("admin detail", rec.Body.Bytes(), "")
	rec = getRequest(t, env.e, "/api/v1/admin/exams/"+examID.String()+"/preview", adminToken)
	check("admin preview", rec.Body.Bytes(), "")
	rec = getRequest(t, env.e, "/api/v1/exam/registrations/"+regID.String(), env.token(t, studentID, service.RoleStudent))
	check("student detail", rec.Body.Bytes(), "exam")

	rec = getRequest(t, env.e, "/api/v1/admin/exams", adminToken)
	var list struct {
		Data []eventInfoBody `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, it := range list.Data {
		if it.ID == examID.String() {
			found = true
			if !eqStr(it.WhatsAppGroupURL, "https://chat.whatsapp.com/Abc") {
				t.Errorf("list item = %+v", it)
			}
		}
	}
	if !found {
		t.Errorf("exam %s not in admin list: %s", examID, rec.Body.String())
	}

	rec = getRequest(t, env.e, "/api/v1/exam/registrations/"+regID.String(), env.token(t, other, service.RoleStudent))
	if rec.Code != http.StatusNotFound {
		t.Errorf("other student's registration: want 404, got %d", rec.Code)
	}
}
