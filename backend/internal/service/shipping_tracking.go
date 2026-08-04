package service

import (
	"context"
	"sort"
	"time"
)

// TrackingEntry is one checkpoint on a parcel's journey.
type TrackingEntry struct {
	Status     string    `json:"status"`
	Note       string    `json:"note,omitempty"`
	OccurredAt time.Time `json:"occurred_at"`
	DriverName string    `json:"driver_name,omitempty"`
}

// TrackingView is everything the tracking dialog needs, in one payload.
//
// Source says where History came from: "courier" when the carrier's own scan
// log answered, "local" when it did not and we fell back to the events our
// webhook happened to record. The dialog shows the difference rather than
// presenting a thinner history as if it were the carrier's.
type TrackingView struct {
	Waybill string `json:"waybill"`
	Courier string `json:"courier"`
	Service string `json:"service"`
	Status  string `json:"status"`
	Source  string `json:"source"`

	History []TrackingEntry `json:"history"`
}

// GetOrderTracking assembles the tracking view for an order.
//
// It asks the courier first, through Biteship's track-any-waybill endpoint,
// which is the only way to get a scan log for an order shipped via the manual
// resi escape hatch — those have no Biteship booking at all, so GetOrder
// cannot answer for them and until now they had no tracking whatsoever.
//
// A failure there is not an error for the caller: the stored shipment events
// still describe the parcel, so the view degrades to those and says so.
func (s *Service) GetOrderTracking(ctx context.Context, orderID string) (TrackingView, error) {
	id, err := parseUUID(orderID)
	if err != nil {
		return TrackingView{}, err
	}

	order, err := s.storeRepo.GetOrderByID(ctx, id)
	if err != nil {
		return TrackingView{}, err
	}
	if order.ID.String() == "" {
		return TrackingView{}, ErrOrderNotFound
	}
	if order.TrackingNumber == "" {
		return TrackingView{}, ErrNoTrackingNumber
	}

	view := TrackingView{
		Waybill: order.TrackingNumber,
		Courier: order.SelectedCourier,
		Service: order.SelectedService,
		Source:  "local",
	}
	if order.ShipmentStatus != nil {
		view.Status = *order.ShipmentStatus
	}

	if order.CourierCode != nil && *order.CourierCode != "" {
		if tracking, tErr := s.logisticsClient().TrackWaybill(ctx, order.TrackingNumber, *order.CourierCode); tErr == nil && len(tracking.History) > 0 {
			view.Source = "courier"
			view.History = sortTrackingDesc(tracking.History)
			if tracking.Status != "" {
				view.Status = tracking.Status
			}
			return view, nil
		}
	}

	events, err := s.storeRepo.ListShipmentEvents(ctx, id)
	if err != nil {
		return TrackingView{}, err
	}
	for _, ev := range events {
		entry := TrackingEntry{Status: ev.Status, OccurredAt: ev.OccurredAt}
		if ev.CourierDriverName != nil {
			entry.DriverName = *ev.CourierDriverName
		}
		view.History = append(view.History, entry)
	}
	view.History = sortTrackingDesc(view.History)
	return view, nil
}

// sortTrackingDesc puts the newest checkpoint first — the order every
// Indonesian courier's own tracking page uses, and the order ShipmentTimeline
// already renders in.
func sortTrackingDesc(entries []TrackingEntry) []TrackingEntry {
	sort.SliceStable(entries, func(i, j int) bool {
		return entries[i].OccurredAt.After(entries[j].OccurredAt)
	})
	return entries
}
