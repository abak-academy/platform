package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"

	"akademi-bimbel/internal/repository"
)

var (
	// ErrConfigEncryption is returned when AES-256-GCM encryption or decryption
	// fails (invalid key, tampered ciphertext, etc.). Maps to 500 internal_error.
	ErrConfigEncryption = errors.New("config encryption failed")

	// ErrUnknownConfigKey is returned when a provided config key is not in the
	// fixed catalog. Maps to 400 invalid_request.
	ErrUnknownConfigKey = errors.New("unknown config key")
)

// configKeyDef describes a single key in the system_config fixed catalog.
type configKeyDef struct {
	group      string   // app, notification, payment
	valueType  string   // "string", "bool", "enum"
	secret     bool     // true → stored encrypted, returned as "***"
	enumValues []string // valid values for "enum" type
}

// configKeyCatalog is the fixed set of keys managed via /admin/system/config.
// Adding or removing keys is a code change (not data-driven).
var configKeyCatalog = map[string]configKeyDef{
	"app_name":                     {group: "app", valueType: "string"},
	"app_address":                  {group: "app", valueType: "string"},
	"app_logo_url":                 {group: "app", valueType: "string"},
	"app_contact_email":            {group: "app", valueType: "string"},
	"app_contact_phone":            {group: "app", valueType: "string"},
	"app_province_id":              {group: "app", valueType: "string"},
	"app_city_id":                  {group: "app", valueType: "string"},
	"app_district_id":              {group: "app", valueType: "string"},
	"app_kode_pos":                 {group: "app", valueType: "string"},
	"exam_platform":                {group: "app", valueType: "string"},
	"notify_on_purchase_admin_store": {group: "notification", valueType: "bool"},
	"notify_on_purchase_admin_exam":  {group: "notification", valueType: "bool"},
	"midtrans_server_key":          {group: "payment", valueType: "string", secret: true},
	"midtrans_client_key":          {group: "payment", valueType: "string", secret: true},
	"midtrans_env":                 {group: "payment", valueType: "enum", enumValues: []string{"sandbox", "production"}},
	"shipping_fallback_flat_rate":  {group: "shipping", valueType: "string"},
	"biteship_api_key":             {group: "shipping", valueType: "string", secret: true},
}

// encryptConfigValue encrypts plaintext with AES-256-GCM.
// hexKey is a 64-char hex string (decoded to 32 bytes).
// Returns base64(nonce || ciphertext+tag).
func encryptConfigValue(hexKey, plaintext string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return "", ErrConfigEncryption
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrConfigEncryption
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrConfigEncryption
	}

	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", ErrConfigEncryption
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(append(nonce, ciphertext...)), nil
}

// EncryptConfigValue encrypts plaintext with AES-256-GCM.
// hexKey is a 64-char hex string (decoded to 32 bytes).
func EncryptConfigValue(hexKey, plaintext string) (string, error) {
	return encryptConfigValue(hexKey, plaintext)
}

// DecryptConfigValue decrypts a base64(nonce || ciphertext+tag) string.
// hexKey is a 64-char hex string (decoded to 32 bytes).
func DecryptConfigValue(hexKey, encoded string) (string, error) {
	return decryptConfigValue(hexKey, encoded)
}

// decryptConfigValue decrypts a base64(nonce || ciphertext+tag) string.
// hexKey is a 64-char hex string (decoded to 32 bytes).
func decryptConfigValue(hexKey, encoded string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil || len(key) != 32 {
		return "", ErrConfigEncryption
	}

	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(data) < 12 {
		return "", ErrConfigEncryption
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", ErrConfigEncryption
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrConfigEncryption
	}

	plaintext, err := gcm.Open(nil, data[:12], data[12:], nil)
	if err != nil {
		return "", ErrConfigEncryption
	}

	return string(plaintext), nil
}

// validateConfigKeys checks that every key in values belongs to the catalog and
// that its value satisfies the key's type constraints (bool, enum). Returns the
// first error encountered.
func validateConfigKeys(values map[string]string) error {
	for key, val := range values {
		def, ok := configKeyCatalog[key]
		if !ok {
			return fmt.Errorf("%w: %s", ErrUnknownConfigKey, key)
		}
		switch def.valueType {
		case "bool":
			if val != "true" && val != "false" {
				return fmt.Errorf("%w: %q must be \"true\" or \"false\"", ErrUnknownConfigKey, key)
			}
		case "enum":
			allowed := false
			for _, e := range def.enumValues {
				if val == e {
					allowed = true
					break
				}
			}
			if !allowed {
				return fmt.Errorf("%w: %q invalid value %q for enum", ErrUnknownConfigKey, key, val)
			}
		}
	}
	return nil
}

// buildConfigMap builds a full config map from stored rows. Every catalog key
// is present in the result (defaults to ""). Secret values with non-empty
// stored values are masked as "***".
func buildConfigMap(rows []repository.SystemConfigRow) map[string]string {
	// Seed with all catalog keys defaulting to ""
	result := make(map[string]string, len(configKeyCatalog))
	for key := range configKeyCatalog {
		result[key] = ""
	}

	// Overlay stored values
	for _, row := range rows {
		def, ok := configKeyCatalog[row.Key]
		if !ok {
			continue // unknown stored key — skip
		}
		if def.secret && row.Value != "" {
			result[row.Key] = "***"
		} else {
			result[row.Key] = row.Value
		}
	}

	return result
}

// processConfigValues processes update values for upsert:
//   - Secret keys with value "***" are skipped (left unchanged).
//   - Secret keys with value "" are stored as empty (not encrypted) so buildConfigMap returns "".
//   - Secret keys with other values are AES-256-GCM encrypted.
//   - Non-secret keys are passed through as-is.
//
// Returns the processed key→value map and the list of changed keys.
func processConfigValues(values map[string]string, hexKey string) (processed map[string]string, changedKeys []string, err error) {
	processed = make(map[string]string, len(values))
	for key, val := range values {
		def := configKeyCatalog[key]

		// "***" for secret key means "leave unchanged"
		if def.secret && val == "***" {
			continue
		}

		var storedValue string
		if def.secret {
			if val == "" {
				storedValue = ""
			} else {
				storedValue, err = encryptConfigValue(hexKey, val)
				if err != nil {
					return nil, nil, err
				}
			}
		} else {
			storedValue = val
		}

		processed[key] = storedValue
		changedKeys = append(changedKeys, key)
	}
	return processed, changedKeys, nil
}

// GetPaymentClientKey returns the Midtrans client key from DB config (decrypted).
// Falls back to env var. Client key is public per Midtrans documentation.
func (s *Service) GetPaymentClientKey(ctx context.Context) (string, error) {
	rows, err := s.storeRepo.ListSystemConfig(ctx)
	if err != nil {
		// If DB read fails, fall back to env
		if s.cfg.MidtransClientKey != "" {
			return s.cfg.MidtransClientKey, nil
		}
		return "", err
	}

	for _, row := range rows {
		if row.Key == "midtrans_client_key" && row.IsSecret && row.Value != "" {
			return decryptConfigValue(s.cfg.ConfigEncryptionKey, row.Value)
		}
	}

	// Fall back to env
	return s.cfg.MidtransClientKey, nil
}

// GetSystemConfig returns the full config map. Secret values are masked as
// "***". Missing keys default to "". Every key in the fixed catalog is present.
func (s *Service) GetSystemConfig(ctx context.Context) (map[string]string, error) {
	rows, err := s.storeRepo.ListSystemConfig(ctx)
	if err != nil {
		return nil, err
	}
	return buildConfigMap(rows), nil
}

// UpdateSystemConfig upserts the provided config keys. It validates keys
// against the catalog, encrypts secret values, skips "***" sentinels, writes
// a single audit row, and returns the full masked config map.
func (s *Service) UpdateSystemConfig(ctx context.Context, actorID string, values map[string]string) (map[string]string, error) {
	if err := validateConfigKeys(values); err != nil {
		return nil, err
	}

	processed, changedKeys, err := processConfigValues(values, s.cfg.ConfigEncryptionKey)
	if err != nil {
		return nil, err
	}

	for key, val := range processed {
		def := configKeyCatalog[key]
		if err := s.storeRepo.UpsertSystemConfig(ctx, key, val, def.secret); err != nil {
			return nil, err
		}
	}

	// One audit row per batch — never log values
	actor := &actorID
	if auditErr := s.storeRepo.InsertAuditLogMeta(ctx, nil, actor, "system_config", "config", "config.update", map[string]any{
		"changed_keys": changedKeys,
	}); auditErr != nil {
		// Non-fatal: log and continue
		return nil, auditErr
	}

	return s.GetSystemConfig(ctx)
}
