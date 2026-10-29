package service

import (
	"context"
	"errors"
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
)

const (
	affiliateDailyLastRunKey      = "affiliate_daily_last_run"
	affiliateLevelsLastQuarterKey = "affiliate_levels_last_quarter"
	affiliateDayKeyFormat         = "2006-01-02"

	affiliateCommissionBatchLimit = 100
	affiliatePhysicalBatchLimit   = 100

	affiliateScopeBindings    = "bindings"
	affiliateScopeCommissions = "commissions"
	affiliateScopeLevels      = "levels"

	affiliateLevelSourceAuto     = "auto"
	affiliateLevelReasonAutoEval = "auto_eval"
	affiliateLevelReasonAutoWarn = "auto_warn"
	affiliateLevelMissQuarters   = 2 // BRD: drop one level after 2 missed quarters

	affiliateLevelActionRise = "rise"
	affiliateLevelActionHold = "hold"
	affiliateLevelActionWarn = "warn"
	affiliateLevelActionDrop = "drop"
	affiliateLevelActionNone = "none"
)

var affiliateJobScopes = map[string]bool{
	affiliateScopeBindings:    true,
	affiliateScopeCommissions: true,
	affiliateScopeLevels:      true,
}

var affiliateQuarterPattern = regexp.MustCompile(`^(\d{4})-Q([1-4])$`)

// affiliateLevelPlan is the quarterly decision for one affiliator: rise to
// toLevel, hold (reset the warning counter), warn (increment), drop to
// toLevel, or none (bronze floor).
type affiliateLevelPlan struct {
	action  string
	toLevel string
}

// AffiliateJobResult summarizes one rerun invocation for the admin endpoint.
type AffiliateJobResult struct {
	Scope               string `json:"scope"`
	Quarter             string `json:"quarter,omitempty"`
	BindingsExpired     int    `json:"bindings_expired"`
	PhysicalBackfilled  int    `json:"physical_backfilled"`
	CommissionsReleased int    `json:"commissions_released"`
	OrdersReconciled    int    `json:"orders_reconciled"`
	LevelsEvaluated     int    `json:"levels_evaluated"`
	LevelsChanged       int    `json:"levels_changed"`
}

var (
	ErrAffiliateJobUnknownScope   = errors.New("unknown affiliate job scope")
	ErrAffiliateJobInvalidQuarter = errors.New("affiliate job quarter must look like 2026-Q3")
)

// RunAffiliateDaily is the worker's daily affiliate pass: the batch runs once
// per WIB calendar day (day-key in system_config) and the quarterly level
// evaluation rides along on its own quarter guard, so it also retries on its
// own when the batch day has already been claimed.
func (s *Service) RunAffiliateDaily(ctx context.Context, now time.Time) error {
	cfg, err := s.affiliateConfigMap(ctx)
	if err != nil {
		return fmt.Errorf("load affiliate job config: %w", err)
	}
	if !affiliateEnabled(cfg) {
		return nil
	}
	if err := s.runAffiliateDailyBatch(ctx, now, cfg); err != nil {
		return err
	}
	_, err = s.runAffiliateQuarterly(ctx, now, nil)
	return err
}

// RunAffiliateJob backs the admin rerun endpoint: the same work items as the
// daily loop, bypassing the day-key/quarter guards when a level quarter is
// given explicitly. Every step is idempotent, so reruns change nothing twice.
func (s *Service) RunAffiliateJob(ctx context.Context, scope string, quarter *string) (AffiliateJobResult, error) {
	res := AffiliateJobResult{Scope: scope}
	if !affiliateJobScopes[scope] {
		return res, ErrAffiliateJobUnknownScope
	}
	if !s.affiliateEnabled(ctx) {
		return res, nil
	}
	now := time.Now()
	switch scope {
	case affiliateScopeBindings:
		expired, err := s.ExpireBindings(ctx, now)
		if err != nil {
			return res, err
		}
		res.BindingsExpired = expired
	case affiliateScopeCommissions:
		reconciled, err := s.reconcileMissingCommissions(ctx, now)
		if err != nil {
			return res, err
		}
		res.OrdersReconciled = reconciled
		backfilled, err := s.backfillPhysicalWaitUntil(ctx)
		if err != nil {
			return res, err
		}
		res.PhysicalBackfilled = backfilled
		released, err := s.releaseDueCommissions(ctx, now)
		if err != nil {
			return res, err
		}
		res.CommissionsReleased = released
	case affiliateScopeLevels:
		return s.runAffiliateQuarterly(ctx, now, quarter)
	}
	return res, nil
}

func (s *Service) runAffiliateDailyBatch(ctx context.Context, now time.Time, cfg map[string]string) error {
	dayKey := affiliateDayKey(now)
	if cfg[affiliateDailyLastRunKey] == dayKey {
		return nil
	}
	// Backfill before release so a wait that becomes due immediately (order
	// completed long ago) still transitions in this same pass.
	if _, err := s.ExpireBindings(ctx, now); err != nil {
		return fmt.Errorf("expire affiliate bindings: %w", err)
	}
	if _, err := s.reconcileMissingCommissions(ctx, now); err != nil {
		return err
	}
	if _, err := s.backfillPhysicalWaitUntil(ctx); err != nil {
		return err
	}
	if _, err := s.releaseDueCommissions(ctx, now); err != nil {
		return err
	}
	return s.storeRepo.UpsertSystemConfig(ctx, affiliateDailyLastRunKey, dayKey, false)
}

// releaseDueCommissions transitions every pending commission whose wait_until
// has passed to available, one transaction per commission with the ledger
// entry, so a replayed job never double-credits.
func (s *Service) releaseDueCommissions(ctx context.Context, now time.Time) (int, error) {
	released := 0
	for {
		due, err := s.storeRepo.GetDuePendingCommissions(ctx, now, affiliateCommissionBatchLimit)
		if err != nil {
			return released, fmt.Errorf("get due pending commissions: %w", err)
		}
		if len(due) == 0 {
			return released, nil
		}
		for _, c := range due {
			done, err := s.releaseAffiliateCommission(ctx, c)
			if err != nil {
				return released, fmt.Errorf("release commission %s: %w", c.Commission.ID, err)
			}
			if done {
				released++
			}
		}
		if len(due) < affiliateCommissionBatchLimit {
			return released, nil
		}
	}
}

func (s *Service) releaseAffiliateCommission(ctx context.Context, c repository.AffiliateDueCommission) (bool, error) {
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	note := fmt.Sprintf("commission %s order %s", c.ItemName, c.Commission.OrderID)
	done, err := s.storeRepo.ReleaseAffiliateCommissionTx(ctx, tx, c.Commission.ID, note)
	if err != nil {
		return false, err
	}
	if !done {
		return false, nil
	}
	return true, tx.Commit(ctx)
}

const (
	affiliateReconcileWindowDays = 14
	affiliateReconcileMinAge     = time.Hour
	affiliateReconcileBatchLimit = 1000
)

// reconcileMissingCommissions recreates commissions the OrderPaid worker step
// failed to write (it logs and moves on so access is never blocked). Creation is
// idempotent per order item, so orders that legitimately earn nothing (0%, full
// discount) are re-examined harmlessly until they age out of the window. It
// returns how many orders gained at least one commission.
func (s *Service) reconcileMissingCommissions(ctx context.Context, now time.Time) (int, error) {
	ids, err := s.storeRepo.ListPaidOrdersMissingAffiliateCommissions(ctx,
		now.AddDate(0, 0, -affiliateReconcileWindowDays), now.Add(-affiliateReconcileMinAge), affiliateReconcileBatchLimit)
	if err != nil {
		return 0, fmt.Errorf("list orders missing affiliate commissions: %w", err)
	}
	reconciled := 0
	for _, id := range ids {
		created, err := s.reconcileOrderCommissions(ctx, id)
		if err != nil {
			return reconciled, fmt.Errorf("reconcile commissions for order %s: %w", id, err)
		}
		if created {
			reconciled++
		}
	}
	return reconciled, nil
}

func (s *Service) reconcileOrderCommissions(ctx context.Context, orderID uuid.UUID) (bool, error) {
	order, err := s.storeRepo.GetOrderByID(ctx, orderID)
	if err != nil {
		return false, err
	}
	if order.ID == uuid.Nil {
		return false, nil
	}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return false, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)

	before, err := s.storeRepo.GetCommissionsByOrderTx(ctx, tx, orderID)
	if err != nil {
		return false, err
	}
	if err := s.CreateCommissionsForOrder(ctx, tx, order, order.Items); err != nil {
		return false, err
	}
	after, err := s.storeRepo.GetCommissionsByOrderTx(ctx, tx, orderID)
	if err != nil {
		return false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return len(after) > len(before), nil
}

// backfillPhysicalWaitUntil derives wait_until for pending physical
// commissions whose order has meanwhile been completed:
// completed_at + physical wait days, per the level's effective rule.
func (s *Service) backfillPhysicalWaitUntil(ctx context.Context) (int, error) {
	_, rules, err := s.LoadAffiliateEffectiveRules(ctx)
	if err != nil {
		return 0, fmt.Errorf("load affiliate effective rules: %w", err)
	}
	backfilled := 0
	for {
		batch, err := s.storeRepo.GetPendingPhysicalCommissions(ctx, affiliatePhysicalBatchLimit)
		if err != nil {
			return backfilled, fmt.Errorf("get pending physical commissions: %w", err)
		}
		if len(batch) == 0 {
			return backfilled, nil
		}
		for _, p := range batch {
			rule, ok := rules[p.LevelSnapshot]
			if !ok {
				// An unknown or empty snapshot falls back to the global
				// defaults (bronze rule) so one corrupt row cannot block the
				// whole daily job.
				rule = rules["bronze"]
			}
			wait := p.CompletedAt.AddDate(0, 0, rule.PhysicalWaitDays)
			if err := s.storeRepo.SetAffiliateCommissionWaitUntil(ctx, p.CommissionID, wait); err != nil {
				return backfilled, fmt.Errorf("set commission wait until %s: %w", p.CommissionID, err)
			}
			backfilled++
		}
		if len(batch) < affiliatePhysicalBatchLimit {
			return backfilled, nil
		}
	}
}

// runAffiliateQuarterly evaluates the just-ended WIB quarter once it reaches
// the settings' eval day; an explicit quarter (admin rerun) bypasses both
// guards and does not move the progress key.
func (s *Service) runAffiliateQuarterly(ctx context.Context, now time.Time, forcedQuarter *string) (AffiliateJobResult, error) {
	res := AffiliateJobResult{Scope: affiliateScopeLevels}
	settings, rules, err := s.LoadAffiliateEffectiveRules(ctx)
	if err != nil {
		return res, fmt.Errorf("load affiliate effective rules: %w", err)
	}

	var quarter string
	if forcedQuarter != nil {
		quarter, err = affiliateParseQuarter(*forcedQuarter)
		if err != nil {
			return res, err
		}
	} else {
		dueQuarter, due, err := affiliateDueQuarter(now, settings.EvalDay)
		if err != nil {
			return res, err
		}
		if !due {
			return res, nil
		}
		cfg, err := s.affiliateConfigMap(ctx)
		if err != nil {
			return res, fmt.Errorf("load affiliate job config: %w", err)
		}
		if cfg[affiliateLevelsLastQuarterKey] == dueQuarter {
			return res, nil
		}
		quarter = dueQuarter
	}

	evaluated, changed, err := s.evaluateAffiliateLevels(ctx, quarter, rules)
	if err != nil {
		return res, err
	}
	res.Quarter = quarter
	res.LevelsEvaluated = evaluated
	res.LevelsChanged = changed
	if forcedQuarter == nil {
		if err := s.storeRepo.UpsertSystemConfig(ctx, affiliateLevelsLastQuarterKey, quarter, false); err != nil {
			return res, fmt.Errorf("mark affiliate level quarter: %w", err)
		}
	}
	return res, nil
}

// evaluateAffiliateLevels applies BR-LVL-02..07 to every eligible affiliator:
// active or suspended, and not registered inside the evaluated quarter or
// later. last_evaluated_quarter is the per-affiliator rerun guard, so changes
// and warnings never repeat.
func (s *Service) evaluateAffiliateLevels(ctx context.Context, quarter string, rules map[string]AffiliateEffectiveRule) (int, int, error) {
	start, end, err := affiliateQuarterRange(quarter, affiliateWIBLocation())
	if err != nil {
		return 0, 0, err
	}
	affiliators, err := s.storeRepo.ListAffiliatorsForLevelEval(ctx)
	if err != nil {
		return 0, 0, fmt.Errorf("list affiliators for level eval: %w", err)
	}

	evaluated, changed := 0, 0
	for i := range affiliators {
		aff := &affiliators[i]
		if !aff.CreatedAt.Before(start) {
			continue
		}
		if aff.LastEvaluatedQuarter != nil && *aff.LastEvaluatedQuarter >= quarter {
			continue
		}
		net, err := s.storeRepo.GetAffiliateQuarterNetRevenue(ctx, aff.ID, start, end)
		if err != nil {
			return evaluated, changed, fmt.Errorf("sum quarter net revenue: %w", err)
		}
		plan := affiliateLevelPlanFor(aff.Level, aff.WarnQuarters, net, rules)
		if err := s.applyAffiliateLevelPlan(ctx, aff, quarter, plan); err != nil {
			return evaluated, changed, fmt.Errorf("apply level plan for %s: %w", aff.ID, err)
		}
		evaluated++
		if plan.action == affiliateLevelActionRise || plan.action == affiliateLevelActionDrop {
			changed++
		}
	}
	return evaluated, changed, nil
}

func (s *Service) applyAffiliateLevelPlan(ctx context.Context, aff *model.Affiliator, quarter string, plan affiliateLevelPlan) error {
	missed := 0
	switch plan.action {
	case affiliateLevelActionWarn:
		missed = aff.WarnQuarters + 1
	case affiliateLevelActionNone: // bronze has no level below to drop to
		missed = aff.WarnQuarters
	}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.SettleAffiliateLevelEvaluationTx(ctx, tx, aff.ID, plan.toLevel, missed, quarter); err != nil {
		return fmt.Errorf("settle affiliate level: %w", err)
	}
	return tx.Commit(ctx)
}

func (s *Service) affiliateConfigMap(ctx context.Context) (map[string]string, error) {
	rows, err := s.storeRepo.ListSystemConfig(ctx)
	if err != nil {
		return nil, err
	}
	cfg := make(map[string]string, len(rows))
	for _, row := range rows {
		cfg[row.Key] = row.Value
	}
	return cfg, nil
}

func affiliateParseQuarter(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if !affiliateQuarterPattern.MatchString(key) {
		return "", ErrAffiliateJobInvalidQuarter
	}
	return key, nil
}

func affiliateDayKey(now time.Time) string {
	return now.In(affiliateWIBLocation()).Format(affiliateDayKeyFormat)
}

func affiliateQuarterKey(nowWIB time.Time) string {
	return fmt.Sprintf("%d-Q%d", nowWIB.Year(), (int(nowWIB.Month())-1)/3+1)
}

func affiliatePrevQuarterKey(key string) (string, error) {
	m := affiliateQuarterPattern.FindStringSubmatch(strings.TrimSpace(key))
	if m == nil {
		return "", ErrAffiliateJobInvalidQuarter
	}
	year, _ := strconv.Atoi(m[1])
	quarter, _ := strconv.Atoi(m[2])
	if quarter == 1 {
		year--
		quarter = 4
	} else {
		quarter--
	}
	return fmt.Sprintf("%d-Q%d", year, quarter), nil
}

// affiliateQuarterRange bounds one quarter in WIB: start inclusive, end
// exclusive, both absolute times for paid_at comparisons.
func affiliateQuarterRange(key string, loc *time.Location) (time.Time, time.Time, error) {
	m := affiliateQuarterPattern.FindStringSubmatch(strings.TrimSpace(key))
	if m == nil {
		return time.Time{}, time.Time{}, ErrAffiliateJobInvalidQuarter
	}
	year, _ := strconv.Atoi(m[1])
	quarter, _ := strconv.Atoi(m[2])
	start := time.Date(year, time.Month((quarter-1)*3+1), 1, 0, 0, 0, 0, loc)
	return start, start.AddDate(0, 3, 0), nil
}

// affiliateQuarterEvalDate is the eval_day-th day of the quarter's first
// month — the day the just-ended quarter gets evaluated.
func affiliateQuarterEvalDate(evalDay int, key string, loc *time.Location) (time.Time, error) {
	start, _, err := affiliateQuarterRange(key, loc)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(start.Year(), start.Month(), evalDay, 0, 0, 0, 0, loc), nil
}

// affiliateDueQuarter reports whether now has reached the current quarter's
// eval day in WIB, and returns the just-ended quarter to evaluate.
func affiliateDueQuarter(now time.Time, evalDay int) (string, bool, error) {
	nowWIB := now.In(affiliateWIBLocation())
	current := affiliateQuarterKey(nowWIB)
	evalDate, err := affiliateQuarterEvalDate(evalDay, current, nowWIB.Location())
	if err != nil {
		return "", false, err
	}
	today := time.Date(nowWIB.Year(), nowWIB.Month(), nowWIB.Day(), 0, 0, 0, 0, nowWIB.Location())
	if today.Before(evalDate) {
		return "", false, nil
	}
	quarter, err := affiliatePrevQuarterKey(current)
	if err != nil {
		return "", false, err
	}
	return quarter, true, nil
}

func affiliateNextLevel(level string) (string, bool) {
	for i, l := range affiliateLevels {
		if l == level && i+1 < len(affiliateLevels) {
			return affiliateLevels[i+1], true
		}
	}
	return "", false
}

func affiliatePrevLevel(level string) (string, bool) {
	for i, l := range affiliateLevels {
		if l == level && i > 0 {
			return affiliateLevels[i-1], true
		}
	}
	return "", false
}

// affiliateRiseThreshold is the revenue that rises INTO next: the level's
// rise_threshold, which falls back to the built-in default for silver and gold.
func affiliateRiseThreshold(next string, rules map[string]AffiliateEffectiveRule) float64 {
	if rule := rules[next]; rule.RiseThreshold != nil {
		return *rule.RiseThreshold
	}
	return math.MaxFloat64
}

// affiliateRiseTarget walks up level by level while each next level's rise
// threshold is met, so a bronze affiliator can jump straight to gold.
func affiliateRiseTarget(current string, net float64, rules map[string]AffiliateEffectiveRule) (string, bool) {
	target := ""
	level := current
	for {
		next, ok := affiliateNextLevel(level)
		if !ok || net < affiliateRiseThreshold(next, rules) {
			break
		}
		target = next
		level = next
	}
	return target, target != ""
}

// affiliateLevelPlanFor decides rise/hold/warn/drop for one affiliator:
// rise beats hold; a miss with no prior miss warns; a second consecutive miss
// drops one level; bronze cannot drop further.
func affiliateLevelPlanFor(current string, warnQuarters int, net float64, rules map[string]AffiliateEffectiveRule) affiliateLevelPlan {
	if to, ok := affiliateRiseTarget(current, net, rules); ok {
		return affiliateLevelPlan{action: affiliateLevelActionRise, toLevel: to}
	}
	hold := rules[current].HoldThreshold
	if hold == nil || net >= *hold {
		return affiliateLevelPlan{action: affiliateLevelActionHold, toLevel: current}
	}
	if warnQuarters+1 < affiliateLevelMissQuarters {
		return affiliateLevelPlan{action: affiliateLevelActionWarn, toLevel: current}
	}
	if to, ok := affiliatePrevLevel(current); ok {
		return affiliateLevelPlan{action: affiliateLevelActionDrop, toLevel: to}
	}
	return affiliateLevelPlan{action: affiliateLevelActionNone, toLevel: current}
}
