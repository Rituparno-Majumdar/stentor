// Package platforms holds one file per social platform, each implementing
// the Platform interface below. Nothing in this package talks to a
// database or runs as a daemon -- every call is a single outbound HTTP
// request, made and done.
package platforms

import (
	"bufio"
	"context"

	"github.com/Rituparno-Majumdar/stentor/internal/config"
	"github.com/Rituparno-Majumdar/stentor/internal/content"
)

// Tier describes who/what gates getting credentials for a platform. It is
// purely informational -- it never disables a platform's own code.
type Tier int

const (
	// TierFrictionless: no developer app, no review, by anyone.
	TierFrictionless Tier = iota
	// TierSelfServe: register your own app, usually approved instantly.
	TierSelfServe
	// TierPlatformReview: register your own app, then wait on that
	// platform's own (sometimes slow, sometimes unpredictable) review.
	TierPlatformReview
)

func (t Tier) String() string {
	switch t {
	case TierFrictionless:
		return "frictionless"
	case TierSelfServe:
		return "self-serve"
	case TierPlatformReview:
		return "platform review required"
	default:
		return "unknown"
	}
}

// Platform is what every social integration implements. Authenticate
// drives `stentor auth <platform>`; Post drives `stentor post`.
type Platform interface {
	Name() string
	Tier() Tier
	// FrictionNote explains, in one sentence, exactly what gates this
	// platform's credentials and who is responsible for clearing it.
	FrictionNote() string
	IsAuthenticated(cfg *config.Store) bool
	Authenticate(ctx context.Context, cfg *config.Store, in *bufio.Reader) error
	Post(ctx context.Context, cfg *config.Store, c content.Content) (content.Result, error)
}

// All returns every registered platform, in a stable, documented order
// (tier, then alphabetical) -- used by `stentor platforms` and `--platform all`.
func All() []Platform {
	return []Platform{
		// Tier 1 -- frictionless.
		NewBluesky(),
		NewDevto(),
		NewDiscord(),
		NewHashnode(),
		NewLemmy(),
		NewMastodon(),
		NewTelegram(),
		// Tier 2 -- self-serve app registration.
		NewLinkedIn(),
		NewReddit(),
		NewTwitter(),
		// Tier 3 -- self-serve app registration + platform-side review.
		NewFacebook(),
		NewInstagram(),
		NewThreads(),
		NewTikTok(),
		NewYouTube(),
	}
}

// ByName looks up one platform by its Name(), case-sensitive lowercase.
func ByName(name string) Platform {
	for _, p := range All() {
		if p.Name() == name {
			return p
		}
	}
	return nil
}
