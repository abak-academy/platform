package main

import (
	"akademi-bimbel/config"
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
	"github.com/redis/go-redis/v9"

	"akademi-bimbel/internal/adapter"
	"akademi-bimbel/internal/infra"
	"akademi-bimbel/internal/repository"
	"akademi-bimbel/internal/service"
	"akademi-bimbel/internal/worker"
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

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := infra.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("connect postgres", "err", err)
		os.Exit(1)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisAddr,
		Password: cfg.RedisPassword,
	})
	defer rdb.Close()

	repo := repository.New(pool)
	jwtSigner := infra.NewJWTSigner(cfg.JWTSecret, cfg.AccessTokenTTL)
	emailProvider := newEmailProvider(cfg)
	paymentClient := adapter.ResolvePaymentClient(ctx, repo, &cfg)
	logisticsClient := adapter.ResolveShippingClient(ctx, repo, &cfg)
	storageClient := newStorageClient(cfg)

	pdfGenerator := service.NewGotenbergPDFGenerator(cfg.GotenbergURL, http.DefaultClient)
	svc := service.NewWithStore(repo, repo, rdb, jwtSigner, &service.NoopOTPProvider{}, emailProvider, paymentClient, logisticsClient, storageClient, &cfg, pdfGenerator)
	objectStore := worker.NewMinioObjectStore(storageClient)

	sweeperInterval := 5 * time.Minute
	announcementPollInterval := 5 * time.Minute
	w := worker.New(pool, rdb, repo, cfg.WorkerPollInterval, sweeperInterval, announcementPollInterval, svc, repo, objectStore, svc, cfg.WorkerPollInterval, cfg.ObjectStoragePrivateBucketName, svc)
	logger.Info("worker started", "poll_interval", cfg.WorkerPollInterval.String(), "sweeper_interval", sweeperInterval.String(), "announcement_poll_interval", announcementPollInterval.String())
	w.Run(ctx)
	logger.Info("worker stopped")
}

func newEmailProvider(cfg config.Config) service.EmailProvider {
	if cfg.SMTPHost != "" {
		return adapter.NewSMTPProvider(adapter.SMTPConfig{
			Host:     cfg.SMTPHost,
			Port:     cfg.SMTPPort,
			Username: cfg.SMTPUsername,
			Password: cfg.SMTPPassword,
			From:     cfg.SMTPFrom,
			FromName: cfg.SMTPFromName,
		})
	}
	if cfg.FazpassMerchantKey == "" || cfg.FazpassAPIKey == "" {
		return &adapter.NoopEmailProvider{}
	}
	return adapter.NewFazpassProvider(adapter.FazpassConfig{
		MerchantKey: cfg.FazpassMerchantKey,
		APIKey:      cfg.FazpassAPIKey,
		BaseURL:     cfg.FazpassBaseURL,
	})
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
