package service

import (
	"testing"
	"time"

	"akademi-bimbel/internal/model"
)

func TestAffiliateQuarterKey_wibCalendarQuarters(t *testing.T) {
	wib := affiliateWIBLocation()
	tests := []struct {
		name string
		now  time.Time
		want string
	}{
		{"mid january", time.Date(2026, 1, 15, 3, 0, 0, 0, wib), "2026-Q1"},
		{"april", time.Date(2026, 4, 2, 10, 0, 0, 0, wib), "2026-Q2"},
		{"september", time.Date(2026, 9, 1, 0, 0, 0, 0, wib), "2026-Q3"},
		{"december 31 wib", time.Date(2026, 12, 31, 23, 0, 0, 0, wib), "2026-Q4"},
		{"utc dec 31 evening is jan 1 wib", time.Date(2026, 12, 31, 17, 0, 0, 0, time.UTC), "2027-Q1"},
		{"utc dec 31 16:59 is still dec 31 wib", time.Date(2026, 12, 31, 16, 59, 0, 0, time.UTC), "2026-Q4"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := affiliateQuarterKey(tt.now.In(wib)); got != tt.want {
				t.Errorf("affiliateQuarterKey = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestAffiliatePrevQuarterKey(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		want    string
		wantErr bool
	}{
		{"first quarter wraps year", "2026-Q1", "2025-Q4", false},
		{"mid year", "2026-Q4", "2026-Q3", false},
		{"third quarter", "2027-Q3", "2027-Q2", false},
		{"free text rejected", "nonsense", "", true},
		{"quarter five rejected", "2026-Q5", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := affiliatePrevQuarterKey(tt.key)
			if (err != nil) != tt.wantErr {
				t.Fatalf("affiliatePrevQuarterKey(%q) err = %v, wantErr %v", tt.key, err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("affiliatePrevQuarterKey(%q) = %q, want %q", tt.key, got, tt.want)
			}
		})
	}
}

func TestAffiliateQuarterRange_wibBounds(t *testing.T) {
	start, end, err := affiliateQuarterRange("2026-Q3", affiliateWIBLocation())
	if err != nil {
		t.Fatalf("affiliateQuarterRange: %v", err)
	}
	if want := time.Date(2026, 6, 30, 17, 0, 0, 0, time.UTC); !start.Equal(want) {
		t.Errorf("start = %v, want %v (Jul 1 00:00 WIB)", start, want)
	}
	if want := time.Date(2026, 9, 30, 17, 0, 0, 0, time.UTC); !end.Equal(want) {
		t.Errorf("end = %v, want %v (Oct 1 00:00 WIB)", end, want)
	}
	if _, _, err := affiliateQuarterRange("bad", affiliateWIBLocation()); err == nil {
		t.Error("affiliateQuarterRange(bad) err = nil, want error")
	}
}

func TestAffiliateDueQuarter_evalDayGuard(t *testing.T) {
	wib := affiliateWIBLocation()
	tests := []struct {
		name    string
		now     time.Time
		evalDay int
		wantQ   string
		wantDue bool
	}{
		{"day before eval day", time.Date(2026, 10, 5, 10, 0, 0, 0, wib), 8, "", false},
		{"eval day", time.Date(2026, 10, 8, 0, 30, 0, 0, wib), 8, "2026-Q3", true},
		{"late in quarter", time.Date(2026, 12, 20, 9, 0, 0, 0, wib), 8, "2026-Q3", true},
		{"early next year", time.Date(2027, 1, 2, 9, 0, 0, 0, wib), 8, "", false},
		{"eval day one", time.Date(2026, 10, 1, 0, 30, 0, 0, wib), 1, "2026-Q3", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotQ, gotDue, err := affiliateDueQuarter(tt.now, tt.evalDay)
			if err != nil {
				t.Fatalf("affiliateDueQuarter: %v", err)
			}
			if gotDue != tt.wantDue {
				t.Errorf("due = %v, want %v", gotDue, tt.wantDue)
			}
			if gotQ != tt.wantQ {
				t.Errorf("quarter = %q, want %q", gotQ, tt.wantQ)
			}
		})
	}
}

func TestAffiliateDayKey_wibCalendarDay(t *testing.T) {
	// 2026-10-25 16:59 UTC is still Oct 25 in WIB; 17:00 UTC crosses to Oct 26.
	if got := affiliateDayKey(time.Date(2026, 10, 25, 16, 59, 0, 0, time.UTC)); got != "2026-10-25" {
		t.Errorf("affiliateDayKey = %q, want 2026-10-25", got)
	}
	if got := affiliateDayKey(time.Date(2026, 10, 25, 17, 0, 0, 0, time.UTC)); got != "2026-10-26" {
		t.Errorf("affiliateDayKey = %q, want 2026-10-26", got)
	}
}

func TestAffiliateLevelPlanFor_matrix(t *testing.T) {
	settings := defaultAffiliateSettings()
	// Admin-tuned thresholds: silver rises at 5M and holds at 3M, gold at 15M/9M.
	tuned := buildAffiliateEffectiveRules(settings, []model.AffiliateLevelSettings{
		{Level: "silver", RiseThreshold: f64Ptr(5000000), HoldThreshold: f64Ptr(3000000)},
		{Level: "gold", RiseThreshold: f64Ptr(15000000), HoldThreshold: f64Ptr(9000000)},
	})
	// Default install: no level rows, so hold thresholds are NULL everywhere.
	defaults := buildAffiliateEffectiveRules(settings, nil)
	// Admin can give bronze a hold threshold; only then can bronze warn/drop.
	bronzeHold := buildAffiliateEffectiveRules(settings, []model.AffiliateLevelSettings{
		{Level: "bronze", HoldThreshold: f64Ptr(3000000)},
	})

	tests := []struct {
		name         string
		rules        map[string]AffiliateEffectiveRule
		current      string
		warnQuarters int
		net          float64
		wantAction   string
		wantTo       string
	}{
		{"bronze jumps to gold when both rises met", tuned, "bronze", 0, 15000000, "rise", "gold"},
		{"bronze rises to silver", tuned, "bronze", 0, 5000000, "rise", "silver"},
		{"bronze below silver rise holds", tuned, "bronze", 0, 4999999, "hold", "bronze"},
		{"silver rises to gold", tuned, "silver", 0, 15000000, "rise", "gold"},
		{"silver at hold threshold holds", tuned, "silver", 0, 3000000, "hold", "silver"},
		{"silver below hold warns", tuned, "silver", 0, 2999999, "warn", "silver"},
		{"silver second miss drops", tuned, "silver", 1, 2999999, "drop", "bronze"},
		{"bronze with hold floor second miss is none", bronzeHold, "bronze", 1, 0, "none", "bronze"},
		{"gold at hold holds", tuned, "gold", 0, 9000000, "hold", "gold"},
		{"gold miss drops to silver", tuned, "gold", 1, 8999999, "drop", "silver"},
		{"gold top never rises", tuned, "gold", 0, 99999999, "hold", "gold"},
		{"default silver holds at any net", defaults, "silver", 0, 0, "hold", "silver"},
		{"rise threshold override gates the rise", buildAffiliateEffectiveRules(settings, []model.AffiliateLevelSettings{
			{Level: "silver", RiseThreshold: f64Ptr(20000000)},
		}), "bronze", 0, 15000000, "hold", "bronze"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			plan := affiliateLevelPlanFor(tt.current, tt.warnQuarters, tt.net, tt.rules, settings)
			if plan.action != tt.wantAction {
				t.Errorf("action = %q, want %q", plan.action, tt.wantAction)
			}
			if plan.toLevel != tt.wantTo {
				t.Errorf("toLevel = %q, want %q", plan.toLevel, tt.wantTo)
			}
		})
	}
}
