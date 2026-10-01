package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func retryKey() string { return "retry-key-" + uniqueSuffix() }

func TestRetryPayment_RefusesWhenTheTargetBecameUnavailable(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name      string
		arrange   func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID
		assertErr func(t *testing.T, err error)
	}{
		{
			name: "exam the student was granted meanwhile",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				exam := seedExamTarget(t, repo)
				product := seedDigitalProduct(t, repo, "exam", 50000, exam)
				orderID := pendingCourseOrderFor(t, svc, student, product)
				registerForExam(t, repo, student, exam, "registered")
				return orderID
			},
			assertErr: func(t *testing.T, err error) { wantConflict(t, err, model.DigitalConflictOwned) },
		},
		{
			name: "order whose target another live order holds",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				course := seedCourseTarget(t, repo)
				holder := seedDigitalProduct(t, repo, "course", 50000, course)
				sameCourse := seedDigitalProduct(t, repo, "course", 40000, course)
				pendingCourseOrderFor(t, svc, student, holder)

				loser := cartOf(t, svc, student)
				forceLine(t, repo, loser, sameCourse, "course", 40000)
				if _, err := repo.Pool().Exec(ctx,
					`UPDATE orders SET status = 'payment_pending', total = 40000 WHERE id = $1`, loser,
				); err != nil {
					t.Fatalf("make second pending order: %v", err)
				}
				return loser
			},
			assertErr: func(t *testing.T, err error) { wantConflict(t, err, model.DigitalConflictClaimed) },
		},
		{
			name: "order with a line that no longer links anything",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				courseA, courseB := seedCourseTarget(t, repo), seedCourseTarget(t, repo)
				productA := seedDigitalProduct(t, repo, "course", 50000, courseA)
				productB := seedDigitalProduct(t, repo, "course", 40000, courseB)
				pending := pendingCourseOrderFor(t, svc, student, productA, productB)
				if _, err := repo.Pool().Exec(ctx, `DELETE FROM product_course WHERE product_id = $1`, productB); err != nil {
					t.Fatalf("unlink product: %v", err)
				}
				return pending
			},
			assertErr: func(t *testing.T, err error) {
				if !errors.Is(err, ErrDigitalProductNoTargets) {
					t.Fatalf("err = %v, want ErrDigitalProductNoTargets", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			orderID := tt.arrange(t, svc, repo, student)
			before := pay.creates.Load()

			_, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey())

			tt.assertErr(t, err)
			if got := pay.creates.Load() - before; got != 0 {
				t.Errorf("gateway calls = %d, want 0", got)
			}
		})
	}
}

func TestRetryPayment_ProceedsWhenTheRecheckHasNothingToRefuse(t *testing.T) {
	ctx := context.Background()
	tests := []struct {
		name    string
		arrange func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID
	}{
		{
			name: "bulk order the buyer already owns the exam of",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				participant := newDigitalStudent(t, repo)
				exam := seedExamTarget(t, repo)
				product := seedDigitalProduct(t, repo, "exam", 50000, exam)
				orderID := pendingCourseOrderFor(t, svc, student, product)
				if _, err := repo.Pool().Exec(ctx, `INSERT INTO order_participant (order_id, student_id) VALUES ($1, $2)`, orderID, participant); err != nil {
					t.Fatalf("seed participant: %v", err)
				}
				registerForExam(t, repo, student, exam, "registered")
				return orderID
			},
		},
		{
			name: "order whose only live claim on its targets is itself",
			arrange: func(t *testing.T, svc *Service, repo *repository.Repository, student uuid.UUID) uuid.UUID {
				productA := seedDigitalProduct(t, repo, "course", 50000, seedCourseTarget(t, repo))
				productB := seedDigitalProduct(t, repo, "course", 40000, seedCourseTarget(t, repo))
				return pendingCourseOrderFor(t, svc, student, productA, productB)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc, repo, pay := newLifecycleService(t)
			student := newDigitalStudent(t, repo)
			orderID := tt.arrange(t, svc, repo, student)
			before := pay.creates.Load()

			if _, err := svc.RetryPayment(ctx, student.String(), orderID.String(), retryKey()); err != nil {
				t.Fatalf("RetryPayment: %v", err)
			}
			if got := pay.creates.Load() - before; got != 1 {
				t.Errorf("gateway calls = %d, want 1", got)
			}
		})
	}
}

func pendingCourseOrderFor(t *testing.T, svc *Service, student uuid.UUID, products ...uuid.UUID) uuid.UUID {
	t.Helper()
	orderID := cartOf(t, svc, student, products...)
	if _, err := svc.Checkout(context.Background(), student.String(), orderID.String(), checkoutKey()); err != nil {
		t.Fatalf("Checkout: %v", err)
	}
	return orderID
}
