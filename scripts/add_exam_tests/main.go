// Add test to exam — attach tests to an exam from a CSV of exam_id + test_id.
//
// PUT /api/v1/admin/exams/:id/tests is replace-semantics (the body is the FULL
// list of attached test IDs), so for each exam this script first GETs the exam
// detail to read currently attached tests, merges the requested test IDs
// (skipping ones already attached), then PUTs the merged list in one call.
//
// Input CSV (header required):
//
//	exam_id,test_id
//	<exam-uuid>,<test-uuid>
//	<exam-uuid>,"<test-uuid-1>,<test-uuid-2>"
//
// Output CSV: exam_id,test_id,status,http_status,error with status
// attached | skipped | failed (skipped = already attached).
//
// Token needs products(exam):write (PUT) and products(exam):read (GET).
//
// Operator guide with examples: docs/runbooks/admin-scripts.md.
//
//	go run scripts/add_exam_tests/main.go
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const (
	baseURL    = "http://localhost:8080"
	token      = ""
	inputFile  = "scripts/add_exam_tests.csv"
	outputFile = "scripts/add_exam_tests_result.csv"
)

type inputRow struct {
	ExamID string
	TestID string
}

type examGroup struct {
	ExamID  string
	TestIDs []string
}

type resultRow struct {
	ExamID     string
	TestID     string
	Status     string
	HTTPStatus int
	Error      string
}

type examTestEntry struct {
	TestID string `json:"test_id"`
}

type examDetail struct {
	Tests []examTestEntry `json:"tests"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func main() {
	if token == "" {
		fatal("set token const")
	}

	rows, err := readInput(inputFile)
	if err != nil {
		fatal(err.Error())
	}
	if len(rows) == 0 {
		fatal("no exam/test rows in " + inputFile)
	}

	client := &http.Client{Timeout: 30 * time.Second}

	var results []resultRow
	var attached, skipped, failed int

	for _, g := range groupByExam(rows) {
		groupResults := attachTests(client, g)
		results = append(results, groupResults...)
		for _, r := range groupResults {
			switch r.Status {
			case "attached":
				attached++
			case "skipped":
				skipped++
			default:
				failed++
			}
		}
	}

	if err := writeOutput(outputFile, results); err != nil {
		fatal(err.Error())
	}

	fmt.Printf("wrote %s  attached=%d  already_attached=%d  failed=%d\n",
		outputFile, attached, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

func readInput(path string) ([]inputRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.TrimLeadingSpace = true
	header, err := r.Read()
	if err != nil {
		return nil, fmt.Errorf("read csv header: %w", err)
	}

	examCol, testCol := -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "exam_id":
			examCol = i
		case "test_id", "test_ids":
			testCol = i
		}
	}
	if examCol < 0 || testCol < 0 {
		return nil, fmt.Errorf("csv must have exam_id and test_id columns")
	}

	var out []inputRow
	line := 1
	for {
		rec, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("csv line %d: %w", line+1, err)
		}
		line++
		if examCol >= len(rec) || testCol >= len(rec) {
			continue
		}
		examID := strings.TrimSpace(rec[examCol])
		if examID == "" {
			continue
		}
		for _, id := range parseList(rec[testCol]) {
			out = append(out, inputRow{ExamID: examID, TestID: id})
		}
	}
	return out, nil
}

func parseList(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "[") {
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err == nil {
			return cleanIDs(ids)
		}
	}
	if strings.Contains(raw, ",") {
		return cleanIDs(strings.Split(raw, ","))
	}
	return []string{raw}
}

func cleanIDs(ids []string) []string {
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(strings.Trim(id, `"'`))
		if id != "" {
			out = append(out, id)
		}
	}
	return out
}

func groupByExam(rows []inputRow) []examGroup {
	order := make([]string, 0)
	seenExam := map[string]bool{}
	groups := map[string][]string{}
	seenTest := map[string]map[string]bool{}

	for _, row := range rows {
		if !seenExam[row.ExamID] {
			seenExam[row.ExamID] = true
			order = append(order, row.ExamID)
			seenTest[row.ExamID] = map[string]bool{}
		}
		if seenTest[row.ExamID][row.TestID] {
			continue
		}
		seenTest[row.ExamID][row.TestID] = true
		groups[row.ExamID] = append(groups[row.ExamID], row.TestID)
	}

	out := make([]examGroup, 0, len(order))
	for _, examID := range order {
		out = append(out, examGroup{ExamID: examID, TestIDs: groups[examID]})
	}
	return out
}

func attachTests(client *http.Client, g examGroup) []resultRow {
	failAll := func(httpStatus int, errMsg string) []resultRow {
		rows := make([]resultRow, 0, len(g.TestIDs))
		for _, id := range g.TestIDs {
			rows = append(rows, resultRow{
				ExamID:     g.ExamID,
				TestID:     id,
				Status:     "failed",
				HTTPStatus: httpStatus,
				Error:      errMsg,
			})
		}
		return rows
	}

	status, body, err := getExam(client, g.ExamID)
	if err != nil {
		return failAll(status, err.Error())
	}
	if status != http.StatusOK {
		return failAll(status, errorMessage(body))
	}

	var detail examDetail
	if err := json.Unmarshal(body, &detail); err != nil {
		return failAll(status, "invalid exam response: "+err.Error())
	}

	current := make([]string, 0, len(detail.Tests))
	currentSet := map[string]bool{}
	for _, t := range detail.Tests {
		if t.TestID != "" && !currentSet[t.TestID] {
			currentSet[t.TestID] = true
			current = append(current, t.TestID)
		}
	}

	var newIDs []string
	seen := map[string]bool{}
	for _, id := range g.TestIDs {
		if currentSet[id] || seen[id] {
			continue
		}
		seen[id] = true
		newIDs = append(newIDs, id)
	}

	if len(newIDs) == 0 {
		rows := make([]resultRow, 0, len(g.TestIDs))
		for _, id := range g.TestIDs {
			rows = append(rows, resultRow{
				ExamID:     g.ExamID,
				TestID:     id,
				Status:     "skipped",
				HTTPStatus: status,
				Error:      "already attached",
			})
		}
		return rows
	}

	status, body, err = replaceExamTests(client, g.ExamID, append(current, newIDs...))
	if err != nil {
		return failAll(0, err.Error())
	}
	if status != http.StatusNoContent {
		return failAll(status, errorMessage(body))
	}

	rows := make([]resultRow, 0, len(g.TestIDs))
	for _, id := range g.TestIDs {
		r := resultRow{ExamID: g.ExamID, TestID: id, HTTPStatus: status}
		if currentSet[id] {
			r.Status = "skipped"
			r.Error = "already attached"
		} else {
			r.Status = "attached"
		}
		rows = append(rows, r)
	}
	return rows
}

func getExam(client *http.Client, examID string) (int, []byte, error) {
	req, err := http.NewRequest(http.MethodGet, strings.TrimRight(baseURL, "/")+"/api/v1/admin/exams/"+examID, nil)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, nil, err
	}
	return res.StatusCode, body, nil
}

func replaceExamTests(client *http.Client, examID string, testIDs []string) (int, []byte, error) {
	payload, err := json.Marshal(testIDs)
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequest(http.MethodPut, strings.TrimRight(baseURL, "/")+"/api/v1/admin/exams/"+examID+"/tests", bytes.NewReader(payload))
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	res, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		return res.StatusCode, nil, err
	}
	return res.StatusCode, body, nil
}

func errorMessage(body []byte) string {
	var resp apiError
	if err := json.Unmarshal(body, &resp); err == nil && (resp.Code != "" || resp.Message != "") {
		return strings.TrimSpace(resp.Code + " " + resp.Message)
	}
	return strings.TrimSpace(string(body))
}

func writeOutput(path string, rows []resultRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"exam_id", "test_id", "status", "http_status", "error",
	}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{
			r.ExamID,
			r.TestID,
			r.Status,
			fmt.Sprintf("%d", r.HTTPStatus),
			r.Error,
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func fatal(msg string) {
	fmt.Fprintln(os.Stderr, msg)
	os.Exit(2)
}
