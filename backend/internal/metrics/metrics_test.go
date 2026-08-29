package metrics

import (
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus/testutil"
)

// The registry is a process-wide singleton; every assertion here reads the
// exposition endpoint the api and worker would serve on :9102.
func TestHandlerExposesExpectedFamilies(t *testing.T) {
	HTTPRequestsTotal.WithLabelValues("/api/v1/exams/:id", "GET", "200").Inc()
	HTTPRequestDuration.WithLabelValues("/api/v1/exams/:id", "GET").Observe(0.042)
	ObservePasswordVerify(OpLogin, 230*time.Millisecond)

	res := httptest.NewRecorder()
	Handler().ServeHTTP(res, httptest.NewRequest("GET", "/metrics", nil))
	body := res.Body.String()

	for _, family := range []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"login_bcrypt_seconds",
		"go_goroutines",
		"process_resident_memory_bytes",
	} {
		if !strings.Contains(body, family) {
			t.Errorf("exposition missing family %q", family)
		}
	}

	if got := testutil.ToFloat64(HTTPRequestsTotal.WithLabelValues("/api/v1/exams/:id", "GET", "200")); got != 1 {
		t.Errorf("http_requests_total = %v, want 1", got)
	}
}

// exam_sessions_active must stay worker-only: Registry() alone — what the api
// process builds — must not export it (it would read 0 forever and split the
// dashboard tile in two), while RegisterWorkerMetrics() opts the worker in.
// Both phases live in one test because the singleton registry is shared by
// the whole test binary.
func TestExamSessionsActiveRegisteredOnlyByWorker(t *testing.T) {
	res := httptest.NewRecorder()
	Handler().ServeHTTP(res, httptest.NewRequest("GET", "/metrics", nil))
	if strings.Contains(res.Body.String(), "exam_sessions_active") {
		t.Error("api registry exports exam_sessions_active; only the worker should")
	}

	ExamSessionsActive.Set(1234)
	RegisterWorkerMetrics()

	res = httptest.NewRecorder()
	Handler().ServeHTTP(res, httptest.NewRequest("GET", "/metrics", nil))
	if !strings.Contains(res.Body.String(), "exam_sessions_active") {
		t.Error("worker registry missing exam_sessions_active after RegisterWorkerMetrics")
	}
	if got := testutil.ToFloat64(ExamSessionsActive); got != 1234 {
		t.Errorf("exam_sessions_active = %v, want 1234", got)
	}
}
