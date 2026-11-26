package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

var (
	ErrTestimonialNotFound   = errors.New("testimonial not found")
	ErrTestimonialIncomplete = errors.New("testimonial cannot be visible yet")
)

const (
	testimonialPhotoPrefix = "question/"
	testimonialMaxChips    = 3
)

// TestimonialIncompleteError lists what a testimonial still lacks before it can
// be visible: "has_consent", "consent_note", "content.id.quote", "content.en.quote", "photo_key".
type TestimonialIncompleteError struct{ Missing []string }

func (e *TestimonialIncompleteError) Error() string {
	return "testimonial cannot be visible yet, missing: " + strings.Join(e.Missing, ", ")
}

func (e *TestimonialIncompleteError) Is(target error) bool { return target == ErrTestimonialIncomplete }

// NullableString tells an absent JSON key (Set=false) from null or a value (Set=true).
type NullableString struct {
	Set   bool
	Value *string
}

func (n *NullableString) UnmarshalJSON(b []byte) error {
	n.Set = true
	if bytes.Equal(bytes.TrimSpace(b), []byte("null")) {
		n.Value = nil
		return nil
	}
	return json.Unmarshal(b, &n.Value)
}

// TestimonialPatch is both the POST and PATCH body: nil/unset leaves a field unchanged.
// consent_by and consent_at are deliberately not here; the server sets them.
type TestimonialPatch struct {
	Name        *string                              `json:"name"`
	RoleLabel   *string                              `json:"role_label"`
	PhotoKey    NullableString                       `json:"photo_key"`
	Content     *map[string]model.TestimonialContent `json:"content"`
	HasConsent  *bool                                `json:"has_consent"`
	ConsentNote NullableString                       `json:"consent_note"`
	IsVisible   *bool                                `json:"is_visible"`
	SortOrder   *int                                 `json:"sort_order"`
}

func (p TestimonialPatch) apply(t *model.Testimonial, actor string, now time.Time) error {
	if p.Name != nil {
		t.Name = strings.TrimSpace(*p.Name)
	}
	if p.RoleLabel != nil {
		t.RoleLabel = strings.TrimSpace(*p.RoleLabel)
	}
	if t.Name == "" || t.RoleLabel == "" {
		return validationf("name and role_label are required")
	}
	forceHidden := false
	if p.PhotoKey.Set {
		if p.PhotoKey.Value == nil {
			t.PhotoKey, forceHidden = nil, true
		} else {
			key := *p.PhotoKey.Value
			if !strings.HasPrefix(key, testimonialPhotoPrefix) || strings.Contains(key, "..") {
				return validationf("photo_key must be an upload key under %s", testimonialPhotoPrefix)
			}
			t.PhotoKey = &key
		}
	}
	if p.Content != nil {
		for locale, c := range *p.Content {
			if locale != "id" && locale != "en" {
				return validationf("content locale %q is not supported", locale)
			}
			if len(c.Achievements) > testimonialMaxChips {
				return validationf("content.%s has more than %d achievements", locale, testimonialMaxChips)
			}
		}
		t.Content = *p.Content
	}
	for locale, c := range t.Content {
		if c.Achievements == nil {
			c.Achievements = []string{}
			t.Content[locale] = c
		}
	}
	if p.HasConsent != nil && *p.HasConsent != t.HasConsent {
		t.HasConsent = *p.HasConsent
		if t.HasConsent {
			t.ConsentBy, t.ConsentAt = &actor, &now
		} else {
			t.ConsentBy, t.ConsentAt, forceHidden = nil, nil, true
		}
	}
	if p.ConsentNote.Set {
		t.ConsentNote = nil
		if p.ConsentNote.Value != nil {
			if note := strings.TrimSpace(*p.ConsentNote.Value); note != "" {
				t.ConsentNote = &note
			}
		}
	}
	if p.SortOrder != nil {
		t.SortOrder = *p.SortOrder
	}
	if p.IsVisible != nil {
		t.IsVisible = *p.IsVisible
	}
	if forceHidden {
		t.IsVisible = false
	}
	if t.IsVisible {
		return testimonialVisibilityGate(t)
	}
	return nil
}

func testimonialVisibilityGate(t *model.Testimonial) error {
	var missing []string
	if !t.HasConsent {
		missing = append(missing, "has_consent")
	}
	if t.ConsentNote == nil {
		missing = append(missing, "consent_note")
	}
	if strings.TrimSpace(t.Content["id"].Quote) == "" {
		missing = append(missing, "content.id.quote")
	}
	if strings.TrimSpace(t.Content["en"].Quote) == "" {
		missing = append(missing, "content.en.quote")
	}
	if t.PhotoKey == nil {
		missing = append(missing, "photo_key")
	}
	if len(missing) > 0 {
		return &TestimonialIncompleteError{Missing: missing}
	}
	return nil
}

func (s *Service) AdminListTestimonials(ctx context.Context) ([]model.Testimonial, error) {
	return s.storeRepo.ListTestimonials(ctx)
}

func (s *Service) AdminCreateTestimonial(ctx context.Context, actor string, p TestimonialPatch) (*model.Testimonial, error) {
	t := &model.Testimonial{Content: map[string]model.TestimonialContent{}}
	if err := p.apply(t, actor, time.Now()); err != nil {
		return nil, err
	}
	if err := s.storeRepo.CreateTestimonial(ctx, t); err != nil {
		return nil, err
	}
	if err := s.auditTestimonialFlags(ctx, nil, actor, t.ID, model.Testimonial{}, *t); err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Service) AdminUpdateTestimonial(ctx context.Context, actor, id string, p TestimonialPatch) (*model.Testimonial, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidUUID
	}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)
	t, err := s.storeRepo.GetTestimonialForUpdateTx(ctx, tx, id)
	if err != nil {
		return nil, testimonialRepoErr(err)
	}
	old := *t
	if err := p.apply(t, actor, time.Now()); err != nil {
		return nil, err
	}
	if err := s.storeRepo.UpdateTestimonialTx(ctx, tx, id, t); err != nil {
		return nil, testimonialRepoErr(err)
	}
	if err := s.auditTestimonialFlags(ctx, tx, actor, id, old, *t); err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.loadTestimonial(ctx, id)
}

func (s *Service) AdminDeleteTestimonial(ctx context.Context, actor, id string) error {
	if _, err := uuid.Parse(id); err != nil {
		return ErrInvalidUUID
	}
	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	t, err := s.storeRepo.GetTestimonialForUpdateTx(ctx, tx, id)
	if err != nil {
		return testimonialRepoErr(err)
	}
	if err := s.storeRepo.DeleteTestimonialTx(ctx, tx, id); err != nil {
		return testimonialRepoErr(err)
	}
	if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actor, "testimonial", id, "testimonial.delete", map[string]any{
		"old": map[string]any{"name": t.Name, "has_consent": t.HasConsent, "is_visible": t.IsVisible},
		"new": nil,
	}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Service) loadTestimonial(ctx context.Context, id string) (*model.Testimonial, error) {
	if _, err := uuid.Parse(id); err != nil {
		return nil, ErrInvalidUUID
	}
	t, err := s.storeRepo.GetTestimonial(ctx, id)
	return t, testimonialRepoErr(err)
}

func testimonialRepoErr(err error) error {
	if errors.Is(err, repository.ErrNotFound) {
		return ErrTestimonialNotFound
	}
	return err
}

func (s *Service) auditTestimonialFlags(ctx context.Context, tx pgx.Tx, actor, id string, old, cur model.Testimonial) error {
	flags := []struct {
		action   string
		old, new bool
	}{
		{"testimonial.consent_change", old.HasConsent, cur.HasConsent},
		{"testimonial.visibility_change", old.IsVisible, cur.IsVisible},
	}
	for _, f := range flags {
		if f.old == f.new {
			continue
		}
		if err := s.storeRepo.InsertAuditLogMeta(ctx, tx, &actor, "testimonial", id, f.action, map[string]any{"old": f.old, "new": f.new}); err != nil {
			return err
		}
	}
	return nil
}
