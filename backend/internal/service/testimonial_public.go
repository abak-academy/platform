package service

import "context"

// PublicTestimonial carries only publishable fields; PhotoURL is the stored
// photo key, resolved by clients the way event-series logo_url is.
type PublicTestimonial struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	RoleLabel    string   `json:"role_label"`
	PhotoURL     string   `json:"photo_url"`
	Quote        string   `json:"quote"`
	Achievements []string `json:"achievements"`
}

func (s *Service) ListPublicTestimonials(ctx context.Context, lang string) ([]PublicTestimonial, error) {
	lang = normalizeLang(lang)
	items, err := s.storeRepo.ListVisibleTestimonials(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]PublicTestimonial, 0, len(items))
	for _, t := range items {
		c := t.Content[lang]
		p := PublicTestimonial{ID: t.ID, Name: t.Name, RoleLabel: t.RoleLabel, Quote: c.Quote, Achievements: nonNil(c.Achievements)}
		if t.PhotoKey != nil {
			p.PhotoURL = *t.PhotoKey
		}
		out = append(out, p)
	}
	return out, nil
}
