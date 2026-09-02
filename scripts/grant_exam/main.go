// Grant student to exam — resolve usernames to student IDs, then bulk-enroll
// them into an exam from a CSV of exam_id + username.
//
// For each username, GET /api/v1/admin/exam-grants/students/search?q=<username>
// to resolve student id (exact username match). Then POST /api/v1/admin/exam-grants
// in batches of 20 students per exam.
//
// Input CSV (header required):
//
//	exam_id,username
//	<exam-uuid>,zalf6539
//	<exam-uuid>,"user-a,user-b"
//
// Output CSV: exam_id,username,student_id,status,http_status,name,error with
// status granted | skipped | failed (skipped = already registered).
//
// Operator guide with examples: docs/runbooks/admin-scripts.md.
//
//	go run scripts/grant_exam/main.go
package main

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	baseURL    = "http://localhost:8080"
	token      = ""
	inputFile  = "scripts/exam_grants.csv"
	outputFile = "scripts/exam_grants_result.csv"
	batchSize  = 20
)

type grantRequest struct {
	ExamID     string   `json:"exam_id"`
	StudentIDs []string `json:"student_ids"`
}

type grantedStudent struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
}

type grantResponse struct {
	GrantedCount    int              `json:"granted_count"`
	GrantedStudents []grantedStudent `json:"granted_students"`
	Code            string           `json:"code"`
	Message         string           `json:"message"`
}

type searchStudent struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Username *string `json:"username"`
}

type searchResponse struct {
	Data       []searchStudent `json:"data"`
	NextCursor string          `json:"next_cursor"`
	Code       string          `json:"code"`
	Message    string          `json:"message"`
}

type studentRef struct {
	ID       string
	Username string
	Name     string
}

type resultRow struct {
	ExamID     string
	Username   string
	StudentID  string
	Status     string
	HTTPStatus int
	Name       string
	Error      string
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
		fatal("no exam/username rows in " + inputFile)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	lookupCache := map[string]lookupResult{}

	var results []resultRow
	var resolved []inputResolved
	var granted, skipped, failed int

	for _, row := range rows {
		lr, ok := lookupCache[strings.ToLower(row.Username)]
		if !ok {
			lr = lookupUsername(client, row.Username)
			lookupCache[strings.ToLower(row.Username)] = lr
		}
		if lr.Err != "" {
			results = append(results, resultRow{
				ExamID:     row.ExamID,
				Username:   row.Username,
				StudentID:  lr.ID,
				Status:     "failed",
				HTTPStatus: lr.HTTPStatus,
				Name:       lr.Name,
				Error:      lr.Err,
			})
			failed++
			continue
		}
		resolved = append(resolved, inputResolved{
			ExamID: row.ExamID,
			Ref: studentRef{
				ID:       lr.ID,
				Username: row.Username,
				Name:     lr.Name,
			},
		})
	}

	for _, g := range groupByExam(resolved) {
		for _, batch := range chunkRefs(g.Students, batchSize) {
			batchResults := grantBatch(client, g.ExamID, batch)
			results = append(results, batchResults...)
			for _, r := range batchResults {
				switch r.Status {
				case "granted":
					granted++
				case "skipped":
					skipped++
				default:
					failed++
				}
			}
		}
	}

	if err := writeOutput(outputFile, results); err != nil {
		fatal(err.Error())
	}

	fmt.Printf("wrote %s  granted=%d  already_registered=%d  failed=%d\n",
		outputFile, granted, skipped, failed)
	if failed > 0 {
		os.Exit(1)
	}
}

type inputRow struct {
	ExamID   string
	Username string
}

type inputResolved struct {
	ExamID string
	Ref    studentRef
}

type lookupResult struct {
	ID         string
	Name       string
	HTTPStatus int
	Err        string
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

	examCol, userCol := -1, -1
	for i, h := range header {
		switch strings.ToLower(strings.TrimSpace(h)) {
		case "exam_id":
			examCol = i
		case "username", "usernames":
			userCol = i
		}
	}
	if examCol < 0 || userCol < 0 {
		return nil, fmt.Errorf("csv must have exam_id and username columns")
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
		if examCol >= len(rec) || userCol >= len(rec) {
			continue
		}
		examID := strings.TrimSpace(rec[examCol])
		if examID == "" {
			continue
		}
		for _, u := range parseList(rec[userCol]) {
			out = append(out, inputRow{ExamID: examID, Username: u})
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

func lookupUsername(client *http.Client, username string) lookupResult {
	cursor := ""
	for {
		status, body, err := searchStudents(client, username, cursor)
		if err != nil {
			return lookupResult{HTTPStatus: status, Err: err.Error()}
		}
		var resp searchResponse
		if err := json.Unmarshal(body, &resp); err != nil {
			return lookupResult{HTTPStatus: status, Err: "invalid search response: " + err.Error()}
		}
		if status != http.StatusOK {
			msg := strings.TrimSpace(string(body))
			if resp.Code != "" || resp.Message != "" {
				msg = strings.TrimSpace(resp.Code + " " + resp.Message)
			}
			return lookupResult{HTTPStatus: status, Err: msg}
		}

		var matches []searchStudent
		for _, s := range resp.Data {
			if s.Username != nil && strings.EqualFold(*s.Username, username) {
				matches = append(matches, s)
			}
		}
		if len(matches) == 1 {
			return lookupResult{ID: matches[0].ID, Name: matches[0].Name, HTTPStatus: status}
		}
		if len(matches) > 1 {
			return lookupResult{HTTPStatus: status, Err: "ambiguous username match"}
		}
		if resp.NextCursor == "" {
			return lookupResult{HTTPStatus: status, Err: "username not found"}
		}
		cursor = resp.NextCursor
	}
}

func searchStudents(client *http.Client, q, cursor string) (int, []byte, error) {
	u, err := url.Parse(strings.TrimRight(baseURL, "/") + "/api/v1/admin/exam-grants/students/search")
	if err != nil {
		return 0, nil, err
	}
	qs := u.Query()
	qs.Set("q", q)
	qs.Set("limit", "100")
	if cursor != "" {
		qs.Set("cursor", cursor)
	}
	u.RawQuery = qs.Encode()

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
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

type examGroup struct {
	ExamID   string
	Students []studentRef
}

func groupByExam(rows []inputResolved) []examGroup {
	order := make([]string, 0)
	seenExam := map[string]bool{}
	groups := map[string][]studentRef{}
	seenStudent := map[string]map[string]bool{}

	for _, row := range rows {
		if !seenExam[row.ExamID] {
			seenExam[row.ExamID] = true
			order = append(order, row.ExamID)
			seenStudent[row.ExamID] = map[string]bool{}
		}
		if seenStudent[row.ExamID][row.Ref.ID] {
			continue
		}
		seenStudent[row.ExamID][row.Ref.ID] = true
		groups[row.ExamID] = append(groups[row.ExamID], row.Ref)
	}

	out := make([]examGroup, 0, len(order))
	for _, examID := range order {
		out = append(out, examGroup{ExamID: examID, Students: groups[examID]})
	}
	return out
}

func chunkRefs(refs []studentRef, size int) [][]studentRef {
	if size <= 0 {
		size = 20
	}
	var out [][]studentRef
	for i := 0; i < len(refs); i += size {
		end := i + size
		if end > len(refs) {
			end = len(refs)
		}
		out = append(out, refs[i:end])
	}
	return out
}

func grantBatch(client *http.Client, examID string, students []studentRef) []resultRow {
	failAll := func(httpStatus int, errMsg string) []resultRow {
		rows := make([]resultRow, 0, len(students))
		for _, s := range students {
			rows = append(rows, resultRow{
				ExamID:     examID,
				Username:   s.Username,
				StudentID:  s.ID,
				Status:     "failed",
				HTTPStatus: httpStatus,
				Name:       s.Name,
				Error:      errMsg,
			})
		}
		return rows
	}

	ids := make([]string, len(students))
	for i, s := range students {
		ids[i] = s.ID
	}

	status, body, err := grant(client, examID, ids)
	if err != nil {
		return failAll(0, err.Error())
	}

	var resp grantResponse
	_ = json.Unmarshal(body, &resp)

	if status != http.StatusCreated {
		msg := strings.TrimSpace(string(body))
		if resp.Code != "" || resp.Message != "" {
			msg = strings.TrimSpace(resp.Code + " " + resp.Message)
		}
		return failAll(status, msg)
	}

	grantedByID := map[string]grantedStudent{}
	for _, s := range resp.GrantedStudents {
		grantedByID[s.ID] = s
	}

	rows := make([]resultRow, 0, len(students))
	for _, s := range students {
		r := resultRow{
			ExamID:     examID,
			Username:   s.Username,
			StudentID:  s.ID,
			HTTPStatus: status,
			Name:       s.Name,
		}
		if g, ok := grantedByID[s.ID]; ok {
			r.Status = "granted"
			r.Name = g.Name
			r.Username = g.Username
		} else {
			r.Status = "skipped"
			r.Error = "already registered"
		}
		rows = append(rows, r)
	}
	return rows
}

func grant(client *http.Client, examID string, studentIDs []string) (int, []byte, error) {
	payload, err := json.Marshal(grantRequest{
		ExamID:     examID,
		StudentIDs: studentIDs,
	})
	if err != nil {
		return 0, nil, err
	}

	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/api/v1/admin/exam-grants", bytes.NewReader(payload))
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

func writeOutput(path string, rows []resultRow) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	if err := w.Write([]string{
		"exam_id", "username", "student_id", "status", "http_status", "name", "error",
	}); err != nil {
		return err
	}
	for _, r := range rows {
		if err := w.Write([]string{
			r.ExamID,
			r.Username,
			r.StudentID,
			r.Status,
			fmt.Sprintf("%d", r.HTTPStatus),
			r.Name,
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
