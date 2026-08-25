package service

import (
	"strings"
	"testing"

	"akademi-bimbel/internal/model"

	"github.com/google/uuid"
)

func TestBuildQuestionBundleHTML_NaskahDoesNotLeakAnswerKeyMaterial(t *testing.T) {
	answer := "SECRET_CORRECT_ANSWER"
	explanation := "SECRET_EXPLANATION"
	accepted := "SECRET_ACCEPTED"
	image := "question/admin/image.png"
	audio := "question/admin/audio.mp3"
	bundle := &model.QuestionBundle{ID: uuid.New(), Variant: "naskah", CreatedBy: uuid.New()}

	html, err := buildQuestionBundleHTML(bundle, "Tryout Paket A", []model.TestDetail{{
		Test: model.Test{Title: "TPS", Subject: "TPS", Topic: "Penalaran", DurationMinutes: 60},
		Questions: []model.QuestionWithOptions{
			{
				Question: model.Question{
					Format:          "mcq",
					Body:            `<p>Pilih jawaban.</p><script>alert('x')</script>`,
					CorrectAnswer:   &answer,
					AcceptedAnswers: []string{accepted},
					Explanation:     &explanation,
					ImageURL:        &image,
					AudioURL:        &audio,
					PointCorrect:    4,
					PointWrong:      1,
				},
				Options: []model.QuestionOption{
					{Key: "a", Text: "Distraktor", IsCorrect: false},
					{Key: "b", Text: "Pilihan benar", IsCorrect: true},
				},
			},
			{
				Question: model.Question{
					Format:     "true_false",
					Body:       `<p>Tentukan benar salah.</p>`,
					Statements: []model.QuestionStatement{{Body: "Langit berwarna biru", IsTrue: true}},
				},
			},
		},
	}}, func(stored string) string { return "https://signed.example/" + stored })
	if err != nil {
		t.Fatalf("buildQuestionBundleHTML: %v", err)
	}
	out := string(html)

	for _, leaked := range []string{answer, accepted, explanation, "Opsi benar", "Jawaban diterima", "Benar", "Salah", "KUNCI JAWABAN"} {
		if strings.Contains(out, leaked) {
			t.Fatalf("naskah variant leaked answer material %q in HTML:\n%s", leaked, out)
		}
	}
	for _, want := range []string{"Pilih jawaban.", "Langit berwarna biru", "Soal ini memiliki audio", "https://signed.example/question/admin/image.png", "break-inside: avoid", "page-break-after: always"} {
		if !strings.Contains(out, want) {
			t.Fatalf("naskah HTML missing %q in:\n%s", want, out)
		}
	}
	if strings.Contains(out, "script") || strings.Contains(out, "alert") {
		t.Fatalf("question body was not sanitized:\n%s", out)
	}
}

func TestBuildQuestionBundleHTML_KunciIncludesBannerAndAnswers(t *testing.T) {
	answer := "SECRET_CORRECT_ANSWER"
	accepted := "SECRET_ACCEPTED"
	bundle := &model.QuestionBundle{ID: uuid.New(), Variant: "kunci", CreatedBy: uuid.New()}

	html, err := buildQuestionBundleHTML(bundle, "Tryout Paket A", []model.TestDetail{{
		Test: model.Test{Title: "TPS", Subject: "TPS", Topic: "Penalaran", DurationMinutes: 60},
		Questions: []model.QuestionWithOptions{{
			Question: model.Question{
				Format:          "short",
				Body:            `<p>Isi jawaban.</p>`,
				CorrectAnswer:   &answer,
				AcceptedAnswers: []string{accepted},
				PointCorrect:    2,
			},
		}},
	}}, nil)
	if err != nil {
		t.Fatalf("buildQuestionBundleHTML: %v", err)
	}
	out := string(html)
	for _, want := range []string{"KUNCI JAWABAN — JANGAN DIBAGIKAN", answer, accepted, "Poin benar: 2"} {
		if !strings.Contains(out, want) {
			t.Fatalf("kunci HTML missing %q in:\n%s", want, out)
		}
	}
}
