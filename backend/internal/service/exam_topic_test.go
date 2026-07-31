package service

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func TestListTopics_counts_questions_and_filters_by_subject(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	suffix := uniqueSuffix()
	mathSubject := "math-" + suffix
	bioSubject := "biology-" + suffix
	mathTopic := seedTopicDirect(t, ctx, repo, "Algebra "+suffix, mathSubject)
	bioTopic := seedTopicDirect(t, ctx, repo, "Genetics "+suffix, bioSubject)

	// One math question, two biology questions.
	seedBankQuestionWithTopicDirect(t, ctx, repo, "short", "math q", mathTopic)
	seedBankQuestionWithTopicDirect(t, ctx, repo, "short", "bio q1", bioTopic)
	seedBankQuestionWithTopicDirect(t, ctx, repo, "short", "bio q2", bioTopic)

	all, err := svc.ListTopics(ctx, repository.TopicFilter{})
	require.NoError(t, err)

	byID := map[uuid.UUID]model.ExamTopic{}
	for _, topic := range all {
		byID[topic.ID] = topic
	}
	require.Contains(t, byID, mathTopic)
	require.Contains(t, byID, bioTopic)
	assert.Equal(t, 1, byID[mathTopic].QuestionCount)
	assert.Equal(t, 2, byID[bioTopic].QuestionCount)

	mathOnly, err := svc.ListTopics(ctx, repository.TopicFilter{Subject: mathSubject})
	require.NoError(t, err)
	require.Len(t, mathOnly, 1)
	assert.Equal(t, mathTopic, mathOnly[0].ID)
	assert.Equal(t, 1, mathOnly[0].QuestionCount)
}

func TestCreateTopic_rejects_blank_name_or_subject(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	_, err := svc.CreateTopic(ctx, model.ExamTopic{Name: "", Subject: "math"})
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "topic name required")

	_, err = svc.CreateTopic(ctx, model.ExamTopic{Name: "algebra", Subject: "   "})
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "topic subject required")
}

func TestCreateTopic_rejects_duplicate_subject_name(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	name := "Duplicate Topic " + uniqueSuffix()
	_, err := svc.CreateTopic(ctx, model.ExamTopic{Name: name, Subject: "math"})
	require.NoError(t, err)

	_, err = svc.CreateTopic(ctx, model.ExamTopic{Name: name, Subject: "math"})
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "topic already exists")
}

func TestUpdateTopic_updates_name_and_subject(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	created, err := svc.CreateTopic(ctx, model.ExamTopic{Name: "Old Name " + uniqueSuffix(), Subject: "math"})
	require.NoError(t, err)

	updated, err := svc.UpdateTopic(ctx, created.ID, model.ExamTopic{Name: "New Name " + uniqueSuffix(), Subject: "biology"})
	require.NoError(t, err)
	assert.Equal(t, created.ID, updated.ID)
	assert.NotEqual(t, created.Name, updated.Name)
	assert.Equal(t, "biology", updated.Subject)
}

func TestUpdateTopic_returns_ErrTopicNotFound_for_missing_id(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	_, err := svc.UpdateTopic(ctx, uuid.New(), model.ExamTopic{Name: "x", Subject: "y"})
	assert.ErrorIs(t, err, ErrTopicNotFound)
}

func TestUpdateTopic_rejects_duplicate_subject_name(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	name := "Shared " + uniqueSuffix()
	a, err := svc.CreateTopic(ctx, model.ExamTopic{Name: name, Subject: "math"})
	require.NoError(t, err)
	_, err = svc.CreateTopic(ctx, model.ExamTopic{Name: name, Subject: "biology"})
	require.NoError(t, err)

	_, err = svc.UpdateTopic(ctx, a.ID, model.ExamTopic{Name: name, Subject: "biology"})
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "topic already exists")
}

func TestDeleteTopic_rejects_when_referenced_by_question(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	topicID := seedTopicDirect(t, ctx, repo, "Guarded "+uniqueSuffix(), "math")
	seedBankQuestionWithTopicDirect(t, ctx, repo, "essay", "explain", topicID)

	err := svc.DeleteTopic(ctx, topicID)
	assert.ErrorIs(t, err, ErrValidation)
	assert.Contains(t, err.Error(), "referenced")

	// Guard must leave the row intact.
	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM exam_topic WHERE id = $1)`, topicID).Scan(&exists))
	assert.True(t, exists)
}

func TestDeleteTopic_succeeds_when_unreferenced(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	topicID := seedTopicDirect(t, ctx, repo, "Orphan "+uniqueSuffix(), "math")

	err := svc.DeleteTopic(ctx, topicID)
	require.NoError(t, err)

	var exists bool
	require.NoError(t, repo.Pool().QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM exam_topic WHERE id = $1)`, topicID).Scan(&exists))
	assert.False(t, exists)
}

func TestDeleteTopic_returns_ErrTopicNotFound_for_missing_id(t *testing.T) {
	svc, _ := newRealDBService(t)
	ctx := context.Background()

	err := svc.DeleteTopic(ctx, uuid.New())
	assert.ErrorIs(t, err, ErrTopicNotFound)
}

func TestCreateBankQuestion_roundtrips_topic_id(t *testing.T) {
	svc, repo := newRealDBService(t)
	ctx := context.Background()

	topicID := seedTopicDirect(t, ctx, repo, "Roundtrip "+uniqueSuffix(), "math")

	q := model.Question{
		Format:       "essay",
		Body:         "explain gravity",
		TopicID:      &topicID,
		PointCorrect: 1,
		PointWrong:   0,
	}
	out, err := svc.CreateBankQuestion(ctx, q, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, out.Question.TopicID)
	assert.Equal(t, topicID, *out.Question.TopicID)

	// Read shape returns topic name.
	items, _, err := svc.ListBankQuestions(ctx, repository.QuestionFilter{TopicID: topicID.String()})
	require.NoError(t, err)
	require.Len(t, items, 1)
	assert.Equal(t, out.Question.ID, items[0].Question.ID)
	require.NotNil(t, items[0].Question.Topic)
	assert.Contains(t, *items[0].Question.Topic, "Roundtrip")
}

// seedBankQuestionWithTopicDirect inserts a bank question with a non-null topic_id.
func seedBankQuestionWithTopicDirect(t *testing.T, ctx context.Context, repo *repository.Repository, format, body string, topicID uuid.UUID) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	err := repo.Pool().QueryRow(ctx,
		`INSERT INTO question (format, body, topic_id, point_correct, point_wrong) VALUES ($1, $2, $3, 1, 0) RETURNING id`,
		format, body, topicID,
	).Scan(&id)
	require.NoError(t, err)
	return id
}

// silence unused-import lint if tests are trimmed
var _ = errors.New
