package service

import (
	"context"
	"testing"
)

func TestStudentRegistrationDetail_onlyDetailPathReadsSections(t *testing.T) {
	f := seedResumeFixture(t, false)
	ctx := context.Background()
	regID, studentID := f.regID.String(), f.studentID.String()

	shared, err := f.svc.GetExamRegistration(ctx, regID, studentID)
	if err != nil {
		t.Fatalf("GetExamRegistration: %v", err)
	}
	if shared.Exam.Sections != nil || shared.Exam.SectionCount != 0 || shared.Exam.SectionsDurationMinutes != nil {
		t.Errorf("GetExamRegistration (card path) carries section data: %+v", shared.Exam)
	}

	detail, err := f.svc.GetStudentRegistrationDetail(ctx, regID, studentID)
	if err != nil {
		t.Fatalf("GetStudentRegistrationDetail: %v", err)
	}
	if detail.Exam.SectionCount != 1 || len(detail.Exam.Sections) != 1 || detail.Exam.NavigationPolicy == "" || detail.Exam.TimerMode == "" {
		t.Errorf("detail exam = %+v, want 1 section and effective rules", detail.Exam)
	}
	if detail.Exam.MaxAttempts == nil || *detail.Exam.MaxAttempts != 1 {
		t.Errorf("detail MaxAttempts = %v, want 1", detail.Exam.MaxAttempts)
	}
}
