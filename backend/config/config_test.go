package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeYAML(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0644); err != nil {
		t.Fatalf("writeYAML %s: %v", name, err)
	}
}

func TestLoad_devHappyPath(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "dev")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "9090"
redis_addr: "redis:6379"
worker_poll_interval: "10s"
cors_origins:
  - "http://localhost:3000"
  - "http://app.local:3000"
access_token_ttl: "30m"
refresh_token_ttl: "720h"
otp_ttl: "10m"
google_client_id: "gcid-test"
fazpass_base_url: "https://fazpass.test"
midtrans_env: "sandbox"
object_storage_endpoint: "minio:9000"
object_storage_public_endpoint: "minio.public:9000"
object_storage_use_ssl: true
object_storage_bucket_name: "test-bucket"
object_storage_private_bucket_name: "test-private-bucket"
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: "postgres://u:p@host/db"
jwt_secret: "jwt-secret-val"
config_encryption_key: "enc-key-val"
otp_secret: "otp-secret-val"
object_storage_access_key: "minio-ak"
object_storage_secret_key: "minio-sk"
redis_password: "redis-pw"
fazpass_merchant_key: "fz-mk"
fazpass_api_key: "fz-ak"
midtrans_server_key: "mt-sk"
midtrans_client_key: "mt-ck"
`)

	cfg, err := Load("dev", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	if cfg.Env != "dev" {
		t.Errorf("Env: got %q want %q", cfg.Env, "dev")
	}
	if cfg.HTTPPort != "9090" {
		t.Errorf("HTTPPort: got %q want %q", cfg.HTTPPort, "9090")
	}
	if cfg.DatabaseURL != "postgres://u:p@host/db" {
		t.Errorf("DatabaseURL: got %q", cfg.DatabaseURL)
	}
	if cfg.JWTSecret != "jwt-secret-val" {
		t.Errorf("JWTSecret: got %q", cfg.JWTSecret)
	}
	if cfg.AccessTokenTTL != 30*time.Minute {
		t.Errorf("AccessTokenTTL: got %v want 30m", cfg.AccessTokenTTL)
	}
	if cfg.RedisPassword != "redis-pw" {
		t.Errorf("RedisPassword: got %q", cfg.RedisPassword)
	}
	if cfg.FazpassMerchantKey != "fz-mk" {
		t.Errorf("FazpassMerchantKey: got %q", cfg.FazpassMerchantKey)
	}
	if cfg.ObjectStorageUseSSL != true {
		t.Errorf("ObjectStorageUseSSL: got %v want true", cfg.ObjectStorageUseSSL)
	}
	if len(cfg.CORSOrigins) != 2 || cfg.CORSOrigins[0] != "http://localhost:3000" {
		t.Errorf("CORSOrigins: got %v", cfg.CORSOrigins)
	}
}

func TestLoad_devSecretsFileAbsent(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "dev")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins:
  - "http://localhost:3000"
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)

	cfg, err := Load("dev", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.JWTSecret != "" {
		t.Errorf("JWTSecret: got %q want empty", cfg.JWTSecret)
	}
	if cfg.DatabaseURL != "" {
		t.Errorf("DatabaseURL: got %q want empty", cfg.DatabaseURL)
	}
}

func TestLoad_stagingSecretsFileAbsent(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "staging")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins: []
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)

	_, err := Load("staging", dir)
	if err == nil {
		t.Fatal("expected error for missing secrets file in staging, got nil")
	}
}

func TestLoad_stagingRequiredSecretEmpty(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "staging")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins: []
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: "postgres://host/db"
jwt_secret: ""
config_encryption_key: "enc-key"
otp_secret: "otp-secret"
object_storage_access_key: "minio-ak"
object_storage_secret_key: "minio-sk"
`)

	_, err := Load("staging", dir)
	if err == nil {
		t.Fatal("expected error for empty jwt_secret, got nil")
	}
	if !contains(err.Error(), "jwt_secret") {
		t.Errorf("error should name jwt_secret, got: %v", err)
	}
}

func TestLoad_stagingTwoRequiredSecretsEmpty(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "staging")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins: []
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: ""
jwt_secret: ""
config_encryption_key: "enc-key"
otp_secret: "otp-secret"
object_storage_access_key: "minio-ak"
object_storage_secret_key: "minio-sk"
`)

	_, err := Load("staging", dir)
	if err == nil {
		t.Fatal("expected error for empty fields, got nil")
	}
	msg := err.Error()
	if !contains(msg, "database_url") {
		t.Errorf("error should name database_url, got: %v", err)
	}
	if !contains(msg, "jwt_secret") {
		t.Errorf("error should name jwt_secret, got: %v", err)
	}
}

func TestLoad_mergeCorrectness(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "dev")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "3000"
redis_addr: "r.example.com:6379"
worker_poll_interval: "30s"
cors_origins:
  - "https://example.com"
access_token_ttl: "5m"
refresh_token_ttl: "24h"
otp_ttl: "3m"
google_client_id: "gcid"
fazpass_base_url: "https://fazpass.example.com"
midtrans_env: "production"
smtp_host: "smtp.gmail.com"
smtp_port: "587"
smtp_from: "no-reply@example.com"
smtp_from_name: "Abak Academy"
object_storage_endpoint: "s3.example.com"
object_storage_public_endpoint: "s3-public.example.com"
object_storage_use_ssl: true
object_storage_region: "asia-southeast2"
object_storage_bucket_name: "my-bucket"
object_storage_private_bucket_name: "my-private-bucket"
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: "postgres://user:pass@db.example.com/dbname"
jwt_secret: "jwt-secret-123"
config_encryption_key: "enc-key-456"
otp_secret: "otp-secret-789"
object_storage_access_key: "AK123"
object_storage_secret_key: "SK456"
redis_password: "redis-pass"
fazpass_merchant_key: "fz-merchant"
fazpass_api_key: "fz-api"
midtrans_server_key: "mt-server"
midtrans_client_key: "mt-client"
biteship_api_key: "bs-key-xyz"
smtp_username: "smtp-user"
smtp_password: "smtp-pass"
`)

	cfg, err := Load("dev", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	checks := []struct {
		field string
		got   string
		want  string
	}{
		{"Env", cfg.Env, "dev"},
		{"HTTPPort", cfg.HTTPPort, "3000"},
		{"RedisAddr", cfg.RedisAddr, "r.example.com:6379"},
		{"DatabaseURL", cfg.DatabaseURL, "postgres://user:pass@db.example.com/dbname"},
		{"RedisPassword", cfg.RedisPassword, "redis-pass"},
		{"JWTSecret", cfg.JWTSecret, "jwt-secret-123"},
		{"ConfigEncryptionKey", cfg.ConfigEncryptionKey, "enc-key-456"},
		{"OTPSecret", cfg.OTPSecret, "otp-secret-789"},
		{"GoogleClientID", cfg.GoogleClientID, "gcid"},
		{"FazpassMerchantKey", cfg.FazpassMerchantKey, "fz-merchant"},
		{"FazpassAPIKey", cfg.FazpassAPIKey, "fz-api"},
		{"FazpassBaseURL", cfg.FazpassBaseURL, "https://fazpass.example.com"},
		{"MidtransServerKey", cfg.MidtransServerKey, "mt-server"},
		{"MidtransClientKey", cfg.MidtransClientKey, "mt-client"},
		{"BiteshipAPIKey", cfg.BiteshipAPIKey, "bs-key-xyz"},
		{"MidtransEnv", cfg.MidtransEnv, "production"},
		{"SMTPHost", cfg.SMTPHost, "smtp.gmail.com"},
		{"SMTPPort", cfg.SMTPPort, "587"},
		{"SMTPFrom", cfg.SMTPFrom, "no-reply@example.com"},
		{"SMTPFromName", cfg.SMTPFromName, "Abak Academy"},
		{"SMTPUsername", cfg.SMTPUsername, "smtp-user"},
		{"SMTPPassword", cfg.SMTPPassword, "smtp-pass"},
		{"ObjectStorageEndpoint", cfg.ObjectStorageEndpoint, "s3.example.com"},
		{"ObjectStoragePublicEndpoint", cfg.ObjectStoragePublicEndpoint, "s3-public.example.com"},
		{"ObjectStorageRegion", cfg.ObjectStorageRegion, "asia-southeast2"},
		{"ObjectStorageAccessKey", cfg.ObjectStorageAccessKey, "AK123"},
		{"ObjectStorageSecretKey", cfg.ObjectStorageSecretKey, "SK456"},
		{"ObjectStorageBucketName", cfg.ObjectStorageBucketName, "my-bucket"},
		{"ObjectStoragePrivateBucketName", cfg.ObjectStoragePrivateBucketName, "my-private-bucket"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q want %q", c.field, c.got, c.want)
		}
	}

	if cfg.WorkerPollInterval != 30*time.Second {
		t.Errorf("WorkerPollInterval: got %v want 30s", cfg.WorkerPollInterval)
	}
	if cfg.AccessTokenTTL != 5*time.Minute {
		t.Errorf("AccessTokenTTL: got %v want 5m", cfg.AccessTokenTTL)
	}
	if cfg.RefreshTokenTTL != 24*time.Hour {
		t.Errorf("RefreshTokenTTL: got %v want 24h", cfg.RefreshTokenTTL)
	}
	if cfg.OTPTTL != 3*time.Minute {
		t.Errorf("OTPTTL: got %v want 3m", cfg.OTPTTL)
	}
	if cfg.ObjectStorageUseSSL != true {
		t.Errorf("ObjectStorageUseSSL: got %v want true", cfg.ObjectStorageUseSSL)
	}
}

func TestLoad_envArgumentSetsEnv(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "custom-env")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins: []
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: "postgres://host/db"
jwt_secret: "jwt"
config_encryption_key: "enc"
otp_secret: "otp"
object_storage_access_key: "ak"
object_storage_secret_key: "sk"
`)

	cfg, err := Load("custom-env", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.Env != "custom-env" {
		t.Errorf("Env: got %q want %q", cfg.Env, "custom-env")
	}
}

func TestLoad_migrationDatabaseURLPresent(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "dev")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins: []
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: "postgres://pooled@pgbouncer:6432/db"
migration_database_url: "postgres://direct@postgres:5432/db"
jwt_secret: "jwt"
config_encryption_key: "enc"
otp_secret: "otp"
object_storage_access_key: "ak"
object_storage_secret_key: "sk"
`)

	cfg, err := Load("dev", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MigrationDatabaseURL != "postgres://direct@postgres:5432/db" {
		t.Errorf("MigrationDatabaseURL: got %q want direct DSN", cfg.MigrationDatabaseURL)
	}
}

func TestLoad_migrationDatabaseURLAbsentFallsBackToDatabaseURL(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "dev")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins: []
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: "postgres://pooled@pgbouncer:6432/db"
jwt_secret: "jwt"
config_encryption_key: "enc"
otp_secret: "otp"
object_storage_access_key: "ak"
object_storage_secret_key: "sk"
`)

	cfg, err := Load("dev", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MigrationDatabaseURL != cfg.DatabaseURL {
		t.Errorf("MigrationDatabaseURL: got %q want fallback to DatabaseURL %q", cfg.MigrationDatabaseURL, cfg.DatabaseURL)
	}
}

func TestLoad_migrationDatabaseURLEmptyFallsBackToDatabaseURL(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "dev")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins: []
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: "postgres://pooled@pgbouncer:6432/db"
migration_database_url: ""
jwt_secret: "jwt"
config_encryption_key: "enc"
otp_secret: "otp"
object_storage_access_key: "ak"
object_storage_secret_key: "sk"
`)

	cfg, err := Load("dev", dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if cfg.MigrationDatabaseURL != cfg.DatabaseURL {
		t.Errorf("MigrationDatabaseURL: got %q want fallback to DatabaseURL %q", cfg.MigrationDatabaseURL, cfg.DatabaseURL)
	}
}

func TestLoad_stagingSecretsFileWithoutMigrationDatabaseURLStillLoads(t *testing.T) {
	dir := t.TempDir()
	envDir := filepath.Join(dir, "staging")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		t.Fatal(err)
	}

	writeYAML(t, envDir, "config.yaml", `
http_port: "8080"
redis_addr: "redis:6379"
worker_poll_interval: "5s"
cors_origins: []
access_token_ttl: "15m"
refresh_token_ttl: "168h"
otp_ttl: "5m"
google_client_id: ""
fazpass_base_url: ""
midtrans_env: ""
object_storage_endpoint: ""
object_storage_public_endpoint: ""
object_storage_use_ssl: false
object_storage_bucket_name: ""
object_storage_private_bucket_name: ""
`)
	writeYAML(t, envDir, "secrets.yaml", `
database_url: "postgres://akademi:realpass@postgres:5432/akademi?sslmode=disable"
jwt_secret: "jwt-secret"
config_encryption_key: "enc-key"
otp_secret: "otp-secret"
object_storage_access_key: "ak"
object_storage_secret_key: "sk"
`)

	cfg, err := Load("staging", dir)
	if err != nil {
		t.Fatalf("Load: expected staging secrets file without migration_database_url to still load, got err: %v", err)
	}
	if cfg.MigrationDatabaseURL != cfg.DatabaseURL {
		t.Errorf("MigrationDatabaseURL: got %q want fallback to DatabaseURL %q", cfg.MigrationDatabaseURL, cfg.DatabaseURL)
	}
}

// Guards the committed backend/config/env/prod/config.yaml directly (not a fixture
// copy) so drift in the real file fails this test. secrets.yaml is gitignored and
// absent on disk, so a temp one is placed alongside it for the duration of the test.
func TestLoad_prodConfigValues(t *testing.T) {
	secretsPath := filepath.Join("env", "prod", "secrets.yaml")
	if _, err := os.Stat(secretsPath); err == nil {
		t.Fatalf("%s already exists; refusing to overwrite", secretsPath)
	}
	writeYAML(t, filepath.Join("env", "prod"), "secrets.yaml", `
database_url: "postgres://user:pass@pgbouncer:6432/db"
jwt_secret: "jwt-secret"
config_encryption_key: "enc-key"
otp_secret: "otp-secret"
object_storage_access_key: "ak"
object_storage_secret_key: "sk"
`)
	t.Cleanup(func() {
		if err := os.Remove(secretsPath); err != nil {
			t.Errorf("cleanup secrets.yaml: %v", err)
		}
	})

	cfg, err := Load("prod", "env")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	checks := []struct {
		field string
		got   string
		want  string
	}{
		{"RedisAddr", cfg.RedisAddr, "redis:6379"},
		{"ObjectStorageBucketName", cfg.ObjectStorageBucketName, "abak-academy-media-prod"},
		{"ObjectStoragePrivateBucketName", cfg.ObjectStoragePrivateBucketName, "abak-academy-media-prod"},
		{"ObjectStorageEndpoint", cfg.ObjectStorageEndpoint, "storage.googleapis.com"},
		{"SMTPHost", cfg.SMTPHost, "smtp.gmail.com"},
		{"SMTPFromName", cfg.SMTPFromName, "Abak Academy"},
		{"MidtransEnv", cfg.MidtransEnv, "production"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q want %q", c.field, c.got, c.want)
		}
	}

	if len(cfg.CORSOrigins) != 1 || cfg.CORSOrigins[0] != "https://hub.abakacademy.id" {
		t.Errorf("CORSOrigins: got %v want [https://hub.abakacademy.id]", cfg.CORSOrigins)
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
