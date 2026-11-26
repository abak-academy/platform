package service

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func letterPrefix() string {
	n := time.Now().UnixNano()
	b := make([]byte, 6)
	for i := range b {
		b[i] = byte('A' + n%26)
		n /= 26
	}
	return string(b)
}

func seedGrouping(t *testing.T, repo *repository.Repository, prefix *string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	suffix := uniqueSuffix()
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product_grouping (name, slug, participant_prefix) VALUES ($1, $2, $3) RETURNING id`,
		"Event "+suffix, "event-"+suffix, prefix).Scan(&id))
	return id
}

func sellExamIn(t *testing.T, repo *repository.Repository, examID uuid.UUID, groupingID *uuid.UUID) {
	t.Helper()
	var productID uuid.UUID
	require.NoError(t, repo.Pool().QueryRow(context.Background(),
		`INSERT INTO product (type, name, price, status, grouping_id) VALUES ('exam', $1, 0, 'published', $2) RETURNING id`,
		"Edition "+uniqueSuffix(), groupingID).Scan(&productID))
	_, err := repo.Pool().Exec(context.Background(),
		`INSERT INTO product_exam (product_id, exam_id) VALUES ($1, $2)`, productID, examID)
	require.NoError(t, err)
}

type seededReg struct {
	examID, regID, studentID uuid.UUID
	examNo                   int
}

func seedRegisteredExam(t *testing.T, repo *repository.Repository) seededReg {
	t.Helper()
	ctx := context.Background()
	var r seededReg
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO exam (title, status, scheduled_at) VALUES ($1, 'draft', $2) RETURNING id, exam_number`,
		"Prefix Exam "+uniqueSuffix(), time.Date(2025, 6, 20, 9, 0, 0, 0, time.UTC)).Scan(&r.examID, &r.examNo))
	r.studentID = seedStudentDirect(t, ctx, repo, "Peserta Uji", "uji-"+uniqueSuffix())
	require.NoError(t, repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token, status, participant_number)
		VALUES ($1, $2, $3, 'registered', 7) RETURNING id`,
		r.studentID, r.examID, "TOK"+uniqueSuffix()).Scan(&r.regID))
	return r
}

func TestParticipantNo_EventPrefixOnEverySurface(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()
	pfx := letterPrefix()
	noPrefix := (*string)(nil)

	prefixedEvent := seedGrouping(t, repo, &pfx)
	prefixlessEvent := seedGrouping(t, repo, noPrefix)
	otherPfx := letterPrefix()
	if otherPfx == pfx {
		otherPfx = "ZZZZZZ"
	}
	otherEvent := seedGrouping(t, repo, &otherPfx)

	prefixed := seedRegisteredExam(t, repo)
	sellExamIn(t, repo, prefixed.examID, &prefixedEvent)
	sellExamIn(t, repo, prefixed.examID, &prefixedEvent)
	noEvent := seedRegisteredExam(t, repo)
	sellExamIn(t, repo, noEvent.examID, nil)
	prefixless := seedRegisteredExam(t, repo)
	sellExamIn(t, repo, prefixless.examID, &prefixlessEvent)
	twoEvents := seedRegisteredExam(t, repo)
	sellExamIn(t, repo, twoEvents.examID, &prefixedEvent)
	sellExamIn(t, repo, twoEvents.examID, &otherEvent)
	unsold := seedRegisteredExam(t, repo)

	cases := []struct {
		name   string
		reg    seededReg
		prefix string
	}{
		{"one event with prefix, two products", prefixed, pfx + "-"},
		{"product outside any event", noEvent, ""},
		{"event without prefix", prefixless, ""},
		{"two different events", twoEvents, ""},
		{"exam sold by no product", unsold, ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			want := fmt.Sprintf("%s250620-%s-000007", tc.prefix, formatExamNumber(tc.reg.examNo))

			detail, err := svc.GetExamRegistration(ctx, tc.reg.regID.String(), tc.reg.studentID.String())
			require.NoError(t, err)
			assert.Equal(t, want, detail.ParticipantNo)
			require.NotNil(t, detail.ParticipantNumber)
			assert.Equal(t, 7, *detail.ParticipantNumber, "stored number is untouched")

			card, err := svc.GetCardPrintData(ctx, tc.reg.regID.String())
			require.NoError(t, err)
			assert.Equal(t, want, card.ParticipantNo)

			roster, err := svc.AdminGetExamRoster(ctx, tc.reg.examID, nil)
			require.NoError(t, err)
			require.Len(t, roster, 1)
			assert.Equal(t, want, roster[0].ParticipantNo)

			page, _, err := svc.AdminListExamRoster(ctx, tc.reg.examID, nil, model.ExamRosterFilter{Limit: 10})
			require.NoError(t, err)
			require.Len(t, page, 1)
			assert.Equal(t, want, page[0].ParticipantNo)

			csvBytes, err := svc.ExportExamRosterCSV(ctx, tc.reg.examID, nil)
			require.NoError(t, err)
			assert.True(t, strings.Contains(string(csvBytes), want+","), "csv first column: %s", csvBytes)

			var stored int
			require.NoError(t, repo.Pool().QueryRow(ctx,
				`SELECT participant_number FROM exam_registration WHERE id = $1`, tc.reg.regID).Scan(&stored))
			assert.Equal(t, 7, stored)
		})
	}

	t.Run("GetExamEventPrefix", func(t *testing.T) {
		got, err := repo.GetExamEventPrefix(ctx, prefixed.examID)
		require.NoError(t, err)
		require.NotNil(t, got)
		assert.Equal(t, pfx, *got)
		for _, r := range []seededReg{noEvent, prefixless, twoEvents, unsold} {
			got, err := repo.GetExamEventPrefix(ctx, r.examID)
			require.NoError(t, err)
			assert.Nil(t, got)
		}
	})
}

func TestBuildExamCardHTML_CarriesPrefixedParticipantNo(t *testing.T) {
	html := buildExamCardHTML(&CardPrintData{ParticipantNo: "ESOL-250620-0042-000005", StudentName: "Uji"})
	assert.Contains(t, html, "ESOL-250620-0042-000005")
	html = buildExamCardHTML(&CardPrintData{ParticipantNo: "250620-0042-000005", StudentName: "Uji"})
	assert.Contains(t, html, "250620-0042-000005")
	assert.NotContains(t, html, "ESOL-")
}
