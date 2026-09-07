package infra_test

import (
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/infra"
)

// tamperPayload replaces the last character of the JWT's payload segment,
// guaranteeing a signature mismatch regardless of base64url encoding.
func tamperPayload(token string) string {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return token[:len(token)-1] + "X"
	}
	b := []byte(parts[1])
	if len(b) == 0 {
		return token[:len(token)-1] + "X"
	}
	b[len(b)-1] ^= 0x01 // flip one bit
	parts[1] = string(b)
	return strings.Join(parts, ".")
}

func TestJWT_RoundTrip(t *testing.T) {
	signer := infra.NewJWTSigner("supersecret", 15*time.Minute)
	schoolID := "school-1"
	caps := []string{"read", "write"}

	token, _, err := signer.SignAccess("user-42", "admin", &schoolID, caps)
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}

	claims, err := signer.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}

	if claims.Sub != "user-42" {
		t.Errorf("Sub: got %q want %q", claims.Sub, "user-42")
	}
	if claims.Role != "admin" {
		t.Errorf("Role: got %q want %q", claims.Role, "admin")
	}
	if claims.SchoolID == nil || *claims.SchoolID != "school-1" {
		t.Errorf("SchoolID: got %v want school-1", claims.SchoolID)
	}
	if len(claims.Capabilities) != 2 || claims.Capabilities[0] != "read" {
		t.Errorf("Capabilities: got %v", claims.Capabilities)
	}
}

func TestJWT_TamperedToken(t *testing.T) {
	signer := infra.NewJWTSigner("supersecret", 15*time.Minute)
	token, _, err := signer.SignAccess("user-1", "student", nil, nil)
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}

	// tamper the payload segment, guaranteeing a signature mismatch
	tampered := tamperPayload(token)
	if _, err := signer.ParseAccess(tampered); err == nil {
		t.Fatal("expected error for tampered token, got nil")
	}
}

func TestJWT_ExpiredToken(t *testing.T) {
	signer := infra.NewJWTSigner("supersecret", -1*time.Second)
	token, _, err := signer.SignAccess("user-1", "student", nil, nil)
	if err != nil {
		t.Fatalf("SignAccess: %v", err)
	}

	_, err = signer.ParseAccess(token)
	if err == nil {
		t.Fatal("expected error for expired token, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "expir") {
		t.Errorf("expected expiry message, got: %v", err)
	}
}

func TestJWT_MustChangeClaim(t *testing.T) {
	signer := infra.NewJWTSigner("supersecret", 15*time.Minute)

	token, _, err := signer.SignAccess("user-1", "student", nil, nil, true)
	if err != nil {
		t.Fatalf("SignAccess with flag: %v", err)
	}
	claims, err := signer.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}
	if !claims.MustChange {
		t.Error("want must_change_password claim set when flag passed")
	}

	token, _, err = signer.SignAccess("user-1", "student", nil, nil)
	if err != nil {
		t.Fatalf("SignAccess without flag: %v", err)
	}
	claims, err = signer.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess: %v", err)
	}
	if claims.MustChange {
		t.Error("want must_change_password claim absent when flag not passed")
	}
}
