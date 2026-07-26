package service

import (
	"akademi-bimbel/config"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"context"
	"net/http"
	"sync"
	"sync/atomic"

	"github.com/minio/minio-go/v7"
	"github.com/redis/go-redis/v9"
)

type Service struct {
	repo          UserRepository
	storeRepo     *repository.Repository
	rdb           *redis.Client
	jwtSigner     *infra.JWTSigner
	otpProvider   OTPProvider
	emailProvider EmailProvider
	payment       PaymentClient
	// logistics is swapped by ReloadLogisticsClient while quote requests read
	// it concurrently, so it's held behind an atomic pointer rather than a
	// plain field.
	logistics     atomic.Pointer[LogisticsClient]
	storage       *minio.Client
	announceRepo  AnnounceRepo
	presignOnce   sync.Once
	presignClient *minio.Client
	cfg           *config.Config
	renderer      certificateRenderer

	// reloadPaymentFn is called by ReloadPaymentClient to rebuild the
	// payment client from current config (DB or env). Injected by main.
	reloadPaymentFn func(ctx context.Context) PaymentClient

	// reloadLogisticsFn is called by ReloadLogisticsClient to rebuild the
	// logistics client from current config (DB or env). Injected by main.
	reloadLogisticsFn func(ctx context.Context) LogisticsClient
}

// NewForTest builds a Service with only a Redis client — sufficient for middleware tests.
func NewForTest(rdb *redis.Client) *Service {
	return &Service{rdb: rdb}
}

func New(
	repo UserRepository,
	rdb *redis.Client,
	jwtSigner *infra.JWTSigner,
	otpProvider OTPProvider,
	emailProvider EmailProvider,
	cfg *config.Config,
) *Service {
	return &Service{
		repo:          repo,
		rdb:           rdb,
		jwtSigner:     jwtSigner,
		otpProvider:   otpProvider,
		emailProvider: emailProvider,
		cfg:           cfg,
	}
}

func NewWithStore(
	repo UserRepository,
	storeRepo *repository.Repository,
	rdb *redis.Client,
	jwtSigner *infra.JWTSigner,
	otpProvider OTPProvider,
	emailProvider EmailProvider,
	payment PaymentClient,
	logistics LogisticsClient,
	storage *minio.Client,
	cfg *config.Config,
) *Service {
	// cfg is nil in many unit/integration tests that don't render certificates;
	// guard the renderer URL so construction never nil-derefs.
	gotenbergURL := ""
	if cfg != nil {
		gotenbergURL = cfg.GotenbergURL
	}
	s := &Service{
		repo:          repo,
		storeRepo:     storeRepo,
		rdb:           rdb,
		jwtSigner:     jwtSigner,
		otpProvider:   otpProvider,
		emailProvider: emailProvider,
		payment:       payment,
		storage:       storage,
		announceRepo:  storeRepo,
		cfg:           cfg,
		renderer:      newGotenbergRenderer(gotenbergURL, http.DefaultClient),
	}
	s.logistics.Store(&logistics)
	return s
}

type Health struct {
	Status   string `json:"status"`
	Postgres string `json:"postgres"`
	Redis    string `json:"redis"`
}

func (s *Service) ParseAccess(tokenString string) (*infra.Claims, error) {
	return s.jwtSigner.ParseAccess(tokenString)
}

// SetReloadPaymentFn sets the callback used by ReloadPaymentClient to
// rebuild the payment client from current config.
func (s *Service) SetReloadPaymentFn(fn func(ctx context.Context) PaymentClient) {
	s.reloadPaymentFn = fn
}

// ReloadPaymentClient replaces s.payment by calling the injected reload
// function. No-op when no reload function has been set.
func (s *Service) ReloadPaymentClient(ctx context.Context) {
	if s.reloadPaymentFn == nil {
		return
	}
	s.payment = s.reloadPaymentFn(ctx)
}

// SetReloadLogisticsFn sets the callback used by ReloadLogisticsClient to
// rebuild the logistics client from current config.
func (s *Service) SetReloadLogisticsFn(fn func(ctx context.Context) LogisticsClient) {
	s.reloadLogisticsFn = fn
}

// ReloadLogisticsClient replaces s.logistics by calling the injected reload
// function. No-op when no reload function has been set.
func (s *Service) ReloadLogisticsClient(ctx context.Context) {
	if s.reloadLogisticsFn == nil {
		return
	}
	client := s.reloadLogisticsFn(ctx)
	s.logistics.Store(&client)
}

// logisticsClient returns the currently active logistics client.
func (s *Service) logisticsClient() LogisticsClient {
	p := s.logistics.Load()
	if p == nil {
		return nil
	}
	return *p
}

func (s *Service) Health(ctx context.Context) Health {
	h := Health{Status: "ok", Postgres: "ok", Redis: "ok"}
	if err := s.repo.Ping(ctx); err != nil {
		h.Postgres = "down"
		h.Status = "degraded"
	}
	if err := s.rdb.Ping(ctx).Err(); err != nil {
		h.Redis = "down"
		h.Status = "degraded"
	}
	return h
}
