package kannon

import (
	trackingtypes "github.com/kannon-email/kannon.go/internal/proto/kannon/tracking/types"
)

// TrackingMode states how much of one engagement channel may be observed.
//
// The scale is ordered by increasing collection: TrackingModeOff collects
// nothing and TrackingModeFull collects the most. A policy stated closer to
// the recipient may only restrict what the level above allows, never widen it.
//
// The zero value ("") states nothing and imposes no restriction of its own,
// leaving the domain ceiling configured on the server to decide.
type TrackingMode string

const (
	// TrackingModeOff means the channel is not observed at all.
	TrackingModeOff TrackingMode = "off"
	// TrackingModeAnonymous means the channel is counted in aggregate only;
	// nothing is retained that isolates one recipient.
	TrackingModeAnonymous TrackingMode = "anonymous"
	// TrackingModePseudonymous means linkable within a single batch, carrying
	// no recipient identity.
	//
	// Deprecated: reserved by the server; selecting it rejects the recipient
	// with reason RejectionReasonUnsupportedTrackingMode.
	TrackingModePseudonymous TrackingMode = "pseudonymous"
	// TrackingModeIdentified means engagement is attributed to the recipient.
	TrackingModeIdentified TrackingMode = "identified"
	// TrackingModeFull means engagement is attributed to the recipient, plus
	// the IP address and user agent of the request.
	TrackingModeFull TrackingMode = "full"
)

// TrackingPolicy is a pair of tracking modes, one governing opens and one
// governing links.
//
// It can be stated per batch (via WithTracking) and per recipient (via
// Recipient.Tracking). An omitted axis (the zero value "") states nothing and
// imposes no restriction of its own, leaving the domain ceiling configured on
// the server to decide.
//
// A batch asking for more than its domain allows fails the whole call; a
// recipient asking for more is rejected on its own, with reason
// RejectionReasonTrackingAboveCeiling, while the rest of the batch proceeds.
type TrackingPolicy struct {
	// Opens states how opens may be observed.
	Opens TrackingMode
	// Links states how link clicks may be observed.
	Links TrackingMode
}

var trackingModes = map[TrackingMode]trackingtypes.TrackingMode{
	TrackingModeOff:          trackingtypes.TrackingMode_TRACKING_MODE_OFF,
	TrackingModeAnonymous:    trackingtypes.TrackingMode_TRACKING_MODE_ANONYMOUS,
	TrackingModePseudonymous: trackingtypes.TrackingMode_TRACKING_MODE_PSEUDONYMOUS,
	TrackingModeIdentified:   trackingtypes.TrackingMode_TRACKING_MODE_IDENTIFIED,
	TrackingModeFull:         trackingtypes.TrackingMode_TRACKING_MODE_FULL,
}

func (m TrackingMode) toProto() trackingtypes.TrackingMode {
	return trackingModes[m] // absent (including "") maps to UNSPECIFIED
}

// toProto maps a policy to its proto message. A nil policy states nothing and
// leaves the proto field absent; a non-nil policy is always sent, with each
// empty axis as UNSPECIFIED.
func (p *TrackingPolicy) toProto() *trackingtypes.TrackingPolicy {
	if p == nil {
		return nil
	}
	return &trackingtypes.TrackingPolicy{
		Opens: p.Opens.toProto(),
		Links: p.Links.toProto(),
	}
}
