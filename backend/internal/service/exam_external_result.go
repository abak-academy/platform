package service

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/minio/minio-go/v7"
)

const (
	externalResultsMaxBytes = 2 << 20
	externalLabelMaxLen     = 80
	externalAwardMaxLen     = 60

	ReasonUnreadableParticipantNo = "unreadable_participant_no"
	ReasonOtherExamParticipantNo  = "other_exam_participant_no"
	ReasonUnknownParticipant      = "unknown_participant"
	ReasonRevokedParticipant      = "revoked_participant"
	ReasonDuplicateParticipant    = "duplicate_participant"
	ReasonBadScore                = "bad_score"
	ReasonBadRank                 = "bad_rank"
	ReasonBadCertificateURL       = "bad_certificate_url"
	ReasonAwardTooLong            = "award_too_long"
	ReasonBadRow                  = "bad_row"
)

var ErrExternalRoundNotFound = errors.New("external round not found")

var externalScoreRe = regexp.MustCompile(`^\d+(\.\d+)?$`)

var externalColumns = map[string]bool{"participant_no": true, "score": true, "rank": true, "award": true, "certificate_url": true}

func externalKeyPrefix(examID uuid.UUID) string {
	return fmt.Sprintf("exam-external-results/%s/", examID)
}

func (s *Service) PresignExternalRoundUpload(ctx context.Context, examID uuid.UUID, filename string) (*PrivateUploadURL, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, fmt.Errorf("%w: filename is required", ErrValidation)
	}
	return s.presignPrivatePut(ctx, externalKeyPrefix(examID)+uuid.NewString()+"-"+filename)
}

func (s *Service) readExternalRoundObject(ctx context.Context, examID uuid.UUID, key string) ([]byte, error) {
	if !strings.HasPrefix(key, externalKeyPrefix(examID)) || strings.Contains(key, "..") {
		return nil, ErrUploadNotFound
	}
	if s.storage == nil {
		return nil, ErrStorageNotConfigured
	}
	obj, err := s.storage.GetObject(ctx, s.cfg.ObjectStoragePrivateBucketName, key, minio.GetObjectOptions{})
	if err != nil {
		return nil, err
	}
	defer obj.Close()
	data, err := io.ReadAll(io.LimitReader(obj, externalResultsMaxBytes+1))
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrUploadNotFound
		}
		return nil, err
	}
	if len(data) > externalResultsMaxBytes {
		return nil, fmt.Errorf("%w: file_too_large (max 2 MB)", ErrValidation)
	}
	return data, nil
}

func cleanExternalLabel(label string) (string, error) {
	label = strings.TrimSpace(label)
	if label == "" || utf8.RuneCountInString(label) > externalLabelMaxLen {
		return "", fmt.Errorf("%w: round_label must be 1-%d characters", ErrValidation, externalLabelMaxLen)
	}
	return label, nil
}

func mapExternalNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrExamNotFound
	}
	return err
}

// evaluateExternalRound reads and validates the file; it is the single source
// for both preview and apply so the preview is never trusted as state.
func (s *Service) evaluateExternalRound(ctx context.Context, examID uuid.UUID, key, label string) (string, []model.ExternalResultInput, *model.ExternalRoundPreview, error) {
	label, err := cleanExternalLabel(label)
	if err != nil {
		return "", nil, nil, err
	}
	data, err := s.readExternalRoundObject(ctx, examID, key)
	if err != nil {
		return "", nil, nil, err
	}
	examNumber, regs, err := s.storeRepo.GetExternalMatchData(ctx, examID)
	if err != nil {
		return "", nil, nil, mapExternalNotFound(err)
	}
	inputs, rowErrs, err := parseExternalRoundCSV(data, examNumber, regs)
	if err != nil {
		return "", nil, nil, err
	}
	preview := &model.ExternalRoundPreview{Matched: len(inputs), Errors: rowErrs}
	round, err := s.storeRepo.GetExternalRoundByLabel(ctx, examID, label)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return "", nil, nil, err
	}
	if round != nil {
		preview.RoundExists = true
		preview.RoundPublished = round.PublishedAt != nil
	}
	return label, inputs, preview, nil
}

func (s *Service) PreviewExternalRound(ctx context.Context, examID uuid.UUID, key, label string) (*model.ExternalRoundPreview, error) {
	_, _, preview, err := s.evaluateExternalRound(ctx, examID, key, label)
	return preview, err
}

// ApplyExternalRound writes nothing and returns the preview when the file has
// row errors; the caller reports those as 422.
func (s *Service) ApplyExternalRound(ctx context.Context, examID uuid.UUID, actorID, key, label string) (*model.ExternalRoundPreview, *model.ExternalRound, error) {
	label, inputs, preview, err := s.evaluateExternalRound(ctx, examID, key, label)
	if err != nil || len(preview.Errors) > 0 {
		return preview, nil, err
	}
	round, err := s.storeRepo.ApplyExternalRound(ctx, examID, label, key, actorID, inputs)
	if err != nil {
		return nil, nil, err
	}
	return preview, round, nil
}

func parseExternalRoundCSV(data []byte, examNumber *int, regs map[int]repository.ExternalRegistration) ([]model.ExternalResultInput, []model.ExternalRowError, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\xef\xbb\xbf")))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: invalid_csv: %v", ErrValidation, err)
	}
	col := map[string]int{}
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(h))
		if !externalColumns[h] {
			return nil, nil, fmt.Errorf("%w: unknown_column %q", ErrValidation, h)
		}
		if _, dup := col[h]; dup {
			return nil, nil, fmt.Errorf("%w: duplicate_column %q", ErrValidation, h)
		}
		col[h] = i
	}
	for _, req := range []string{"participant_no", "score"} {
		if _, ok := col[req]; !ok {
			return nil, nil, fmt.Errorf("%w: missing_column %q", ErrValidation, req)
		}
	}
	get := func(rec []string, name string) string {
		if i, ok := col[name]; ok {
			return strings.TrimSpace(rec[i])
		}
		return ""
	}

	inputs := []model.ExternalResultInput{}
	rowErrs := []model.ExternalRowError{}
	seen := map[int]bool{}
	rows := 0
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, fmt.Errorf("%w: invalid_csv: %v", ErrValidation, err)
		}
		rows++
		line, _ := r.FieldPos(0)
		no := get(rec, "participant_no")
		fail := func(reason string) {
			rowErrs = append(rowErrs, model.ExternalRowError{Line: line, ParticipantNo: no, Reason: reason})
		}
		if len(rec) != len(header) {
			fail(ReasonBadRow)
			continue
		}
		examNo, participantNo, err := parseParticipantNo(no)
		if err != nil {
			fail(ReasonUnreadableParticipantNo)
			continue
		}
		if examNumber == nil || examNo != *examNumber {
			fail(ReasonOtherExamParticipantNo)
			continue
		}
		reg, ok := regs[participantNo]
		if !ok {
			fail(ReasonUnknownParticipant)
			continue
		}
		if reg.Revoked {
			fail(ReasonRevokedParticipant)
			continue
		}
		if seen[participantNo] {
			fail(ReasonDuplicateParticipant)
			continue
		}
		seen[participantNo] = true

		in := model.ExternalResultInput{RegistrationID: reg.ID, Score: get(rec, "score")}
		if f, err := strconv.ParseFloat(in.Score, 64); !externalScoreRe.MatchString(in.Score) || err != nil || f >= 1e6 {
			fail(ReasonBadScore)
			continue
		}
		if v := get(rec, "rank"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 1 {
				fail(ReasonBadRank)
				continue
			}
			in.Rank = &n
		}
		if v := get(rec, "award"); v != "" {
			if utf8.RuneCountInString(v) > externalAwardMaxLen {
				fail(ReasonAwardTooLong)
				continue
			}
			in.Award = &v
		}
		if v := get(rec, "certificate_url"); v != "" {
			if u, err := url.Parse(v); err != nil || !strings.HasPrefix(v, "https://") || u.Host == "" {
				fail(ReasonBadCertificateURL)
				continue
			}
			in.CertificateURL = &v
		}
		inputs = append(inputs, in)
	}
	if rows == 0 {
		return nil, nil, fmt.Errorf("%w: no_rows", ErrValidation)
	}
	return inputs, rowErrs, nil
}

func (s *Service) ListExternalRounds(ctx context.Context, examID uuid.UUID) ([]model.ExternalRound, error) {
	return s.storeRepo.ListExternalRounds(ctx, examID)
}

func (s *Service) SetExternalRoundPublished(ctx context.Context, roundID uuid.UUID, publish bool, actorID string) (*model.ExternalRound, error) {
	rd, err := s.storeRepo.SetExternalRoundPublished(ctx, roundID, publish, actorID)
	return rd, mapExternalRoundNotFound(err)
}

func (s *Service) DeleteExternalRound(ctx context.Context, roundID uuid.UUID, actorID string) error {
	return mapExternalRoundNotFound(s.storeRepo.DeleteExternalRound(ctx, roundID, actorID))
}

func mapExternalRoundNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrExternalRoundNotFound
	}
	return err
}

func (s *Service) ListExternalResults(ctx context.Context, roundID uuid.UUID) ([]model.ExternalResultRow, error) {
	round, err := s.storeRepo.GetExternalRound(ctx, roundID)
	if err != nil {
		return nil, mapExternalRoundNotFound(err)
	}
	prefix, err := s.storeRepo.GetExamEventPrefix(ctx, round.ExamID)
	if err != nil {
		return nil, err
	}
	rows, err := s.storeRepo.ListExternalResults(ctx, roundID)
	if err != nil {
		return nil, err
	}
	wib, _ := time.LoadLocation("Asia/Jakarta")
	for i := range rows {
		if rows[i].ParticipantNumber == nil {
			continue
		}
		date := rows[i].DisplayDate
		if rows[i].ExamScheduledAt != nil {
			date = *rows[i].ExamScheduledAt
		}
		if wib != nil {
			date = date.In(wib)
		}
		examNo := 0
		if rows[i].ExamNumber != nil {
			examNo = *rows[i].ExamNumber
		}
		rows[i].ParticipantNo = formatParticipantNo(derefPrefix(prefix), date, examNo, *rows[i].ParticipantNumber)
	}
	return rows, nil
}

// attachExternalResults fills ExternalResults on every item with one query.
func (s *Service) attachExternalResults(ctx context.Context, items []model.RegistrationListItem, lang string) error {
	ids := make([]uuid.UUID, len(items))
	for i := range items {
		ids[i] = items[i].ID
		items[i].ExternalResults = []model.StudentExternalResult{}
	}
	results, err := s.storeRepo.ListPublishedExternalResults(ctx, ids)
	if err != nil {
		return err
	}
	byReg := map[uuid.UUID][]model.StudentExternalResult{}
	for _, x := range results {
		if lang == "en" && x.LabelEN != nil && *x.LabelEN != "" {
			x.RoundLabel = *x.LabelEN
		}
		byReg[x.RegistrationID] = append(byReg[x.RegistrationID], x)
	}
	for i := range items {
		if xs := byReg[items[i].ID]; xs != nil {
			items[i].ExternalResults = xs
		}
	}
	return nil
}
