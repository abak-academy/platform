package service

import (
	"context"
	"fmt"
	"time"

	"akademi-bimbel/internal/metrics"

	"github.com/redis/go-redis/v9"
)

// OTP send-protection limits (konsep-otp.md §6/§9). Knobs come from config;
// zero values fall back to these free-tier starting points.
const (
	defaultOTPQuotaHour   = 3
	defaultOTPQuotaDay    = 8
	defaultOTPQuotaIPHour = 20
	defaultOTPSendfailMax = 3
	defaultSMTPQuotaDay   = 500

	otpHourWindow = time.Hour
	otpDayWindow  = 24 * time.Hour
)

type otpSendKeys struct {
	cd       string
	hr       string
	day      string
	lvl      string
	gq       string
	ip       string
	sendfail string
	abuse    string
}

// loginOTPSendKeys builds the account-verification key set. Existing key
// names (otp:cooldown, otp:count) are kept so old data keeps working.
func loginOTPSendKeys(userID, channel, ip string) otpSendKeys {
	return otpSendKeys{
		cd:       otpCooldownKey(userID),
		hr:       "otp:hr:" + userID,
		day:      "otp:day:" + userID,
		lvl:      otpCountKey(userID),
		gq:       "smtp:quota:placeholder",
		ip:       "otp:ip:" + ip,
		sendfail: otpSendfailKey(userID, channel),
		abuse:    "otp:abuse:" + userID,
	}
}

func (s *Service) otpQuotaHour() int {
	if s.cfg != nil && s.cfg.OTPQuotaHour > 0 {
		return s.cfg.OTPQuotaHour
	}
	return defaultOTPQuotaHour
}

func (s *Service) otpQuotaDay() int {
	if s.cfg != nil && s.cfg.OTPQuotaDay > 0 {
		return s.cfg.OTPQuotaDay
	}
	return defaultOTPQuotaDay
}

func (s *Service) otpQuotaIPHour() int {
	if s.cfg != nil && s.cfg.OTPQuotaIPHour > 0 {
		return s.cfg.OTPQuotaIPHour
	}
	return defaultOTPQuotaIPHour
}

func (s *Service) otpSendfailMax() int {
	if s.cfg != nil && s.cfg.OTPSendfailMax > 0 {
		return s.cfg.OTPSendfailMax
	}
	return defaultOTPSendfailMax
}

func (s *Service) otpSendfailPause() time.Duration {
	if s.cfg != nil && s.cfg.OTPSendfailPause > 0 {
		return s.cfg.OTPSendfailPause
	}
	return otpSendfailPause
}

func (s *Service) otpSMTPQuotaDay() int {
	if s.cfg != nil && s.cfg.SMTPQuotaDay > 0 {
		return s.cfg.SMTPQuotaDay
	}
	return defaultSMTPQuotaDay
}

// smtpQuotaDateKey returns today's global provider-quota key (YYYYMMDD in
// WIB) and the unix seconds when tomorrow's WIB midnight begins — konsep
// §12: the window resets when the provider resets.
func smtpQuotaDateKey(now time.Time) (string, int64) {
	loc := time.FixedZone("WIB", 7*3600)
	d := now.In(loc)
	key := fmt.Sprintf("smtp:quota:%04d%02d%02d", d.Year(), d.Month(), d.Day())
	nextMidnight := time.Date(d.Year(), d.Month(), d.Day()+1, 0, 0, 0, 0, loc)
	return key, nextMidnight.Unix()
}

// otpClaimScript atomically checks every send gate. On denial it consumes
// nothing and reports the longest tripped TTL; on approval it increments it
// (hour/day/IP/global with fixed windows — EXPIRE only on the first INCR —
// plus the level and ladder cooldown), konsep-otp.md §4/§6/§7.
var otpClaimScript = redis.NewScript(`
local denial = nil
local maxTTL = 0
local function mark(name, ttl)
	if ttl and ttl > maxTTL then
		maxTTL = ttl
		denial = {0, name, ttl}
	end
end

local cdTTL = redis.call('TTL', KEYS[1])
if cdTTL > 0 then mark('cooldown', cdTTL) end

local hrLim = tonumber(ARGV[1])
if hrLim > 0 then
	local ttl = redis.call('TTL', KEYS[2])
	if ttl > 0 and (tonumber(redis.call('GET', KEYS[2])) or 0) >= hrLim then mark('hour', ttl) end
end
local dayLim = tonumber(ARGV[2])
if dayLim > 0 then
	local ttl = redis.call('TTL', KEYS[3])
	if ttl > 0 and (tonumber(redis.call('GET', KEYS[3])) or 0) >= dayLim then mark('day', ttl) end
end
local ipLim = tonumber(ARGV[3])
if ipLim > 0 then
	local ttl = redis.call('TTL', KEYS[4])
	if ttl > 0 and (tonumber(redis.call('GET', KEYS[4])) or 0) >= ipLim then mark('ip', ttl) end
end
local gLim = tonumber(ARGV[4])
if gLim > 0 then
	local ttl = redis.call('TTL', KEYS[6])
	if ttl > 0 and (tonumber(redis.call('GET', KEYS[6])) or 0) >= gLim then mark('global', ttl) end
end

local sfMax = tonumber(ARGV[9])
if sfMax > 0 then
	local ttl = redis.call('TTL', KEYS[7])
	if ttl > 0 and (tonumber(redis.call('GET', KEYS[7])) or 0) >= sfMax then mark('sendfail', ttl) end
end

if denial then return denial end

local hr = redis.call('INCR', KEYS[2])
if hr == 1 then redis.call('EXPIRE', KEYS[2], ARGV[5]) end
local day = redis.call('INCR', KEYS[3])
if day == 1 then redis.call('EXPIRE', KEYS[3], ARGV[6]) end
if ipLim > 0 then
	local ip = redis.call('INCR', KEYS[4])
	if ip == 1 then redis.call('EXPIRE', KEYS[4], ARGV[7]) end
end
if gLim > 0 then
	local gq = redis.call('INCR', KEYS[6])
	if gq == 1 then redis.call('EXPIREAT', KEYS[6], ARGV[8]) end
end
local lvl = redis.call('INCR', KEYS[5])
local ladderBase = 10
local n = #ARGV - ladderBase
redis.call('EXPIRE', KEYS[5], ARGV[ladderBase])
local idx = lvl - 1
if idx >= n then idx = n end
local cdSecs = tonumber(ARGV[ladderBase + idx + 1])
redis.call('SET', KEYS[1], '1')
redis.call('EXPIRE', KEYS[1], cdSecs)
return {1, cdSecs}
`)

// otpRollbackScript reverses a claim after a provider failure: counters with
// a positive value are decremented, konsep-otp.md §9.
var otpRollbackScript = redis.NewScript(`
local function decr(key)
	local v = tonumber(redis.call('GET', key)) or 0
	if v > 0 then redis.call('DECR', key) end
end
decr(KEYS[2])
decr(KEYS[3])
decr(KEYS[4])
decr(KEYS[5])
decr(KEYS[6])
return {1, 0}
`)

// claimOTPSend atomically checks every send gate (cooldown, hour/day/IP
// quotas, global provider quota, sendfail streak) and, when all pass,
// increments them in one Redis op. A denial consumes nothing.
func (s *Service) claimOTPSend(ctx context.Context, keys otpSendKeys) error {
	gKey, gExp := smtpQuotaDateKey(time.Now())
	keys.gq = gKey
	ladder := ladderSeconds()
	args := []interface{}{
		s.otpQuotaHour(), s.otpQuotaDay(), s.otpQuotaIPHour(), s.otpSMTPQuotaDay(),
		int(otpHourWindow.Seconds()), int(otpDayWindow.Seconds()), int(time.Hour.Seconds()),
		gExp, s.otpSendfailMax(), int(otpLadderWindow.Seconds()),
	}
	for _, d := range ladder {
		args = append(args, d)
	}
	res, err := otpClaimScript.Run(ctx, s.rdb,
		[]string{keys.cd, keys.hr, keys.day, keys.ip, keys.lvl, gKey, keys.sendfail},
		args...,
	).Result()
	if err != nil {
		return fmt.Errorf("otp claim: %w", err)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) == 0 {
		return fmt.Errorf("otp claim: unexpected reply %v", res)
	}
	status, _ := arr[0].(int64)
	if status == 1 {
		return nil
	}
	bucket, _ := arr[1].(string)
	ttl, _ := arr[2].(int64)
	metrics.ObserveOTPGateDenial(bucket)
	return &OTPGateError{OTPRateLimitError: &OTPRateLimitError{RetryAfter: time.Duration(ttl) * time.Second}, Bucket: bucket}
}

func ladderSeconds() []int {
	out := make([]int, 0, len(otpResendLadder))
	for _, d := range otpResendLadder {
		out = append(out, int(d.Seconds()))
	}
	return out
}

// rollbackOTPClaim reverses a claim after a provider failure: quotas and the
// level are decremented, so failures never consume the budget (konsep §9).
func (s *Service) rollbackOTPClaim(ctx context.Context, keys otpSendKeys) error {
	gKey, _ := smtpQuotaDateKey(time.Now())
	_, err := otpRollbackScript.Run(ctx, s.rdb,
		[]string{keys.cd, keys.hr, keys.day, keys.ip, keys.lvl, gKey},
	).Result()
	return err
}

// recordOTPGateReject counts a rate-limited attempt in the abuse bucket —
// rejected requests must never consume the send quota; the counter is only a
// signal (concept §4).
func (s *Service) recordOTPGateReject(ctx context.Context, keys otpSendKeys) {
	n, err := s.rdb.Incr(ctx, keys.abuse).Result()
	if err == nil && n == 1 {
		s.rdb.Expire(ctx, keys.abuse, otpHourWindow)
	}
}

// SetOTPConfigForTest overrides the OTP quota knobs in handler tests.
func (s *Service) SetOTPConfigForTest(ipQuota int) {
	if s.cfg != nil {
		s.cfg.OTPQuotaIPHour = ipQuota
	}
}

// Verification-failure budget defaults (konsep §6): 10-15 wrong entries per
// hour and 20-30 per day per target; the per-IP budget is looser.
const (
	defaultOTPVfailHour   = 10
	defaultOTPVfailDay    = 20
	defaultOTPVfailIPHour = 30
	defaultOTPAttempts    = 3
)

func (s *Service) otpVfailHour() int {
	if s.cfg != nil && s.cfg.OTPVfailHour > 0 {
		return s.cfg.OTPVfailHour
	}
	return defaultOTPVfailHour
}

func (s *Service) otpVfailDay() int {
	if s.cfg != nil && s.cfg.OTPVfailDay > 0 {
		return s.cfg.OTPVfailDay
	}
	return defaultOTPVfailDay
}

func (s *Service) otpVfailIPHour() int {
	if s.cfg != nil && s.cfg.OTPVfailIPHour > 0 {
		return s.cfg.OTPVfailIPHour
	}
	return defaultOTPVfailIPHour
}

func (s *Service) otpAttempts() int {
	if s.cfg != nil && s.cfg.OTPAttempts > 0 {
		return s.cfg.OTPAttempts
	}
	return defaultOTPAttempts
}

type otpVerifyKeys struct {
	vfailHour string
	vfailDay  string
	vfailIP   string
}

func loginOTPVerifyKeys(userID, ip string) otpVerifyKeys {
	return otpVerifyKeys{
		vfailHour: "otp:vfail:" + userID,
		vfailDay:  "otp:vfail:day:" + userID,
		vfailIP:   "otp:vfail:ip:" + ip,
	}
}

// otpVerifyGateScript runs the verify-side budget atomically (konsep §5): the
// vfail gates are checked first, then the per-code attempt counter is
// incremented BEFORE the comparison — the script returns the stored value and
// the raised attempt count so the caller can compare safely under parallel
// submissions.
var otpVerifyGateScript = redis.NewScript(`
local denial, maxTTL = nil, 0
local function mark(name, ttl)
	if ttl and ttl > maxTTL then
		maxTTL = ttl
		denial = {0, name, ttl}
	end
end

local hf = redis.call('TTL', KEYS[2])
if hf > 0 and (tonumber(redis.call('GET', KEYS[2])) or 0) >= tonumber(ARGV[1]) then mark('vfail_hour', hf) end
local df = redis.call('TTL', KEYS[3])
if df > 0 and (tonumber(redis.call('GET', KEYS[3])) or 0) >= tonumber(ARGV[2]) then mark('vfail_day', df) end
local ipLim = tonumber(ARGV[3])
if ipLim > 0 then
	local ttl = redis.call('TTL', KEYS[4])
	if ttl > 0 and (tonumber(redis.call('GET', KEYS[4])) or 0) >= ipLim then mark('vfail_ip', ttl) end
end

if denial then return denial end

local otpVal = redis.call('GET', KEYS[1])
if not otpVal then return {2, '', 0} end
local attempts = redis.call('INCR', KEYS[5])
if attempts == 1 then redis.call('EXPIRE', KEYS[5], ARGV[4]) end
return {1, otpVal, attempts}
`)

// runOTPVerifyGate returns the stored code value and the atomically raised
// attempt count. A nil error with status 2 means the code is gone (expired).
func (s *Service) runOTPVerifyGate(ctx context.Context, userID, ip string) (otpVal string, attempts int64, deny error) {
	vk := loginOTPVerifyKeys(userID, ip)
	res, err := otpVerifyGateScript.Run(ctx, s.rdb,
		[]string{"otp:" + userID, vk.vfailHour, vk.vfailDay, vk.vfailIP, otpVerifyKey(userID)},
		s.otpVfailHour(), s.otpVfailDay(), s.otpVfailIPHour(), int(s.cfg.OTPTTL.Seconds()),
	).Result()
	if err != nil {
		return "", 0, fmt.Errorf("otp verify gate: %w", err)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 3 {
		return "", 0, fmt.Errorf("otp verify gate: unexpected reply %v", res)
	}
	status, _ := arr[0].(int64)
	if status == 0 {
		bucket, _ := arr[1].(string)
		ttl, _ := arr[2].(int64)
		metrics.ObserveOTPGateDenial(bucket)
		return "", 0, &OTPGateError{OTPRateLimitError: &OTPRateLimitError{RetryAfter: time.Duration(ttl) * time.Second}, Bucket: bucket}
	}
	otpVal, _ = arr[1].(string)
	attempts, _ = arr[2].(int64)
	return otpVal, attempts, nil
}

// recordOTPVerifyFailure raises the hour/day/IP wrong-entry budgets (konsep
// §6); these survive a resend.
func (s *Service) recordOTPVerifyFailure(ctx context.Context, userID, ip string) {
	vk := loginOTPVerifyKeys(userID, ip)
	fails := []struct {
		key string
		win time.Duration
	}{
		{vk.vfailHour, otpHourWindow},
		{vk.vfailDay, otpDayWindow},
	}
	if ip != "" {
		fails = append(fails, struct {
			key string
			win time.Duration
		}{vk.vfailIP, otpHourWindow})
	}
	for _, f := range fails {
		n, err := s.rdb.Incr(ctx, f.key).Result()
		if err == nil && n == 1 {
			s.rdb.Expire(ctx, f.key, f.win)
		}
	}
}

// passwordResetSendKeys builds the forgot/reset-purpose key set; the ":reset:"
// segment keeps these counters from colliding with the login ladder.
func passwordResetSendKeys(email, ip string) otpSendKeys {
	id := normalizeEmail(email)
	return otpSendKeys{
		cd:       "otp:cooldown:reset:" + id,
		hr:       "otp:hr:reset:" + id,
		day:      "otp:day:reset:" + id,
		lvl:      "otp:count:reset:" + id,
		sendfail: "otp:sendfail:reset:" + id + ":email",
		abuse:    "otp:abuse:reset:" + id,
	}
}

func resetVerifyKeys(userID, ip string) otpVerifyKeys {
	return otpVerifyKeys{
		vfailHour: "otp:vfail:reset:" + userID,
		vfailDay:  "otp:vfail:day:reset:" + userID,
		vfailIP:   "otp:vfail:ip:" + ip,
	}
}

// recordOTPSendFailureOn bumps the sendfail counter on an explicit key set and
// replaces the cooldown with the fixed short pause (konsep §9).
func (s *Service) recordOTPSendFailureOn(ctx context.Context, keys otpSendKeys) {
	n, err := s.rdb.Incr(ctx, keys.sendfail).Result()
	if err == nil && n == 1 {
		s.rdb.Expire(ctx, keys.sendfail, otpSendfailWindow)
	}
	s.rdb.Set(ctx, keys.cd, "1", s.otpSendfailPause())
}

// resetVerifyGateScript runs the reset-token budget atomically: the reset-
// purpose vfail budgets are checked before the attempt counter increments and
// the stored hash is handed back for comparison (konsep §5).
var resetVerifyGateScript = redis.NewScript(`
local denial, maxTTL = nil, 0
local function mark(name, ttl)
	if ttl and ttl > maxTTL then
		maxTTL = ttl
		denial = {0, name, ttl}
	end
end

local uid = redis.call('HGET', KEYS[1], 'uid')
if not uid then return {2, '', '', 0} end
local hash = redis.call('HGET', KEYS[1], 'hash')
if not hash then return {2, '', '', 0} end

local vfHourKey = ARGV[4] .. uid
local vfDayKey = ARGV[5] .. uid
local hf = redis.call('TTL', vfHourKey)
if hf > 0 and (tonumber(redis.call('GET', vfHourKey)) or 0) >= tonumber(ARGV[1]) then mark('vfail_hour', hf) end
local df = redis.call('TTL', vfDayKey)
if df > 0 and (tonumber(redis.call('GET', vfDayKey)) or 0) >= tonumber(ARGV[2]) then mark('vfail_day', df) end
local ipLim = tonumber(ARGV[3])
if ipLim > 0 then
	local ttl = redis.call('TTL', KEYS[2])
	if ttl > 0 and (tonumber(redis.call('GET', KEYS[2])) or 0) >= ipLim then mark('vfail_ip', ttl) end
end

if denial then return denial end

local attempts = redis.call('HINCRBY', KEYS[1], 'attempts', 1)
return {1, uid, hash, attempts}
`)

// runResetVerifyGate mirrors runOTPVerifyGate for the reset token. The vfail
// key prefixes ride ARGV because the per-uid suffix only exists once the
// script has read the token hash.
func (s *Service) runResetVerifyGate(ctx context.Context, token, ip string) (uid, hash string, attempts int64, deny error) {
	res, err := resetVerifyGateScript.Run(ctx, s.rdb,
		[]string{"reset:" + token, "otp:vfail:ip:" + ip},
		s.otpVfailHour(), s.otpVfailDay(), s.otpVfailIPHour(),
		"otp:vfail:reset:", "otp:vfail:day:reset:", s.otpAttempts(),
	).Result()
	if err != nil {
		return "", "", 0, fmt.Errorf("reset verify gate: %w", err)
	}
	arr, ok := res.([]interface{})
	if !ok || len(arr) != 4 {
		// A denial reply is {0, name, ttl}.
		if len(arr) == 3 {
			status, _ := arr[0].(int64)
			if status == 0 {
				bucket, _ := arr[1].(string)
				ttl, _ := arr[2].(int64)
				metrics.ObserveOTPGateDenial(bucket)
				return "", "", 0, &OTPGateError{OTPRateLimitError: &OTPRateLimitError{RetryAfter: time.Duration(ttl) * time.Second}, Bucket: bucket}
			}
		}
		return "", "", 0, fmt.Errorf("reset verify gate: unexpected reply %v", res)
	}
	status, _ := arr[0].(int64)
	if status == 0 {
		ttl, _ := arr[3].(int64)
		return "", "", 0, &OTPRateLimitError{RetryAfter: time.Duration(ttl) * time.Second}
	}
	uid, _ = arr[1].(string)
	hash, _ = arr[2].(string)
	attempts, _ = arr[3].(int64)
	return uid, hash, attempts, nil
}

// recordResetVerifyFailure raises the reset-purpose wrong-entry budgets.
func (s *Service) recordResetVerifyFailure(ctx context.Context, userID, ip string) {
	vk := resetVerifyKeys(userID, ip)
	for _, f := range []struct{ key string }{
		{vk.vfailHour},
		{vk.vfailDay},
	} {
		s.rdb.Incr(ctx, f.key)
		s.rdb.Expire(ctx, f.key, otpHourWindow)
	}
	if ip != "" {
		s.rdb.Incr(ctx, vk.vfailIP)
		s.rdb.Expire(ctx, vk.vfailIP, otpHourWindow)
	}
}

// OTPEmailRetryAfter exposes the delivery-failure pause for the handler's
// Retry-After header without leaking the secret or key space.
func (s *Service) OTPEmailRetryAfter() int {
	return int(s.otpSendfailPause().Seconds())
}

// ResendOTPResult carries what the resend endpoint needs to hand back.
type ResendOTPResult struct {
	PendingToken string
	ResendAfter  int
}
