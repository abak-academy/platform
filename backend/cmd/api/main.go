package main

import (
	"akademi-bimbel/config"
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"akademi-bimbel/internal/adapter"
	"akademi-bimbel/internal/handler"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/server"
	"akademi-bimbel/internal/service"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
)

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load(envDefault("APP_ENV", "dev"), envDefault("CONFIG_DIR", "config/env"))
	if err != nil {
		logger.Error("load config", "err", err)
		os.Exit(1)
	}
	ctx := context.Background()

	if err := infra.RunMigrations(ctx, cfg.MigrationDatabaseURL); err != nil {
		logger.Error("run migrations", "err", err)
		os.Exit(1)
	}

	pool, err := infra.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := infra.NewRedis(cfg.RedisAddr, cfg.RedisPassword)
	defer rdb.Close()

	storeRepo := repository.New(pool)
	jwtSigner := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	otpProvider, emailProvider := newNotifyProviders(cfg)
	paymentClient := adapter.ResolvePaymentClient(ctx, storeRepo, &cfg)
	logisticsClient := adapter.ResolveShippingClient(ctx, storeRepo, &cfg)
	storageClient := newStorageClient(cfg)
	pdfGenerator := service.NewGotenbergPDFGenerator(cfg.GotenbergURL, http.DefaultClient)
	svc := service.NewWithStore(storeRepo, storeRepo, rdb, jwtSigner, otpProvider, emailProvider, paymentClient, logisticsClient, storageClient, &cfg, pdfGenerator)
	svc.SetReloadPaymentFn(func(ctx context.Context) service.PaymentClient {
		return adapter.ResolvePaymentClient(ctx, storeRepo, &cfg)
	})
	svc.SetReloadLogisticsFn(func(ctx context.Context) service.LogisticsClient {
		return adapter.ResolveShippingClient(ctx, storeRepo, &cfg)
	})
	h := handler.New(svc)
	e := server.New(h, svc, jwtSigner, cfg)

	go func() {
		if err := e.Start(":" + cfg.HTTPPort); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("http server", "err", err)
			os.Exit(1)
		}
	}()
	logger.Info("api started", "port", cfg.HTTPPort, "env", cfg.Env)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := e.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown", "err", err)
	}
	logger.Info("api stopped")
}

func newNotifyProviders(cfg config.Config) (service.OTPProvider, service.EmailProvider) {
	// SMTP (email OTP) takes precedence when configured — staging uses it.
	if cfg.SMTPHost != "" {
		sm := adapter.NewSMTPProvider(adapter.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			FromName: cfg.SMTPFromName,
		})
		return sm, sm
	}
	if cfg.FazpassMerchantKey == "" || cfg.FazpassAPIKey == "" {
		return &adapter.NoopOTPProvider{}, &adapter.NoopEmailProvider{}
	}
	fz := adapter.NewFazpassProvider(adapter.FazpassConfig{
		MerchantKey: cfg.FazpassMerchantKey,
		APIKey:      cfg.FazpassAPIKey,
		BaseURL:     cfg.FazpassBaseURL,
	})
	return fz, fz
}

func newStorageClient(cfg config.Config) *minio.Client {
	client, err := minio.New(cfg.ObjectStorageEndpoint, &minio.Options{
		Creds:  credentials.NewStaticV4(cfg.ObjectStorageAccessKey, cfg.ObjectStorageSecretKey, ""),
		Secure: cfg.ObjectStorageUseSSL,
		Region: cfg.ObjectStorageRegion,
	})
	if err != nil {
		slog.Default().Error("init minio client", "err", err)
		return nil
	}
	return client
}
