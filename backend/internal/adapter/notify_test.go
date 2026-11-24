package adapter

import (
	"context"
	"testing"

	"akademi-bimbel/internal/service"
)

func TestPreferredOTPChannel(t *testing.T) {
	tests := []struct {
		name        string
		phone       string
		email       string
		wantChannel string
		wantDest    string
	}{
		{"both present", "+6281234567890", "user@example.com", "email", "user@example.com"},
		{"phone only", "+6281234567890", "", "whatsapp", "+6281234567890"},
		{"phone empty email present", "", "user@example.com", "email", "user@example.com"},
		{"both empty", "", "", "email", ""},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ch, dest := service.PreferredOTPChannel(tc.phone, tc.email)
			if ch != tc.wantChannel {
				t.Errorf("channel: got %q, want %q", ch, tc.wantChannel)
			}
			if dest != tc.wantDest {
				t.Errorf("dest: got %q, want %q", dest, tc.wantDest)
			}
		})
	}
}

func TestNoopOTPProvider(t *testing.T) {
	p := &NoopOTPProvider{}
	err := p.SendOTP(context.Background(), "whatsapp", "+6281234567890", "123456")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

func TestNoopEmailProvider(t *testing.T) {
	p := &NoopEmailProvider{}
	err := p.SendEmail(context.Background(), "user@example.com", "Test Subject", "Test body")
	if err != nil {
		t.Fatalf("expected nil, got %v", err)
	}
}

// Compile-time interface checks for SMTPProvider.
var _ service.OTPProvider = (*SMTPProvider)(nil)
var _ service.EmailProvider = (*SMTPProvider)(nil)
