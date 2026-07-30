package service

import (
	"context"
	"encoding/hex"
	"testing"

	"akademi-bimbel/internal/repository"
)

// ---------------------------------------------------------------------------
// AES-256-GCM encrypt/decrypt round-trip tests
// ---------------------------------------------------------------------------

func TestEncryptDecryptRoundTrip(t *testing.T) {
	hexKey := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	plaintext := "SB-Mid-server-xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

	encrypted, err := encryptConfigValue(hexKey, plaintext)
	if err != nil {
		t.Fatalf("encryptConfigValue: %v", err)
	}
	if encrypted == "" {
		t.Fatal("encrypted value is empty")
	}

	decrypted, err := decryptConfigValue(hexKey, encrypted)
	if err != nil {
		t.Fatalf("decryptConfigValue: %v", err)
	}
	if decrypted != plaintext {
		t.Fatalf("round-trip mismatch: want %q, got %q", plaintext, decrypted)
	}
}

func TestEncryptConfigValue_EmptyKey(t *testing.T) {
	_, err := encryptConfigValue("", "some-value")
	if err != ErrConfigEncryption {
		t.Fatalf("want ErrConfigEncryption, got %v", err)
	}
}

func TestEncryptConfigValue_InvalidHexKey(t *testing.T) {
	_, err := encryptConfigValue("not-hex", "some-value")
	if err != ErrConfigEncryption {
		t.Fatalf("want ErrConfigEncryption, got %v", err)
	}
}

func TestDecryptConfigValue_WrongKey(t *testing.T) {
	keyA := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	keyB := "b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2c3"
	plaintext := "my-secret"

	encrypted, err := encryptConfigValue(keyA, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	_, err = decryptConfigValue(keyB, encrypted)
	if err != ErrConfigEncryption {
		t.Fatalf("decrypt with wrong key: want ErrConfigEncryption, got %v", err)
	}
}

func TestDecryptConfigValue_TamperedCiphertext(t *testing.T) {
	hexKey := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	plaintext := "my-secret"

	encrypted, err := encryptConfigValue(hexKey, plaintext)
	if err != nil {
		t.Fatalf("encrypt: %v", err)
	}

	// Flip a byte in the nonce+ciphertext
	garbled := []byte(encrypted)
	if len(garbled) > 0 {
		garbled[len(garbled)-1] ^= 0xFF
	}

	_, err = decryptConfigValue(hexKey, string(garbled))
	if err != ErrConfigEncryption {
		t.Fatalf("decrypt with tampered data: want ErrConfigEncryption, got %v", err)
	}
}

// ---------------------------------------------------------------------------
// buildConfigMap — masking logic
// ---------------------------------------------------------------------------

func TestBuildConfigMap_MasksSecrets(t *testing.T) {
	rows := []repository.SystemConfigRow{
		{Key: "app_name", Value: "Akademi Bimbel", IsSecret: false},
		{Key: "midtrans_server_key", Value: "ciphertext-blob", IsSecret: true},
		{Key: "midtrans_client_key", Value: "", IsSecret: true},
		{Key: "midtrans_env", Value: "sandbox", IsSecret: false},
	}

	result := buildConfigMap(rows)

	// Non-secret value passes through
	if result["app_name"] != "Akademi Bimbel" {
		t.Errorf("app_name: want 'Akademi Bimbel', got %q", result["app_name"])
	}

	// Secret with non-empty value returns "***"
	if result["midtrans_server_key"] != "***" {
		t.Errorf("midtrans_server_key: want '***', got %q", result["midtrans_server_key"])
	}

	// Secret with empty value returns ""
	if result["midtrans_client_key"] != "" {
		t.Errorf("midtrans_client_key: want '', got %q", result["midtrans_client_key"])
	}

	// Non-secret env passes through
	if result["midtrans_env"] != "sandbox" {
		t.Errorf("midtrans_env: want 'sandbox', got %q", result["midtrans_env"])
	}
}

func TestBuildConfigMap_MissingKeysDefault(t *testing.T) {
	rows := []repository.SystemConfigRow{
		{Key: "app_name", Value: "Test", IsSecret: false},
	}
	// Only 1 row; rest should default to ""

	result := buildConfigMap(rows)

	if result["app_name"] != "Test" {
		t.Errorf("app_name: want 'Test', got %q", result["app_name"])
	}
	if result["midtrans_server_key"] != "" {
		t.Errorf("midtrans_server_key should default to empty, got %q", result["midtrans_server_key"])
	}
	if result["app_address"] != "" {
		t.Errorf("app_address should default to empty, got %q", result["app_address"])
	}
	if result["notify_on_purchase_admin_store"] != "" {
		t.Errorf("notify_on_purchase_admin_store should default to empty, got %q", result["notify_on_purchase_admin_store"])
	}

	// Ensure all catalog keys are present
	for key := range configKeyCatalog {
		if _, ok := result[key]; !ok {
			t.Errorf("key %q missing from result", key)
		}
	}
}

func TestBuildConfigMap_IgnoresUnknownKeys(t *testing.T) {
	rows := []repository.SystemConfigRow{
		{Key: "unknown_key", Value: "whatever", IsSecret: false},
		{Key: "app_name", Value: "Known", IsSecret: false},
	}

	result := buildConfigMap(rows)
	if result["app_name"] != "Known" {
		t.Errorf("app_name: want 'Known', got %q", result["app_name"])
	}
	// unknown_key should not appear
	if _, ok := result["unknown_key"]; ok {
		t.Error("unknown_key should not be in result")
	}
}

// ---------------------------------------------------------------------------
// validateConfigKeys — validation logic
// ---------------------------------------------------------------------------

func TestValidateConfigKeys_Valid(t *testing.T) {
	err := validateConfigKeys(map[string]string{
		"app_name":                       "Test",
		"notify_on_purchase_admin_store": "true",
		"midtrans_env":                   "production",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateConfigKeys_UnknownKey(t *testing.T) {
	err := validateConfigKeys(map[string]string{
		"unknown_key": "value",
	})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestValidateConfigKeys_BoolKeyInvalidValue(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"yes", "yes"},
		{"no", "no"},
		{"1", "1"},
		{"0", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfigKeys(map[string]string{
				"notify_on_purchase_admin_store": tt.value,
			})
			if err == nil {
				t.Errorf("expected error for bool key value %q", tt.value)
			}
		})
	}
}

func TestValidateConfigKeys_BoolKeyValidValues(t *testing.T) {
	for _, v := range []string{"true", "false"} {
		err := validateConfigKeys(map[string]string{
			"notify_on_purchase_admin_exam": v,
		})
		if err != nil {
			t.Errorf("unexpected error for bool key value %q: %v", v, err)
		}
	}
}

func TestValidateConfigKeys_EnumKeyInvalidValue(t *testing.T) {
	err := validateConfigKeys(map[string]string{
		"midtrans_env": "staging",
	})
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}
}

func TestValidateConfigKeys_EnumKeyValidValues(t *testing.T) {
	for _, v := range []string{"sandbox", "production"} {
		err := validateConfigKeys(map[string]string{
			"midtrans_env": v,
		})
		if err != nil {
			t.Errorf("unexpected error for enum value %q: %v", v, err)
		}
	}
}

// ---------------------------------------------------------------------------
// processConfigValues — skip "***" for secrets, encrypt others
// ---------------------------------------------------------------------------

func TestProcessConfigValues_SkipStarStarStarForSecrets(t *testing.T) {
	hexKey := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	values := map[string]string{
		"app_name":                       "New Name",
		"midtrans_server_key":            "***",
		"notify_on_purchase_admin_store": "true",
	}

	processed, changedKeys, err := processConfigValues(values, hexKey)
	if err != nil {
		t.Fatalf("processConfigValues: %v", err)
	}

	// "***" for secret should be skipped — not in processed
	if _, ok := processed["midtrans_server_key"]; ok {
		t.Error("midtrans_server_key with '***' should be skipped, but was included in processed")
	}
	// "***" should NOT be in changedKeys
	for _, k := range changedKeys {
		if k == "midtrans_server_key" {
			t.Error("midtrans_server_key should not appear in changedKeys when value is '***'")
		}
	}

	// Non-secret keys should be present
	if processed["app_name"] != "New Name" {
		t.Errorf("app_name: want 'New Name', got %q", processed["app_name"])
	}
	if processed["notify_on_purchase_admin_store"] != "true" {
		t.Errorf("notify_on_purchase_admin_store: want 'true', got %q", processed["notify_on_purchase_admin_store"])
	}
}

func TestProcessConfigValues_EncryptsSecrets(t *testing.T) {
	hexKey := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	secretValue := "SB-Mid-server-xxxx"
	values := map[string]string{
		"midtrans_server_key": secretValue,
	}

	processed, changedKeys, err := processConfigValues(values, hexKey)
	if err != nil {
		t.Fatalf("processConfigValues: %v", err)
	}

	encrypted, ok := processed["midtrans_server_key"]
	if !ok {
		t.Fatal("midtrans_server_key missing from processed")
	}
	if encrypted == secretValue {
		t.Fatal("secret value should be encrypted, but stored as plaintext")
	}
	if encrypted == "" {
		t.Fatal("encrypted value should not be empty")
	}

	// Verify it can be decrypted back
	decrypted, err := decryptConfigValue(hexKey, encrypted)
	if err != nil {
		t.Fatalf("decrypt back: %v", err)
	}
	if decrypted != secretValue {
		t.Fatalf("decrypt back: want %q, got %q", secretValue, decrypted)
	}

	if len(changedKeys) != 1 || changedKeys[0] != "midtrans_server_key" {
		t.Fatalf("changedKeys: want [midtrans_server_key], got %v", changedKeys)
	}
}

func TestProcessConfigValues_ClearSecretWithEmptyString(t *testing.T) {
	hexKey := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	values := map[string]string{
		"midtrans_client_key": "",
	}

	processed, changedKeys, err := processConfigValues(values, hexKey)
	if err != nil {
		t.Fatalf("processConfigValues: %v", err)
	}

	// Empty string for a secret means "clear it" — stored as empty, not encrypted
	stored, ok := processed["midtrans_client_key"]
	if !ok {
		t.Fatal("midtrans_client_key missing from processed")
	}
	if stored != "" {
		t.Fatalf("empty string for secret should be stored as empty, got %q", stored)
	}

	if len(changedKeys) != 1 || changedKeys[0] != "midtrans_client_key" {
		t.Fatalf("changedKeys: want [midtrans_client_key], got %v", changedKeys)
	}
}

func TestProcessConfigValues_EncryptFailsOnEmptyKey(t *testing.T) {
	values := map[string]string{
		"midtrans_server_key": "some-value",
	}

	_, _, err := processConfigValues(values, "")
	if err != ErrConfigEncryption {
		t.Fatalf("want ErrConfigEncryption, got %v", err)
	}
}

func TestProcessConfigValues_NonSecretPassesThrough(t *testing.T) {
	hexKey := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	values := map[string]string{
		"app_name":    "New Name",
		"app_address": "Jl. Example 123",
	}

	processed, changedKeys, err := processConfigValues(values, hexKey)
	if err != nil {
		t.Fatalf("processConfigValues: %v", err)
	}

	if processed["app_name"] != "New Name" {
		t.Errorf("app_name: want 'New Name', got %q", processed["app_name"])
	}
	if processed["app_address"] != "Jl. Example 123" {
		t.Errorf("app_address: want 'Jl. Example 123', got %q", processed["app_address"])
	}
	if len(changedKeys) != 2 {
		t.Fatalf("changedKeys: want 2 keys, got %v", changedKeys)
	}
}

// ---------------------------------------------------------------------------
// Compile-time check: ensure hex encoding/decoding works as expected
// ---------------------------------------------------------------------------

func TestHexKeyDecoding(t *testing.T) {
	// 64 hex chars = 32 bytes
	raw := "a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2"
	key, err := hex.DecodeString(raw)
	if err != nil {
		t.Fatalf("hex.DecodeString: %v", err)
	}
	if len(key) != 32 {
		t.Fatalf("want 32 bytes, got %d", len(key))
	}
}

// ---------------------------------------------------------------------------
// Integration-level test: GetSystemConfig + UpdateSystemConfig via fake repo
// ---------------------------------------------------------------------------

// fakeSystemConfigRepo implements in-memory storage for system_config table.
type fakeSystemConfigRepo struct {
	rows map[string]repository.SystemConfigRow
}

func newFakeSystemConfigRepo() *fakeSystemConfigRepo {
	return &fakeSystemConfigRepo{rows: make(map[string]repository.SystemConfigRow)}
}

func (f *fakeSystemConfigRepo) ListSystemConfig(_ context.Context) ([]repository.SystemConfigRow, error) {
	var out []repository.SystemConfigRow
	for _, row := range f.rows {
		out = append(out, row)
	}
	return out, nil
}

func (f *fakeSystemConfigRepo) UpsertSystemConfig(_ context.Context, key, value string, isSecret bool) error {
	f.rows[key] = repository.SystemConfigRow{
		Key:      key,
		Value:    value,
		IsSecret: isSecret,
	}
	return nil
}

func (f *fakeSystemConfigRepo) InsertAuditLogMeta(_ context.Context, _ interface{}, _ *string, _, _, _ string, _ map[string]any) error {
	return nil
}

// fakeSystemConfigService wraps a fake repo with the real service logic.
type fakeSystemConfigService struct {
	repo   *fakeSystemConfigRepo
	hexKey string
}

func newFakeSystemConfigService(hexKey string) *fakeSystemConfigService {
	return &fakeSystemConfigService{
		repo:   newFakeSystemConfigRepo(),
		hexKey: hexKey,
	}
}

func (s *fakeSystemConfigService) GetSystemConfig(ctx context.Context) (map[string]string, error) {
	rows, err := s.repo.ListSystemConfig(ctx)
	if err != nil {
		return nil, err
	}
	return buildConfigMap(rows), nil
}

func (s *fakeSystemConfigService) UpdateSystemConfig(ctx context.Context, values map[string]string) (map[string]string, error) {
	if err := validateConfigKeys(values); err != nil {
		return nil, err
	}

	processed, _, err := processConfigValues(values, s.hexKey)
	if err != nil {
		return nil, err
	}

	for key, val := range processed {
		def := configKeyCatalog[key]
		if err := s.repo.UpsertSystemConfig(ctx, key, val, def.secret); err != nil {
			return nil, err
		}
	}

	return s.GetSystemConfig(ctx)
}

func TestGetSystemConfig_ReturnsFullMap(t *testing.T) {
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	// Seed some data
	_ = svc.repo.UpsertSystemConfig(ctx, "app_name", "Akademi Bimbel", false)
	_ = svc.repo.UpsertSystemConfig(ctx, "midtrans_server_key", "encrypted-blob", true)
	_ = svc.repo.UpsertSystemConfig(ctx, "midtrans_env", "sandbox", false)

	result, err := svc.GetSystemConfig(ctx)
	if err != nil {
		t.Fatalf("GetSystemConfig: %v", err)
	}

	// All catalog keys present
	for key := range configKeyCatalog {
		if _, ok := result[key]; !ok {
			t.Errorf("key %q missing from result", key)
		}
	}

	// Values as expected
	if result["app_name"] != "Akademi Bimbel" {
		t.Errorf("app_name: want 'Akademi Bimbel', got %q", result["app_name"])
	}
	if result["midtrans_server_key"] != "***" {
		t.Errorf("midtrans_server_key: want '***', got %q", result["midtrans_server_key"])
	}
	if result["midtrans_env"] != "sandbox" {
		t.Errorf("midtrans_env: want 'sandbox', got %q", result["midtrans_env"])
	}
	// Unset key defaults to ""
	if result["app_address"] != "" {
		t.Errorf("app_address should default to '', got %q", result["app_address"])
	}
	if result["midtrans_client_key"] != "" {
		t.Errorf("midtrans_client_key should default to '', got %q", result["midtrans_client_key"])
	}
}

func TestUpdateSystemConfig_Integration(t *testing.T) {
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	// Initial state — all empty
	initial, err := svc.GetSystemConfig(ctx)
	if err != nil {
		t.Fatalf("GetSystemConfig (initial): %v", err)
	}
	for key := range configKeyCatalog {
		if initial[key] != "" {
			t.Errorf("initial %q: want '', got %q", key, initial[key])
		}
	}

	// Update with some values
	result, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"app_name":     "New Name",
		"midtrans_env": "production",
	})
	if err != nil {
		t.Fatalf("UpdateSystemConfig: %v", err)
	}

	if result["app_name"] != "New Name" {
		t.Errorf("app_name: want 'New Name', got %q", result["app_name"])
	}
	if result["midtrans_env"] != "production" {
		t.Errorf("midtrans_env: want 'production', got %q", result["midtrans_env"])
	}
	// midtrans_server_key wasn't provided, should still be ""
	if result["midtrans_server_key"] != "" {
		t.Errorf("midtrans_server_key should still be '', got %q", result["midtrans_server_key"])
	}
}

func TestUpdateSystemConfig_RejectsUnknownKey(t *testing.T) {
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	_, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"nonexistent_key": "value",
	})
	if err == nil {
		t.Fatal("expected error for unknown key")
	}
}

func TestUpdateSystemConfig_RejectsInvalidBool(t *testing.T) {
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	_, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"notify_on_purchase_admin_store": "yes",
	})
	if err == nil {
		t.Fatal("expected error for invalid bool value")
	}
}

func TestUpdateSystemConfig_RejectsInvalidEnum(t *testing.T) {
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	_, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"midtrans_env": "staging",
	})
	if err == nil {
		t.Fatal("expected error for invalid enum value")
	}
}

func TestUpdateSystemConfig_SkipsStarStarStarForSecrets(t *testing.T) {
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	// First, set a secret value
	firstResult, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"midtrans_server_key": "real-secret-value",
	})
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if firstResult["midtrans_server_key"] != "***" {
		t.Fatalf("after setting secret, should be masked as '***', got %q", firstResult["midtrans_server_key"])
	}

	// Now update with "***" — should keep the existing encrypted value
	secondResult, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"app_name":            "Updated",
		"midtrans_server_key": "***",
	})
	if err != nil {
		t.Fatalf("second update with '***': %v", err)
	}

	// midtrans_server_key should still be "***" (existing encrypted value preserved)
	if secondResult["midtrans_server_key"] != "***" {
		t.Errorf("after '***' skip, want '***', got %q", secondResult["midtrans_server_key"])
	}

	// Verify the underlying stored value wasn't overwritten
	rows, _ := svc.repo.ListSystemConfig(ctx)
	var storedSecret string
	for _, r := range rows {
		if r.Key == "midtrans_server_key" {
			storedSecret = r.Value
		}
	}
	// Should still be the encrypted version of "real-secret-value"
	decrypted, err := decryptConfigValue("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2", storedSecret)
	if err != nil {
		t.Fatalf("decrypt stored secret: %v", err)
	}
	if decrypted != "real-secret-value" {
		t.Errorf("stored secret should still be encrypted version of 'real-secret-value', decrypted to %q", decrypted)
	}
}

// ---------------------------------------------------------------------------
// New Biteship + Origin Location keys
// ---------------------------------------------------------------------------

func TestBuildConfigMap_NewKeysPresent(t *testing.T) {
	// Verify that all new keys appear in the catalog and default to ""
	result := buildConfigMap([]repository.SystemConfigRow{})

	newKeys := []string{
		"app_province_id",
		"app_city_id",
		"app_district_id",
		"app_kode_pos",
		"shipping_fallback_flat_rate",
		"biteship_api_key",
	}

	for _, key := range newKeys {
		if _, ok := result[key]; !ok {
			t.Errorf("key %q missing from result", key)
		}
		if result[key] != "" {
			t.Errorf("key %q should default to empty, got %q", key, result[key])
		}
	}
}

func TestBuildConfigMap_BiteshipKeyMasked(t *testing.T) {
	// biteship_api_key is secret: true, so it should be masked as "***" when non-empty
	rows := []repository.SystemConfigRow{
		{Key: "biteship_api_key", Value: "encrypted-blob", IsSecret: true},
		{Key: "shipping_fallback_flat_rate", Value: "50000", IsSecret: false},
	}

	result := buildConfigMap(rows)

	if result["biteship_api_key"] != "***" {
		t.Errorf("biteship_api_key: want '***', got %q", result["biteship_api_key"])
	}
	if result["shipping_fallback_flat_rate"] != "50000" {
		t.Errorf("shipping_fallback_flat_rate: want '50000', got %q", result["shipping_fallback_flat_rate"])
	}
}

func TestUpdateSystemConfig_NewNonSecretKeys(t *testing.T) {
	// Verify that non-secret origin/fallback keys round-trip through UpdateSystemConfig
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	result, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"app_province_id":             "1",
		"app_city_id":                 "2",
		"app_district_id":             "3",
		"app_kode_pos":                "12345",
		"shipping_fallback_flat_rate": "50000",
	})
	if err != nil {
		t.Fatalf("UpdateSystemConfig: %v", err)
	}

	if result["app_province_id"] != "1" {
		t.Errorf("app_province_id: want '1', got %q", result["app_province_id"])
	}
	if result["app_city_id"] != "2" {
		t.Errorf("app_city_id: want '2', got %q", result["app_city_id"])
	}
	if result["app_district_id"] != "3" {
		t.Errorf("app_district_id: want '3', got %q", result["app_district_id"])
	}
	if result["app_kode_pos"] != "12345" {
		t.Errorf("app_kode_pos: want '12345', got %q", result["app_kode_pos"])
	}
	if result["shipping_fallback_flat_rate"] != "50000" {
		t.Errorf("shipping_fallback_flat_rate: want '50000', got %q", result["shipping_fallback_flat_rate"])
	}
}

func TestUpdateSystemConfig_BiteshipKeyIsSecret(t *testing.T) {
	// Verify that biteship_api_key is encrypted and returned masked
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	result, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"biteship_api_key": "real-biteship-key-12345",
	})
	if err != nil {
		t.Fatalf("UpdateSystemConfig: %v", err)
	}

	// Should be masked as "***"
	if result["biteship_api_key"] != "***" {
		t.Errorf("biteship_api_key: want '***', got %q", result["biteship_api_key"])
	}

	// Verify it was encrypted in storage
	rows, _ := svc.repo.ListSystemConfig(ctx)
	var storedEncrypted string
	for _, r := range rows {
		if r.Key == "biteship_api_key" {
			storedEncrypted = r.Value
		}
	}

	if storedEncrypted == "" {
		t.Fatal("biteship_api_key should be stored with encrypted value")
	}
	if storedEncrypted == "real-biteship-key-12345" {
		t.Fatal("biteship_api_key should be encrypted, not plaintext")
	}

	// Verify it can be decrypted
	decrypted, err := decryptConfigValue("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2", storedEncrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "real-biteship-key-12345" {
		t.Errorf("decrypted: want 'real-biteship-key-12345', got %q", decrypted)
	}
}

func TestUpdateSystemConfig_BiteshipKeySkipsStarStarStar(t *testing.T) {
	// Verify that "***" for biteship_api_key skips the update, preserving the existing encrypted value
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	// First update: set a real value
	firstResult, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"biteship_api_key": "original-key-value",
	})
	if err != nil {
		t.Fatalf("first update: %v", err)
	}
	if firstResult["biteship_api_key"] != "***" {
		t.Fatalf("after setting secret, should be masked as '***', got %q", firstResult["biteship_api_key"])
	}

	// Get the encrypted value from storage
	rows1, _ := svc.repo.ListSystemConfig(ctx)
	var originalEncrypted string
	for _, r := range rows1 {
		if r.Key == "biteship_api_key" {
			originalEncrypted = r.Value
		}
	}

	// Second update: send "***" — should skip it
	secondResult, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"app_province_id":  "5",
		"biteship_api_key": "***",
	})
	if err != nil {
		t.Fatalf("second update with '***': %v", err)
	}

	// biteship_api_key should still be "***"
	if secondResult["biteship_api_key"] != "***" {
		t.Errorf("after '***' skip, want '***', got %q", secondResult["biteship_api_key"])
	}

	// Verify the underlying encrypted value wasn't changed
	rows2, _ := svc.repo.ListSystemConfig(ctx)
	var afterSkipEncrypted string
	for _, r := range rows2 {
		if r.Key == "biteship_api_key" {
			afterSkipEncrypted = r.Value
		}
	}

	if afterSkipEncrypted != originalEncrypted {
		t.Error("biteship_api_key encrypted value should not change when '***' is sent")
	}

	// Verify the value can still be decrypted to the original
	decrypted, err := decryptConfigValue("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2", afterSkipEncrypted)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "original-key-value" {
		t.Errorf("decrypted: want 'original-key-value', got %q", decrypted)
	}
}

// ---------------------------------------------------------------------------
// biteship_webhook_secret catalog key (FR-C-5) — the one new key Track C adds
// ---------------------------------------------------------------------------

func TestConfigKeyCatalog_WebhookSecretIsShippingGroupSecret(t *testing.T) {
	def, ok := configKeyCatalog["biteship_webhook_secret"]
	if !ok {
		t.Fatal("biteship_webhook_secret missing from configKeyCatalog")
	}
	if def.group != "shipping" {
		t.Errorf("group: want 'shipping', got %q", def.group)
	}
	if !def.secret {
		t.Error("biteship_webhook_secret must be secret: true")
	}
}

func TestBuildConfigMap_WebhookSecretKeyDefaultsEmpty(t *testing.T) {
	result := buildConfigMap([]repository.SystemConfigRow{})
	if _, ok := result["biteship_webhook_secret"]; !ok {
		t.Error("biteship_webhook_secret missing from result")
	}
	if result["biteship_webhook_secret"] != "" {
		t.Errorf("biteship_webhook_secret should default to empty, got %q", result["biteship_webhook_secret"])
	}
}

func TestBuildConfigMap_WebhookSecretMasked(t *testing.T) {
	rows := []repository.SystemConfigRow{
		{Key: "biteship_webhook_secret", Value: "encrypted-blob", IsSecret: true},
	}
	result := buildConfigMap(rows)
	if result["biteship_webhook_secret"] != "***" {
		t.Errorf("biteship_webhook_secret: want '***', got %q", result["biteship_webhook_secret"])
	}
}

func TestUpdateSystemConfig_WebhookSecretIsEncryptedAndMasked(t *testing.T) {
	ctx := context.Background()
	svc := newFakeSystemConfigService("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2")

	result, err := svc.UpdateSystemConfig(ctx, map[string]string{
		"biteship_webhook_secret": "test-webhook-secret",
	})
	if err != nil {
		t.Fatalf("UpdateSystemConfig: %v", err)
	}
	if result["biteship_webhook_secret"] != "***" {
		t.Errorf("biteship_webhook_secret: want '***', got %q", result["biteship_webhook_secret"])
	}

	rows, _ := svc.repo.ListSystemConfig(ctx)
	var stored string
	for _, r := range rows {
		if r.Key == "biteship_webhook_secret" {
			stored = r.Value
		}
	}
	if stored == "" || stored == "test-webhook-secret" {
		t.Fatalf("biteship_webhook_secret should be stored encrypted, got %q", stored)
	}
	decrypted, err := decryptConfigValue("a1b2c3d4e5f6a7b8c9d0e1f2a3b4c5d6e7f8a9b0c1d2e3f4a5b6c7d8e9f0a1b2", stored)
	if err != nil {
		t.Fatalf("decrypt: %v", err)
	}
	if decrypted != "test-webhook-secret" {
		t.Errorf("decrypted: want 'test-webhook-secret', got %q", decrypted)
	}
}
