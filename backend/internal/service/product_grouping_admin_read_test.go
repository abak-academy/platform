package service

import (
	"context"
	"testing"
	"time"

	"akademi-bimbel/internal/model"
)

func listItem(t *testing.T, items []GroupingListItem, id string) GroupingListItem {
	t.Helper()
	for _, it := range items {
		if it.ID == id {
			return it
		}
	}
	t.Fatalf("event %s missing from admin list", id)
	return GroupingListItem{}
}

func TestAdminListProductGroupings_CountsLogoAndNextMilestone(t *testing.T) {
	f := newVisibleEvent(t, true)
	now := wib(2026, 12, 1, 9, 0)
	liveStart, liveEnd := now.Add(-time.Hour), now.Add(time.Hour)
	later := wib(2026, 12, 20, 8, 0)
	doneStart, doneEnd := now.Add(-5*time.Hour), now.Add(-4*time.Hour)
	f.addEdition(t, editionSeed{name: "Live", exams: [][2]*time.Time{{&liveStart, &liveEnd}}})
	f.addEdition(t, editionSeed{name: "Later", exams: [][2]*time.Time{{&later, nil}}})
	f.addEdition(t, editionSeed{name: "Done", exams: [][2]*time.Time{{&doneStart, &doneEnd}}})
	f.addEdition(t, editionSeed{name: "Draft", status: "draft", exams: [][2]*time.Time{{&later, nil}}})
	f.addEdition(t, editionSeed{name: "Merch", productType: "merchandise"})
	end := "2026-12-03"
	f.addSchedule(t,
		model.ProductGroupingScheduleItem{Label: "Pendaftaran", StartsOn: "2026-11-01", EndsOn: &end},
		model.ProductGroupingScheduleItem{Label: "Babak Final", StartsOn: "2026-11-20", EndsOn: nil},
		model.ProductGroupingScheduleItem{Label: "Pengumuman", StartsOn: "2026-12-30"},
	)
	// "Pendaftaran" runs 11-01..12-03 so it is ongoing on 12-01; the done "Babak Final" (11-20) is skipped.
	got := listItem(t, mustList(t, f, now), f.g.ID)

	if got.LogoURL != "question/u/logo.png" {
		t.Errorf("LogoURL = %q, want the stored key", got.LogoURL)
	}
	if got.EditionCount != 4 || got.OpenEditionCount != 2 {
		t.Errorf("edition_count/open = %d/%d, want 4/2 (every exam member counts, non-exam does not; only the live and later published ones are open)", got.EditionCount, got.OpenEditionCount)
	}
	if got.NextMilestone == nil || got.NextMilestone.Label != "Pendaftaran" || got.NextMilestone.State != MilestoneOngoing || got.NextMilestone.EndsOn == nil || *got.NextMilestone.EndsOn != end {
		t.Errorf("next_milestone = %+v, want ongoing Pendaftaran", got.NextMilestone)
	}
}

func mustList(t *testing.T, f eventFixture, now time.Time) []GroupingListItem {
	t.Helper()
	items, err := f.svc.adminListProductGroupings(context.Background(), now)
	if err != nil {
		t.Fatalf("adminListProductGroupings: %v", err)
	}
	return items
}

func TestAdminListProductGroupings_DoneMilestoneSkippedAndNilWhenAllDone(t *testing.T) {
	f := newVisibleEvent(t, true)
	now := wib(2026, 12, 1, 9, 0)
	f.addSchedule(t,
		model.ProductGroupingScheduleItem{Label: "Lama", StartsOn: "2026-10-01"},
		model.ProductGroupingScheduleItem{Label: "Berikut", StartsOn: "2026-12-10"},
	)
	got := listItem(t, mustList(t, f, now), f.g.ID)
	if got.NextMilestone == nil || got.NextMilestone.Label != "Berikut" || got.NextMilestone.State != MilestoneUpcoming {
		t.Fatalf("next_milestone = %+v, want upcoming Berikut (done Lama skipped)", got.NextMilestone)
	}
	if got.EditionCount != 0 || got.OpenEditionCount != 0 {
		t.Errorf("counts = %d/%d, want 0/0", got.EditionCount, got.OpenEditionCount)
	}

	later := listItem(t, mustList(t, f, wib(2027, 1, 1, 9, 0)), f.g.ID)
	if later.NextMilestone != nil {
		t.Fatalf("next_milestone = %+v after every milestone is done, want nil", later.NextMilestone)
	}
}

func TestAdminGetProductGroupingDetail_PhaseAndMultiEvent(t *testing.T) {
	f := newVisibleEvent(t, true)
	other := newVisibleEvent(t, true)
	ctx := context.Background()
	now := wib(2026, 12, 1, 9, 0)
	liveStart, liveEnd := now.Add(-time.Hour), now.Add(time.Hour)
	later := wib(2026, 12, 20, 8, 0)
	doneStart, doneEnd := now.Add(-5*time.Hour), now.Add(-4*time.Hour)
	liveID := f.addEdition(t, editionSeed{name: "Live", exams: [][2]*time.Time{{&liveStart, &liveEnd}}})
	openID := f.addEdition(t, editionSeed{name: "Open", exams: [][2]*time.Time{{&later, nil}}})
	doneID := f.addEdition(t, editionSeed{name: "Done", exams: [][2]*time.Time{{&doneStart, &doneEnd}}})
	soloID := f.addEdition(t, editionSeed{name: "Solo", exams: [][2]*time.Time{{&later, nil}}})
	merchID := f.addEdition(t, editionSeed{name: "Merch", productType: "merchandise"})

	// a product in ANOTHER event selling Live's exam makes Live multi-event; one in the SAME event does not.
	shareExam := func(from string, to eventFixture, name string) {
		var pid string
		if err := to.repo.Pool().QueryRow(ctx, `INSERT INTO product (type, name, price, status, grouping_id) VALUES ('exam',$1,0,'published',$2) RETURNING id`, name, to.g.ID).Scan(&pid); err != nil {
			t.Fatal(err)
		}
		if _, err := to.repo.Pool().Exec(ctx, `INSERT INTO product_exam (product_id, exam_id) SELECT $1, exam_id FROM product_exam WHERE product_id = $2`, pid, from); err != nil {
			t.Fatal(err)
		}
	}
	shareExam(liveID, other, "Other event bundle")
	shareExam(openID, f, "Same event bundle")

	d, err := f.svc.adminGetProductGroupingDetail(ctx, f.g.ID, now)
	if err != nil {
		t.Fatalf("detail: %v", err)
	}
	if d.LogoURL != "question/u/logo.png" {
		t.Errorf("LogoURL = %q", d.LogoURL)
	}
	byID := map[string]GroupingMember{}
	for _, m := range d.Products {
		byID[m.ID] = m
	}
	wantPhase := map[string]string{liveID: PhaseLive, openID: PhaseOpen, doneID: PhaseDone, soloID: PhaseOpen}
	for id, want := range wantPhase {
		if got := byID[id].Phase; got == nil || *got != want {
			t.Errorf("phase of %s = %v, want %s", byID[id].Name, got, want)
		}
	}
	if byID[merchID].Phase != nil {
		t.Errorf("non-exam product phase = %v, want nil", *byID[merchID].Phase)
	}
	wantMulti := map[string]bool{liveID: true, openID: false, doneID: false, soloID: false, merchID: false}
	for id, want := range wantMulti {
		if byID[id].MultiEvent != want {
			t.Errorf("multi_event of %s = %v, want %v", byID[id].Name, byID[id].MultiEvent, want)
		}
	}
}
