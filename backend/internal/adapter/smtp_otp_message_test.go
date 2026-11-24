package adapter

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/mail"
	"strings"
	"testing"
	"time"
)

func TestBuildOTPMessage_MultipartWithCodeInBothParts(t *testing.T) {
	raw, err := buildOTPMessage("Abak Academy <noreply@abakacademy.id>", "siswa@example.com", "482913", 10*time.Minute)
	if err != nil {
		t.Fatalf("buildOTPMessage: %v", err)
	}
	msg, err := mail.ReadMessage(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("not a parseable RFC 5322 message: %v", err)
	}
	if got := msg.Header.Get("Subject"); got != "Kode OTP Abak Academy" {
		t.Errorf("Subject: got %q", got)
	}
	if _, err := msg.Header.Date(); err != nil {
		t.Errorf("Date header missing or unparseable: %v", err)
	}
	if id := msg.Header.Get("Message-ID"); !strings.HasPrefix(id, "<") || !strings.HasSuffix(id, "@abakacademy.id>") {
		t.Errorf("Message-ID: got %q, want <...@abakacademy.id>", id)
	}
	if again, _ := buildOTPMessage("noreply@abakacademy.id", "x@example.com", "1", time.Minute); bytes.Contains(again, []byte(msg.Header.Get("Message-ID"))) {
		t.Error("two messages share a Message-ID")
	}
	mediaType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/alternative" {
		t.Fatalf("Content-Type: got %q (%v), want multipart/alternative", mediaType, err)
	}

	mr := multipart.NewReader(msg.Body, params["boundary"])
	var types []string
	for {
		part, err := mr.NextPart() // decodes quoted-printable transparently
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("NextPart: %v", err)
		}
		body, err := io.ReadAll(part)
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		ct := part.Header.Get("Content-Type")
		types = append(types, strings.SplitN(ct, ";", 2)[0])
		if !strings.Contains(ct, "UTF-8") {
			t.Errorf("part %q is not UTF-8", ct)
		}
		if !strings.Contains(string(body), "482913") {
			t.Errorf("part %q does not contain the code", ct)
		}
		if strings.Contains(string(body), "{{") {
			t.Errorf("part %q still contains a {{...}} placeholder", ct)
		}
		if !strings.Contains(string(body), "10 menit") {
			t.Errorf("part %q does not show the configured TTL", ct)
		}
	}
	// RFC 2046: the richest alternative goes last, so text/plain must come first.
	if strings.Join(types, ",") != "text/plain,text/html" {
		t.Errorf("parts: got %v, want [text/plain text/html]", types)
	}
}
