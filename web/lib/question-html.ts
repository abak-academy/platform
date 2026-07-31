// Tag allowlist for question-body HTML, shared by RichTextEditor's paste
// sanitizer and the two render sites (RichContent, exam session page).
// Must stay in lockstep with the Go allowlist at
// backend/internal/service/exam.go's questionBodyAllowedTags — enforced by
// TestQuestionBodyAllowedTags_matchesFrontendAllowlist. Attribute lists are
// NOT shared; they deliberately differ per site.
export const QUESTION_BODY_ALLOWED_TAGS = ["b", "i", "u", "ul", "ol", "li", "sup", "sub", "img", "br", "p", "table", "thead", "tbody", "tr", "td", "th"];
