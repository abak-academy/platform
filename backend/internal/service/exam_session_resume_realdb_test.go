package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

type resumeFixture struct {
	svc       *Service
	repo      *repository.Repository
	studentID uuid.UUID
	examID    uuid.UUID
	regID     uuid.UUID
	token     string
}

func seedResumeFixture(t *testing.T, requiresCheckin bool) resumeFixture {
	t.Helper()
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	var studentID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO users (name, username, jenjang, role, status, auth_provider)
		VALUES ($1, $2, 'sd', 'student', 'active', 'password')
		RETURNING id`,
		"Resume Student "+uniqueSuffix(), "res_"+uniqueSuffix(),
	).Scan(&studentID); err != nil {
		t.Fatalf("insert student: %v", err)
	}

	duration := 60
	maxAttempts := 1
	scheduledAt := time.Now().Add(-time.Hour)
	scheduledEnd := time.Now().Add(time.Hour)
	exam := &model.Exam{
		Title:           "Resume Exam " + uniqueSuffix(),
		RequiresCheckin: requiresCheckin,
		ScheduledAt:     &scheduledAt,
		ScheduledEndAt:  &scheduledEnd,
		DurationMinutes: &duration,
		MaxAttempts:     &maxAttempts,
		ResultConfig:    "hidden",
	}
	if err := repo.CreateExam(ctx, exam); err != nil {
		t.Fatalf("CreateExam: %v", err)
	}
	seedRealDBAttachedQuestion(t, repo, exam.ID)

	token := uuid.NewString()
	var regID uuid.UUID
	if err := repo.Pool().QueryRow(ctx,
		`INSERT INTO exam_registration (student_id, exam_id, token) VALUES ($1, $2, $3) RETURNING id`,
		studentID, exam.ID, token,
	).Scan(&regID); err != nil {
		t.Fatalf("insert exam_registration: %v", err)
	}
	return resumeFixture{svc: svc, repo: repo, studentID: studentID, examID: exam.ID, regID: regID, token: token}
}

func (f resumeFixture) closeWindow(t *testing.T) {
	t.Helper()
	if _, err := f.repo.Pool().Exec(context.Background(),
		`UPDATE exam SET scheduled_end_at = now() - interval '1 minute' WHERE id = $1`, f.examID); err != nil {
		t.Fatalf("close window: %v", err)
	}
}

func (f resumeFixture) deviceKey() string { return "exam:device:" + f.regID.String() }

func (f resumeFixture) start(device DeviceProof) (SessionStartPayload, error) {
	return f.svc.StartSession(context.Background(), f.studentID.String(), f.regID.String(), device)
}

func (f resumeFixture) checkIn(t *testing.T, device DeviceProof) {
	t.Helper()
	if _, err := f.svc.CheckIn(context.Background(), f.studentID.String(), f.token, device); err != nil {
		t.Fatalf("CheckIn: %v", err)
	}
}

func TestStartSession_RealDB_ResumeIgnoresClosedWindow(t *testing.T) {
	tests := []struct {
		name            string
		requiresCheckin bool
	}{
		{name: "no check-in exam", requiresCheckin: false},
		{name: "check-in exam", requiresCheckin: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := seedResumeFixture(t, tc.requiresCheckin)
			device := DeviceProof{ID: uuid.NewString(), LegacyFP: "fp"}
			if tc.requiresCheckin {
				f.checkIn(t, device)
			}
			first, err := f.start(device)
			if err != nil {
				t.Fatalf("first start: %v", err)
			}

			f.closeWindow(t)

			resumed, err := f.start(device)
			if err != nil {
				t.Fatalf("resume after window closed: %v", err)
			}
			if resumed.SessionID != first.SessionID {
				t.Errorf("resume: want session %s, got %s", first.SessionID, resumed.SessionID)
			}
		})
	}
}

func TestStartSession_RealDB_ResumeDeviceMatching(t *testing.T) {
	deviceA := uuid.NewString()
	deviceB := uuid.NewString()
	tests := []struct {
		name            string
		requiresCheckin bool
		resumeWith      DeviceProof
		wantErr         error
	}{
		{name: "same device id", requiresCheckin: true, resumeWith: DeviceProof{ID: deviceA, LegacyFP: "fp"}},
		{name: "same device id after ip change", requiresCheckin: true, resumeWith: DeviceProof{ID: deviceA, LegacyFP: "other-ip-fp"}},
		{name: "different device id", requiresCheckin: true, resumeWith: DeviceProof{ID: deviceB, LegacyFP: "fp"}, wantErr: ErrDeviceMismatch},
		{name: "no device id and different fingerprint", requiresCheckin: true, resumeWith: DeviceProof{LegacyFP: "other-ip-fp"}, wantErr: ErrDeviceMismatch},
		{name: "no check-in exam needs no device", requiresCheckin: false, resumeWith: DeviceProof{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := seedResumeFixture(t, tc.requiresCheckin)
			original := DeviceProof{ID: deviceA, LegacyFP: "fp"}
			if tc.requiresCheckin {
				f.checkIn(t, original)
			}
			first, err := f.start(original)
			if err != nil {
				t.Fatalf("first start: %v", err)
			}

			resumed, err := f.start(tc.resumeWith)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("resume: want err %v, got %v", tc.wantErr, err)
			}
			if tc.wantErr == nil && resumed.SessionID != first.SessionID {
				t.Errorf("resume: want session %s, got %s", first.SessionID, resumed.SessionID)
			}
		})
	}
}

func TestCheckIn_RealDB_StoresDeviceIDWithFallbackToFingerprint(t *testing.T) {
	tests := []struct {
		name   string
		device DeviceProof
		want   string
	}{
		{name: "device id preferred", device: DeviceProof{ID: "11111111-1111-4111-8111-111111111111", LegacyFP: "fp"}, want: "11111111-1111-4111-8111-111111111111"},
		{name: "legacy fingerprint when no id", device: DeviceProof{LegacyFP: "fp"}, want: "fp"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := seedResumeFixture(t, true)
			f.checkIn(t, tc.device)
			got, err := realDBRedis.Get(f.deviceKey())
			if err != nil || got != tc.want {
				t.Fatalf("stored device: want %q, got %q (err %v)", tc.want, got, err)
			}
		})
	}
}

func TestStartSession_RealDB_OldClientWithoutDeviceIDKeepsFingerprintKey(t *testing.T) {
	f := seedResumeFixture(t, true)
	old := DeviceProof{LegacyFP: "fp"}
	f.checkIn(t, old)
	first, err := f.start(old)
	if err != nil {
		t.Fatalf("first start: %v", err)
	}

	resumed, err := f.start(old)
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if resumed.SessionID != first.SessionID {
		t.Errorf("resume: want session %s, got %s", first.SessionID, resumed.SessionID)
	}
	if got, _ := realDBRedis.Get(f.deviceKey()); got != "fp" {
		t.Fatalf("key after old-client resume: want %q, got %q", "fp", got)
	}
}

func TestStartSession_RealDB_ResumeRejectsRevokedRegistration(t *testing.T) {
	f := seedResumeFixture(t, false)
	if _, err := f.start(DeviceProof{}); err != nil {
		t.Fatalf("first start: %v", err)
	}
	if _, err := f.repo.Pool().Exec(context.Background(),
		`UPDATE exam_registration SET status = 'revoked', revoked_at = now() WHERE id = $1`, f.regID); err != nil {
		t.Fatalf("revoke registration: %v", err)
	}

	if _, err := f.start(DeviceProof{}); !errors.Is(err, ErrRegistrationRevoked) {
		t.Fatalf("resume after revoke: want ErrRegistrationRevoked, got %v", err)
	}
}

func TestStartSession_RealDB_LegacyFingerprintKeyMigratesToDeviceID(t *testing.T) {
	f := seedResumeFixture(t, true)
	f.checkIn(t, DeviceProof{LegacyFP: "legacy-fp"})
	deviceID := uuid.NewString()

	if _, err := f.start(DeviceProof{ID: deviceID, LegacyFP: "legacy-fp"}); err != nil {
		t.Fatalf("start with legacy key: %v", err)
	}
	if got, _ := realDBRedis.Get(f.deviceKey()); got != deviceID {
		t.Fatalf("key after migration: want %q, got %q", deviceID, got)
	}

	if _, err := f.start(DeviceProof{ID: deviceID, LegacyFP: "changed-ip-fp"}); err != nil {
		t.Fatalf("resume after migration with new ip: %v", err)
	}
	if _, err := f.start(DeviceProof{ID: uuid.NewString(), LegacyFP: "legacy-fp"}); !errors.Is(err, ErrDeviceMismatch) {
		t.Fatalf("other device with old fingerprint after migration: want ErrDeviceMismatch, got %v", err)
	}
}

func TestDeviceKeyTTL_RealDB(t *testing.T) {
	f := seedResumeFixture(t, true)
	device := DeviceProof{ID: uuid.NewString(), LegacyFP: "fp"}

	f.checkIn(t, device)
	if got := realDBRedis.TTL(f.deviceKey()); got != 24*time.Hour {
		t.Fatalf("TTL after check-in: want 24h, got %v", got)
	}

	realDBRedis.SetTTL(f.deviceKey(), time.Minute)
	if _, err := f.start(device); err != nil {
		t.Fatalf("start: %v", err)
	}
	if got := realDBRedis.TTL(f.deviceKey()); got != 24*time.Hour {
		t.Fatalf("TTL after start: want 24h, got %v", got)
	}

	realDBRedis.SetTTL(f.deviceKey(), time.Minute)
	if _, err := f.start(device); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := realDBRedis.TTL(f.deviceKey()); got != 24*time.Hour {
		t.Fatalf("TTL after resume: want 24h, got %v", got)
	}
}

func TestReconnectSession_RealDB_DeviceCheck(t *testing.T) {
	deviceA := uuid.NewString()
	tests := []struct {
		name            string
		requiresCheckin bool
		reconnectWith   DeviceProof
		wantErr         error
	}{
		{name: "check-in exam same device", requiresCheckin: true, reconnectWith: DeviceProof{ID: deviceA, LegacyFP: "new-ip-fp"}},
		{name: "check-in exam different device", requiresCheckin: true, reconnectWith: DeviceProof{ID: uuid.NewString(), LegacyFP: "fp"}, wantErr: ErrDeviceMismatch},
		{name: "check-in exam no proof", requiresCheckin: true, reconnectWith: DeviceProof{}, wantErr: ErrDeviceMismatch},
		{name: "no check-in exam unaffected", requiresCheckin: false, reconnectWith: DeviceProof{}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			f := seedResumeFixture(t, tc.requiresCheckin)
			original := DeviceProof{ID: deviceA, LegacyFP: "fp"}
			if tc.requiresCheckin {
				f.checkIn(t, original)
			}
			sess, err := f.start(original)
			if err != nil {
				t.Fatalf("start: %v", err)
			}

			_, err = f.svc.ReconnectSession(context.Background(), f.studentID.String(), sess.SessionID.String(), tc.reconnectWith)
			if !errors.Is(err, tc.wantErr) {
				t.Fatalf("reconnect: want err %v, got %v", tc.wantErr, err)
			}
		})
	}
}

func TestReconnectSession_RealDB_SubmittedSessionSkipsDeviceCheck(t *testing.T) {
	f := seedResumeFixture(t, true)
	original := DeviceProof{ID: uuid.NewString(), LegacyFP: "fp"}
	f.checkIn(t, original)
	sess, err := f.start(original)
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if _, err := f.repo.Pool().Exec(context.Background(),
		`UPDATE exam_session SET status = 'submitted', submitted_at = now() WHERE id = $1`, sess.SessionID); err != nil {
		t.Fatalf("mark submitted: %v", err)
	}

	other := DeviceProof{ID: uuid.NewString(), LegacyFP: "other-fp"}
	if _, err := f.svc.ReconnectSession(context.Background(), f.studentID.String(), sess.SessionID.String(), other); err != nil {
		t.Fatalf("reconnect submitted from other device: %v", err)
	}
}

func TestStartSession_RealDB_NewSessionRulesStillApply(t *testing.T) {
	t.Run("window closed", func(t *testing.T) {
		f := seedResumeFixture(t, false)
		f.closeWindow(t)
		if _, err := f.start(DeviceProof{}); !errors.Is(err, ErrExamWindowClosed) {
			t.Fatalf("want ErrExamWindowClosed, got %v", err)
		}
	})
	t.Run("not checked in", func(t *testing.T) {
		f := seedResumeFixture(t, true)
		if _, err := f.start(DeviceProof{ID: uuid.NewString(), LegacyFP: "fp"}); !errors.Is(err, ErrNotCheckedIn) {
			t.Fatalf("want ErrNotCheckedIn, got %v", err)
		}
	})
	t.Run("attempts exhausted", func(t *testing.T) {
		f := seedResumeFixture(t, false)
		first, err := f.start(DeviceProof{})
		if err != nil {
			t.Fatalf("first start: %v", err)
		}
		if _, err := f.svc.SubmitSession(context.Background(), f.studentID.String(), first.SessionID.String()); err != nil {
			t.Fatalf("SubmitSession: %v", err)
		}
		if _, err := f.start(DeviceProof{}); !errors.Is(err, ErrAlreadyAttempted) {
			t.Fatalf("want ErrAlreadyAttempted, got %v", err)
		}
	})
}
