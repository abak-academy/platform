package service

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func validateTopic(t model.ExamTopic) error {
	if strings.TrimSpace(t.Name) == "" {
		return fmt.Errorf("%w: topic name required", ErrValidation)
	}
	if strings.TrimSpace(t.Subject) == "" {
		return fmt.Errorf("%w: topic subject required", ErrValidation)
	}
	return nil
}

// ListTopics returns all topics with a per-topic question count (FR-16).
func (s *Service) ListTopics(ctx context.Context, filter repository.TopicFilter) ([]model.ExamTopic, error) {
	return s.storeRepo.ListTopics(ctx, filter)
}

// CreateTopic creates a topic, rejecting blank name/subject and duplicate (subject,name) (FR-17).
func (s *Service) CreateTopic(ctx context.Context, t model.ExamTopic) (model.ExamTopic, error) {
	if err := validateTopic(t); err != nil {
		return model.ExamTopic{}, err
	}
	t.Name = strings.TrimSpace(t.Name)
	t.Subject = strings.TrimSpace(t.Subject)

	if err := s.storeRepo.CreateTopic(ctx, &t); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.ExamTopic{}, fmt.Errorf("%w: topic already exists for this subject", ErrValidation)
		}
		return model.ExamTopic{}, err
	}
	return t, nil
}

// UpdateTopic updates a topic's name and subject (FR-18).
func (s *Service) UpdateTopic(ctx context.Context, id uuid.UUID, t model.ExamTopic) (model.ExamTopic, error) {
	if err := validateTopic(t); err != nil {
		return model.ExamTopic{}, err
	}
	if _, err := s.storeRepo.GetTopicByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ExamTopic{}, ErrTopicNotFound
		}
		return model.ExamTopic{}, err
	}

	t.Name = strings.TrimSpace(t.Name)
	t.Subject = strings.TrimSpace(t.Subject)
	if err := s.storeRepo.UpdateTopic(ctx, id, &t); err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return model.ExamTopic{}, fmt.Errorf("%w: topic already exists for this subject", ErrValidation)
		}
		return model.ExamTopic{}, err
	}
	t.ID = id
	return t, nil
}

// DeleteTopic removes a topic if no question references it (FR-19).
func (s *Service) DeleteTopic(ctx context.Context, id uuid.UUID) error {
	if _, err := s.storeRepo.GetTopicByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrTopicNotFound
		}
		return err
	}

	count, err := s.storeRepo.CountQuestionsByTopic(ctx, id)
	if err != nil {
		return err
	}
	if count > 0 {
		return fmt.Errorf("%w: topic is referenced by %d question(s)", ErrValidation, count)
	}

	return s.storeRepo.DeleteTopic(ctx, id)
}
