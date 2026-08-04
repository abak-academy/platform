package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// QuestionImportRow is one CSV row after header parsing.
type QuestionImportRow struct {
	Format        string
	Body          string
	Subject       string
	Topic         string
	Difficulty    *string
	PointCorrect  float64
	PointWrong    float64
	CorrectAnswer *string
	Options       []model.QuestionOption
	Error         string
}

// QuestionImportResult is the per-row report for a question import.
type QuestionImportResult struct {
	Inserted int                       `json:"inserted"`
	Rows     []QuestionImportResultRow `json:"rows"`
}

// QuestionImportResultRow reports the outcome of one CSV row.
type QuestionImportResultRow struct {
	RowNumber  int        `json:"row_number"`
	Status     string     `json:"status"`
	QuestionID *uuid.UUID `json:"question_id,omitempty"`
	Error      string     `json:"error,omitempty"`
}

// QuestionImportRequiredHeaders is the required header list for question import
// CSVs, in parser order. BuildQuestionImportTemplate reads from this same slice
// so the parser and the generated template cannot drift (FR-11).
var QuestionImportRequiredHeaders = []string{"format", "body", "subject", "topic", "point_correct", "point_wrong"}

// optionalHeaders are the optional columns included in the generated template.
var optionalHeaders = []string{"difficulty", "correct_answer", "option_a", "option_b", "option_c", "option_d"}

// ParseQuestionImportCSV reads a question CSV. Required headers (case-insensitive):
//
//	format, body, subject, topic, point_correct, point_wrong
//
// Optional headers:
//
//	difficulty, correct_answer, option_a, option_b, option_c, option_d, ...
//
// Any column whose lowercase name starts with "option_" is treated as an option;
// the key is the suffix (option_a -> key "a"). Empty option cells are skipped.
func ParseQuestionImportCSV(data []byte) ([]QuestionImportRow, error) {
	r := csv.NewReader(bytes.NewReader(data))

	header, err := r.Read()
	if err != nil {
		if err == io.EOF {
			return nil, ErrInvalidCSV
		}
		return nil, ErrInvalidCSV
	}

	colIndex := map[string]int{}
	optionCols := []struct {
		idx int
		key string
	}{}
	for i, h := range header {
		name := strings.ToLower(strings.TrimSpace(h))
		colIndex[name] = i
		if strings.HasPrefix(name, "option_") {
			key := strings.TrimPrefix(name, "option_")
			optionCols = append(optionCols, struct {
				idx int
				key string
			}{idx: i, key: key})
		}
	}

	for _, h := range QuestionImportRequiredHeaders {
		if _, ok := colIndex[h]; !ok {
			return nil, fmt.Errorf("%w: missing required header %q", ErrValidation, h)
		}
	}

	var rows []QuestionImportRow
	line := 1
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, ErrInvalidCSV
		}
		line++

		get := func(name string) string {
			idx, ok := colIndex[name]
			if !ok || idx >= len(record) {
				return ""
			}
			return strings.TrimSpace(record[idx])
		}

		pointCorrect, pcErr := parseImportFloat(get("point_correct"), "point_correct")
		pointWrong, pwErr := parseImportFloat(get("point_wrong"), "point_wrong")

		rowErr := func() string {
			if pcErr != nil {
				return pcErr.Error()
			}
			if pwErr != nil {
				return pwErr.Error()
			}
			return ""
		}()

		var difficulty *string
		if d := get("difficulty"); d != "" {
			difficulty = &d
		}

		var correctAnswer *string
		if ca := get("correct_answer"); ca != "" {
			correctAnswer = &ca
		}

		var options []model.QuestionOption
		for _, opt := range optionCols {
			if opt.idx >= len(record) {
				continue
			}
			text := strings.TrimSpace(record[opt.idx])
			if text == "" {
				continue
			}
			isCorrect := false
			if correctAnswer != nil {
				isCorrect = importCorrectKeyMatches(*correctAnswer, opt.key)
			}
			options = append(options, model.QuestionOption{
				Key:       opt.key,
				Text:      text,
				IsCorrect: isCorrect,
				SortOrder: len(options) + 1,
			})
		}

		rows = append(rows, QuestionImportRow{
			Format:        get("format"),
			Body:          get("body"),
			Subject:       get("subject"),
			Topic:         get("topic"),
			Difficulty:    difficulty,
			PointCorrect:  pointCorrect,
			PointWrong:    pointWrong,
			CorrectAnswer: correctAnswer,
			Options:       options,
			Error:         rowErr,
		})
	}

	return rows, nil
}

func parseImportFloat(raw, field string) (float64, error) {
	if raw == "" {
		return 0, fmt.Errorf("%w: %s cannot be empty", ErrValidation, field)
	}
	v, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %s must be a number", ErrValidation, field)
	}
	return v, nil
}

func importCorrectKeyMatches(correctAnswer, key string) bool {
	for _, k := range strings.Split(correctAnswer, ",") {
		if strings.EqualFold(strings.TrimSpace(k), key) {
			return true
		}
	}
	return false
}

// ProcessQuestionImportRows validates and inserts each row into the bank.
// Per-row errors are collected; valid rows are inserted even when other rows fail.
func (s *Service) ProcessQuestionImportRows(ctx context.Context, rows []QuestionImportRow) (QuestionImportResult, error) {
	result := QuestionImportResult{
		Rows: make([]QuestionImportResultRow, len(rows)),
	}

	for i, row := range rows {
		res := QuestionImportResultRow{RowNumber: i + 1}

		if row.Error != "" {
			res.Status = "error"
			res.Error = row.Error
			result.Rows[i] = res
			continue
		}

		q, err := importRowToQuestion(row)
		if err != nil {
			res.Status = "error"
			res.Error = err.Error()
			result.Rows[i] = res
			continue
		}

		topicID, err := s.resolveImportTopic(ctx, row.Subject, row.Topic)
		if err != nil {
			res.Status = "error"
			res.Error = err.Error()
			result.Rows[i] = res
			continue
		}
		if topicID == nil {
			res.Status = "error"
			res.Error = fmt.Sprintf("topic not found: %s / %s", row.Subject, row.Topic)
			result.Rows[i] = res
			continue
		}
		q.TopicID = topicID

		q.Body = sanitizeQuestionBody(q.Body)
		sanitizedOpts := sanitizeQuestionOptions(row.Options)
		if err := validateQuestion(q, sanitizedOpts, nil, nil); err != nil {
			res.Status = "error"
			res.Error = err.Error()
			result.Rows[i] = res
			continue
		}

		out, err := s.CreateBankQuestion(ctx, q, sanitizedOpts, nil, nil)
		if err != nil {
			res.Status = "error"
			res.Error = err.Error()
			result.Rows[i] = res
			continue
		}

		res.Status = "inserted"
		res.QuestionID = &out.Question.ID
		result.Rows[i] = res
		result.Inserted++
	}

	return result, nil
}

func importRowToQuestion(row QuestionImportRow) (model.Question, error) {
	if row.Subject == "" {
		return model.Question{}, fmt.Errorf("%w: subject cannot be empty", ErrValidation)
	}
	if row.Topic == "" {
		return model.Question{}, fmt.Errorf("%w: topic cannot be empty", ErrValidation)
	}
	if row.Body == "" {
		return model.Question{}, fmt.Errorf("%w: body cannot be empty", ErrValidation)
	}

	q := model.Question{
		Format:       row.Format,
		Body:         row.Body,
		Difficulty:   row.Difficulty,
		PointCorrect: row.PointCorrect,
		PointWrong:   row.PointWrong,
	}
	// For option-based formats the correct answer is encoded in options.is_correct;
	// the question row must not carry correct_answer or validateQuestion rejects it.
	if row.Format == "short" || row.Format == "fill_blank" {
		q.CorrectAnswer = row.CorrectAnswer
	}
	return q, nil
}

// ImportQuestionsFromCSV parses a CSV and imports all rows. It is the single
// entry point used by the handler.
func (s *Service) ImportQuestionsFromCSV(ctx context.Context, data []byte) (QuestionImportResult, error) {
	rows, err := ParseQuestionImportCSV(data)
	if err != nil {
		return QuestionImportResult{}, err
	}
	return s.ProcessQuestionImportRows(ctx, rows)
}

// BuildQuestionImportTemplate generates a CSV template for question import: the
// header row from QuestionImportRequiredHeaders/optionalHeaders, followed by one
// example row. subject/topic are filled from the first existing exam_topic so
// the template imports unedited (FR-10/FR-12); when no topic exists, placeholder
// values are used instead.
func (s *Service) BuildQuestionImportTemplate(ctx context.Context) ([]byte, error) {
	subject, topic := "Math", "Arithmetic"
	topics, err := s.storeRepo.ListTopics(ctx, repository.TopicFilter{})
	if err != nil {
		return nil, err
	}
	if len(topics) > 0 {
		subject = topics[0].Subject
		topic = topics[0].Name
	}

	headers := append(append([]string{}, QuestionImportRequiredHeaders...), optionalHeaders...)
	// One row per format the CSV can actually express, so the file doubles as the
	// legend that exists nowhere else — an admin opening it in Excel otherwise sees
	// the bare enum "mcq" with no hint the other values exist. true_false and
	// multi_blank are deliberately absent: their answers live in child tables
	// (statements / blanks) that QuestionImportRow has no column for, so a sample
	// row for either would fail validation on import. short and fill_blank are
	// absent too — still accepted by the parser, but retired from the editor.
	examples := []map[string]string{
		{
			"format":         "mcq",
			"body":           "2 + 2 = ?",
			"subject":        subject,
			"topic":          topic,
			"point_correct":  "1",
			"point_wrong":    "0",
			"difficulty":     "easy",
			"correct_answer": "a",
			"option_a":       "4",
			"option_b":       "5",
			"option_c":       "6",
			"option_d":       "7",
		},
		{
			// correct_answer takes a comma-separated key list for multi_answer.
			"format":         "multi_answer",
			"body":           "Pilih semua bilangan genap",
			"subject":        subject,
			"topic":          topic,
			"point_correct":  "2",
			"point_wrong":    "0",
			"difficulty":     "medium",
			"correct_answer": "a,c",
			"option_a":       "2",
			"option_b":       "3",
			"option_c":       "4",
			"option_d":       "5",
		},
		{
			// essay carries no options and no correct_answer — it is graded by hand.
			"format":        "essay",
			"body":          "Jelaskan proses fotosintesis",
			"subject":       subject,
			"topic":         topic,
			"point_correct": "5",
			"point_wrong":   "0",
			"difficulty":    "hard",
		},
	}

	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(headers); err != nil {
		return nil, err
	}
	for _, example := range examples {
		row := make([]string, len(headers))
		for i, h := range headers {
			row[i] = example[h]
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// resolveImportTopic resolves a CSV row's (subject, topic) to an exam_topic ID.
// A nil return with no error means the topic was not found; callers should report
// this as a row-level validation error.
func (s *Service) resolveImportTopic(ctx context.Context, subject, topic string) (*uuid.UUID, error) {
	t, err := s.storeRepo.GetTopicByNameAndSubject(ctx, topic, subject)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, nil
		}
		return nil, err
	}
	return &t.ID, nil
}
