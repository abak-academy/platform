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
	competitionResultsMaxBytes = 2 << 20
	competitionAwardMaxLen     = 60

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

var ErrCompetitionResultNotFound = errors.New("competition result not found")

var competitionScoreRe = regexp.MustCompile(`^\d+(\.\d+)?$`)

var competitionColumns = map[string]bool{"participant_no": true, "score": true, "rank": true, "award": true, "certificate_url": true}

func competitionKeyPrefix(productID uuid.UUID) string {
	return fmt.Sprintf("competition-results/%s/", productID)
}

func (s *Service) PresignCompetitionResultUpload(ctx context.Context, productID uuid.UUID, filename string) (*PrivateUploadURL, error) {
	if strings.TrimSpace(filename) == "" {
		return nil, fmt.Errorf("%w: filename is required", ErrValidation)
	}
	return s.presignPrivatePut(ctx, competitionKeyPrefix(productID)+uuid.NewString()+"-"+filename)
}

func (s *Service) readCompetitionResultObject(ctx context.Context, productID uuid.UUID, key string) ([]byte, error) {
	if !strings.HasPrefix(key, competitionKeyPrefix(productID)) || strings.Contains(key, "..") {
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
	data, err := io.ReadAll(io.LimitReader(obj, competitionResultsMaxBytes+1))
	if err != nil {
		if minio.ToErrorResponse(err).Code == "NoSuchKey" {
			return nil, ErrUploadNotFound
		}
		return nil, err
	}
	if len(data) > competitionResultsMaxBytes {
		return nil, fmt.Errorf("%w: file_too_large (max 2 MB)", ErrValidation)
	}
	return data, nil
}

// evaluateCompetitionResult reads and validates the file; it is the single
// source for both preview and apply so the preview is never trusted as state.
func (s *Service) evaluateCompetitionResult(ctx context.Context, productID uuid.UUID, key string) ([]model.CompetitionResultInput, *model.CompetitionResultPreview, error) {
	data, err := s.readCompetitionResultObject(ctx, productID, key)
	if err != nil {
		return nil, nil, err
	}
	regs, err := s.storeRepo.GetCompetitionMatchData(ctx, productID)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil, ErrProductNotFound
		}
		return nil, nil, err
	}
	inputs, rowErrs, err := parseCompetitionResultCSV(data, regs)
	if err != nil {
		return nil, nil, err
	}
	preview := &model.CompetitionResultPreview{Matched: len(inputs), Errors: rowErrs}
	existing, err := s.storeRepo.GetCompetitionResult(ctx, productID)
	if err != nil && !errors.Is(err, repository.ErrNotFound) {
		return nil, nil, err
	}
	if existing != nil {
		preview.ResultExists = true
		preview.ResultPublished = existing.PublishedAt != nil
	}
	return inputs, preview, nil
}

func (s *Service) PreviewCompetitionResult(ctx context.Context, productID uuid.UUID, key string) (*model.CompetitionResultPreview, error) {
	_, preview, err := s.evaluateCompetitionResult(ctx, productID, key)
	return preview, err
}

// ApplyCompetitionResult writes nothing and returns the preview when the file
// has row errors; the caller reports those as 422.
func (s *Service) ApplyCompetitionResult(ctx context.Context, productID uuid.UUID, actorID, key string) (*model.CompetitionResultPreview, *model.CompetitionResult, error) {
	inputs, preview, err := s.evaluateCompetitionResult(ctx, productID, key)
	if err != nil || len(preview.Errors) > 0 {
		return preview, nil, err
	}
	result, err := s.storeRepo.ApplyCompetitionResult(ctx, productID, key, actorID, inputs)
	if err != nil {
		return nil, nil, err
	}
	return preview, result, nil
}

func parseCompetitionResultCSV(data []byte, regs map[int]map[int]repository.CompetitionRegistration) ([]model.CompetitionResultInput, []model.CompetitionRowError, error) {
	r := csv.NewReader(strings.NewReader(strings.TrimPrefix(string(data), "\xef\xbb\xbf")))
	r.FieldsPerRecord = -1
	header, err := r.Read()
	if err != nil {
		return nil, nil, fmt.Errorf("%w: invalid_csv: %v", ErrValidation, err)
	}
	col := map[string]int{}
	for i, h := range header {
		h = strings.ToLower(strings.TrimSpace(h))
		if !competitionColumns[h] {
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

	inputs := []model.CompetitionResultInput{}
	rowErrs := []model.CompetitionRowError{}
	seen := map[uuid.UUID]bool{}
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
			rowErrs = append(rowErrs, model.CompetitionRowError{Line: line, ParticipantNo: no, Reason: reason})
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
		examRegs, ok := regs[examNo]
		if !ok {
			fail(ReasonOtherExamParticipantNo)
			continue
		}
		reg, ok := examRegs[participantNo]
		if !ok {
			fail(ReasonUnknownParticipant)
			continue
		}
		if reg.Revoked {
			fail(ReasonRevokedParticipant)
			continue
		}
		if seen[reg.ID] {
			fail(ReasonDuplicateParticipant)
			continue
		}
		seen[reg.ID] = true

		in := model.CompetitionResultInput{RegistrationID: reg.ID, Score: get(rec, "score")}
		if f, err := strconv.ParseFloat(in.Score, 64); !competitionScoreRe.MatchString(in.Score) || err != nil || f >= 1e6 {
			fail(ReasonBadScore)
			continue
		}
		if v := get(rec, "rank"); v != "" {
			n, err := strconv.ParseInt(v, 10, 32)
			if err != nil || n < 1 {
				fail(ReasonBadRank)
				continue
			}
			rank := int(n)
			in.Rank = &rank
		}
		if v := get(rec, "award"); v != "" {
			if utf8.RuneCountInString(v) > competitionAwardMaxLen {
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

func (s *Service) GetCompetitionResult(ctx context.Context, productID uuid.UUID) (*model.CompetitionResult, error) {
	res, err := s.storeRepo.GetCompetitionResult(ctx, productID)
	if errors.Is(err, repository.ErrNotFound) {
		return nil, nil
	}
	return res, err
}

func (s *Service) SetCompetitionResultPublished(ctx context.Context, productID uuid.UUID, publish bool, actorID string) (*model.CompetitionResult, error) {
	res, err := s.storeRepo.SetCompetitionResultPublished(ctx, productID, publish, actorID)
	return res, mapCompetitionResultNotFound(err)
}

func (s *Service) DeleteCompetitionResult(ctx context.Context, productID uuid.UUID, actorID string) error {
	return mapCompetitionResultNotFound(s.storeRepo.DeleteCompetitionResult(ctx, productID, actorID))
}

func mapCompetitionResultNotFound(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrCompetitionResultNotFound
	}
	return err
}

func (s *Service) ListCompetitionResultRows(ctx context.Context, productID uuid.UUID) ([]model.CompetitionResultRow, error) {
	result, err := s.storeRepo.GetCompetitionResult(ctx, productID)
	if err != nil {
		return nil, mapCompetitionResultNotFound(err)
	}
	rows, err := s.storeRepo.ListCompetitionResultRows(ctx, result.ID)
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
		rows[i].ParticipantNo = formatParticipantNo(derefPrefix(rows[i].ExamPrefix), date, examNo, *rows[i].ParticipantNumber)
	}
	return rows, nil
}

// attachCompetitionResults fills CompetitionResult on every item with one query.
func (s *Service) attachCompetitionResults(ctx context.Context, items []model.RegistrationListItem) error {
	ids := make([]uuid.UUID, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	results, err := s.storeRepo.ListPublishedCompetitionResults(ctx, ids)
	if err != nil {
		return err
	}
	byReg := map[uuid.UUID]*model.StudentCompetitionResult{}
	for i := range results {
		byReg[results[i].RegistrationID] = &results[i]
	}
	for i := range items {
		items[i].CompetitionResult = byReg[items[i].ID]
	}
	return nil
}
