package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

func (s *Service) AdminListProductGroupings(ctx context.Context) ([]model.ProductGrouping, error) {
	return s.storeRepo.ListProductGroupings(ctx, false)
}

// ListPublishedProductGroupings is the student-facing landing list.
func (s *Service) ListPublishedProductGroupings(ctx context.Context) ([]model.ProductGrouping, error) {
	return s.storeRepo.ListProductGroupings(ctx, true)
}

var (
	ErrGroupingHasProducts = errors.New("event still has member products; hide it or remove its products first")
	ErrGroupingIncomplete  = errors.New("event cannot be visible yet")

	groupingPrefixRE   = regexp.MustCompile(`^[A-Z]{2,6}$`)
	groupingSlugRunRE  = regexp.MustCompile(`[^a-z0-9]+`)
	groupingTones      = []string{"brand", "ember", "teal", "gold", "sky", "berry", "navy"}
	groupingIllustrate = []string{"math", "science", "language", "coding", "trophy"}
	groupingScopes     = []string{"national", "international"}
)

// GroupingIncompleteError lists what an event still lacks before it can be
// visible, e.g. "participant_prefix", "en.tagline", "schedule[2].label_en".
type GroupingIncompleteError struct{ Missing []string }

func (e *GroupingIncompleteError) Error() string {
	return "event cannot be visible yet, missing: " + strings.Join(e.Missing, ", ")
}

func (e *GroupingIncompleteError) Is(target error) bool { return target == ErrGroupingIncomplete }

// ProductGroupingPatch is the PATCH body: nil leaves a field unchanged; for the
// nullable text fields an empty string clears the value.
type ProductGroupingPatch struct {
	Name              *string                              `json:"name"`
	Description       *string                              `json:"description"`
	Status            *string                              `json:"status"`
	ShortName         *string                              `json:"short_name"`
	Scope             *string                              `json:"scope"`
	Subjects          *[]string                            `json:"subjects"`
	Tone              *string                              `json:"tone"`
	Illustration      *string                              `json:"illustration"`
	LogoKey           *string                              `json:"logo_key"`
	OfficialURL       *string                              `json:"official_url"`
	Content           *map[string]model.EventNarrative     `json:"content"`
	IsVisible         *bool                                `json:"is_visible"`
	ParticipantPrefix *string                              `json:"participant_prefix"`
	Schedule          *[]model.ProductGroupingScheduleItem `json:"schedule"`
}

// GroupingDetail is the admin read: the event, its members, schedule and EN completeness.
type GroupingDetail struct {
	model.ProductGrouping
	Products   []model.Product                     `json:"products"`
	Schedule   []model.ProductGroupingScheduleItem `json:"schedule"`
	EnComplete bool                                `json:"en_complete"`
}

func slugify(name string) string {
	slug := strings.Trim(groupingSlugRunRE.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if slug == "" {
		return "event"
	}
	return slug
}

func defaultGroupingPrefix(name string) *string {
	var b strings.Builder
	for _, r := range strings.ToUpper(name) {
		if r >= 'A' && r <= 'Z' {
			b.WriteRune(r)
		}
	}
	p := b.String()
	if len(p) > 4 {
		p = p[:4]
	}
	if len(p) < 2 {
		return nil
	}
	return &p
}

func blankToNil(p *string) *string {
	if p == nil || strings.TrimSpace(*p) == "" {
		return nil
	}
	return p
}

func validationf(format string, args ...any) error {
	return fmt.Errorf("%w: "+format, append([]any{ErrValidation}, args...)...)
}

func validateGroupingFields(g *model.ProductGrouping, tonesRequired bool) error {
	if g.ParticipantPrefix != nil && !groupingPrefixRE.MatchString(*g.ParticipantPrefix) {
		return validationf("participant_prefix must be 2 to 6 uppercase letters")
	}
	if g.Scope != nil && !slices.Contains(groupingScopes, *g.Scope) {
		return validationf("scope must be national or international")
	}
	if (tonesRequired || g.Tone != "") && !slices.Contains(groupingTones, g.Tone) {
		return validationf("tone must be one of %s", strings.Join(groupingTones, ", "))
	}
	if (tonesRequired || g.Illustration != "") && !slices.Contains(groupingIllustrate, g.Illustration) {
		return validationf("illustration must be one of %s", strings.Join(groupingIllustrate, ", "))
	}
	for k := range g.Content {
		if k != "id" && k != "en" {
			return validationf("content locale %q is not supported, use id or en", k)
		}
	}
	if g.OfficialURL != nil {
		u, err := url.Parse(*g.OfficialURL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return validationf("official_url must be an http(s) URL")
		}
	}
	return nil
}

func normalizeSchedule(items []model.ProductGroupingScheduleItem) ([]model.ProductGroupingScheduleItem, error) {
	out := make([]model.ProductGroupingScheduleItem, len(items))
	for i, it := range items {
		it.Label = strings.TrimSpace(it.Label)
		if it.Label == "" {
			return nil, validationf("schedule[%d].label is required", i)
		}
		it.LabelEN = blankToNil(it.LabelEN)
		it.EndsOn = blankToNil(it.EndsOn)
		start, err := time.Parse(time.DateOnly, it.StartsOn)
		if err != nil {
			return nil, validationf("schedule[%d].starts_on must be YYYY-MM-DD", i)
		}
		if it.EndsOn != nil {
			end, err := time.Parse(time.DateOnly, *it.EndsOn)
			if err != nil {
				return nil, validationf("schedule[%d].ends_on must be YYYY-MM-DD", i)
			}
			if end.Before(start) {
				return nil, validationf("schedule[%d].ends_on is before starts_on", i)
			}
		}
		out[i] = it
	}
	return out, nil
}

// groupingMissing lists what keeps an event from being visible (behavior 9).
func groupingMissing(g *model.ProductGrouping, schedule []model.ProductGroupingScheduleItem) []string {
	missing := []string{}
	if g.Status != model.ProductGroupingStatusPublished {
		missing = append(missing, "status")
	}
	if g.ParticipantPrefix == nil {
		missing = append(missing, "participant_prefix")
	}
	for _, loc := range []string{"id", "en"} {
		n := g.Content[loc]
		if strings.TrimSpace(n.Tagline) == "" {
			missing = append(missing, loc+".tagline")
		}
		if !slices.ContainsFunc(n.Story, func(p string) bool { return strings.TrimSpace(p) != "" }) {
			missing = append(missing, loc+".story")
		}
	}
	for i, it := range schedule {
		if it.LabelEN == nil || strings.TrimSpace(*it.LabelEN) == "" {
			missing = append(missing, fmt.Sprintf("schedule[%d].label_en", i))
		}
	}
	return missing
}

func groupingEnComplete(g *model.ProductGrouping, schedule []model.ProductGroupingScheduleItem) bool {
	for _, m := range groupingMissing(g, schedule) {
		if strings.HasPrefix(m, "en.") || strings.HasPrefix(m, "schedule[") {
			return false
		}
	}
	return true
}

func (s *Service) AdminCreateProductGrouping(ctx context.Context, g model.ProductGrouping) (model.ProductGrouping, error) {
	g.Name = strings.TrimSpace(g.Name)
	if g.Name == "" {
		return model.ProductGrouping{}, fmt.Errorf("%w: name is required", ErrValidation)
	}
	g.ShortName, g.Scope, g.LogoKey, g.OfficialURL, g.ParticipantPrefix =
		blankToNil(g.ShortName), blankToNil(g.Scope), blankToNil(g.LogoKey), blankToNil(g.OfficialURL), blankToNil(g.ParticipantPrefix)
	if err := validateGroupingFields(&g, false); err != nil {
		return model.ProductGrouping{}, err
	}
	g.Status = model.ProductGroupingStatusDraft
	g.IsVisible = false
	derivedPrefix := g.ParticipantPrefix == nil
	if derivedPrefix {
		basis := g.Name
		if g.ShortName != nil {
			basis = *g.ShortName
		}
		g.ParticipantPrefix = defaultGroupingPrefix(basis)
	}
	base := slugify(g.Name)
	for n := 1; n < 100; n++ {
		g.Slug = base
		if n > 1 {
			g.Slug = fmt.Sprintf("%s-%d", base, n)
		}
		err := s.storeRepo.CreateProductGrouping(ctx, &g)
		switch {
		case errors.Is(err, repository.ErrGroupingSlugTaken):
		case errors.Is(err, repository.ErrGroupingPrefixTaken) && derivedPrefix:
			g.ParticipantPrefix, derivedPrefix = nil, false
			n--
		case err != nil:
			return model.ProductGrouping{}, err
		default:
			return g, nil
		}
	}
	return model.ProductGrouping{}, repository.ErrGroupingSlugTaken
}

// AdminGetProductGrouping returns a grouping and every one of its member
// products, regardless of status, for the admin management page.
func (s *Service) AdminGetProductGrouping(ctx context.Context, id string) (model.ProductGrouping, []model.Product, error) {
	g, err := s.storeRepo.GetProductGroupingByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ProductGrouping{}, nil, ErrProductGroupingNotFound
		}
		return model.ProductGrouping{}, nil, err
	}
	products, err := s.storeRepo.ListProductsByGrouping(ctx, id)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	return *g, products, nil
}

func (s *Service) AdminGetProductGroupingDetail(ctx context.Context, id string) (GroupingDetail, error) {
	g, products, err := s.AdminGetProductGrouping(ctx, id)
	if err != nil {
		return GroupingDetail{}, err
	}
	schedule, err := s.storeRepo.ListGroupingSchedule(ctx, id)
	if err != nil {
		return GroupingDetail{}, err
	}
	return GroupingDetail{ProductGrouping: g, Products: products, Schedule: schedule, EnComplete: groupingEnComplete(&g, schedule)}, nil
}

// AdminUpdateProductGrouping overlays only the fields the caller set. The
// grouping, a replaced schedule, and the card-cache clear on a prefix change
// commit together.
func (s *Service) AdminUpdateProductGrouping(ctx context.Context, id string, p ProductGroupingPatch) (model.ProductGrouping, error) {
	existing, err := s.storeRepo.GetProductGroupingByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ProductGrouping{}, ErrProductGroupingNotFound
		}
		return model.ProductGrouping{}, err
	}
	oldPrefix := existing.ParticipantPrefix
	if p.Name != nil {
		if strings.TrimSpace(*p.Name) == "" {
			return model.ProductGrouping{}, fmt.Errorf("%w: name is required", ErrValidation)
		}
		existing.Name = strings.TrimSpace(*p.Name)
	}
	if p.Description != nil {
		existing.Description = *p.Description
	}
	if p.Status != nil {
		if *p.Status != model.ProductGroupingStatusDraft && *p.Status != model.ProductGroupingStatusPublished {
			return model.ProductGrouping{}, fmt.Errorf("%w: status must be draft or published", ErrValidation)
		}
		existing.Status = *p.Status
	}
	if p.ShortName != nil {
		existing.ShortName = blankToNil(p.ShortName)
	}
	if p.Scope != nil {
		existing.Scope = blankToNil(p.Scope)
	}
	if p.Subjects != nil {
		existing.Subjects = *p.Subjects
	}
	if p.Tone != nil {
		existing.Tone = *p.Tone
	}
	if p.Illustration != nil {
		existing.Illustration = *p.Illustration
	}
	if p.LogoKey != nil {
		existing.LogoKey = blankToNil(p.LogoKey)
	}
	if p.OfficialURL != nil {
		existing.OfficialURL = blankToNil(p.OfficialURL)
	}
	if p.Content != nil {
		existing.Content = *p.Content
	}
	if p.ParticipantPrefix != nil {
		existing.ParticipantPrefix = blankToNil(p.ParticipantPrefix)
	}
	if p.IsVisible != nil {
		existing.IsVisible = *p.IsVisible
	}
	if existing.Status == model.ProductGroupingStatusDraft && (p.IsVisible == nil || !*p.IsVisible) {
		existing.IsVisible = false
	}
	if err := validateGroupingFields(existing, true); err != nil {
		return model.ProductGrouping{}, err
	}

	var schedule []model.ProductGroupingScheduleItem
	if p.Schedule != nil {
		if schedule, err = normalizeSchedule(*p.Schedule); err != nil {
			return model.ProductGrouping{}, err
		}
	}
	if existing.IsVisible {
		current := schedule
		if p.Schedule == nil {
			if current, err = s.storeRepo.ListGroupingSchedule(ctx, id); err != nil {
				return model.ProductGrouping{}, err
			}
		}
		if missing := groupingMissing(existing, current); len(missing) > 0 {
			return model.ProductGrouping{}, &GroupingIncompleteError{Missing: missing}
		}
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.ProductGrouping{}, err
	}
	defer tx.Rollback(ctx)
	if err := s.storeRepo.UpdateProductGroupingTx(ctx, tx, id, existing); err != nil {
		return model.ProductGrouping{}, err
	}
	if p.Schedule != nil {
		if err := s.storeRepo.ReplaceGroupingScheduleTx(ctx, tx, id, schedule); err != nil {
			return model.ProductGrouping{}, err
		}
	}
	if !samePrefix(oldPrefix, existing.ParticipantPrefix) {
		examIDs, err := s.storeRepo.ListGroupingExamIDsTx(ctx, tx, id)
		if err != nil {
			return model.ProductGrouping{}, err
		}
		if err := s.clearCardsTx(ctx, tx, examIDs); err != nil {
			return model.ProductGrouping{}, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return model.ProductGrouping{}, err
	}
	return *existing, nil
}

func samePrefix(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func (s *Service) clearCardsTx(ctx context.Context, tx pgx.Tx, examIDs []uuid.UUID) error {
	for _, examID := range examIDs {
		if err := s.storeRepo.ClearRegistrationCardsByExamTx(ctx, tx, examID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) AdminDeleteProductGrouping(ctx context.Context, id string) error {
	if _, err := s.storeRepo.GetProductGroupingByID(ctx, id); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return ErrProductGroupingNotFound
		}
		return err
	}
	n, err := s.storeRepo.CountGroupingProducts(ctx, id)
	if err != nil {
		return err
	}
	if n > 0 {
		return ErrGroupingHasProducts
	}
	return s.storeRepo.DeleteProductGrouping(ctx, id)
}

func (s *Service) AdminReorderProductGroupings(ctx context.Context, groupingIDs []string) error {
	ids, err := parseUUIDs(groupingIDs)
	if err != nil {
		return err
	}
	return s.storeRepo.ReorderProductGroupings(ctx, ids)
}

// AdminSetGroupingProducts replaces a grouping's member products in one
// transaction: every product named gets this grouping with grouping_order
// set to its position in productIDs; every product previously in the
// grouping but left out is unassigned.
func (s *Service) AdminSetGroupingProducts(ctx context.Context, groupingID string, productIDs []string) (model.ProductGrouping, []model.Product, error) {
	gID, err := parseUUID(groupingID)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	if _, err := s.storeRepo.GetProductGroupingByID(ctx, groupingID); err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return model.ProductGrouping{}, nil, ErrProductGroupingNotFound
		}
		return model.ProductGrouping{}, nil, err
	}
	ids, err := parseUUIDs(productIDs)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}

	tx, err := s.storeRepo.BeginTx(ctx)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	defer tx.Rollback(ctx)

	before, err := s.storeRepo.ListGroupingProductIDsTx(ctx, tx, gID)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	if err := s.storeRepo.ReplaceGroupingProducts(ctx, tx, gID, ids); err != nil {
		return model.ProductGrouping{}, nil, err
	}
	changed := symmetricDifference(before, ids)
	examIDs, err := s.storeRepo.ListProductExamIDsTx(ctx, tx, changed)
	if err != nil {
		return model.ProductGrouping{}, nil, err
	}
	if err := s.clearCardsTx(ctx, tx, examIDs); err != nil {
		return model.ProductGrouping{}, nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return model.ProductGrouping{}, nil, err
	}
	return s.AdminGetProductGrouping(ctx, groupingID)
}

func symmetricDifference(a, b []uuid.UUID) []uuid.UUID {
	inA, inB := map[uuid.UUID]bool{}, map[uuid.UUID]bool{}
	for _, id := range a {
		inA[id] = true
	}
	for _, id := range b {
		inB[id] = true
	}
	var out []uuid.UUID
	for id := range inA {
		if !inB[id] {
			out = append(out, id)
		}
	}
	for id := range inB {
		if !inA[id] {
			out = append(out, id)
		}
	}
	return out
}

func parseUUIDs(ids []string) ([]uuid.UUID, error) {
	parsed := make([]uuid.UUID, 0, len(ids))
	for _, id := range ids {
		u, err := parseUUID(id)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, u)
	}
	return parsed, nil
}
