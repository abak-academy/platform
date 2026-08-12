package service

import "strings"

// ShipmentFailureStatuses are the Biteship statuses that mean the parcel is
// not going to arrive without someone intervening.
//
// on_hold is deliberately absent: it resolves on its own, and surfacing it as
// a failure would train admins to ignore the signal.
//
// This matters because the webhook never walks orders.status back (FR-C-15) —
// an order whose courier vanished still reads "shipped" everywhere else, so
// this list is the only thing that can surface it.
var ShipmentFailureStatuses = []string{
	"rejected",
	"courier_not_found",
	"cancelled",
	"return_in_transit",
	"returned",
	"disposed",
}

// shipmentStatusSpellings expands a canonical snake_case status into every
// spelling that could be sitting in the column.
//
// We store whatever Biteship sends, verbatim and unnormalised. Their docs
// render these statuses camelCase while payloads appear to use snake_case, and
// most have never been observed here at all — so a filter that matched only
// one spelling would quietly return nothing for the other. Matching both is
// cheap; guessing is not.
func shipmentStatusSpellings(canonical string) []string {
	parts := strings.Split(canonical, "_")
	if len(parts) == 1 {
		return []string{canonical}
	}
	camel := parts[0]
	for _, p := range parts[1:] {
		if p == "" {
			continue
		}
		camel += strings.ToUpper(p[:1]) + p[1:]
	}
	return []string{canonical, camel}
}

// ShipmentFailureStatusValues is what the repository filter is given: every
// failure status in every spelling it might have been stored under.
func ShipmentFailureStatusValues() []string {
	var out []string
	for _, s := range ShipmentFailureStatuses {
		out = append(out, shipmentStatusSpellings(s)...)
	}
	return out
}

// isShipmentFailureStatus matches one stored shipment_status against the
// failure set, in either spelling. Built from ShipmentFailureStatusValues so a
// status added to the list is matched here without a second edit — the same
// reason the repository filter reads from it rather than its own literal.
// A nil or empty status is not a failure: it means nothing has been reported.
func isShipmentFailureStatus(status *string) bool {
	if status == nil || *status == "" {
		return false
	}
	for _, s := range ShipmentFailureStatusValues() {
		if s == *status {
			return true
		}
	}
	return false
}
