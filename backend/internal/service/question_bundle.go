package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	stdhtml "html"
	"strings"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"

	"github.com/google/uuid"
	"github.com/microcosm-cc/bluemonday"
	"github.com/minio/minio-go/v7"
	xhtml "golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const questionBundlePDFContentType = "application/pdf"

type QuestionBundleTemplate struct {
	Document   string `json:"document"`
	Test       string `json:"test"`
	Question   string `json:"question"`
	Option     string `json:"option"`
	Statement  string `json:"statement"`
	Image      string `json:"image"`
	Audio      string `json:"audio"`
	Answer     string `json:"answer"`
	AnswerItem string `json:"answer_item"`
}

type QuestionBundleNeededPayload struct {
	TestID   uuid.UUID              `json:"test_id"`
	Variant  string                 `json:"variant"`
	Template QuestionBundleTemplate `json:"template"`
}

// ValidateQuestionBundlePayload distinguishes malformed permanent outbox
// messages from transient render/storage failures. Workers may safely mark an
// invalid payload processed; retrying it cannot ever produce a PDF.
func ValidateQuestionBundlePayload(payload QuestionBundleNeededPayload) error {
	if payload.TestID == uuid.Nil {
		return fmt.Errorf("%w: question bundle test_id is required", ErrValidation)
	}
	if err := validateQuestionBundleVariant(payload.Variant); err != nil {
		return err
	}
	_, err := ValidateQuestionBundleTemplate(payload.Template)
	return err
}

var questionBundleTemplatePolicy = func() *bluemonday.Policy {
	p := bluemonday.NewPolicy()
	p.AllowElements("html", "head", "meta", "style", "body", "div", "span", "p", "img",
		"section", "article", "header", "footer", "aside", "h1", "h2", "h3", "h4",
		"ol", "ul", "li", "table", "thead", "tbody", "tr", "td", "th", "b", "i",
		"u", "strong", "em", "br")
	p.AllowAttrs("class", "style").Globally()
	p.AllowAttrs("charset").OnElements("meta")
	p.AllowAttrs("src", "alt").OnElements("img")
	p.AllowAttrs("type", "start").OnElements("ol")
	p.AllowAttrs("colspan", "rowspan").OnElements("td", "th")
	p.AllowUnsafe(true)
	return p
}()

type questionBundleTemplatePart struct {
	name            string
	value           string
	allowed         map[string]bool
	required        []string
	attributeTokens map[string]string
	document        bool
	set             func(string)
}

func tokenSet(tokens ...string) map[string]bool {
	out := make(map[string]bool, len(tokens))
	for _, token := range tokens {
		out[token] = true
	}
	return out
}

func ValidateQuestionBundleTemplate(input QuestionBundleTemplate) (QuestionBundleTemplate, error) {
	out := input
	parts := []questionBundleTemplatePart{
		{name: "document", value: input.Document, allowed: tokenSet("bundle_title", "bundle_variant_label", "tests_html"), required: []string{"bundle_title", "bundle_variant_label", "tests_html"}, document: true, set: func(v string) { out.Document = v }},
		{name: "test", value: input.Test, allowed: tokenSet("test_title", "test_meta", "questions_html"), required: []string{"test_title", "questions_html"}, set: func(v string) { out.Test = v }},
		{name: "question", value: input.Question, allowed: tokenSet("question_number", "question_format", "question_body_html", "statements_html", "question_image_html", "audio_html", "options_html", "answer_html"), required: []string{"question_number", "question_body_html", "options_html", "answer_html"}, set: func(v string) { out.Question = v }},
		{name: "option", value: input.Option, allowed: tokenSet("option_key", "option_text", "option_image_html"), required: []string{"option_key", "option_text"}, set: func(v string) { out.Option = v }},
		{name: "statement", value: input.Statement, allowed: tokenSet("statement_text"), required: []string{"statement_text"}, set: func(v string) { out.Statement = v }},
		{name: "image", value: input.Image, allowed: tokenSet("image_url", "image_alt"), required: []string{"image_url"}, attributeTokens: map[string]string{"image_url": "img.src", "image_alt": "img.alt"}, set: func(v string) { out.Image = v }},
		{name: "audio", value: input.Audio, allowed: tokenSet(), set: func(v string) { out.Audio = v }},
		{name: "answer", value: input.Answer, allowed: tokenSet("answer_items_html"), required: []string{"answer_items_html"}, set: func(v string) { out.Answer = v }},
		{name: "answer_item", value: input.AnswerItem, allowed: tokenSet("answer_item"), required: []string{"answer_item"}, set: func(v string) { out.AnswerItem = v }},
	}

	for _, part := range parts {
		if strings.TrimSpace(part.value) == "" {
			return QuestionBundleTemplate{}, fmt.Errorf("%w: question bundle template %s is required", ErrValidation, part.name)
		}
		sanitized := questionBundleTemplatePolicy.Sanitize(part.value)
		if part.document {
			sanitized = "<!DOCTYPE html>\n" + sanitized
		}
		for _, token := range certificateTokens(sanitized) {
			if !part.allowed[token] {
				return QuestionBundleTemplate{}, fmt.Errorf("%w: question bundle template %s contains undeclared token {{%s}}", ErrValidation, part.name, token)
			}
		}
		for _, token := range part.required {
			if !strings.Contains(sanitized, "{{"+token+"}}") {
				return QuestionBundleTemplate{}, fmt.Errorf("%w: question bundle template %s is missing {{%s}}", ErrValidation, part.name, token)
			}
		}
		if err := validateQuestionBundleTokenContexts(part.name, sanitized, part.attributeTokens); err != nil {
			return QuestionBundleTemplate{}, err
		}
		if certificateTemplateHasExternalResource(sanitized) {
			return QuestionBundleTemplate{}, fmt.Errorf("%w: question bundle template %s references an external resource", ErrValidation, part.name)
		}
		part.set(sanitized)
	}
	return out, nil
}

func validateQuestionBundleTokenContexts(partName, content string, attributeTokens map[string]string) error {
	document, err := xhtml.Parse(strings.NewReader(content))
	if err != nil {
		return fmt.Errorf("%w: parse question bundle template %s: %v", ErrValidation, partName, err)
	}
	var walk func(*xhtml.Node) error
	walk = func(node *xhtml.Node) error {
		if node.Type == xhtml.TextNode {
			for _, token := range certificateTokens(node.Data) {
				if attributeTokens[token] != "" || (node.Parent != nil && node.Parent.DataAtom == atom.Style) {
					return fmt.Errorf("%w: question bundle template %s places {{%s}} in an unsafe context", ErrValidation, partName, token)
				}
			}
		}
		if node.Type == xhtml.ElementNode {
			for _, attr := range node.Attr {
				for _, token := range certificateTokens(attr.Val) {
					location := node.Data + "." + attr.Key
					if attributeTokens[token] != location || attr.Val != "{{"+token+"}}" {
						return fmt.Errorf("%w: question bundle template %s places {{%s}} in an unsafe context", ErrValidation, partName, token)
					}
				}
			}
		}
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if err := walk(child); err != nil {
				return err
			}
		}
		return nil
	}
	return walk(document)
}

func validateQuestionBundleVariant(variant string) error {
	if variant != "naskah" && variant != "kunci" {
		return fmt.Errorf("%w: variant must be naskah or kunci", ErrValidation)
	}
	return nil
}

func questionBundleState(testID uuid.UUID, variant, status string, owner *model.QuestionBundleOwner) model.QuestionBundleState {
	state := model.QuestionBundleState{TestID: testID, Variant: variant, Status: status}
	if owner != nil {
		state.GeneratedAt = owner.GeneratedAt
	}
	return state
}

func (s *Service) RequestQuestionBundle(ctx context.Context, actor uuid.UUID, actorRole string, testID uuid.UUID, variant string, input QuestionBundleTemplate) (*model.QuestionBundleState, error) {
	if !HasCapability(actorRole, "question-bundles:write") {
		return nil, ErrForbidden
	}
	if err := validateQuestionBundleVariant(variant); err != nil {
		return nil, err
	}
	owner, err := s.storeRepo.GetQuestionBundleOwner(ctx, testID, variant)
	if err != nil {
		return nil, err
	}
	if owner.ObjectKey != nil && *owner.ObjectKey != "" {
		state := questionBundleState(testID, variant, "ready", owner)
		return &state, nil
	}
	pending, err := s.storeRepo.HasPendingQuestionBundleEvent(ctx, testID, variant)
	if err != nil {
		return nil, err
	}
	if pending {
		state := questionBundleState(testID, variant, "queued", owner)
		return &state, nil
	}

	sanitized, err := ValidateQuestionBundleTemplate(input)
	if err != nil {
		return nil, err
	}
	payload := QuestionBundleNeededPayload{TestID: testID, Variant: variant, Template: sanitized}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.InsertOutboxEvent(ctx, tx, "question_bundle_test", testID, "QuestionBundleNeeded", payload); err != nil {
		return nil, err
	}
	actorID := actor.String()
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actorID, "test", testID.String(), "question_bundle.request", map[string]any{"variant": variant}); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	state := questionBundleState(testID, variant, "queued", owner)
	return &state, nil
}

func (s *Service) GetQuestionBundleState(ctx context.Context, actorRole string, testID uuid.UUID, variant string) (*model.QuestionBundleState, error) {
	if !HasCapability(actorRole, "question-bundles:write") {
		return nil, ErrForbidden
	}
	if err := validateQuestionBundleVariant(variant); err != nil {
		return nil, err
	}
	owner, err := s.storeRepo.GetQuestionBundleOwner(ctx, testID, variant)
	if err != nil {
		return nil, err
	}
	status := "idle"
	if owner.ObjectKey != nil && *owner.ObjectKey != "" {
		status = "ready"
	} else {
		pending, err := s.storeRepo.HasPendingQuestionBundleEvent(ctx, testID, variant)
		if err != nil {
			return nil, err
		}
		if pending {
			status = "queued"
		}
	}
	state := questionBundleState(testID, variant, status, owner)
	return &state, nil
}

func (s *Service) GenerateQuestionBundlePDF(ctx context.Context, payload QuestionBundleNeededPayload) error {
	if err := ValidateQuestionBundlePayload(payload); err != nil {
		return err
	}
	conn, err := s.storeRepo.Pool().Acquire(ctx)
	if err != nil {
		return err
	}
	defer conn.Release()
	lockName := fmt.Sprintf("question-bundle:test:%s:%s", payload.TestID, payload.Variant)
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock(hashtext($1))`, lockName); err != nil {
		return err
	}
	defer conn.Exec(context.Background(), `SELECT pg_advisory_unlock(hashtext($1))`, lockName)

	owner, err := s.storeRepo.GetQuestionBundleOwner(ctx, payload.TestID, payload.Variant)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil
		}
		return err
	}
	if owner.ObjectKey != nil && *owner.ObjectKey != "" {
		return nil
	}
	sanitized, err := ValidateQuestionBundleTemplate(payload.Template)
	if err != nil {
		return err
	}
	if s.renderer == nil {
		return ErrPDFRendererNotConfigured
	}
	if s.storage == nil || s.cfg == nil || s.cfg.ObjectStorageBucketName == "" {
		return ErrStorageNotConfigured
	}
	test, err := s.storeRepo.GetTestDetail(ctx, payload.TestID)
	if err != nil {
		return err
	}
	document, err := buildQuestionBundleDocument(sanitized, payload.Variant, test.Test.Title, []model.TestDetail{*test}, func(stored string) string {
		return s.loadableQuestionAssetURL(ctx, stored)
	})
	if err != nil {
		return err
	}
	pdf, err := s.renderer.RenderHTML(ctx, document)
	if err != nil {
		return err
	}
	key := questionBundleObjectKey(payload.TestID, payload.Variant)
	if _, err := s.storage.PutObject(ctx, s.cfg.ObjectStorageBucketName, key, bytes.NewReader(pdf), int64(len(pdf)), minio.PutObjectOptions{ContentType: questionBundlePDFContentType}); err != nil {
		return err
	}
	_, err = s.storeRepo.SetQuestionBundleReadyIfCurrent(ctx, payload.TestID, payload.Variant, key, owner.Revision)
	return err
}

func questionBundleObjectKey(testID uuid.UUID, variant string) string {
	return fmt.Sprintf("question-bundles/tests/%s/%s.pdf", testID, variant)
}

func substituteQuestionBundleFragment(fragment string, textValues, markupValues map[string]string) string {
	return certificateTokenPattern.ReplaceAllStringFunc(fragment, func(match string) string {
		token := match[2 : len(match)-2]
		if value, ok := markupValues[token]; ok {
			return value
		}
		return stdhtml.EscapeString(textValues[token])
	})
}

func resolveQuestionBundleRichHTML(value string, resolveAsset func(string) string) (string, error) {
	contextNode := &xhtml.Node{Type: xhtml.ElementNode, DataAtom: atom.Div, Data: "div"}
	nodes, err := xhtml.ParseFragment(strings.NewReader(sanitizeQuestionBody(value)), contextNode)
	if err != nil {
		return "", err
	}
	root := &xhtml.Node{Type: xhtml.DocumentNode}
	for _, node := range nodes {
		root.AppendChild(node)
	}

	var rewriteImages func(*xhtml.Node)
	rewriteImages = func(parent *xhtml.Node) {
		for node := parent.FirstChild; node != nil; {
			next := node.NextSibling
			if node.Type == xhtml.ElementNode && node.DataAtom == atom.Img {
				resolved := ""
				for index := range node.Attr {
					if node.Attr[index].Key == "src" {
						resolved = resolveAsset(node.Attr[index].Val)
						if resolved != "" {
							node.Attr[index].Val = resolved
						}
						break
					}
				}
				if resolved == "" {
					parent.RemoveChild(node)
					node = next
					continue
				}
			}
			rewriteImages(node)
			node = next
		}
	}
	rewriteImages(root)

	var output strings.Builder
	for node := root.FirstChild; node != nil; node = node.NextSibling {
		if err := xhtml.Render(&output, node); err != nil {
			return "", err
		}
	}
	return output.String(), nil
}

func buildQuestionBundleDocument(contract QuestionBundleTemplate, variant, title string, tests []model.TestDetail, resolveAsset func(string) string) ([]byte, error) {
	if variant != "naskah" && variant != "kunci" {
		return nil, fmt.Errorf("%w: invalid question bundle variant", ErrValidation)
	}
	if resolveAsset == nil {
		resolveAsset = func(value string) string { return value }
	}
	testMarkup := make([]string, 0, len(tests))
	for _, test := range tests {
		questionMarkup := make([]string, 0, len(test.Questions))
		for index, question := range test.Questions {
			optionMarkup := make([]string, 0, len(question.Options))
			correctOptions := make([]string, 0)
			for _, option := range question.Options {
				optionText, err := resolveQuestionBundleRichHTML(option.Text, resolveAsset)
				if err != nil {
					return nil, err
				}
				image := ""
				if option.ImageURL != nil && *option.ImageURL != "" {
					if resolved := resolveAsset(*option.ImageURL); resolved != "" {
						image = substituteQuestionBundleFragment(contract.Image, map[string]string{"image_url": resolved, "image_alt": "Gambar opsi " + strings.ToUpper(option.Key)}, nil)
					}
				}
				optionMarkup = append(optionMarkup, substituteQuestionBundleFragment(contract.Option, map[string]string{
					"option_key": strings.ToUpper(option.Key),
				}, map[string]string{"option_text": optionText, "option_image_html": image}))
				if option.IsCorrect {
					correctOptions = append(correctOptions, strings.ToUpper(option.Key))
				}
			}

			statementMarkup := make([]string, 0, len(question.Question.Statements))
			for _, statement := range question.Question.Statements {
				statementMarkup = append(statementMarkup, substituteQuestionBundleFragment(contract.Statement, map[string]string{"statement_text": statement.Body}, nil))
			}
			image := ""
			if question.Question.ImageURL != nil && *question.Question.ImageURL != "" {
				if resolved := resolveAsset(*question.Question.ImageURL); resolved != "" {
					image = substituteQuestionBundleFragment(contract.Image, map[string]string{"image_url": resolved, "image_alt": fmt.Sprintf("Gambar soal %d", index+1)}, nil)
				}
			}
			audio := ""
			if question.Question.AudioURL != nil && *question.Question.AudioURL != "" {
				audio = contract.Audio
			}
			answer := ""
			if variant == "kunci" {
				items := questionBundleAnswerItems(question, correctOptions)
				itemMarkup := make([]string, 0, len(items))
				for _, item := range items {
					itemMarkup = append(itemMarkup, substituteQuestionBundleFragment(contract.AnswerItem, map[string]string{"answer_item": item}, nil))
				}
				answer = substituteQuestionBundleFragment(contract.Answer, nil, map[string]string{"answer_items_html": strings.Join(itemMarkup, "")})
			}
			questionBody, err := resolveQuestionBundleRichHTML(question.Question.Body, resolveAsset)
			if err != nil {
				return nil, err
			}
			questionMarkup = append(questionMarkup, substituteQuestionBundleFragment(contract.Question, map[string]string{
				"question_number": fmt.Sprintf("%d", index+1), "question_format": question.Question.Format,
			}, map[string]string{
				"question_body_html": questionBody,
				"statements_html":    strings.Join(statementMarkup, ""), "question_image_html": image,
				"audio_html": audio, "options_html": strings.Join(optionMarkup, ""), "answer_html": answer,
			}))
		}
		meta := fmt.Sprintf("%s • %s • %d menit", test.Test.Subject, test.Test.Topic, test.Test.DurationMinutes)
		testMarkup = append(testMarkup, substituteQuestionBundleFragment(contract.Test, map[string]string{
			"test_title": test.Test.Title, "test_meta": meta,
		}, map[string]string{"questions_html": strings.Join(questionMarkup, "")}))
	}
	variantLabel := "Naskah soal PDF"
	if variant == "kunci" {
		variantLabel = "Kunci jawaban PDF"
	}
	document := substituteQuestionBundleFragment(contract.Document, map[string]string{"bundle_title": title, "bundle_variant_label": variantLabel}, map[string]string{"tests_html": strings.Join(testMarkup, "")})
	return []byte(document), nil
}

func questionBundleAnswerItems(question model.QuestionWithOptions, correctOptions []string) []string {
	items := make([]string, 0)
	if len(correctOptions) > 0 {
		items = append(items, "Opsi benar: "+strings.Join(correctOptions, ", "))
	}
	if question.Question.CorrectAnswer != nil && *question.Question.CorrectAnswer != "" {
		items = append(items, "Jawaban: "+*question.Question.CorrectAnswer)
	}
	if len(question.Question.AcceptedAnswers) > 0 {
		items = append(items, "Jawaban diterima: "+strings.Join(question.Question.AcceptedAnswers, ", "))
	}
	items = append(items, fmt.Sprintf("Poin benar: %s • Poin salah: -%s", trimQuestionBundleFloat(question.Question.PointCorrect), trimQuestionBundleFloat(question.Question.PointWrong)))
	for index, blank := range question.Blanks {
		value := fmt.Sprintf("Isian %d: %s", index+1, blank.CorrectAnswer)
		if len(blank.AcceptedAnswers) > 0 {
			value += " (diterima: " + strings.Join(blank.AcceptedAnswers, ", ") + ")"
		}
		items = append(items, value)
	}
	for index, statement := range question.Question.Statements {
		truth := "Salah"
		if statement.IsTrue {
			truth = "Benar"
		}
		items = append(items, fmt.Sprintf("Pernyataan %d: %s — %s", index+1, statement.Body, truth))
	}
	if question.Question.Explanation != nil && *question.Question.Explanation != "" {
		items = append(items, "Pembahasan: "+*question.Question.Explanation)
	}
	return items
}

func trimQuestionBundleFloat(value float64) string {
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.2f", value), "0"), ".")
}

func (s *Service) GetQuestionBundleDownloadURL(ctx context.Context, actor uuid.UUID, actorRole string, testID uuid.UUID, variant string) (string, error) {
	if !HasCapability(actorRole, "question-bundles:write") {
		return "", ErrForbidden
	}
	if err := validateQuestionBundleVariant(variant); err != nil {
		return "", err
	}
	owner, err := s.storeRepo.GetQuestionBundleOwner(ctx, testID, variant)
	if err != nil {
		return "", err
	}
	if owner.ObjectKey == nil || *owner.ObjectKey == "" {
		return "", fmt.Errorf("%w: question bundle is not ready", ErrValidation)
	}
	if s.storage == nil || s.cfg == nil || s.cfg.ObjectStorageBucketName == "" {
		return "", ErrStorageNotConfigured
	}
	presigned, err := s.presignReadURL(ctx, s.cfg.ObjectStorageBucketName, *owner.ObjectKey, presignedDocumentURLTTL)
	if err != nil {
		return "", fmt.Errorf("presign question bundle: %w", err)
	}
	actorID := actor.String()
	if err := s.storeRepo.InsertAuditLogMeta(ctx, nil, &actorID, "test", testID.String(), "question_bundle.download_url_issued", map[string]any{"variant": variant}); err != nil {
		return "", err
	}
	return presigned, nil
}

func (s *Service) loadableQuestionAssetURL(ctx context.Context, stored string) string {
	key := questionAssetKeyFromStored(stored)
	if key == "" || s.storage == nil || s.cfg == nil || s.cfg.ObjectStorageBucketName == "" {
		return ""
	}
	signed, err := s.presignInternalReadURL(ctx, s.cfg.ObjectStorageBucketName, key, presignedDocumentURLTTL)
	if err != nil {
		return ""
	}
	return signed
}

func questionAssetKeyFromStored(stored string) string {
	if strings.HasPrefix(stored, "question/") {
		return stored
	}
	marker := "/files/"
	idx := strings.Index(stored, marker)
	if idx < 0 {
		return ""
	}
	key := strings.TrimPrefix(stored[idx+len(marker):], "/")
	if strings.HasPrefix(key, "question/") {
		return key
	}
	return ""
}
