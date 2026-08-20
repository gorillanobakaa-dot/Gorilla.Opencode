// GORILLA OVERRIDE: this file did not exist upstream. It bundles the network
// tunables into five named connection profiles the user can pick, instead of
// leaving them as undiscoverable environment variables.
//
// WHY THIS EXISTS. Every knob a profile sets already shipped — FirstByteTimeout,
// StreamStallTimeout, TurnUploadBudgetBytes, request gzip, the retry cap. They
// were measured against a deliberately broken link on 2026-08-18 and written up
// in docs/SATELLITE.md. What did not exist was any way to FIND them: each was an
// env var, so the person on a 2 KB/s Iridium link — exactly who this program is
// for — had to read the source to survive on it.
//
// A profile changes NETWORK BEHAVIOUR ONLY: how long to wait, how many times to
// retry, how many bytes a turn may spend. It deliberately does NOT touch the
// loadout, the tool set, or the model. Owner's decision, 2026-08-20, and it is
// the right one: a preset that silently changed what the agent could DO would
// make "switch profile" an unpredictable act, and switching back would not
// reliably restore what you had.
//
// PRECEDENCE, and it matters: an explicit environment variable always wins over
// the profile. Someone who set GORILLA_OPENCODE_FIRST_BYTE_TIMEOUT meant it, and
// a profile quietly overriding a deliberate choice is the same silent-failure
// class this whole subsystem exists to remove.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const connProfileFileName = "connection.json"

// ConnProfileID is the stored identifier. Kept short and stable — it is written
// to disk, so renaming one is a migration, not a cosmetic change.
type ConnProfileID string

const (
	ProfileAustere       ConnProfileID = "austere"
	ProfileConstrained   ConnProfileID = "constrained"
	ProfileModest        ConnProfileID = "modest"
	ProfileBroadband     ConnProfileID = "broadband"
	ProfileUnconstrained ConnProfileID = "unconstrained"
)

// ConnProfile is one fully-described operating point.
//
// Every duration here answers a different question, and they are easy to
// confuse:
//
//   - FirstByte  — how long to wait for the FIRST byte of an answer. On a slow
//     link the model may be fine and the wire merely slow; this must be
//     generous or a healthy answer is killed before it starts.
//   - StreamStall — how long a gap BETWEEN chunks may last once an answer has
//     begun. This is a stall timer, not a wall clock: it resets on every chunk,
//     so a stream that crawls is never killed, only one making no progress.
//   - UploadMB   — how many megabytes one turn (including its retries) may
//     spend. The whole conversation is re-uploaded every message, so this is
//     the knob that protects a metered allowance.
//   - MaxRetries — how many attempts before giving up and saying so.
type ConnProfile struct {
	ID     ConnProfileID
	Name   string
	Rate   string // human-readable link speed this is tuned for
	Links  string // real-world examples, so someone can recognise their own
	Layman string // what changes and why, no jargon

	FirstByte   time.Duration
	StreamStall time.Duration
	UploadMB    float64
	MaxRetries  int
	// Stream controls whether the reply arrives token-by-token (live typing) or
	// as one piece when it is finished.
	//
	// MEASURED 2026-08-20, same question, same model, same answer:
	//   streaming      22,256 bytes
	//   non-streaming     834 bytes   -- 27x less
	//
	// The cause is the transport, not the model. A streamed reply wraps EVERY
	// token in its own JSON envelope (id, model, object, index, delta,
	// finish_reason) around a payload of a few characters; a non-streamed reply
	// sends one envelope for the whole answer.
	//
	// TOKENS ARE IDENTICAL - 106 either way, verified from both usage blocks. So
	// this changes the DATA bill, not the provider bill. On a metered satellite
	// plan those are the same money; on a flat connection they are not, which is
	// why only the slow profiles turn it off.
	//
	// What it costs: the live typing, and the stall guard's progress signal (a
	// stalled link stops looking different from a slow answer, leaving
	// FirstByteTimeout to carry that job alone). What it does NOT cost: any
	// capability whatsoever. Same words, same tools, same quality.
	Stream bool
}

// NOTE on refusing over-budget turns (owner's decision, 2026-08-20): this is
// NOT a per-profile flag, because refusing before sending is ALREADY universal —
// budgetTransport in the provider layer checks every attempt against the budget
// and returns without putting bytes on the link. A per-profile switch here would
// be dead config that claims to control something it does not. What a profile
// actually sets is the LIMIT; Austere refuses sooner because its limit is 0.5 MB,
// not because it is flagged to.

// ConnProfiles is the ordered ladder, slowest first. Order is display order;
// a map would reshuffle between renders.
//
// The speed bands come from the owner's own survey of satellite and cellular
// tiers (2026-08-20), collapsed from ~25 rows into five operating points. The
// bands are what the NUMBERS were chosen against — they are not enforced, and
// nothing measures your actual link. Picking a slower profile than you need
// costs patience; picking a faster one than you have costs failed turns.
var ConnProfiles = []ConnProfile{
	{
		ID:    ProfileAustere,
		Name:  "Austere environment",
		Rate:  "1-9 KB/s",
		Links: "Iridium Short Burst, Inmarsat C, 2G CSD/GPRS",
		Layman: "For a satellite phone or an old 2G signal. Waits a very long " +
			"time before deciding the connection is dead, because on a link this " +
			"slow a real answer genuinely takes minutes. Retries almost never, " +
			"and refuses to send a message it thinks is too big rather than " +
			"quietly spending your allowance on it.",
		FirstByte: 15 * time.Minute, StreamStall: 5 * time.Minute,
		UploadMB: 0.5, MaxRetries: 2, Stream: false,
	},
	{
		ID:    ProfileConstrained,
		Name:  "Constrained",
		Rate:  "10-60 KB/s",
		Links: "Iridium Certus 100/200, EDGE, 1xRTT",
		Layman: "For a weak mobile signal or a low-end satellite plan. Still " +
			"patient, still careful with how much it uploads, but it will retry " +
			"a failed message rather than giving up immediately.",
		FirstByte: 8 * time.Minute, StreamStall: 3 * time.Minute,
		UploadMB: 1.5, MaxRetries: 3, Stream: false,
	},
	{
		ID:    ProfileModest,
		Name:  "Modest",
		Rate:  "60-250 KB/s",
		Links: "Inmarsat BGAN, Certus 700, UMTS, legacy GEO",
		Layman: "For a usable but unreliable connection. Roughly the shipped " +
			"behaviour, with more patience: long enough for a slow reasoning " +
			"model, short enough that a dead link is reported rather than hung on.",
		FirstByte: 4 * time.Minute, StreamStall: 2 * time.Minute,
		UploadMB: 4, MaxRetries: 4, Stream: true,
	},
	{
		ID:    ProfileBroadband,
		Name:  "Broadband",
		Rate:  "250 KB/s - 5 MB/s",
		Links: "HSPA+, EV-DO Rev A/B, early LTE",
		Layman: "For an ordinary home or mobile broadband connection. Waits a " +
			"couple of minutes, retries a few times, and does not police how much " +
			"it uploads unless something has clearly gone wrong.",
		FirstByte: 2 * time.Minute, StreamStall: 90 * time.Second,
		UploadMB: 8, MaxRetries: 5, Stream: true,
	},
	{
		ID:    ProfileUnconstrained,
		Name:  "Unconstrained",
		Rate:  "5 MB/s and up",
		Links: "LTE-Advanced, 5G, Starlink, modern GEO high-throughput",
		Layman: "For a fast, stable connection. Gives up quickly when something " +
			"is wrong, because on a link this good a long silence means a real " +
			"fault rather than a slow wire.",
		FirstByte: 60 * time.Second, StreamStall: 45 * time.Second,
		UploadMB: 16, MaxRetries: 5, Stream: true,
	},
}

// DefaultConnProfile is what ships. Modest, not Unconstrained: this program is
// built for bad connections, so the default must not assume a good one. Its
// numbers are deliberately close to the pre-profile shipped defaults (120s /
// 90s / 4 MB) so upgrading changes as little as possible for anyone who never
// opens the picker.
const DefaultConnProfile = ProfileModest

var (
	connProfileID   = DefaultConnProfile
	connProfileOnce sync.Once
	connProfileMu   sync.RWMutex
)

func connProfilePath() string {
	return filepath.Join(ConfigBase(), connProfileFileName)
}

func initConnProfile() {
	connProfileOnce.Do(func() {
		// One struct owns connection.json (linkFile, in linkspeed.go). Reading it
		// through a second shape is how a field gets dropped on the next write.
		f := readLinkFile()
		if _, ok := lookupConnProfile(ConnProfileID(f.Profile)); !ok {
			// An unknown id means a downgrade, a hand-edit, or a future version.
			// Keep the default rather than inventing behaviour from a name we do
			// not understand.
			return
		}
		connProfileMu.Lock()
		connProfileID = ConnProfileID(f.Profile)
		connProfileMu.Unlock()
	})
}

func lookupConnProfile(id ConnProfileID) (ConnProfile, bool) {
	for _, p := range ConnProfiles {
		if p.ID == id {
			return p, true
		}
	}
	return ConnProfile{}, false
}

// CurrentConnProfile returns the active profile, always a real one.
func CurrentConnProfile() ConnProfile {
	initConnProfile()
	connProfileMu.RLock()
	id := connProfileID
	connProfileMu.RUnlock()
	p, ok := lookupConnProfile(id)
	if !ok {
		p, _ = lookupConnProfile(DefaultConnProfile)
	}
	return p
}

// SetConnProfile switches profile and persists it. An unknown id is refused
// rather than silently ignored — a typo in a config file should say so.
func SetConnProfile(id ConnProfileID) error {
	if _, ok := lookupConnProfile(id); !ok {
		return fmt.Errorf("unknown connection profile %q", id)
	}
	initConnProfile()
	connProfileMu.Lock()
	connProfileID = id
	connProfileMu.Unlock()
	return saveConnProfile(id)
}

// saveConnProfile writes through the SHARED file struct rather than its own.
// connection.json also holds the link-speed samples and the "user has chosen"
// flag (linkspeed.go); marshalling a profile-only struct over it would silently
// erase both, and the symptom — the picker volunteering itself again after every
// change — would look like a trigger bug rather than a lost field.
func saveConnProfile(id ConnProfileID) error {
	f := readLinkFile()
	f.Profile = string(id)
	return writeLinkFile(f)
}

// ConnProfileNames returns the display names in ladder order, for a KindEnum row.
func ConnProfileNames() []string {
	out := make([]string, 0, len(ConnProfiles))
	for _, p := range ConnProfiles {
		out = append(out, p.Name)
	}
	return out
}

// ConnProfileByName maps a display name back to its id, for the settings dialog.
func ConnProfileByName(name string) (ConnProfileID, bool) {
	for _, p := range ConnProfiles {
		if p.Name == name {
			return p.ID, true
		}
	}
	return "", false
}

// connProfileNameByID is the Default for the settings row: the dialog compares
// against display names, so the default must be one too.
func connProfileNameByID(id ConnProfileID) string {
	if p, ok := lookupConnProfile(id); ok {
		return p.Name
	}
	return ""
}

// StreamRepliesEnabled reports whether replies should arrive token-by-token.
// GORILLA_OPENCODE_STREAM=0/1 overrides the profile, because someone who sets
// it meant it.
func StreamRepliesEnabled() bool {
	if v := os.Getenv("GORILLA_OPENCODE_STREAM"); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return CurrentConnProfile().Stream
}

// SwitchSummary is what to tell the user after they change profile.
//
// GORILLA OVERRIDE (2026-08-20): the old confirmation said only "Connection
// profile updated". The owner switched from austere to unconstrained mid-session
// and the replies visibly changed behaviour — one arriving whole, the next
// typing itself out — with nothing on screen connecting the two. A setting that
// silently changes how answers ARRIVE has to say so at the moment it changes,
// not only on the screen where it was picked.
//
// It names the direction of the cost rather than only the new state, because
// "Broadband" means nothing on its own; "waits less, allows more data, types
// live" is the thing the user actually experiences.
func SwitchSummary(from, to ConnProfileID) string {
	f, okF := lookupConnProfile(from)
	t, okT := lookupConnProfile(to)
	if !okT {
		return "Connection profile updated."
	}
	delivery := "answers type out a word at a time"
	if !t.Stream {
		delivery = "answers arrive whole, in one piece - about 27x less data"
	}
	base := fmt.Sprintf("Now on %s (%s): waits up to %s, allows %.1f MB per message, and %s.",
		t.Name, t.Rate, t.FirstByte, t.UploadMB, delivery)

	if !okF || from == to {
		return base
	}
	// Say which way the cost moved. profileIndex is 0 = slowest.
	fi, ti := profileIndex(from), profileIndex(to)
	switch {
	case ti > fi && f.Stream != t.Stream:
		return base + " That is faster and less patient than " + f.Name +
			", and it will use more data than before because replies now stream."
	case ti > fi:
		return base + " That is faster and less patient than " + f.Name + ", and allows more data per message."
	case ti < fi && f.Stream != t.Stream:
		return base + " That is slower and more patient than " + f.Name +
			", and it will use less data than before because replies no longer stream."
	default:
		return base + " That is slower and more patient than " + f.Name + ", and allows less data per message."
	}
}
