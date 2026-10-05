package models

// GORILLA OVERRIDE (2026-10-05): ask a model whether it actually answers.
//
// A provider's model LIST is not evidence that a model works. Measured on the
// owner's NVIDIA key that day, every one of these sitting in the list or in
// this program's own typed ranking:
//
//	meta/llama-3.3-70b-instruct        HTTP 410  "reached its end of life on 2026-08-26"
//	deepseek-ai/deepseek-v4-pro        HTTP 410  (this program's rank 1)
//	z-ai/glm-5.2                       HTTP 410  (rank 2)
//	minimaxai/minimax-m3               HTTP 410  (rank 3)
//	nvidia/nemotron-3-ultra-550b-a55b  HTTP 500  (rank 5, and still listed)
//	openai/gpt-oss-20b                 HTTP 200  get_weather({"city":"Bucharest"})
//
// 26 of the 31 models ranked in metadata/nim.json were gone, and the model the
// portal started people on was whatever sorted first: "01-ai/yi-large".
//
// Those facts were found with a throwaway script. This file is that script,
// kept: one small request carrying one ordinary tool, sent to a model, and the
// answer recorded with its date. /update runs it; nothing here is typed.
//
// It is NEVER sent to a model server on this machine. A local runtime loads a
// model when asked for it — eighteen gigabytes and a minute, for a question
// nobody asked — so loopback endpoints are refused here, not left to callers.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Probe outcomes. Words, not codes, because they are shown to the user.
const (
	ProbeWorks      = "works"       // answered and called the tool
	ProbeNoToolCall = "no-tools"    // answered, did not call the tool: cannot drive this program
	ProbeGone       = "gone"        // 404 / 410: the provider no longer serves it
	ProbeRefused    = "key-refused" // 401 / 403
	ProbeNeedsPay   = "needs-credit"
	ProbeBusy       = "rate-limited" // 429: says nothing about the model
	ProbeError      = "provider-error"
	ProbeUnreached  = "unreachable"
	ProbeTimeout    = "no-answer-in-time" // listed, accepted the request, never replied
)

// ProbeVerdict is what one request found, and when.
type ProbeVerdict struct {
	Outcome string    `json:"outcome"`
	Status  int       `json:"status,omitempty"`
	Detail  string    `json:"detail,omitempty"`
	At      time.Time `json:"at"`
}

// Usable reports whether the model can drive this program. Anything unknown is
// NOT usable: a default must be something that was seen to work.
func (v ProbeVerdict) Usable() bool { return v.Outcome == ProbeWorks }

// Dead reports a verdict that will not change by waiting.
func (v ProbeVerdict) Dead() bool {
	return v.Outcome == ProbeGone || v.Outcome == ProbeNoToolCall
}

// Label is the short note the picker appends to a row.
func (v ProbeVerdict) Label() string {
	day := v.At.Format("2006-01-02")
	switch v.Outcome {
	case ProbeWorks:
		return "answered with a tool call on " + day
	case ProbeNoToolCall:
		return "answered WITHOUT calling the tool on " + day + ": cannot edit files here"
	case ProbeGone:
		return "RETIRED by the provider (checked " + day + ")"
	case ProbeRefused:
		return "your key was refused for this model on " + day
	case ProbeNeedsPay:
		return "needs paid credit (checked " + day + ")"
	// The next three say nothing lasting about the model. One of them sat on
	// a row as "provider error 500" while that model carried a whole
	// conversation the same evening, so each now says it may have passed.
	case ProbeBusy:
		return "rate-limited when tested on " + day + "; may work now"
	case ProbeError:
		return fmt.Sprintf("provider error %d when tested on %s; may work now", v.Status, day)
	case ProbeTimeout:
		return "did not answer within 45 seconds when tested on " + day + "; may work now"
	}
	return ""
}

var chatProbeHTTP = &http.Client{Timeout: 45 * time.Second}

// probeAllowLoopback exists for the tests alone: httptest can only listen on
// this machine, which is exactly where the probe refuses to go.
var probeAllowLoopback = false

// isLoopback reports an endpoint on this machine.
func isLoopback(baseURL string) bool {
	u, err := url.Parse(baseURL)
	if err != nil {
		return false
	}
	h := strings.ToLower(u.Hostname())
	return h == "localhost" || h == "::1" || strings.HasPrefix(h, "127.") || h == "0.0.0.0"
}

// probeRetryPause is how long ProbeChat waits before asking a second time
// after a 5xx. A variable so the tests do not sleep.
var probeRetryPause = 2 * time.Second

// ProbeChat asks a model one small tool-call question. A 5xx is the server
// failing, not the model: it is asked once more before that goes on record.
func ProbeChat(baseURL, apiKey, apiModel string) ProbeVerdict {
	v := probeChatOnce(baseURL, apiKey, apiModel)
	if v.Outcome == ProbeError && v.Status >= 500 {
		time.Sleep(probeRetryPause)
		v = probeChatOnce(baseURL, apiKey, apiModel)
	}
	return v
}

func probeChatOnce(baseURL, apiKey, apiModel string) ProbeVerdict {
	v := ProbeVerdict{At: time.Now().UTC()}
	if isLoopback(baseURL) && !probeAllowLoopback {
		v.Outcome, v.Detail = ProbeUnreached, "local runtimes are never probed: asking loads the model"
		return v
	}
	body, _ := json.Marshal(map[string]any{
		"model":      apiModel,
		"max_tokens": 200,
		"messages": []map[string]string{
			{"role": "user", "content": "What is the weather in Bucharest? Use the tool."},
		},
		"tools": []map[string]any{{
			"type": "function",
			"function": map[string]any{
				"name": "get_weather", "description": "Get the weather for a city",
				"parameters": map[string]any{
					"type":       "object",
					"properties": map[string]any{"city": map[string]string{"type": "string"}},
					"required":   []string{"city"},
				},
			},
		}},
	})
	req, err := http.NewRequest(http.MethodPost, strings.TrimRight(baseURL, "/")+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		v.Outcome, v.Detail = ProbeUnreached, err.Error()
		return v
	}
	req.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		req.Header.Set("Authorization", "Bearer "+apiKey)
	}
	resp, err := chatProbeHTTP.Do(req)
	if err != nil {
		// A deadline passing means the server took the request and this model
		// did not reply: measured on NVIDIA 2026-10-05, where one listed model
		// hung while its neighbour answered in under a second. That is a fact
		// about the model. Anything else (DNS, refused connection) is the line.
		if ne, ok := err.(interface{ Timeout() bool }); ok && ne.Timeout() {
			v.Outcome = ProbeTimeout
			return v
		}
		v.Outcome, v.Detail = ProbeUnreached, "no answer"
		return v
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256<<10))
	v.Status = resp.StatusCode
	switch {
	case resp.StatusCode == http.StatusOK:
		var out struct {
			Choices []struct {
				Message struct {
					ToolCalls []struct {
						Function struct {
							Name string `json:"name"`
						} `json:"function"`
					} `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if json.Unmarshal(raw, &out) == nil && len(out.Choices) > 0 &&
			len(out.Choices[0].Message.ToolCalls) > 0 &&
			out.Choices[0].Message.ToolCalls[0].Function.Name == "get_weather" {
			v.Outcome = ProbeWorks
		} else {
			v.Outcome = ProbeNoToolCall
		}
	case resp.StatusCode == http.StatusGone, resp.StatusCode == http.StatusNotFound:
		v.Outcome = ProbeGone
	case resp.StatusCode == http.StatusUnauthorized, resp.StatusCode == http.StatusForbidden:
		v.Outcome = ProbeRefused
	case resp.StatusCode == http.StatusPaymentRequired:
		v.Outcome = ProbeNeedsPay
	case resp.StatusCode == http.StatusTooManyRequests:
		v.Outcome = ProbeBusy
	default:
		v.Outcome = ProbeError
	}
	// Never keep the response body: some providers repeat the key back in an
	// auth error (house rule §7). The status code is the evidence.
	return v
}

// ---------------------------------------------------------------------------
// The record of what was found
// ---------------------------------------------------------------------------

var (
	probeMu       sync.RWMutex
	probeVerdicts = map[ModelID]ProbeVerdict{}
)

func probeCachePath(dir string) string { return filepath.Join(dir, "model-probes.json") }

// LoadProbeVerdicts reads the record from disk. A missing or unreadable file is
// the normal first-run state.
func LoadProbeVerdicts(dir string) int {
	blob, err := os.ReadFile(probeCachePath(dir))
	if err != nil {
		return 0
	}
	var in map[ModelID]ProbeVerdict
	if json.Unmarshal(blob, &in) != nil {
		return 0
	}
	probeMu.Lock()
	probeVerdicts = in
	probeMu.Unlock()
	return len(in)
}

func saveProbeVerdicts(dir string) error {
	probeMu.RLock()
	blob, err := json.MarshalIndent(probeVerdicts, "", "  ")
	probeMu.RUnlock()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp := probeCachePath(dir) + ".tmp"
	if err := os.WriteFile(tmp, blob, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, probeCachePath(dir))
}

// ProbeVerdictFor returns what was last found for a model.
func ProbeVerdictFor(id ModelID) (ProbeVerdict, bool) {
	probeMu.RLock()
	defer probeMu.RUnlock()
	v, ok := probeVerdicts[id]
	return v, ok
}

func setProbeVerdict(id ModelID, v ProbeVerdict) {
	probeMu.Lock()
	probeVerdicts[id] = v
	probeMu.Unlock()
}

// ---------------------------------------------------------------------------
// Which models are worth asking, and in what order
// ---------------------------------------------------------------------------

// chatTier orders ids by how likely they are to be a capable coding model, from
// the WORDS in the id. Families, never versions: "glm" matches glm-5.2 and
// whatever follows it. 9 = not a chat model at all.
func chatTier(id string) int {
	s := strings.ToLower(id)
	has := func(subs ...string) bool {
		for _, sub := range subs {
			if strings.Contains(s, sub) {
				return true
			}
		}
		return false
	}
	switch {
	case has("embed", "rerank", "retriever", "bge-", "guard", "safety", "moderation",
		"deplot", "cosmos", "gliner", "parse", "video", "vision", "-vl-", "vlm",
		"diffusion", "tts", "-image", "ocr", "riva", "nvclip", "neva", "fuyu", "kosmos",
		"translate", "calibration", "whisper", "audio"):
		return 9
	case has("coder", "codestral", "devstral", "deepseek", "glm", "kimi", "qwen", "minimax", "gpt-oss"):
		return 1
	case has("nemotron", "llama", "mistral", "gemma", "phi", "granite", "claude", "gpt"):
		return 2
	}
	return 3
}

// sizeHint pulls the parameter count out of an id ("...-120b-...") so that,
// inside one tier, the larger model is asked first. 0 when the id states none.
func sizeHint(id string) float64 {
	s := strings.ToLower(id)
	best := 0.0
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			continue
		}
		j := i
		for j < len(s) && (s[j] >= '0' && s[j] <= '9' || s[j] == '.') {
			j++
		}
		if j < len(s) && s[j] == 'b' && (j+1 == len(s) || s[j+1] == '-' || s[j+1] == ':' || s[j+1] == '_') {
			var n float64
			fmt.Sscanf(s[i:j], "%g", &n)
			if n > best && n < 2000 {
				best = n
			}
		}
		i = j
	}
	return best
}

// candidateOrder sorts an endpoint's models best-guess-first for a default:
// seen working, then never checked, then (last) seen failing; inside each, by
// tier, then size, then id. Deterministic: the same list gives the same order.
func candidateOrder(ids []ModelID) []ModelID {
	out := make([]ModelID, 0, len(ids))
	for _, id := range ids {
		if chatTier(string(id)) < 9 {
			out = append(out, id)
		}
	}
	bucket := func(id ModelID) int {
		v, ok := ProbeVerdictFor(id)
		switch {
		case ok && v.Usable():
			return 0
		case !ok:
			return 1
		case v.Dead():
			return 3
		}
		return 2
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		x, y := bucket(a), bucket(b)
		if x != y {
			return x < y
		}
		// Among models SEEN to answer, family is no longer a guess worth
		// making: all of them work, so the larger one leads.
		if x == 0 {
			if sa, sb := sizeHint(string(a)), sizeHint(string(b)); sa != sb {
				return sa > sb
			}
		}
		if x, y := chatTier(string(a)), chatTier(string(b)); x != y {
			return x < y
		}
		if x, y := sizeHint(string(a)), sizeHint(string(b)); x != y {
			return x > y
		}
		return a < b
	})
	return out
}

// EndpointProbeReport is what probing one endpoint found.
type EndpointProbeReport struct {
	Endpoint string
	Asked    int
	Working  []ModelID
	Failed   map[ModelID]ProbeVerdict
	Skipped  string // why nothing was asked, when nothing was
}

// probeBudget bounds one endpoint's check: stop after this many models answer
// properly, or after this many requests, whichever comes first. The line this
// runs on may be metered and slow; "check all eighty" is not a check, it is an
// outage.
const (
	probeWantWorking = 3
	probeMaxRequests = 10
)

// ProbeEndpoint asks the likeliest models on one configured endpoint whether
// they answer, records every answer, and saves the record.
func ProbeEndpoint(name, cacheDir string) EndpointProbeReport {
	rep := EndpointProbeReport{Endpoint: name, Failed: map[ModelID]ProbeVerdict{}}
	var ids []ModelID
	var baseURL, key string
	for id, r := range localRoute {
		if r.Endpoint == name {
			ids = append(ids, id)
			baseURL, key = r.BaseURL, r.APIKey
		}
	}
	switch {
	case len(ids) == 0:
		rep.Skipped = "no models registered"
		return rep
	case isLoopback(baseURL) && !probeAllowLoopback:
		rep.Skipped = "on this machine: not probed, asking would load the model"
		return rep
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for _, id := range candidateOrder(ids) {
		if rep.Asked >= probeMaxRequests || len(rep.Working) >= probeWantWorking {
			break
		}
		if v, ok := ProbeVerdictFor(id); ok && v.Dead() && time.Since(v.At) < 7*24*time.Hour {
			continue // found dead within the week: do not spend a request re-asking
		}
		v := ProbeChat(baseURL, key, SupportedModels[id].APIModel)
		rep.Asked++
		setProbeVerdict(id, v)
		if v.Usable() {
			rep.Working = append(rep.Working, id)
		} else {
			rep.Failed[id] = v
		}
		if v.Outcome == ProbeRefused || v.Outcome == ProbeUnreached {
			break // the key or the line, not the model: more requests prove nothing
		}
	}
	_ = saveProbeVerdicts(cacheDir)
	return rep
}

// ProbedEndpoints lists the configured remote endpoints, for /update.
func ProbedEndpoints() []string {
	seen := map[string]bool{}
	for _, r := range localRoute {
		if !isLoopback(r.BaseURL) {
			seen[r.Endpoint] = true
		}
	}
	out := make([]string, 0, len(seen))
	for n := range seen {
		out = append(out, n)
	}
	sort.Strings(out)
	return out
}

// Note renders a report as one line for the /update summary.
func (r EndpointProbeReport) Note() string {
	if r.Skipped != "" {
		return fmt.Sprintf("%s: %s", r.Endpoint, r.Skipped)
	}
	names := func(ids []ModelID) string {
		out := make([]string, 0, len(ids))
		for _, id := range ids {
			out = append(out, SupportedModels[id].APIModel)
		}
		return strings.Join(out, ", ")
	}
	note := fmt.Sprintf("%s: asked %d model(s), %d answered with a tool call", r.Endpoint, r.Asked, len(r.Working))
	if len(r.Working) > 0 {
		note += " (" + names(r.Working) + ")"
	}
	if len(r.Failed) > 0 {
		var bad []string
		for id, v := range r.Failed {
			bad = append(bad, fmt.Sprintf("%s=%s", SupportedModels[id].APIModel, v.Outcome))
		}
		sort.Strings(bad)
		note += "; not usable: " + strings.Join(bad, ", ")
	}
	return note
}
