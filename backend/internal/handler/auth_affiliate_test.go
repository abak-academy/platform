package handler_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// The referral fields are optional additions to the verify and Google
// requests: they must never turn a valid activation into a 4xx, even when the
// affiliate feature is not configured (service runs with no store repo).
func TestVerifyOTPHandler_AcceptsReferralFields(t *testing.T) {
	env := newTestEnv(t)

	regRec := postJSON(t, env.e, "/api/v1/auth/register", map[string]string{
		"email":    "aff-verify@example.com",
		"password": "password123",
		"name":     "Aff Verify",
	})
	if regRec.Code != http.StatusCreated {
		t.Fatalf("want 201, got %d body=%s", regRec.Code, regRec.Body.String())
	}
	var regResp map[string]any
	if err := json.NewDecoder(regRec.Body).Decode(&regResp); err != nil {
		t.Fatalf("decode register response: %v", err)
	}
	pendingToken, _ := regResp["pending_token"].(string)
	if pendingToken == "" {
		t.Fatal("want pending_token")
	}
	if len(env.otpCodes.Codes) == 0 {
		t.Fatal("no OTP delivered to the recording provider")
	}

	rec := postJSON(t, env.e, "/api/v1/auth/otp/verify", map[string]string{
		"pending_token":        pendingToken,
		"code":                 env.otpCodes.Codes[0],
		"referral_code":        "SOMECODE",
		"referral_code_cookie": "COOKIECODE",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with referral fields, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestGoogleLoginHandler_AcceptsReferralFieldsOnRegistration(t *testing.T) {
	tokenInfo := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"aud":            "handler-google-client",
			"email":          "aff-google@example.com",
			"email_verified": "true",
			"name":           "Aff Google",
		})
	}))
	t.Cleanup(tokenInfo.Close)

	originalTransport := http.DefaultClient.Transport
	http.DefaultClient.Transport = googleTokenInfoTransport{fakeURL: tokenInfo.URL}
	t.Cleanup(func() { http.DefaultClient.Transport = originalTransport })

	env := newTestEnv(t)

	rec := postJSON(t, env.e, "/api/v1/auth/google", map[string]string{
		"id_token":             "google-id-token",
		"referral_code":        "SOMECODE",
		"referral_code_cookie": "COOKIECODE",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("want 200 with referral fields, got %d body=%s", rec.Code, rec.Body.String())
	}

	var resp map[string]any
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	user, ok := resp["user"].(map[string]any)
	if !ok {
		t.Fatalf("want user object, got %T", resp["user"])
	}
	if got := user["auth_provider"]; got != "google" {
		t.Errorf("auth_provider: want google, got %v", got)
	}
}
