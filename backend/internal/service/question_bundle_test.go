package service

import (
	"context"
	"encoding/json"
	"testing"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/stretchr/testify/require"
)

func validQuestionBundleTemplate() QuestionBundleTemplate {
	return QuestionBundleTemplate{
		Document:   `<!doctype html><html><head><style>.key{color:#b91c1c}</style></head><body><h1>{{bundle_title}}</h1><p>{{bundle_variant_label}}</p>{{tests_html}}</body></html>`,
		Test:       `<section><h2>{{test_title}}</h2><p>{{test_meta}}</p>{{questions_html}}</section>`,
		Question:   `<article><h3>Soal {{question_number}} ({{question_format}})</h3>{{question_body_html}}{{statements_html}}{{question_image_html}}{{audio_html}}<ol>{{options_html}}</ol>{{answer_html}}</article>`,
		Option:     `<li>{{option_key}}. {{option_text}}{{option_image_html}}</li>`,
		Statement:  `<li>{{statement_text}}</li>`,
		Image:      `<img src="{{image_url}}" alt="{{image_alt}}">`,
		Audio:      `<p>Soal ini memiliki audio</p>`,
		Answer:     `<aside class="key"><strong>KUNCI JAWABAN - JANGAN DIBAGIKAN</strong>{{answer_items_html}}</aside>`,
		AnswerItem: `<p>{{answer_item}}</p>`,
	}
}

func TestValidateQuestionBundleTemplateRejectsUndeclaredTokensAndExternalResources(t *testing.T) {
	template := validQuestionBundleTemplate()
	template.Option = `<li>{{client_answer}}</li>`
	_, err := ValidateQuestionBundleTemplate(template)
	require.ErrorIs(t, err, ErrValidation)

	template = validQuestionBundleTemplate()
	template.Document = `<html><body><img src="https://evil.example/track.png">{{tests_html}}</body></html>`
	_, err = ValidateQuestionBundleTemplate(template)
	require.ErrorIs(t, err, ErrValidation)
}

func TestValidateQuestionBundleTemplateRejectsTokensInUnsafeContexts(t *testing.T) {
	template := validQuestionBundleTemplate()
	template.Document = `<html><body><img src="{{bundle_title}}">{{tests_html}}</body></html>`
	_, err := ValidateQuestionBundleTemplate(template)
	require.ErrorIs(t, err, ErrValidation)

	template = validQuestionBundleTemplate()
	template.Document = `<html><head><style>body{background:url({{bundle_title}})}</style></head><body>{{tests_html}}</body></html>`
	_, err = ValidateQuestionBundleTemplate(template)
	require.ErrorIs(t, err, ErrValidation)

	template = validQuestionBundleTemplate()
	template.Image = `<img src="prefix-{{image_url}}" alt="{{image_alt}}">`
	_, err = ValidateQuestionBundleTemplate(template)
	require.ErrorIs(t, err, ErrValidation)
}

func TestQuestionBundlePayloadUsesTestIDAndRejectsLegacyScopePayload(t *testing.T) {
	testID := uuid.New()
	encoded, err := json.Marshal(map[string]any{
		"test_id":  testID,
		"variant":  "naskah",
		"template": validQuestionBundleTemplate(),
	})
	require.NoError(t, err)

	var payload QuestionBundleNeededPayload
	require.NoError(t, json.Unmarshal(encoded, &payload))
	require.NoError(t, ValidateQuestionBundlePayload(payload))

	legacyJSON, err := json.Marshal(map[string]any{
		"scope_type": "exam",
		"scope_id":   uuid.New(),
		"variant":    "naskah",
		"template":   validQuestionBundleTemplate(),
	})
	require.NoError(t, err)
	var legacy QuestionBundleNeededPayload
	require.NoError(t, json.Unmarshal(legacyJSON, &legacy))
	require.ErrorIs(t, ValidateQuestionBundlePayload(legacy), ErrValidation)
}

func TestBuildQuestionBundleDocumentSeparatesPublicAndKeyMaterial(t *testing.T) {
	answer := "SECRET_CORRECT_ANSWER"
	explanation := "SECRET_EXPLANATION"
	accepted := "SECRET_ACCEPTED"
	tests := []model.TestDetail{{
		Test: model.Test{Title: "TPS", Subject: "TPS", Topic: "Penalaran", DurationMinutes: 60},
		Questions: []model.QuestionWithOptions{{
			Question: model.Question{
				Format:          "mcq",
				Body:            `<p>Pilih jawaban.</p><script>alert('x')</script>`,
				CorrectAnswer:   &answer,
				AcceptedAnswers: []string{accepted},
				Explanation:     &explanation,
				PointCorrect:    4,
				PointWrong:      1,
			},
			Options: []model.QuestionOption{
				{Key: "a", Text: "Distraktor", IsCorrect: false},
				{Key: "b", Text: "Pilihan benar", IsCorrect: true},
			},
		}},
	}}
	template, err := ValidateQuestionBundleTemplate(validQuestionBundleTemplate())
	require.NoError(t, err)

	publicHTML, err := buildQuestionBundleDocument(template, "naskah", "Tryout A", tests, func(value string) string { return value })
	require.NoError(t, err)
	public := string(publicHTML)
	for _, secret := range []string{answer, accepted, explanation, "KUNCI JAWABAN", "Opsi benar"} {
		require.NotContains(t, public, secret)
	}
	require.Contains(t, public, "Pilih jawaban.")
	require.NotContains(t, public, "script")

	keyHTML, err := buildQuestionBundleDocument(template, "kunci", "Tryout A", tests, func(value string) string { return value })
	require.NoError(t, err)
	key := string(keyHTML)
	for _, want := range []string{"KUNCI JAWABAN", answer, accepted, explanation, "Opsi benar: B"} {
		require.Contains(t, key, want)
	}
}

func TestBuildQuestionBundleDocumentResolvesEmbeddedImagesAndOmitsUntrustedSources(t *testing.T) {
	tests := []model.TestDetail{{
		Test: model.Test{Title: "TPS", Subject: "TPS", Topic: "Penalaran", DurationMinutes: 60},
		Questions: []model.QuestionWithOptions{{
			Question: model.Question{
				Format:       "mcq",
				Body:         `<p>Stem <img src="/api/v1/files/question/admin/stem.png" alt="Stem"></p><img src="https://evil.example/track.png">`,
				PointCorrect: 4,
				PointWrong:   1,
			},
			Options: []model.QuestionOption{
				{Key: "a", Text: `<p>Option <img src="question/admin/option.png" alt="Option"></p>`},
			},
		}},
	}}
	template, err := ValidateQuestionBundleTemplate(validQuestionBundleTemplate())
	require.NoError(t, err)

	document, err := buildQuestionBundleDocument(template, "naskah", "Tryout A", tests, func(stored string) string {
		if key := questionAssetKeyFromStored(stored); key != "" {
			return "https://storage.internal/" + key
		}
		return ""
	})
	require.NoError(t, err)
	out := string(document)
	require.Contains(t, out, `src="https://storage.internal/question/admin/stem.png"`)
	require.Contains(t, out, `src="https://storage.internal/question/admin/option.png"`)
	require.NotContains(t, out, "evil.example")
}

func TestLoadableQuestionAssetURLRejectsForeignOrUnavailableAssets(t *testing.T) {
	svc := &Service{}
	require.Empty(t, svc.loadableQuestionAssetURL(context.Background(), "https://evil.example/track.png"))
	require.Empty(t, svc.loadableQuestionAssetURL(context.Background(), "question/admin/stem.png"))
}

type countingQuestionBundleRenderer struct{ calls int }

func (r *countingQuestionBundleRenderer) RenderHTML(context.Context, []byte) ([]byte, error) {
	r.calls++
	return []byte("%PDF-1.4"), nil
}

func insertServiceQuestionBundleTest(t *testing.T, pool *pgxpool.Pool) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(context.Background(),
		`INSERT INTO test (title, subject, topic, duration_minutes) VALUES ('Bundle Test', 'Math', 'Algebra', 60) RETURNING id`,
	).Scan(&id))
	return id
}

func TestGenerateQuestionBundlePDFCacheHitDoesNotRender(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	pool, _ := gateTestPool(t)
	repo := repository.New(pool)
	testID := insertServiceQuestionBundleTest(t, pool)
	key := "question-bundles/tests/" + testID.String() + "/naskah.pdf"
	require.NoError(t, repo.SetQuestionBundleReady(context.Background(), testID, "naskah", key))
	renderer := &countingQuestionBundleRenderer{}
	svc := &Service{storeRepo: repo, renderer: renderer}

	err := svc.GenerateQuestionBundlePDF(context.Background(), QuestionBundleNeededPayload{
		TestID:   testID,
		Variant:  "naskah",
		Template: validQuestionBundleTemplate(),
	})
	require.NoError(t, err)
	require.Zero(t, renderer.calls)
}

func TestRequestQuestionBundleCacheHitDoesNotEnqueue(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	pool, _ := gateTestPool(t)
	repo := repository.New(pool)
	testID := insertServiceQuestionBundleTest(t, pool)
	key := "question-bundles/tests/" + testID.String() + "/kunci.pdf"
	require.NoError(t, repo.SetQuestionBundleReady(context.Background(), testID, "kunci", key))
	svc := &Service{storeRepo: repo}

	state, err := svc.RequestQuestionBundle(context.Background(), uuid.New(), RoleAdminExam, testID, "kunci", QuestionBundleTemplate{})
	require.NoError(t, err)
	require.Equal(t, "ready", state.Status)

	var count int
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM outbox WHERE event_type = 'QuestionBundleNeeded'`).Scan(&count))
	require.Zero(t, count)
}

func TestRequestQuestionBundleDoesNotDuplicateQueuedVariant(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	pool, _ := gateTestPool(t)
	repo := repository.New(pool)
	testID := insertServiceQuestionBundleTest(t, pool)
	svc := &Service{storeRepo: repo}
	actor := uuid.New()

	first, err := svc.RequestQuestionBundle(context.Background(), actor, RoleAdminExam, testID, "naskah", validQuestionBundleTemplate())
	require.NoError(t, err)
	require.Equal(t, "queued", first.Status)
	second, err := svc.RequestQuestionBundle(context.Background(), actor, RoleAdminExam, testID, "naskah", validQuestionBundleTemplate())
	require.NoError(t, err)
	require.Equal(t, "queued", second.Status)

	var count int
	var hasTestID, hasScopeType, hasScopeID bool
	require.NoError(t, pool.QueryRow(context.Background(), `SELECT COUNT(*), bool_and(payload ? 'test_id'), bool_or(payload ? 'scope_type'), bool_or(payload ? 'scope_id') FROM outbox WHERE event_type = 'QuestionBundleNeeded'`).Scan(&count, &hasTestID, &hasScopeType, &hasScopeID))
	require.Equal(t, 1, count)
	require.True(t, hasTestID)
	require.False(t, hasScopeType)
	require.False(t, hasScopeID)
}

func TestQuestionBundleAuthorizationIsServerSide(t *testing.T) {
	svc := &Service{}
	_, err := svc.RequestQuestionBundle(context.Background(), uuid.New(), RoleAdminSchool, uuid.New(), "naskah", validQuestionBundleTemplate())
	require.ErrorIs(t, err, ErrForbidden)
	_, err = svc.GetQuestionBundleState(context.Background(), RoleAdminSchool, uuid.New(), "naskah")
	require.ErrorIs(t, err, ErrForbidden)
}

func TestQuestionBundleAuthoringMutationsInvalidateOwners(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	pool, _ := gateTestPool(t)
	repo := repository.New(pool)
	ctx := context.Background()
	svc := &Service{storeRepo: repo}
	testID := insertServiceQuestionBundleTest(t, pool)
	secondTestID := insertServiceQuestionBundleTest(t, pool)
	var firstQuestionID, secondQuestionID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('essay', 'First', 1, 0) RETURNING id`,
	).Scan(&firstQuestionID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO question (format, body, point_correct, point_wrong) VALUES ('essay', 'Second', 1, 0) RETURNING id`,
	).Scan(&secondQuestionID))
	require.NoError(t, insertTestQuestion(ctx, pool, testID, firstQuestionID, 0))
	require.NoError(t, insertTestQuestion(ctx, pool, testID, secondQuestionID, 1))
	require.NoError(t, insertTestQuestion(ctx, pool, secondTestID, firstQuestionID, 0))

	seed := func(testIDs ...uuid.UUID) {
		for _, testID := range testIDs {
			for _, variant := range []string{"naskah", "kunci"} {
				require.NoError(t, repo.SetQuestionBundleReady(ctx, testID, variant, questionBundleObjectKey(testID, variant)))
			}
		}
	}
	assertCleared := func(testID uuid.UUID) {
		for _, variant := range []string{"naskah", "kunci"} {
			owner, err := repo.GetQuestionBundleOwner(ctx, testID, variant)
			require.NoError(t, err)
			require.Nil(t, owner.ObjectKey, "test %s key must be invalidated", variant)
			require.Nil(t, owner.GeneratedAt, "test %s generated timestamp must be invalidated", variant)
		}
	}

	seed(testID)
	_, err := svc.UpdateTest(ctx, testID, model.Test{Title: "Renamed", Subject: "Math", Topic: "Algebra", DurationMinutes: 60})
	require.NoError(t, err)
	assertCleared(testID)

	seed(testID)
	third, err := svc.CreateQuestionForTest(ctx, testID, model.Question{Format: "essay", Body: "Third", PointCorrect: 1}, nil, nil, nil)
	require.NoError(t, err)
	assertCleared(testID)

	seed(testID)
	require.NoError(t, svc.DetachQuestion(ctx, testID, third.Question.ID))
	assertCleared(testID)

	seed(testID)
	require.NoError(t, svc.ReorderTestQuestions(ctx, testID, []uuid.UUID{secondQuestionID, firstQuestionID}))
	assertCleared(testID)

	seed(testID, secondTestID)
	_, err = svc.SaveQuestion(ctx, model.Question{ID: firstQuestionID, Format: "essay", Body: "Edited", PointCorrect: 1}, nil, nil, nil)
	require.NoError(t, err)
	assertCleared(testID)
	assertCleared(secondTestID)

	deletable, err := svc.CreateQuestionForTest(ctx, testID, model.Question{Format: "essay", Body: "Delete me", PointCorrect: 1}, nil, nil, nil)
	require.NoError(t, err)
	seed(testID)
	require.NoError(t, svc.DeleteQuestion(ctx, deletable.Question.ID))
	assertCleared(testID)
}

func TestExamEditsDoNotInvalidateQuestionBundles(t *testing.T) {
	if testing.Short() {
		t.Skip("integration test")
	}
	pool, _ := gateTestPool(t)
	repo := repository.New(pool)
	ctx := context.Background()
	svc := &Service{storeRepo: repo}
	testID := insertServiceQuestionBundleTest(t, pool)
	var examID uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO exam (title, timer_mode, duration_minutes) VALUES ('Bundle Exam', 'overall', 60) RETURNING id`,
	).Scan(&examID))
	require.NoError(t, insertExamTest(ctx, pool, examID, testID, 0))

	seed := func() {
		for _, variant := range []string{"naskah", "kunci"} {
			require.NoError(t, repo.SetQuestionBundleReady(ctx, testID, variant, questionBundleObjectKey(testID, variant)))
		}
	}
	assertPreserved := func() {
		for _, variant := range []string{"naskah", "kunci"} {
			owner, err := repo.GetQuestionBundleOwner(ctx, testID, variant)
			require.NoError(t, err)
			require.NotNil(t, owner.ObjectKey, "exam edits must preserve test %s", variant)
		}
	}

	seed()
	exam, err := repo.GetExamByID(ctx, examID)
	require.NoError(t, err)
	exam.Title = "Renamed Bundle Exam"
	_, err = svc.UpdateExam(ctx, examID, *exam)
	require.NoError(t, err)
	assertPreserved()

	seed()
	require.NoError(t, svc.ReplaceExamTests(ctx, examID, []uuid.UUID{testID}))
	assertPreserved()
}

func insertTestQuestion(ctx context.Context, pool *pgxpool.Pool, testID, questionID uuid.UUID, sortOrder int) error {
	_, err := pool.Exec(ctx, `INSERT INTO test_question (test_id, question_id, sort_order) VALUES ($1, $2, $3)`, testID, questionID, sortOrder)
	return err
}

func insertExamTest(ctx context.Context, pool *pgxpool.Pool, examID, testID uuid.UUID, sortOrder int) error {
	_, err := pool.Exec(ctx, `INSERT INTO exam_test (exam_id, test_id, sort_order) VALUES ($1, $2, $3)`, examID, testID, sortOrder)
	return err
}

func TestQuestionBundleDeterministicKey(t *testing.T) {
	id := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	require.Equal(t, "question-bundles/tests/11111111-1111-1111-1111-111111111111/naskah.pdf", questionBundleObjectKey(id, "naskah"))
	require.Equal(t, "question-bundles/tests/11111111-1111-1111-1111-111111111111/kunci.pdf", questionBundleObjectKey(id, "kunci"))
}
