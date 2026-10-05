package models

// GORILLA OVERRIDE (2026-10-05): the Gemini API-key list is FETCHED too.
//
// It was the last typed list. /update ended with the line "Gemini ships with
// the app and updates with it", which is a polite way of saying it does not
// update. Measured that day with the owner's key: Google listed 44 models that
// accept generateContent, among them gemini-3.6-flash, 3.7-flash and 3.8-flash;
// this program offered the fourteen somebody typed in.
//
// The typed entries are KEPT where Google still lists them: they carry prices
// and notes a listing does not. What the fetch does is add what is missing,
// correct the token limits from the wire, and remove what Google has stopped
// serving (mapping it forward, so a config that names it keeps working).
//
// Ids stay flat ("gemini-3.8-flash"), as the typed ones are: an existing
// config must not stop resolving because its list was refreshed.

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const geminiListURL = "https://generativelanguage.googleapis.com/v1beta/models?pageSize=1000"

type geminiListed struct {
	Name             string   `json:"name"` // "models/gemini-3.8-flash"
	DisplayName      string   `json:"displayName"`
	Description      string   `json:"description"`
	InputTokenLimit  int64    `json:"inputTokenLimit"`
	OutputTokenLimit int64    `json:"outputTokenLimit"`
	Methods          []string `json:"supportedGenerationMethods"`
	Thinking         bool     `json:"thinking"`
}

type cachedGemini struct {
	Refreshed time.Time      `json:"refreshed"`
	Listed    []geminiListed `json:"listed"`
}

func geminiCachePath(dir string) string { return filepath.Join(dir, "gemini-models.json") }

// geminiNotChat are words in an id that mark a model which is not a text chat
// model: speech, pictures, transcription, robots, screen control, embeddings.
var geminiNotChat = []string{
	"tts", "image", "transcribe", "robotics", "computer-use", "embedding", "aqa", "audio", "live", "veo", "imagen",
}

func (g geminiListed) id() string { return strings.TrimPrefix(g.Name, "models/") }

func (g geminiListed) usable() bool {
	id := g.id()
	if !strings.HasPrefix(id, "gemini-") {
		return false
	}
	ok := false
	for _, m := range g.Methods {
		ok = ok || m == "generateContent"
	}
	if !ok {
		return false
	}
	for _, w := range geminiNotChat {
		if strings.Contains(id, w) {
			return false
		}
	}
	return true
}

// GeminiRefreshResult reports what changed.
type GeminiRefreshResult struct {
	Fetched, Usable int
	Added, Removed  []string
}

// FetchGeminiList asks Google what this key may use. The key travels in a
// header, never in the address.
func FetchGeminiList(apiKey string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, geminiListURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("x-goog-api-key", apiKey)
	resp, err := chatProbeHTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("could not reach Google's model list")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// The body is not quoted: an auth error can repeat the key.
		return nil, fmt.Errorf("Google answered HTTP %d to the model list request", resp.StatusCode)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// RefreshGemini parses a fetched listing, applies it and caches it.
func RefreshGemini(cacheDir string, raw []byte) (*GeminiRefreshResult, error) {
	var in struct {
		Models []geminiListed `json:"models"`
	}
	if err := json.Unmarshal(raw, &in); err != nil {
		return nil, fmt.Errorf("Google's model list was unreadable: %w", err)
	}
	res := &GeminiRefreshResult{Fetched: len(in.Models)}
	var usable []geminiListed
	for _, g := range in.Models {
		if g.usable() {
			usable = append(usable, g)
		}
	}
	res.Usable = len(usable)
	if res.Usable == 0 {
		// An empty list is not success: applying it would empty the picker.
		return nil, fmt.Errorf("Google listed %d models, none of them a chat model", res.Fetched)
	}
	res.Added, res.Removed = applyGemini(usable)

	blob, err := json.MarshalIndent(cachedGemini{Refreshed: time.Now().UTC(), Listed: usable}, "", " ")
	if err == nil && os.MkdirAll(cacheDir, 0o755) == nil {
		tmp := geminiCachePath(cacheDir) + ".tmp"
		if os.WriteFile(tmp, blob, 0o644) == nil {
			_ = os.Rename(tmp, geminiCachePath(cacheDir))
		}
	}
	return res, nil
}

// LoadRefreshedGemini applies the cached listing at startup. Disk only.
func LoadRefreshedGemini(cacheDir string) int {
	blob, err := os.ReadFile(geminiCachePath(cacheDir))
	if err != nil {
		return 0
	}
	var c cachedGemini
	if json.Unmarshal(blob, &c) != nil || len(c.Listed) == 0 {
		return 0
	}
	applyGemini(c.Listed)
	return len(c.Listed)
}

// GeminiCatalogueAge reports when the list was last fetched.
func GeminiCatalogueAge(cacheDir string) (time.Duration, bool) {
	blob, err := os.ReadFile(geminiCachePath(cacheDir))
	if err != nil {
		return 0, false
	}
	var c cachedGemini
	if json.Unmarshal(blob, &c) != nil || c.Refreshed.IsZero() {
		return 0, false
	}
	return time.Since(c.Refreshed), true
}

func applyGemini(listed []geminiListed) (added, removed []string) {
	want := map[ModelID]geminiListed{}
	for _, g := range listed {
		want[ModelID(g.id())] = g
	}
	func() {
		SupportedModels, _, commit := beginRegistryEdit()
		defer commit()
		for id, m := range SupportedModels {
			if m.Provider != ProviderGemini {
				continue
			}
			if _, still := want[id]; !still {
				delete(SupportedModels, id)
				removed = append(removed, string(id))
			}
		}
		for id, g := range want {
			m, known := SupportedModels[id]
			if !known || m.Provider != ProviderGemini {
				name := g.DisplayName
				if name == "" {
					name = g.id()
				}
				m = Model{
					ID: id, Name: name, Provider: ProviderGemini, APIModel: g.id(),
					Description:         firstSentence(g.Description),
					SupportsAttachments: true,
				}
				added = append(added, string(id))
			}
			// The limits come from the wire for typed and fetched entries alike.
			if g.InputTokenLimit > 0 {
				m.ContextWindow = g.InputTokenLimit
			}
			if m.ContextWindow == 0 {
				m.ContextWindow = 32768
			}
			if g.OutputTokenLimit > 0 {
				m.DefaultMaxTokens = min(g.OutputTokenLimit, 50000, m.ContextWindow/2)
			}
			if m.DefaultMaxTokens == 0 {
				m.DefaultMaxTokens = min(m.ContextWindow/2, 8192)
			}
			m.CanReason = m.CanReason || g.Thinking
			SupportedModels[id] = m
			delete(LegacyModelIDs, id)
		}
	}()
	// A removed model is mapped to the rolling Flash alias, which Google keeps
	// pointing at something current, if that alias is itself still listed.
	if _, ok := want[GeminiFlashLatest]; ok {
		for _, id := range removed {
			LegacyModelIDs[ModelID(id)] = GeminiFlashLatest
		}
	}
	sort.Strings(added)
	sort.Strings(removed)
	return added, removed
}

// firstSentence keeps a vendor description to one line a picker can show.
func firstSentence(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if i := strings.Index(s, ". "); i > 0 && i < 160 {
		return s[:i+1]
	}
	if len(s) > 160 {
		return s[:157] + "..."
	}
	return s
}
