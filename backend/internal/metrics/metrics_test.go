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
	ObservePasswordVerify(230 * time.Millisecond)
	ExamSessionsActive.Set(1234)

	res := httptest.NewRecorder()
	Handler().ServeHTTP(res, httptest.NewRequest("GET", "/metrics", nil))
	body := res.Body.String()

	for _, family := range []string{
		"http_requests_total",
		"http_request_duration_seconds",
		"login_bcrypt_seconds",
		"exam_sessions_active",
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
	if got := testutil.ToFloat64(ExamSessionsActive); got != 1234 {
		t.Errorf("exam_sessions_active = %v, want 1234", got)
	}

	// Route label must carry the template, never a raw session id.
	if strings.Contains(body, "/api/v1/exams/") && !strings.Contains(body, `route="/api/v1/exams/:id"`) {
		t.Error("route label leaked a raw URI instead of the template")
	}
}
