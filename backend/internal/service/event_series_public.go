package service

import (
	"context"
	"errors"
	"time"

	"akademi-bimbel/internal/model"
	"akademi-bimbel/internal/repository"
)

// PublicEdition is an open or live exam product. ImageURL is the stored
// product image reference; clients resolve keys the way they do for products.
type PublicEdition struct {
	ProductID      string     `json:"product_id"`
	ExamIDs        []string   `json:"exam_ids"`
	Title          string     `json:"title"`
	Stages         []string   `json:"stages"`
	GradeMin       *int       `json:"grade_min"`
	GradeMax       *int       `json:"grade_max"`
	Price          int64      `json:"price"`
	ImageURL       string     `json:"image_url"`
	Phase          string     `json:"phase"`
	ScheduledAt    *time.Time `json:"scheduled_at"`
	ScheduledEndAt *time.Time `json:"scheduled_end_at"`
	AvailableUntil *time.Time `json:"available_until"`
}

type PublicMilestone struct {
	Label    string  `json:"label"`
	StartsOn string  `json:"starts_on"`
	EndsOn   *string `json:"ends_on"`
	State    string  `json:"state"`
}

type PublicEvent struct {
	Slug         string                `json:"slug"`
	Name         string                `json:"name"`
	ShortName    *string               `json:"short_name"`
	Scope        *string               `json:"scope"`
	Subjects     []string              `json:"subjects"`
	Tone         string                `json:"tone"`
	Illustration string                `json:"illustration"`
	LogoURL      string                `json:"logo_url"`
	OfficialURL  *string               `json:"official_url"`
	FullName     string                `json:"full_name"`
	Tagline      string                `json:"tagline"`
	LevelsLabel  string                `json:"levels_label"`
	Story        []string              `json:"story"`
	Rounds       []string              `json:"rounds"`
	Categories   []model.EventCategory `json:"categories"`
	Facts        []model.EventFact     `json:"facts"`
	Schedule     []PublicMilestone     `json:"schedule"`
	OpenEditions []PublicEdition       `json:"open_editions"`
}

// GroupingEventSummary feeds the admin list and editor.
type GroupingEventSummary struct {
	Phases           map[string]string
	EditionCount     int
	OpenEditionCount int
	NextMilestone    *PublicMilestone
}

func normalizeLang(lang string) string {
	if lang == "en" {
		return "en"
	}
	return "id"
}

func (s *Service) ListPublicEventSeries(ctx context.Context, lang string) ([]PublicEvent, error) {
	return s.listPublicEventSeries(ctx, lang, time.Now())
}

func (s *Service) GetPublicEventSeries(ctx context.Context, slug, lang string) (*PublicEvent, error) {
	return s.getPublicEventSeries(ctx, slug, lang, time.Now())
}

func (s *Service) listPublicEventSeries(ctx context.Context, lang string, now time.Time) ([]PublicEvent, error) {
	groupings, err := s.storeRepo.ListVisibleProductGroupings(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PublicEvent, 0, len(groupings))
	for i := range groupings {
		ev, err := s.publicEvent(ctx, &groupings[i], lang, now)
		if err != nil {
			return nil, err
		}
		out = append(out, *ev)
	}
	return out, nil
}

func (s *Service) getPublicEventSeries(ctx context.Context, slug, lang string, now time.Time) (*PublicEvent, error) {
	g, err := s.storeRepo.GetProductGroupingBySlug(ctx, slug)
	if err != nil {
		if errors.Is(err, repository.ErrNotFound) {
			return nil, ErrProductGroupingNotFound
		}
		return nil, err
	}
	if !g.IsVisible || g.Status != model.ProductGroupingStatusPublished {
		return nil, ErrProductGroupingNotFound
	}
	return s.publicEvent(ctx, g, lang, now)
}

type editionInfo struct {
	examIDs []string
	sched   editionSchedule
	phase   string
}

// groupingEditions returns the grouping's exam products with their phase.
func (s *Service) groupingEditions(ctx context.Context, groupingID string, now time.Time) ([]model.Product, map[string]*editionInfo, error) {
	products, err := s.storeRepo.ListProductsByGrouping(ctx, groupingID)
	if err != nil {
		return nil, nil, err
	}
	exams, err := s.storeRepo.ListGroupingEditionExams(ctx, groupingID)
	if err != nil {
		return nil, nil, err
	}
	byProduct := map[string][]repository.EditionExam{}
	for _, x := range exams {
		byProduct[x.ProductID] = append(byProduct[x.ProductID], x)
	}
	editions := []model.Product{}
	info := map[string]*editionInfo{}
	for _, p := range products {
		if p.Type != "exam" {
			continue
		}
		in := &editionInfo{examIDs: []string{}, sched: aggregateEditionSchedule(byProduct[p.ID])}
		for _, x := range byProduct[p.ID] {
			in.examIDs = append(in.examIDs, x.ExamID)
		}
		in.phase = editionPhase(in.sched, productInWindow(p, now), now)
		info[p.ID] = in
		editions = append(editions, p)
	}
	return editions, info, nil
}

func (s *Service) publicEvent(ctx context.Context, g *model.ProductGrouping, lang string, now time.Time) (*PublicEvent, error) {
	lang = normalizeLang(lang)
	schedule, err := s.storeRepo.ListGroupingSchedule(ctx, g.ID)
	if err != nil {
		return nil, err
	}
	editions, info, err := s.groupingEditions(ctx, g.ID, now)
	if err != nil {
		return nil, err
	}

	n := g.Content[lang]
	ev := &PublicEvent{
		Slug: g.Slug, Name: g.Name, ShortName: g.ShortName, Scope: g.Scope, Subjects: nonNil(g.Subjects),
		Tone: g.Tone, Illustration: g.Illustration, OfficialURL: g.OfficialURL,
		FullName: n.FullName, Tagline: n.Tagline, LevelsLabel: n.LevelsLabel,
		Story: nonNil(n.Story), Rounds: nonNil(n.Rounds), Categories: nonNil(n.Categories), Facts: nonNil(n.Facts),
		Schedule: publicSchedule(schedule, lang, now), OpenEditions: []PublicEdition{},
	}
	if g.LogoKey != nil {
		ev.LogoURL = *g.LogoKey
	}
	for _, p := range editions {
		in := info[p.ID]
		if !isOpenEdition(p, in.phase, now) {
			continue
		}
		ev.OpenEditions = append(ev.OpenEditions, PublicEdition{
			ProductID: p.ID, ExamIDs: in.examIDs, Title: p.Name, Stages: nonNil(p.Stages),
			GradeMin: p.GradeMin, GradeMax: p.GradeMax, Price: p.Price, ImageURL: p.ImageURL,
			Phase: in.phase, ScheduledAt: in.sched.Start, ScheduledEndAt: in.sched.End, AvailableUntil: p.AvailableUntil,
		})
	}
	sortOpenEditions(ev.OpenEditions)
	return ev, nil
}

func publicSchedule(items []model.ProductGroupingScheduleItem, lang string, now time.Time) []PublicMilestone {
	today := jakartaDay(now)
	out := []PublicMilestone{}
	for _, it := range sortedSchedule(items) {
		label := it.Label
		if lang == "en" && it.LabelEN != nil && *it.LabelEN != "" {
			label = *it.LabelEN
		}
		out = append(out, PublicMilestone{Label: label, StartsOn: it.StartsOn, EndsOn: it.EndsOn, State: milestoneState(it, today)})
	}
	return out
}

func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// GroupingEventSummaryAt returns per-product phase, edition counts and the
// next not-done milestone (Indonesian label) for the admin list and editor.
func (s *Service) GroupingEventSummaryAt(ctx context.Context, groupingID string, now time.Time) (*GroupingEventSummary, error) {
	editions, info, err := s.groupingEditions(ctx, groupingID, now)
	if err != nil {
		return nil, err
	}
	schedule, err := s.storeRepo.ListGroupingSchedule(ctx, groupingID)
	if err != nil {
		return nil, err
	}
	sum := &GroupingEventSummary{Phases: map[string]string{}, EditionCount: len(editions)}
	for _, p := range editions {
		sum.Phases[p.ID] = info[p.ID].phase
		if isOpenEdition(p, info[p.ID].phase, now) {
			sum.OpenEditionCount++
		}
	}
	if item, state := nextMilestone(schedule, now); item != nil {
		sum.NextMilestone = &PublicMilestone{Label: item.Label, StartsOn: item.StartsOn, EndsOn: item.EndsOn, State: state}
	}
	return sum, nil
}
