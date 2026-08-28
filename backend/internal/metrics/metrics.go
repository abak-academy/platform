// Package metrics owns every Prometheus series the api and worker expose.
//
// The metric objects are package-level so call sites (echo middleware in
// server, the bcrypt comparison in service) can observe without plumbing a
// struct through constructors; api and worker are separate processes, so one
// package-scoped registry per process is safe. Registration into the registry
// happens lazily on the first Handler() call — observing an unregistered
// collector still accumulates state, nothing is lost.
//
// Exposed on an internal-only port (:9102 by default, METRICS_ADDR to
// override), never routed through nginx.
package metrics

import (
	"net/http"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// §2 of issue #98: rate, p95/p99 latency and error rate per route. The
	// route label carries the echo route TEMPLATE (/api/v1/exam/sessions/:id),
	// never the raw URI — raw URIs would explode cardinality with one series
	// per session id.
	HTTPRequestsTotal = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "http_requests_total",
		Help: "HTTP requests served, labelled by route template, method and status code.",
	}, []string{"route", "method", "status"})

	HTTPRequestDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_request_duration_seconds",
		Help:    "End-to-end request duration as seen by the api process.",
		Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10},
	}, []string{"route", "method"})

	// §3: validates the ~234 ms/op bcrypt figure from the capacity model
	// against real hardware under real load. Labelled by op because
	// CompareHashAndPassword runs on both Login and ChangePassword, while the
	// capacity model (and the dashboard panel) quotes only the login series.
	LoginBcryptSeconds = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "login_bcrypt_seconds",
		Help:    "Duration of bcrypt.CompareHashAndPassword, labelled by operation (login, change_password).",
		Buckets: []float64{0.05, 0.1, 0.15, 0.2, 0.25, 0.3, 0.4, 0.6, 0.8, 1, 2},
	}, []string{"op"})

	// Password-verify call sites (op label values above).
	OpLogin          = "login"
	OpChangePassword = "change_password"

	// §4: the "N" the entire capacity arithmetic rests on. Set periodically by
	// the worker from a DB count of sessions inside their exam's live window,
	// so it stays correct across processes. Registered ONLY in the worker
	// process (RegisterWorkerMetrics) — the api never Sets it, and an
	// unconditionally registered gauge would make the api export
	// exam_sessions_active 0 forever, splitting the dashboard tile in two.
	ExamSessionsActive = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "exam_sessions_active",
		Help: "Exam sessions inside their exam's live window (students currently working).",
	})
)

var (
	regOnce    sync.Once
	reg        *prometheus.Registry
	poolOnce   sync.Once
	workerOnce sync.Once
)

// Registry lazily builds the shared registry. Runtime collectors (§5:
// goroutines, heap, GC pauses) are registered alongside the app metrics.
func Registry() *prometheus.Registry {
	regOnce.Do(func() {
		reg = prometheus.NewRegistry()
		reg.MustRegister(
			collectors.NewGoCollector(),
			collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
			HTTPRequestsTotal,
			HTTPRequestDuration,
			LoginBcryptSeconds,
		)
	})
	return reg
}

// RegisterWorkerMetrics opts the worker process into worker-only collectors.
// The api must not call this: ExamSessionsActive is Set by the worker alone,
// and an api-exported exam_sessions_active would sit at 0 forever.
func RegisterWorkerMetrics() {
	Registry()
	workerOnce.Do(func() {
		reg.MustRegister(ExamSessionsActive)
	})
}

// Handler returns the /metrics handler for the internal port.
func Handler() http.Handler {
	return promhttp.HandlerFor(Registry(), promhttp.HandlerOpts{})
}

// RegisterDBPool wires §1 (pgxpool.Stat) into the registry as scrape-time
// callbacks, so no polling goroutine is needed — values are read when
// Prometheus scrapes. Safe to call once per process; later calls are no-ops.
func RegisterDBPool(pool *pgxpool.Pool) {
	Registry()
	poolOnce.Do(func() {
		reg.MustRegister(
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: "dbpool_acquired_conns",
				Help: "Connections currently checked out of the pool.",
			}, func() float64 { return float64(pool.Stat().AcquiredConns()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: "dbpool_idle_conns",
				Help: "Idle connections held open in the pool.",
			}, func() float64 { return float64(pool.Stat().IdleConns()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: "dbpool_total_conns",
				Help: "Total connections currently established.",
			}, func() float64 { return float64(pool.Stat().TotalConns()) }),
			prometheus.NewGaugeFunc(prometheus.GaugeOpts{
				Name: "dbpool_max_conns",
				Help: "MaxConns ceiling of this pool.",
			}, func() float64 { return float64(pool.Stat().MaxConns()) }),

			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: "dbpool_acquire_total",
				Help: "Successful Acquire calls since start.",
			}, func() float64 { return float64(pool.Stat().AcquireCount()) }),

			// THE metric for the #96 condition: acquires that had to wait
			// because the pool was empty. Rising above zero = pool too small.
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: "dbpool_empty_acquire_total",
				Help: "Acquires that had to wait because the pool had no free connection.",
			}, func() float64 { return float64(pool.Stat().EmptyAcquireCount()) }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: "dbpool_acquire_duration_seconds_total",
				Help: "Cumulative time spent in Acquire (successful waits included).",
			}, func() float64 { return pool.Stat().AcquireDuration().Seconds() }),
			prometheus.NewCounterFunc(prometheus.CounterOpts{
				Name: "dbpool_canceled_acquire_total",
				Help: "Acquire calls canceled before a connection was returned.",
			}, func() float64 { return float64(pool.Stat().CanceledAcquireCount()) }),
		)
	})
}

// ObservePasswordVerify records one bcrypt comparison duration under the
// given op label (OpLogin or OpChangePassword).
func ObservePasswordVerify(op string, d time.Duration) {
	LoginBcryptSeconds.WithLabelValues(op).Observe(d.Seconds())
}
