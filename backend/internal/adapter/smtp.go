package adapter

import (
	"bytes"
	"context"
	"crypto/tls"
	_ "embed"
	"errors"
	"fmt"
	"html"
	"log/slog"
	"mime/multipart"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"net/textproto"
	"strings"
	"time"
)

var (
	//go:embed otp_email.html
	otpEmailHTML string
	//go:embed otp_email.txt
	otpEmailText string
)

type SMTPConfig struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string // full "Name <addr>" override; falls back below when empty
	FromName string // display name paired with Username when From is empty
	OTPTTL   time.Duration
}

// smtpDialTimeout bounds the TCP connect; smtpSessionTimeout bounds the whole
// SMTP session (STARTTLS handshake, AUTH, MAIL/DATA) after the connection is
// up. Without these a wedged relay hangs the request path indefinitely.
const (
	smtpDialTimeout    = 30 * time.Second
	smtpSessionTimeout = 60 * time.Second
)

// SMTPProvider sends OTP codes and emails over SMTP (STARTTLS on :587). It
// satisfies both service.OTPProvider and service.EmailProvider, so a single
// instance can back both. Delivery is email-only.
type SMTPProvider struct {
	host string
	addr string
	auth smtp.Auth
	// envelopeFrom is the bare address for the SMTP MAIL FROM — must match the
	// authenticated user (Gmail rejects otherwise). headerFrom is the From:
	// header, which may carry a display name.
	envelopeFrom string
	headerFrom   string
	otpTTL       time.Duration
}

func NewSMTPProvider(cfg SMTPConfig) *SMTPProvider {
	headerFrom := cfg.From
	if headerFrom != "" && cfg.FromName != "" && !strings.Contains(headerFrom, "<") {
		headerFrom = fmt.Sprintf("%s <%s>", cfg.FromName, headerFrom)
	}
	if headerFrom == "" {
		if cfg.FromName != "" {
			headerFrom = fmt.Sprintf("%s <%s>", cfg.FromName, cfg.Username)
		} else {
			headerFrom = cfg.Username
		}
	}
	return &SMTPProvider{
		host:         cfg.Host,
		addr:         net.JoinHostPort(cfg.Host, cfg.Port),
		auth:         smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host),
		envelopeFrom: cfg.Username,
		headerFrom:   headerFrom,
		otpTTL:       cfg.OTPTTL,
	}
}

func (p *SMTPProvider) SendOTP(_ context.Context, channel, to, code string) error {
	if channel != "email" {
		return fmt.Errorf("smtp provider: unsupported OTP channel %q (email only)", channel)
	}
	msg, err := buildOTPMessage(p.headerFrom, to, code, p.otpTTL)
	if err != nil {
		return err
	}
	return p.send(to, msg)
}

func (p *SMTPProvider) SendEmail(_ context.Context, to, subject, body string) error {
	return p.send(to, buildMessage(p.headerFrom, to, subject, body))
}

func (p *SMTPProvider) send(to string, msg []byte) (err error) {
	defer func() {
		if err != nil {
			slog.Error("smtp send failed", "addr", p.addr, "err", err)
		}
	}()
	if p.host == "" {
		return errors.New("smtp send: smtp_host is not configured")
	}
	conn, err := net.DialTimeout("tcp", p.addr, smtpDialTimeout)
	if err != nil {
		return fmt.Errorf("smtp dial %s: %w", p.addr, err)
	}
	defer conn.Close()
	if err := conn.SetDeadline(time.Now().Add(smtpSessionTimeout)); err != nil {
		return fmt.Errorf("smtp deadline: %w", err)
	}

	c, err := smtp.NewClient(conn, p.host)
	if err != nil {
		return fmt.Errorf("smtp client: %w", err)
	}
	defer c.Close()

	// Mirrors net/smtp.SendMail: upgrade to STARTTLS when the relay offers it.
	// StartTLS does not fill in ServerName the way SendMail does; an unnamed config fails the handshake.
	if ok, _ := c.Extension("STARTTLS"); ok {
		if err = c.StartTLS(&tls.Config{ServerName: p.host}); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	if p.auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err = c.Auth(p.auth); err != nil {
				return fmt.Errorf("smtp auth: %w", err)
			}
		}
	}
	if err = c.Mail(p.envelopeFrom); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err = c.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	w, err := c.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	if _, err = w.Write(msg); err != nil {
		return fmt.Errorf("smtp write message: %w", err)
	}
	if err = w.Close(); err != nil {
		return fmt.Errorf("smtp close data: %w", err)
	}
	return c.Quit()
}

func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}

// buildOTPMessage renders the OTP as multipart/alternative: the text part first, so clients that can't show HTML still get the code.
func buildOTPMessage(from, to, code string, ttl time.Duration) ([]byte, error) {
	r := strings.NewReplacer("{{TTL}}", fmt.Sprintf("%d menit", int(ttl.Minutes())))
	var parts bytes.Buffer
	mw := multipart.NewWriter(&parts)
	for _, part := range []struct{ ctype, body string }{
		{"text/plain", r.Replace(strings.ReplaceAll(otpEmailText, "{{CODE}}", code))},
		{"text/html", r.Replace(strings.ReplaceAll(otpEmailHTML, "{{CODE}}", html.EscapeString(code)))},
	} {
		pw, err := mw.CreatePart(textproto.MIMEHeader{
			"Content-Type":              {part.ctype + `; charset="UTF-8"`},
			"Content-Transfer-Encoding": {"quoted-printable"},
		})
		if err != nil {
			return nil, fmt.Errorf("otp email part: %w", err)
		}
		qw := quotedprintable.NewWriter(pw)
		if _, err := qw.Write([]byte(part.body)); err != nil {
			return nil, fmt.Errorf("otp email part: %w", err)
		}
		if err := qw.Close(); err != nil {
			return nil, fmt.Errorf("otp email part: %w", err)
		}
	}
	if err := mw.Close(); err != nil {
		return nil, fmt.Errorf("otp email: %w", err)
	}

	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Date: " + time.Now().Format(time.RFC1123Z) + "\r\n")
	b.WriteString("Subject: Kode OTP Abak Academy\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: multipart/alternative; boundary=\"" + mw.Boundary() + "\"\r\n")
	b.WriteString("\r\n")
	b.Write(parts.Bytes())
	return []byte(b.String()), nil
}
