package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Env                  string
	HTTPPort             string
	DatabaseURL          string
	MigrationDatabaseURL string
	RedisAddr            string
	RedisPassword        string
	WorkerPollInterval   time.Duration
	CORSOrigins          []string
	DBMaxConns           int32

	JWTSecret           string
	AccessTokenTTL      time.Duration
	RefreshTokenTTL     time.Duration
	OTPSecret           string
	OTPTTL              time.Duration
	GoogleClientID      string
	ConfigEncryptionKey string
	FazpassMerchantKey  string
	FazpassAPIKey       string
	FazpassBaseURL      string
	MidtransServerKey   string
	MidtransClientKey   string
	MidtransEnv         string
	BiteshipAPIKey      string

	SMTPHost     string
	SMTPPort     string
	SMTPFrom     string
	SMTPFromName string
	SMTPUsername string
	SMTPPassword string

	ObjectStorageEndpoint          string
	ObjectStoragePublicEndpoint    string
	ObjectStorageAccessKey         string
	ObjectStorageSecretKey         string
	ObjectStorageUseSSL            bool
	ObjectStorageRegion            string
	ObjectStorageBucketName        string
	ObjectStoragePrivateBucketName string

	GotenbergURL string
}

type fileConfig struct {
	HTTPPort                       string   `yaml:"http_port"`
	RedisAddr                      string   `yaml:"redis_addr"`
	WorkerPollInterval             string   `yaml:"worker_poll_interval"`
	CORSOrigins                    []string `yaml:"cors_origins"`
	DBMaxConns                     int32    `yaml:"db_max_conns"`
	AccessTokenTTL                 string   `yaml:"access_token_ttl"`
	RefreshTokenTTL                string   `yaml:"refresh_token_ttl"`
	OTPTTL                         string   `yaml:"otp_ttl"`
	GoogleClientID                 string   `yaml:"google_client_id"`
	FazpassBaseURL                 string   `yaml:"fazpass_base_url"`
	MidtransEnv                    string   `yaml:"midtrans_env"`
	SMTPHost                       string   `yaml:"smtp_host"`
	SMTPPort                       string   `yaml:"smtp_port"`
	SMTPFrom                       string   `yaml:"smtp_from"`
	SMTPFromName                   string   `yaml:"smtp_from_name"`
	ObjectStorageEndpoint          string   `yaml:"object_storage_endpoint"`
	ObjectStoragePublicEndpoint    string   `yaml:"object_storage_public_endpoint"`
	ObjectStorageUseSSL            bool     `yaml:"object_storage_use_ssl"`
	ObjectStorageRegion            string   `yaml:"object_storage_region"`
	ObjectStorageBucketName        string   `yaml:"object_storage_bucket_name"`
	ObjectStoragePrivateBucketName string   `yaml:"object_storage_private_bucket_name"`
	GotenbergURL                   string   `yaml:"gotenberg_url"`
}

type fileSecrets struct {
	DatabaseURL            string `yaml:"database_url"`
	MigrationDatabaseURL   string `yaml:"migration_database_url"`
	JWTSecret              string `yaml:"jwt_secret"`
	ConfigEncryptionKey    string `yaml:"config_encryption_key"`
	OTPSecret              string `yaml:"otp_secret"`
	ObjectStorageAccessKey string `yaml:"object_storage_access_key"`
	ObjectStorageSecretKey string `yaml:"object_storage_secret_key"`
	RedisPassword          string `yaml:"redis_password"`
	FazpassMerchantKey     string `yaml:"fazpass_merchant_key"`
	FazpassAPIKey          string `yaml:"fazpass_api_key"`
	MidtransServerKey      string `yaml:"midtrans_server_key"`
	MidtransClientKey      string `yaml:"midtrans_client_key"`
	BiteshipAPIKey         string `yaml:"biteship_api_key"`
	SMTPUsername           string `yaml:"smtp_username"`
	SMTPPassword           string `yaml:"smtp_password"`
}

var requiredSecrets = []struct {
	field string
	ptr   func(*fileSecrets) *string
}{
	{"database_url", func(s *fileSecrets) *string { return &s.DatabaseURL }},
	{"jwt_secret", func(s *fileSecrets) *string { return &s.JWTSecret }},
	{"config_encryption_key", func(s *fileSecrets) *string { return &s.ConfigEncryptionKey }},
	{"otp_secret", func(s *fileSecrets) *string { return &s.OTPSecret }},
	{"object_storage_access_key", func(s *fileSecrets) *string { return &s.ObjectStorageAccessKey }},
	{"object_storage_secret_key", func(s *fileSecrets) *string { return &s.ObjectStorageSecretKey }},
}

func Load(env, configDir string) (Config, error) {
	envDir := filepath.Join(configDir, env)

	fc, err := loadFileConfig(envDir)
	if err != nil {
		return Config{}, err
	}

	secrets, err := loadFileSecrets(envDir)
	if err != nil {
		if env == "dev" {
			secrets = fileSecrets{}
		} else {
			return Config{}, err
		}
	}

	if env != "dev" {
		if err := validateRequiredSecrets(secrets); err != nil {
			return Config{}, err
		}
	}

	return merge(env, fc, secrets)
}

func loadFileConfig(envDir string) (fileConfig, error) {
	var fc fileConfig
	path := filepath.Join(envDir, "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return fc, fmt.Errorf("read config file %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &fc); err != nil {
		return fc, fmt.Errorf("parse config file %s: %w", path, err)
	}
	return fc, nil
}

func loadFileSecrets(envDir string) (fileSecrets, error) {
	var s fileSecrets
	path := filepath.Join(envDir, "secrets.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		return s, fmt.Errorf("read secrets file %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, &s); err != nil {
		return s, fmt.Errorf("parse secrets file %s: %w", path, err)
	}
	return s, nil
}

func validateRequiredSecrets(s fileSecrets) error {
	var missing []string
	for _, req := range requiredSecrets {
		if *req.ptr(&s) == "" {
			missing = append(missing, req.field)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("required secret fields are empty: %s", strings.Join(missing, ", "))
	}
	return nil
}

func merge(env string, fc fileConfig, s fileSecrets) (Config, error) {
	if fc.DBMaxConns < 0 {
		return Config{}, fmt.Errorf("db_max_conns: must not be negative, got %d", fc.DBMaxConns)
	}

	workerPoll, err := time.ParseDuration(fc.WorkerPollInterval)
	if err != nil {
		return Config{}, fmt.Errorf("worker_poll_interval: %w", err)
	}
	accessTTL, err := time.ParseDuration(fc.AccessTokenTTL)
	if err != nil {
		return Config{}, fmt.Errorf("access_token_ttl: %w", err)
	}
	refreshTTL, err := time.ParseDuration(fc.RefreshTokenTTL)
	if err != nil {
		return Config{}, fmt.Errorf("refresh_token_ttl: %w", err)
	}
	otpTTL, err := time.ParseDuration(fc.OTPTTL)
	if err != nil {
		return Config{}, fmt.Errorf("otp_ttl: %w", err)
	}

	migrationDatabaseURL := s.MigrationDatabaseURL
	if migrationDatabaseURL == "" {
		migrationDatabaseURL = s.DatabaseURL
	}

	return Config{
		Env:                env,
		HTTPPort:           fc.HTTPPort,
		RedisAddr:          fc.RedisAddr,
		WorkerPollInterval: workerPoll,
		CORSOrigins:        fc.CORSOrigins,
		DBMaxConns:         fc.DBMaxConns,

		AccessTokenTTL:  accessTTL,
		RefreshTokenTTL: refreshTTL,
		OTPTTL:          otpTTL,
		GoogleClientID:  fc.GoogleClientID,
		FazpassBaseURL:  fc.FazpassBaseURL,
		MidtransEnv:     fc.MidtransEnv,
		SMTPHost:        fc.SMTPHost,
		SMTPPort:        fc.SMTPPort,
		SMTPFrom:        fc.SMTPFrom,
		SMTPFromName:    fc.SMTPFromName,

		ObjectStorageEndpoint:          fc.ObjectStorageEndpoint,
		ObjectStoragePublicEndpoint:    fc.ObjectStoragePublicEndpoint,
		ObjectStorageUseSSL:            fc.ObjectStorageUseSSL,
		ObjectStorageRegion:            fc.ObjectStorageRegion,
		ObjectStorageBucketName:        fc.ObjectStorageBucketName,
		ObjectStoragePrivateBucketName: fc.ObjectStoragePrivateBucketName,

		GotenbergURL: fc.GotenbergURL,

		DatabaseURL:            s.DatabaseURL,
		MigrationDatabaseURL:   migrationDatabaseURL,
		JWTSecret:              s.JWTSecret,
		ConfigEncryptionKey:    s.ConfigEncryptionKey,
		OTPSecret:              s.OTPSecret,
		ObjectStorageAccessKey: s.ObjectStorageAccessKey,
		ObjectStorageSecretKey: s.ObjectStorageSecretKey,
		RedisPassword:          s.RedisPassword,
		FazpassMerchantKey:     s.FazpassMerchantKey,
		FazpassAPIKey:          s.FazpassAPIKey,
		MidtransServerKey:      s.MidtransServerKey,
		MidtransClientKey:      s.MidtransClientKey,
		BiteshipAPIKey:         s.BiteshipAPIKey,
		SMTPUsername:           s.SMTPUsername,
		SMTPPassword:           s.SMTPPassword,
	}, nil
}
