package adapter

import (
	"bufio"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestSMTPProvider_SendOTP_RejectsNonEmailChannel(t *testing.T) {
	p := NewSMTPProvider(SMTPConfig{Host: "smtp.gmail.com", Port: "587", Username: "a@b.com"})
	// whatsapp/SMS can't go over SMTP — must error before any network call.
	if err := p.SendOTP(context.Background(), "whatsapp", "+628123", "123456"); err == nil {
		t.Fatal("expected error for non-email channel, got nil")
	}
}

func TestNewSMTPProvider_HeaderFrom(t *testing.T) {
	// bare username when neither From nor FromName is set
	p := NewSMTPProvider(SMTPConfig{Host: "smtp.gmail.com", Port: "587", Username: "sender@gmail.com"})
	if p.headerFrom != "sender@gmail.com" {
		t.Errorf("headerFrom: got %q, want the bare username", p.headerFrom)
	}

	// FromName pairs a display name with the username
	pn := NewSMTPProvider(SMTPConfig{Host: "h", Port: "587", Username: "sender@gmail.com", FromName: "Abak Academy"})
	if pn.headerFrom != "Abak Academy <sender@gmail.com>" {
		t.Errorf("headerFrom: got %q, want display name paired with username", pn.headerFrom)
	}

	// explicit From overrides everything
	po := NewSMTPProvider(SMTPConfig{Host: "h", Port: "587", Username: "u@x.com", From: "Abak <no-reply@x.com>", FromName: "Ignored"})
	if po.headerFrom != "Abak <no-reply@x.com>" {
		t.Errorf("headerFrom: got %q, want the explicit From", po.headerFrom)
	}

	// envelope sender is always the authenticated username, never the display form
	if pn.envelopeFrom != "sender@gmail.com" {
		t.Errorf("envelopeFrom: got %q, want the bare authenticated username", pn.envelopeFrom)
	}
}

func TestBuildMessage_HasHeadersAndBody(t *testing.T) {
	msg := string(buildMessage("from@x.com", "to@y.com", "Kode OTP", "code: 123456"))
	for _, want := range []string{
		"From: from@x.com\r\n",
		"To: to@y.com\r\n",
		"Subject: Kode OTP\r\n",
		"Content-Type: text/plain; charset=\"UTF-8\"\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing header %q", want)
		}
	}
	// headers and body separated by a blank CRLF line
	if !strings.Contains(msg, "\r\n\r\ncode: 123456") {
		t.Errorf("body not separated from headers by a blank line:\n%s", msg)
	}
}

func TestSMTPProvider_SendOTP_STARTTLSVerifiesServerCert(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer ln.Close()
	go serveFakeSTARTTLS(ln, selfSignedLocalCert(t))

	port := strconv.Itoa(ln.Addr().(*net.TCPAddr).Port)
	p := NewSMTPProvider(SMTPConfig{Host: "localhost", Port: port, Username: "otp@abakacademy.id"})

	err = p.SendOTP(context.Background(), "email", "penerima@example.com", "123456")
	if err == nil {
		t.Fatal("expected delivery error against a self-signed test relay, got nil")
	}
	// session-less relay: any failure below the upgrade itself would read EOF
	if strings.Contains(err.Error(), "either ServerName or InsecureSkipVerify must be specified") {
		t.Fatalf("STARTTLS used an unnamed/empty tls.Config: %v", err)
	}
	if strings.Contains(err.Error(), "EOF") {
		t.Fatalf("expected the STARTTLS handshake itself to fail certificate verification, got past the TLS upgrade: %v", err)
	}
}

func selfSignedLocalCert(t *testing.T) tls.Certificate {
	t.Helper()
	_, key, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}
	tmpl := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "localhost"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	der, err := x509.CreateCertificate(rand.Reader, &tmpl, &tmpl, key.Public(), key)
	if err != nil {
		t.Fatalf("create certificate: %v", err)
	}
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key}
}

// serveFakeSTARTTLS speaks just enough SMTP to exercise STARTTLS: greeting,
// EHLO advertising the extension, then an upgrade with a self-signed cert.
func serveFakeSTARTTLS(ln net.Listener, cert tls.Certificate) {
	conn, err := ln.Accept()
	if err != nil {
		return
	}
	defer conn.Close()
	conn.SetDeadline(time.Now().Add(5 * time.Second))
	if _, err := fmt.Fprint(conn, "220 localhost ESMTP\r\n"); err != nil {
		return
	}
	r := bufio.NewReader(conn)
	for {
		line, err := r.ReadString('\n')
		if err != nil {
			return
		}
		cmd := strings.TrimRight(line, "\r\n")
		if strings.HasPrefix(cmd, "EHLO") {
			if _, err := fmt.Fprint(conn, "250-localhost greets you\r\n250-STARTTLS\r\n250 8BITMIME\r\n"); err != nil {
				return
			}
			continue
		}
		if strings.HasPrefix(cmd, "STARTTLS") {
			if _, err := fmt.Fprint(conn, "220 ready for TLS\r\n"); err != nil {
				return
			}
			tlsConn := tls.Server(conn, &tls.Config{Certificates: []tls.Certificate{cert}})
			if err := tlsConn.Handshake(); err != nil {
				return
			}
			// session terminates when the client finishes verifying the cert
			return
		}
	}
}
