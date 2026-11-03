package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/model"
)

func TestValidateExam_eventInfo(t *testing.T) {
	path := "/" + strings.Repeat("a", 10)
	longURL := func(n int, prefix string) string { return prefix + strings.Repeat("a", n-len(prefix)) }
	tests := []struct {
		name     string
		whatsapp *string
		note     *string
		rules    *string
		wantErr  bool
	}{
		{"all unset", nil, nil, nil, false},
		{"whatsapp valid", strPtr("https://chat.whatsapp.com/Abc123"), nil, nil, false},
		{"whatsapp http rejected", strPtr("http://chat.whatsapp.com/Abc123"), nil, nil, true},
		{"whatsapp subdomain rejected", strPtr("https://evil.chat.whatsapp.com/Abc"), nil, nil, true},
		{"whatsapp suffix host rejected", strPtr("https://chat.whatsapp.com.evil.com/Abc"), nil, nil, true},
		{"whatsapp bare domain rejected", strPtr("https://whatsapp.com/Abc"), nil, nil, true},
		{"whatsapp empty path rejected", strPtr("https://chat.whatsapp.com"), nil, nil, true},
		{"whatsapp root path rejected", strPtr("https://chat.whatsapp.com/"), nil, nil, true},
		{"whatsapp relative rejected", strPtr("chat.whatsapp.com/Abc"), nil, nil, true},
		{"whatsapp javascript rejected", strPtr("javascript:alert(1)"), nil, nil, true},
		{"whatsapp 2048 chars ok", strPtr(longURL(2048, "https://chat.whatsapp.com"+path)), nil, nil, false},
		{"whatsapp 2049 chars rejected", strPtr(longURL(2049, "https://chat.whatsapp.com"+path)), nil, nil, true},
		{"rules https ok", nil, nil, strPtr("https://example.com/panduan"), false},
		{"rules http ok", nil, nil, strPtr("http://example.com/panduan"), false},
		{"rules javascript rejected", nil, nil, strPtr("javascript:alert(1)"), true},
		{"rules data rejected", nil, nil, strPtr("data:text/html,x"), true},
		{"rules ftp rejected", nil, nil, strPtr("ftp://example.com/x"), true},
		{"rules relative rejected", nil, nil, strPtr("/panduan"), true},
		{"rules empty host rejected", nil, nil, strPtr("https:///panduan"), true},
		{"rules 2048 chars ok", nil, nil, strPtr(longURL(2048, "https://example.com/")), false},
		{"rules 2049 chars rejected", nil, nil, strPtr(longURL(2049, "https://example.com/")), true},
		{"note 500 multibyte runes ok", nil, strPtr(strings.Repeat("é", 500)), nil, false},
		{"note 501 multibyte runes rejected", nil, strPtr(strings.Repeat("é", 501)), nil, true},
		{"note markup is plain text", nil, strPtr("<b>bold</b>"), nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateExam(model.Exam{
				Title: "Exam", WhatsAppGroupURL: tt.whatsapp, AdditionalRulesNote: tt.note, AdditionalRulesURL: tt.rules,
			})
			if tt.wantErr && !errors.Is(err, ErrValidation) {
				t.Fatalf("validateExam() err = %v, want ErrValidation", err)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("validateExam() err = %v, want nil", err)
			}
		})
	}
}

func TestCreateExam_normalizesEventInfo(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()
	tests := []struct {
		name                              string
		whatsapp, note, rules             *string
		wantWhatsApp, wantNote, wantRules *string
	}{
		{"blank and whitespace become nil", strPtr(""), strPtr("   \n"), strPtr("  "), nil, nil, nil},
		{"values are trimmed", strPtr(" https://chat.whatsapp.com/Abc "), strPtr("  Bawa KTP.\nJangan telat. "), strPtr(" https://example.com/x "),
			strPtr("https://chat.whatsapp.com/Abc"), strPtr("Bawa KTP.\nJangan telat."), strPtr("https://example.com/x")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			created, err := svc.CreateExam(ctx, model.Exam{
				ID: uuid.New(), Title: "Event " + uniqueSuffix(),
				WhatsAppGroupURL: tt.whatsapp, AdditionalRulesNote: tt.note, AdditionalRulesURL: tt.rules,
			})
			require.NoError(t, err)
			stored, err := svc.GetExam(ctx, created.ID)
			require.NoError(t, err)
			require.Equal(t, tt.wantWhatsApp, stored.Exam.WhatsAppGroupURL)
			require.Equal(t, tt.wantNote, stored.Exam.AdditionalRulesNote)
			require.Equal(t, tt.wantRules, stored.Exam.AdditionalRulesURL)
		})
	}
}

func TestUpdateExam_eventInfoIsNotPolicyAndStaysEditableWithSessions(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	for _, status := range []string{"in_progress", "submitted"} {
		t.Run(status, func(t *testing.T) {
			examID := createV2Exam(t, repo, "free")
			seedExamSession(t, repo, examID)
			_, err := repo.Pool().Exec(ctx, `UPDATE exam_session SET status = $1 WHERE exam_id = $2`, status, examID)
			require.NoError(t, err)
			detail, err := svc.GetExam(ctx, examID)
			require.NoError(t, err)

			overlay := detail.Exam
			overlay.WhatsAppGroupURL = strPtr("https://chat.whatsapp.com/Grup1")
			overlay.AdditionalRulesNote = strPtr("Baca panduan.")
			overlay.AdditionalRulesURL = strPtr("https://example.com/panduan")
			_, err = svc.UpdateExam(ctx, examID, overlay, uuid.NewString())
			require.NoError(t, err)

			stored, err := svc.GetExam(ctx, examID)
			require.NoError(t, err)
			require.Equal(t, overlay.WhatsAppGroupURL, stored.Exam.WhatsAppGroupURL)
			require.Equal(t, overlay.AdditionalRulesNote, stored.Exam.AdditionalRulesNote)
			require.Equal(t, overlay.AdditionalRulesURL, stored.Exam.AdditionalRulesURL)

			policy := stored.Exam
			policy.NavigationPolicy = "sequential"
			_, err = svc.UpdateExam(ctx, examID, policy, uuid.NewString())
			require.ErrorIs(t, err, ErrExamHasSessions)
		})
	}
}

func TestUpdateExam_rejectsInvalidEventInfo(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	examID := createV2Exam(t, repo, "free")
	detail, err := svc.GetExam(ctx, examID)
	require.NoError(t, err)
	overlay := detail.Exam
	overlay.WhatsAppGroupURL = strPtr("https://evil.com/x")
	_, err = svc.UpdateExam(ctx, examID, overlay, uuid.NewString())
	require.ErrorIs(t, err, ErrValidation)
}
