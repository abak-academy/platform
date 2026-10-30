package service

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"akademi-bimbel/internal/repository"
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
	"app_name":                       {group: "app", valueType: "string"},
	"app_address":                    {group: "app", valueType: "string"},
	"app_logo_url":                   {group: "app", valueType: "string"},
	"app_contact_email":              {group: "app", valueType: "string"},
	"app_contact_phone":              {group: "app", valueType: "string"},
	"app_province_id":                {group: "app", valueType: "string"},
	"app_city_id":                    {group: "app", valueType: "string"},
	"app_district_id":                {group: "app", valueType: "string"},
	"app_kode_pos":                   {group: "app", valueType: "string"},
	"app_latitude":                   {group: "app", valueType: "string"},
	"app_longitude":                  {group: "app", valueType: "string"},
	"exam_platform":                  {group: "app", valueType: "string"},
	"app_help_url":                   {group: "app", valueType: "string"},
	"app_social_handle":              {group: "app", valueType: "string"},
	"notify_on_purchase_admin_store": {group: "notification", valueType: "bool"},
	"notify_on_purchase_admin_exam":  {group: "notification", valueType: "bool"},
	"midtrans_server_key":            {group: "payment", valueType: "string", secret: true},
	"midtrans_client_key":            {group: "payment", valueType: "string", secret: true},
	"midtrans_env":                   {group: "payment", valueType: "enum", enumValues: []string{"sandbox", "production"}},
	"biteship_services":              {group: "shipping", valueType: "string"},
	"biteship_api_key":               {group: "shipping", valueType: "string", secret: true},
	"biteship_webhook_secret":        {group: "shipping", valueType: "string", secret: true},
	"affiliate_enabled":              {group: "app", valueType: "bool"},
}

// TODO: uncertain — service codes mapped from dashboard names via Biteship docs, reconcile with GET /v1/couriers before prod
var biteshipServiceCatalog = map[string][]string{
	"jne":      {"reg", "yes", "oke", "jtr"},
	"sicepat":  {"reg", "best", "gokil"},
	"anteraja": {"reg", "same_day"},
	"tiki":     {"reg", "eko", "ons", "sds"},
	"lion":     {"reg_pack", "big_pack"},
}

var biteshipCourierOrder = []string{"jne", "sicepat", "anteraja", "tiki", "lion"}

const defaultBiteshipServices = "jne:reg,jne:yes,jne:oke"

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
		if key == "biteship_services" {
			seen := make(map[string]bool)
			for _, pair := range strings.Split(val, ",") {
				courier, service, found := strings.Cut(pair, ":")
				if !found || !slices.Contains(biteshipServiceCatalog[courier], service) || seen[pair] {
					return fmt.Errorf("%w: invalid or duplicate courier service %q", ErrUnknownConfigKey, pair)
				}
				seen[pair] = true
			}
		}

		if (key == "app_latitude" || key == "app_longitude") && val != "" {
			limit := 90.0
			if key == "app_longitude" {
				limit = 180
			}
			coordinate, err := strconv.ParseFloat(val, 64)
			if err != nil || math.IsNaN(coordinate) || coordinate < -limit || coordinate > limit {
				return fmt.Errorf("%w: %q must be a number within [%g, %g]", ErrUnknownConfigKey, key, -limit, limit)
			}
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
	latitude, hasLatitude := values["app_latitude"]
	longitude, hasLongitude := values["app_longitude"]
	if hasLatitude && hasLongitude && (latitude == "") != (longitude == "") {
		return fmt.Errorf("%w: app_latitude and app_longitude must be both empty or both set", ErrUnknownConfigKey)
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
// cardConfigKeys are the system_config values printed on the participant card.
// A cached card PDF is reused, never re-rendered, so changing one of these has
// to invalidate the cache or the edit never reaches a student who already
// downloaded a card.
var cardConfigKeys = []string{
	"app_name", "app_logo_url", "app_contact_phone",
	"app_contact_email", "app_help_url", "app_social_handle",
}

// cardConfigChanged reports whether this submission actually changes a
// card-visible value. changedKeys cannot answer that — the config page PUTs
// every key on every save, so it always lists all of them.
func cardConfigChanged(before map[string]string, values map[string]string) bool {
	for _, k := range cardConfigKeys {
		if v, ok := values[k]; ok && v != before[k] {
			return true
		}
	}
	return false
}

func (s *Service) UpdateSystemConfig(ctx context.Context, actorID string, values map[string]string) (map[string]string, error) {
	if err := validateConfigKeys(values); err != nil {
		return nil, err
	}

	before, err := s.GetSystemConfig(ctx)
	if err != nil {
		return nil, err
	}

	processed, changedKeys, err := processConfigValues(values, s.cfg.ConfigEncryptionKey)
	if err != nil {
		return nil, err
	}
	invalidateCards := cardConfigChanged(before, values)

	for key, val := range processed {
		def := configKeyCatalog[key]
		if err := s.storeRepo.UpsertSystemConfig(ctx, key, val, def.secret); err != nil {
			return nil, err
		}
	}

	if values["affiliate_enabled"] == "true" && before["affiliate_enabled"] != "true" {
		if err := s.storeRepo.UpsertSystemConfig(ctx, affiliateEnabledSinceKey, time.Now().UTC().Format(time.RFC3339), false); err != nil {
			return nil, err
		}
	}

	if invalidateCards {
		if err := s.storeRepo.ClearAllRegistrationCards(ctx); err != nil {
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
