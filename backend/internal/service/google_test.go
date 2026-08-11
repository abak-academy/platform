package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"akademi-bimbel/internal/model"
)

func TestGoogleLogin_CreateSetsGoogleProvider(t *testing.T) {
	// Fake Google tokeninfo endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleTokenInfo{
			Aud:           "google-client-id",
			Email:         "google-user@example.com",
			EmailVerified: "true",
			Name:          "Google User",
		})
	}))
	defer ts.Close()

	// Swap transport so calls to googleapis go to our fake server.
	orig := http.DefaultClient.Transport
	http.DefaultClient.Transport = &googleTokenInfoTransport{fakeURL: ts.URL}
	defer func() { http.DefaultClient.Transport = orig }()

	repo := newFakeUserRepo()
	svc, _ := newTestService(t, repo)

	access, _, err := svc.GoogleLogin(context.Background(), "fake-id-token")
	if err != nil {
		t.Fatalf("GoogleLogin: %v", err)
	}
	if access == "" {
		t.Fatal("empty access token")
	}

	u, _ := repo.GetUserByEmail(context.Background(), "google-user@example.com")
	if u == nil {
		t.Fatal("user not created")
	}
	if u.Role != RoleStudent {
		t.Errorf("Role: want RoleStudent, got '%s'", u.Role)
	}
	if u.AuthProvider != "google" {
		t.Errorf("AuthProvider: want 'google', got '%s'", u.AuthProvider)
	}
	if u.Username == nil {
		t.Fatal("Username should be set on Google-created user (FR-SELFREG-02)")
	}
	if !strings.HasPrefix(*u.Username, "goog") {
		t.Errorf("Username %q: want prefix 'goog'", *u.Username)
	}
	if len(*u.Username) != 8 {
		t.Errorf("Username %q: want 8 chars (4 base + 4 digits), got %d", *u.Username, len(*u.Username))
	}
}

func TestGoogleLogin_ExistingUserKeepsOriginalProvider(t *testing.T) {
	// Fake Google tokeninfo endpoint.
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleTokenInfo{
			Aud:           "google-client-id",
			Email:         "existing@example.com",
			EmailVerified: "true",
			Name:          "Existing User",
		})
	}))
	defer ts.Close()

	orig := http.DefaultClient.Transport
	http.DefaultClient.Transport = &googleTokenInfoTransport{fakeURL: ts.URL}
	defer func() { http.DefaultClient.Transport = orig }()

	repo := newFakeUserRepo()
	repo.seed(&model.User{
		Email:        strptr("existing@example.com"),
		PasswordHash: mustHashStd("password123"),
		Role:         RoleStudent,
		Status:       "active",
		AuthProvider: "password",
	})
	svc, _ := newTestService(t, repo)

	_, _, err := svc.GoogleLogin(context.Background(), "fake-id-token")
	if err != nil {
		t.Fatalf("GoogleLogin: %v", err)
	}

	u, _ := repo.GetUserByEmail(context.Background(), "existing@example.com")
	if u == nil {
		t.Fatal("user not found")
	}
	if u.AuthProvider != "password" {
		t.Errorf("AuthProvider: want 'password' (unchanged), got '%s'", u.AuthProvider)
	}
}

// TestGoogleLogin_ExistingNonStudentRefused covers FR-2/FR-3: Google sign-in
// is student-only, and the role check runs before the status check so a
// deactivated non-student still gets the role refusal, not account_deactivated.
func TestGoogleLogin_ExistingNonStudentRefused(t *testing.T) {
	cases := []struct {
		name   string
		role   string
		status string
	}{
		{"super_admin", RoleSuperAdmin, "active"},
		{"admin_school", RoleAdminSchool, "active"},
		{"deactivated admin_store", RoleAdminStore, "deactivated"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				json.NewEncoder(w).Encode(googleTokenInfo{
					Aud:           "google-client-id",
					Email:         "admin@example.com",
					EmailVerified: "true",
					Name:          "Admin User",
				})
			}))
			defer ts.Close()

			orig := http.DefaultClient.Transport
			http.DefaultClient.Transport = &googleTokenInfoTransport{fakeURL: ts.URL}
			defer func() { http.DefaultClient.Transport = orig }()

			repo := newFakeUserRepo()
			repo.seed(&model.User{
				Email:        strptr("admin@example.com"),
				PasswordHash: mustHashStd("password123"),
				Role:         tc.role,
				Status:       tc.status,
				AuthProvider: "password",
			})
			svc, _ := newTestService(t, repo)

			access, _, err := svc.GoogleLogin(context.Background(), "fake-id-token")
			if !errors.Is(err, ErrGoogleNotStudent) {
				t.Fatalf("want ErrGoogleNotStudent, got %v", err)
			}
			if access != "" {
				t.Errorf("want empty access token, got %q", access)
			}

			u, _ := repo.GetUserByEmail(context.Background(), "admin@example.com")
			if u == nil {
				t.Fatal("user should still exist")
			}
			if u.Role != tc.role {
				t.Errorf("Role: want unchanged %q, got %q", tc.role, u.Role)
			}
		})
	}
}

// TestGoogleLogin_ExistingDeactivatedStudent covers FR-4: a deactivated
// student is still refused with ErrAccountDeactivated (unaffected by FR-2/FR-3).
func TestGoogleLogin_ExistingDeactivatedStudent(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(googleTokenInfo{
			Aud:           "google-client-id",
			Email:         "student@example.com",
			EmailVerified: "true",
			Name:          "Student User",
		})
	}))
	defer ts.Close()

	orig := http.DefaultClient.Transport
	http.DefaultClient.Transport = &googleTokenInfoTransport{fakeURL: ts.URL}
	defer func() { http.DefaultClient.Transport = orig }()

	repo := newFakeUserRepo()
	repo.seed(&model.User{
		Email:        strptr("student@example.com"),
		PasswordHash: mustHashStd("password123"),
		Role:         RoleStudent,
		Status:       "deactivated",
		AuthProvider: "password",
	})
	svc, _ := newTestService(t, repo)

	_, _, err := svc.GoogleLogin(context.Background(), "fake-id-token")
	if !errors.Is(err, ErrAccountDeactivated) {
		t.Errorf("want ErrAccountDeactivated, got %v", err)
	}
}

// googleTokenInfoTransport rewrites all requests to googlesapis to a fake
// server while passing through every other request untouched.
type googleTokenInfoTransport struct {
	fakeURL string
}

func (t *googleTokenInfoTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if strings.Contains(req.URL.Host, "googleapis.com") {
		newReq, _ := http.NewRequestWithContext(req.Context(), req.Method, t.fakeURL+"?id_token=fake", nil)
		return http.DefaultTransport.RoundTrip(newReq)
	}
	return http.DefaultTransport.RoundTrip(req)
}
