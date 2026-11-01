package service

import (
	"context"
	"fmt"
	"sync"
)

// Notification ports the service depends on. Real implementations live in
// internal/adapter (Fazpass, SMTP); the Noop variants let the app run with no
// provider configured.

type OTPProvider interface {
	SendOTP(ctx context.Context, channel, to, code string) error
}

type EmailProvider interface {
	SendEmail(ctx context.Context, to, subject, body string) error
}

type NotifClient interface {
	Send(ctx context.Context, channel, to, message string) error
}

// PreferredOTPChannel returns the best channel+destination given available contact info.
func PreferredOTPChannel(phone, email string) (channel, destination string) {
	if phone != "" {
		return "whatsapp", phone
	}
	if email != "" {
		return "email", email
	}
	return "email", ""
}

type NoopOTPProvider struct{}

func (n *NoopOTPProvider) SendOTP(_ context.Context, channel, to, code string) error {
	fmt.Printf("[noop-otp] channel=%s to=%s code=%s\n", channel, to, code)
	return nil
}

type NoopEmailProvider struct{}

func (n *NoopEmailProvider) SendEmail(_ context.Context, to, subject, body string) error {
	fmt.Printf("[noop-email] to=%s subject=%s body=%s\n", to, subject, body)
	return nil
}

// RecordingOTPProvider is a test double that captures the codes it delivers —
// with stored hashes the raw code only ever exists on the provider boundary.
type RecordingOTPProvider struct {
	mu    sync.Mutex
	Codes []string
	err   error
}

func NewRecordingOTPProvider() *RecordingOTPProvider { return &RecordingOTPProvider{} }

func (r *RecordingOTPProvider) SendOTP(_ context.Context, _, _, code string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.err != nil {
		return r.err
	}
	r.Codes = append(r.Codes, code)
	return nil
}
