package provider

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/openai/openai-go/shared"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/llm/models"
	"github.com/opencode-ai/opencode/internal/llm/tools"
	"github.com/opencode-ai/opencode/internal/logging"
	"github.com/opencode-ai/opencode/internal/message"
)

type openaiOptions struct {
	baseURL         string
	disableCache    bool
	reasoningEffort string
	extraHeaders    map[string]string
}

type OpenAIOption func(*openaiOptions)

type openaiClient struct {
	providerOptions providerClientOptions
	options         openaiOptions
	client          openai.Client
}

type OpenAIClient ProviderClient

func newOpenAIClient(opts providerClientOptions) OpenAIClient {
	openaiOpts := openaiOptions{
		reasoningEffort: "medium",
	}
	for _, o := range opts.openaiOptions {
		o(&openaiOpts)
	}

	openaiClientOptions := []option.RequestOption{}
	// GORILLA OVERRIDE: use the satellite-hardened HTTP client (keep-alive,
	// HTTP/2, no wall-clock stream timeout) instead of the SDK default.
	openaiClientOptions = append(openaiClientOptions, option.WithHTTPClient(resilientHTTPClient()))

	// GORILLA OVERRIDE (2026-08-18): the SDK must not retry. It defaults to
	// MaxRetries: 2 (requestconfig.go), which was never configured here, so
	// every one of OUR attempts was silently three of the SDK's.
	//
	// This is the THIRD independent retry layer found in this one path, after
	// the application loop below and Go's own http.Transport replay (see
	// uploadbudget.go). Their effect is multiplicative, not additive: measured
	// against a live black-holed model with a 20-second first-byte timeout, a
	// cap meant to stop at roughly 41 seconds did not stop until 123.
	//
	// Retries belong to shouldRetry, which is the only layer that knows whether
	// content has already streamed (retrying mid-answer duplicates it), what the
	// turn has spent, and what to tell the user. So the SDK gets zero.
	openaiClientOptions = append(openaiClientOptions, option.WithMaxRetries(0))
	if opts.apiKey != "" {
		openaiClientOptions = append(openaiClientOptions, option.WithAPIKey(opts.apiKey))
	}
	if openaiOpts.baseURL != "" {
		openaiClientOptions = append(openaiClientOptions, option.WithBaseURL(openaiOpts.baseURL))
	}

	if openaiOpts.extraHeaders != nil {
		for key, value := range openaiOpts.extraHeaders {
			openaiClientOptions = append(openaiClientOptions, option.WithHeader(key, value))
		}
	}

	client := openai.NewClient(openaiClientOptions...)
	return &openaiClient{
		providerOptions: opts,
		options:         openaiOpts,
		client:          client,
	}
}

func (o *openaiClient) convertMessages(messages []message.Message) (openaiMessages []openai.ChatCompletionMessageParamUnion) {
	// Add system message first
	openaiMessages = append(openaiMessages, openai.SystemMessage(o.providerOptions.systemMessage))

	for _, msg := range messages {
		switch msg.Role {
		case message.User:
			var content []openai.ChatCompletionContentPartUnionParam
			textBlock := openai.ChatCompletionContentPartTextParam{Text: msg.Content().String()}
			content = append(content, openai.ChatCompletionContentPartUnionParam{OfText: &textBlock})
			for _, binaryContent := range msg.BinaryContent() {
				imageURL := openai.ChatCompletionContentPartImageImageURLParam{URL: binaryContent.String(models.ProviderOpenAI)}
				imageBlock := openai.ChatCompletionContentPartImageParam{ImageURL: imageURL}

				content = append(content, openai.ChatCompletionContentPartUnionParam{OfImageURL: &imageBlock})
			}

			openaiMessages = append(openaiMessages, openai.UserMessage(content))

		case message.Assistant:
			assistantMsg := openai.ChatCompletionAssistantMessageParam{
				Role: "assistant",
			}

			// GORILLA FIX: always send a STRING content, never null.
			//
			// An assistant turn that only calls a tool has no text, and leaving
			// Content unset serialises as `"content": null`. OpenAI's schema allows
			// that; Cloudflare Workers AI validates against a stricter one and
			// rejects the whole request:
			//
			//   400  Type mismatch of '/messages/1/content', 'string' not in 'null'
			//        required properties at '/messages/1' are 'role,content'
			//
			// The effect is that any conversation survives exactly until its first
			// tool call and then fails on every following turn, because the
			// offending message stays in the history forever. Measured 2026-08-05:
			// identical request with "" instead of null returns 200.
			//
			// An empty string is faithful — the turn genuinely produced no text —
			// and is accepted everywhere, so this is not a Cloudflare special case.
			assistantMsg.Content = openai.ChatCompletionAssistantMessageParamContentUnion{
				OfString: openai.String(msg.Content().String()),
			}

			if len(msg.ToolCalls()) > 0 {
				assistantMsg.ToolCalls = make([]openai.ChatCompletionMessageToolCallParam, len(msg.ToolCalls()))
				for i, call := range msg.ToolCalls() {
					assistantMsg.ToolCalls[i] = openai.ChatCompletionMessageToolCallParam{
						ID:   call.ID,
						Type: "function",
						Function: openai.ChatCompletionMessageToolCallFunctionParam{
							Name:      call.Name,
							Arguments: call.Input,
						},
					}
				}
			}

			openaiMessages = append(openaiMessages, openai.ChatCompletionMessageParamUnion{
				OfAssistant: &assistantMsg,
			})

		case message.Tool:
			for _, result := range msg.ToolResults() {
				openaiMessages = append(openaiMessages,
					openai.ToolMessage(result.Content, result.ToolCallID),
				)
			}
		}
	}

	return
}

func (o *openaiClient) convertTools(tools []tools.BaseTool) []openai.ChatCompletionToolParam {
	openaiTools := make([]openai.ChatCompletionToolParam, len(tools))

	for i, tool := range tools {
		info := tool.Info()
		openaiTools[i] = openai.ChatCompletionToolParam{
			Function: openai.FunctionDefinitionParam{
				Name:        info.Name,
				Description: openai.String(info.Description),
				Parameters: openai.FunctionParameters{
					"type":       "object",
					"properties": info.Parameters,
					"required":   info.Required,
				},
			},
		}
	}

	return openaiTools
}

func (o *openaiClient) finishReason(reason string) message.FinishReason {
	switch reason {
	case "stop":
		return message.FinishReasonEndTurn
	case "length":
		return message.FinishReasonMaxTokens
	case "tool_calls":
		return message.FinishReasonToolUse
	default:
		return message.FinishReasonUnknown
	}
}

func (o *openaiClient) preparedParams(messages []openai.ChatCompletionMessageParamUnion, tools []openai.ChatCompletionToolParam) openai.ChatCompletionNewParams {
	params := openai.ChatCompletionNewParams{
		Model:    openai.ChatModel(o.providerOptions.model.APIModel),
		Messages: messages,
	}

	// GORILLA FIX: OMIT `tools` when there are none. Never send `"tools": []`.
	//
	// An empty array is meaningless everywhere — it says "here are the tools you
	// may call: none" — but most providers tolerate it, so it went unnoticed.
	// Cloudflare Workers AI validates it and rejects the whole request:
	//
	//   400  Value error, `tools` must not be an empty array.
	//        Either provide at least one tool or omit the field
	//
	// This is why session titles failed against Cloudflare while ordinary turns
	// worked: generateTitle calls SendMessages with make([]tools.BaseTool, 0),
	// and the summarizer does the same, so long conversations would have started
	// failing the moment they needed compacting. Measured 2026-08-05: identical
	// request with the field omitted returns 200, with `"tools": []` returns 400.
	if len(tools) > 0 {
		params.Tools = tools
	}

	if o.providerOptions.model.CanReason == true {
		params.MaxCompletionTokens = openai.Int(o.providerOptions.maxTokens)
		switch o.options.reasoningEffort {
		case "low":
			params.ReasoningEffort = shared.ReasoningEffortLow
		case "medium":
			params.ReasoningEffort = shared.ReasoningEffortMedium
		case "high":
			params.ReasoningEffort = shared.ReasoningEffortHigh
		default:
			params.ReasoningEffort = shared.ReasoningEffortMedium
		}
	} else {
		params.MaxTokens = openai.Int(o.providerOptions.maxTokens)
	}

	return params
}

// GORILLA OVERRIDE: prompt caching for OpenAI-compatible providers.
// The OpenAI wire protocol caches on a stable prompt PREFIX and can be
// steered with `prompt_cache_key`; a single stable key per (system
// prompt + model) routes every turn of a session to the same cached
// prefix, so the ~thousands of tokens of system prompt + tool schemas
// are not re-processed each turn.
//
// IMPORTANT, and why this is OPT-IN: not every OpenAI-compatible
// endpoint accepts the field. NVIDIA NIM — the provider this fork was
// built for — REJECTS it with HTTP 400 "Unsupported parameter" (verified
// 2026-07-20) and reports no cache metrics at all, i.e. NIM offers no
// prompt caching to enable. Sending the key by default would BREAK every
// NIM request. So it is off unless you opt in with GORILLA_OPENCODE_PROMPT_CACHE=1,
// for endpoints known to support it (OpenAI, DeepSeek's direct API, ...).
// Anthropic caching is separate and always on (see anthropic.go).
func (o *openaiClient) cacheOptions() []option.RequestOption {
	if on, _ := strconv.ParseBool(os.Getenv("GORILLA_OPENCODE_PROMPT_CACHE")); !on {
		return nil
	}
	sum := sha256.Sum256([]byte(o.providerOptions.model.APIModel + "\x00" + o.providerOptions.systemMessage))
	key := "goc-" + hex.EncodeToString(sum[:8])
	return []option.RequestOption{option.WithJSONSet("prompt_cache_key", key)}
}

func (o *openaiClient) send(ctx context.Context, messages []message.Message, tools []tools.BaseTool) (response *ProviderResponse, err error) {
	params := o.preparedParams(o.convertMessages(messages), o.convertTools(tools))
	// GORILLA OVERRIDE: nil-guard. config.Get() returns nil until Load has run,
	// and dereferencing it here panicked — which made this path impossible to
	// test against a fake server without booting the whole application. A debug
	// log line must never be the reason a stream cannot start.
	if cfg := config.Get(); cfg != nil && cfg.Debug {
		jsonData, _ := json.Marshal(params)
		logging.Debug("Prepared messages", "messages", string(jsonData))
	}
	attempts := 0
	for {
		attempts++
		openaiResponse, err := o.client.Chat.Completions.New(
			ctx,
			params,
			o.cacheOptions()...,
		)
		// If there is an error we are going to see if we can retry the call.
		// Non-streaming: nothing was emitted, so a transport drop is always
		// safe to retry (contentEmitted=false).
		if err != nil {
			retry, after, retryErr := o.shouldRetry(attempts, err, false)
			if retryErr != nil {
				return nil, retryErr
			}
			if retry {
				logging.WarnPersist(fmt.Sprintf("Provider busy (rate-limit/5xx), retrying %d/%d in %.1fs", attempts, maxRetries, float64(after)/1000), logging.PersistTimeArg, time.Millisecond*time.Duration(after+100))
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(time.Duration(after) * time.Millisecond):
					continue
				}
			}
			return nil, retryErr
		}

		content := ""
		if openaiResponse.Choices[0].Message.Content != "" {
			content = openaiResponse.Choices[0].Message.Content
		}

		toolCalls := o.toolCalls(*openaiResponse)
		finishReason := o.finishReason(string(openaiResponse.Choices[0].FinishReason))

		if len(toolCalls) > 0 {
			finishReason = message.FinishReasonToolUse
		}

		return &ProviderResponse{
			Content:      content,
			ToolCalls:    toolCalls,
			Usage:        o.usage(*openaiResponse),
			FinishReason: finishReason,
		}, nil
	}
}

func (o *openaiClient) stream(ctx context.Context, messages []message.Message, tools []tools.BaseTool) <-chan ProviderEvent {
	params := o.preparedParams(o.convertMessages(messages), o.convertTools(tools))
	params.StreamOptions = openai.ChatCompletionStreamOptionsParam{
		IncludeUsage: openai.Bool(true),
	}

	// GORILLA OVERRIDE: nil-guard. config.Get() returns nil until Load has run,
	// and dereferencing it here panicked — which made this path impossible to
	// test against a fake server without booting the whole application. A debug
	// log line must never be the reason a stream cannot start.
	if cfg := config.Get(); cfg != nil && cfg.Debug {
		jsonData, _ := json.Marshal(params)
		logging.Debug("Prepared messages", "messages", string(jsonData))
	}

	// GORILLA OVERRIDE: the wire model name, used to remember per-model whether
	// this server accepts the reasoning parameter. The APIModel rather than the
	// internal ID, because two configured endpoints can serve the same model and
	// it is the server's opinion we are recording.
	modelID := string(o.providerOptions.model.APIModel)

	attempts := 0
	eventChan := make(chan ProviderEvent)

	go func() {
		for {
			attempts++
			// GORILLA OVERRIDE: ask the server to emit its reasoning. Measured:
			// without this, GLM-5.2 on NIM streams only [content, role] and the
			// reasoning reader below never fires. See reasoning.go.
			reqOpts := append(o.cacheOptions(), thinkingRequestOptions(modelID)...)

			// GORILLA OVERRIDE: guard against the answer starting and then
			// stopping — headers arrived, so no header timeout can fire, and the
			// read blocks forever. Armed by the first chunk, reset by every
			// chunk after it. See stallguard.go.
			streamCtx, guard := newStallGuard(ctx, config.StreamStallTimeout())
			openaiStream := o.client.Chat.Completions.NewStreaming(
				streamCtx,
				params,
				reqOpts...,
			)

			acc := openai.ChatCompletionAccumulator{}
			currentContent := ""
			toolCalls := make([]message.ToolCall, 0)
			// GORILLA FIX: usage must NOT come from the accumulator.
			//
			// openai-go's ChatCompletionAccumulator ADDS usage from every chunk
			// (streamaccumulator.go: `cc.Usage.PromptTokens += chunk.Usage...`).
			// With IncludeUsage set, backends that report usage on more than the
			// final chunk therefore get their prompt tokens multiplied by roughly
			// the number of chunks in the stream.
			//
			// Measured effect: a short six-message conversation reported 494.3K
			// input tokens against a 131K window — 387% "used" — while the
			// requests kept succeeding, which they could not have done had the
			// figure been real. It also inflated the cost estimate by the same
			// factor, so the session claimed $0.78 for a chat that read no files.
			//
			// The prompt token count is a property of the REQUEST: it is the same
			// number no matter how many chunks the answer arrives in. So take the
			// last non-zero report and overwrite, never accumulate.
			var streamUsage openai.CompletionUsage

			for openaiStream.Next() {
				guard.Progress()
				chunk := openaiStream.Current()
				acc.AddChunk(chunk)
				if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
					streamUsage = chunk.Usage
				}

				for _, choice := range chunk.Choices {
					// GORILLA OVERRIDE: forward the model's reasoning before its
					// answer, so it is persisted and can be shown. Every
					// OpenAI-compatible backend we actually use — NIM, GLM,
					// Ollama, DeepSeek — streams this in a vendor-specific field
					// the SDK has no type for, and it was being dropped on the
					// floor. Read from the raw delta; see reasoning.go.
					//
					// Deliberately not added to currentContent: it is not part of
					// the answer, and mixing it in would send the reasoning back to
					// the model as its own prior output on the next turn.
					if r := reasoningDelta(choice.Delta.RawJSON()); r != "" {
						eventChan <- ProviderEvent{
							Type:    EventThinkingDelta,
							Content: r,
						}
					}
					if choice.Delta.Content != "" {
						eventChan <- ProviderEvent{
							Type:    EventContentDelta,
							Content: choice.Delta.Content,
						}
						currentContent += choice.Delta.Content
					}
				}
			}

			err := openaiStream.Err()
			// GORILLA OVERRIDE: a stall cancels the stream's context, so the
			// SDK reports a bare context.Canceled — indistinguishable from the
			// user pressing ESC. Translate it before anything downstream can
			// treat a failure as a deliberate cancellation and stay silent.
			// The guard is released here rather than by defer because the
			// enclosing loop retries, and each attempt needs its own.
			if guard.Fired() {
				err = &ErrStreamStalled{Idle: config.StreamStallTimeout(), Got: len(currentContent)}
			}
			guard.Stop()
			// GORILLA FIX: always release the stream's underlying HTTP/2
			// connection. The SDK does NOT auto-close the body when the
			// stream is drained; leaving it open keeps the request "in
			// flight" on the server side. On NVIDIA NIM (HTTP/2 endpoint with
			// a low per-worker in-flight cap) these half-open streams pile up
			// over an agentic session — every tool-loop round opens another —
			// until the worker refuses new requests with
			//   "ResourceExhausted: Worker local total request limit reached (N/...)".
			// Closing here covers all three exits below: success, retry, error.
			openaiStream.Close()
			if err == nil || errors.Is(err, io.EOF) {
				// Stream completed successfully
				finishReason := o.finishReason(string(acc.ChatCompletion.Choices[0].FinishReason))
				if len(acc.ChatCompletion.Choices[0].Message.ToolCalls) > 0 {
					toolCalls = append(toolCalls, o.toolCalls(acc.ChatCompletion)...)
				}
				if len(toolCalls) > 0 {
					finishReason = message.FinishReasonToolUse
				}

				// Report the per-request usage captured above, not the
				// accumulator's summed-over-chunks figure. Falls back to the
				// accumulator only if no chunk ever carried usage, which keeps
				// backends that report usage some other way working as before.
				usage := o.usage(acc.ChatCompletion)
				if streamUsage.PromptTokens > 0 || streamUsage.CompletionTokens > 0 {
					final := acc.ChatCompletion
					final.Usage = streamUsage
					usage = o.usage(final)
				}

				eventChan <- ProviderEvent{
					Type: EventComplete,
					Response: &ProviderResponse{
						Content:      currentContent,
						ToolCalls:    toolCalls,
						Usage:        usage,
						FinishReason: finishReason,
					},
				}
				close(eventChan)
				return
			}

			// GORILLA OVERRIDE: a server that refuses the thinking parameter must
			// not cost the user their turn. Drop it for this model and retry once,
			// rather than surfacing an error or maintaining a list of which
			// backends accept a vendor extension.
			//
			// Guarded on currentContent: once tokens have been streamed, retrying
			// would duplicate the answer, so reasoning is sacrificed instead.
			if currentContent == "" && thinkingWasRequested(modelID) && isParameterRejection(err) {
				noteThinkingRejected(modelID)
				logging.Warn("server refused the reasoning parameter; retrying without it",
					"model", modelID, "err", err)
				continue
			}

			// If there is an error we are going to see if we can retry the call.
			// Pass whether we've already streamed visible content: a transport
			// drop can only be safely retried before the first token, else the
			// restarted stream would duplicate the answer mid-flight.
			retry, after, retryErr := o.shouldRetry(attempts, err, currentContent != "")
			if retryErr != nil {
				eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
				close(eventChan)
				return
			}
			if retry {
				logging.WarnPersist(fmt.Sprintf("Provider busy (rate-limit/5xx), retrying %d/%d in %.1fs", attempts, maxRetries, float64(after)/1000), logging.PersistTimeArg, time.Millisecond*time.Duration(after+100))
				select {
				case <-ctx.Done():
					// context cancelled
					if ctx.Err() == nil {
						eventChan <- ProviderEvent{Type: EventError, Error: ctx.Err()}
					}
					close(eventChan)
					return
				case <-time.After(time.Duration(after) * time.Millisecond):
					continue
				}
			}
			eventChan <- ProviderEvent{Type: EventError, Error: retryErr}
			close(eventChan)
			return
		}
	}()

	return eventChan
}

// explainAPIStatus turns a bare HTTP status into a sentence that says what
// happened and what to do about it. Returns "" when the status has no better
// explanation than the raw error already gives.
//
// GORILLA FIX: the status code has to carry the whole explanation, because the
// provider's own words do not survive the SDK.
//
// Measured 2026-08-04 against NVIDIA NIM: asking for a model the account is not
// entitled to returns
//
//	{"status":404,"title":"Not Found",
//	 "detail":"Function '<uuid>': Not found for account '<id>'"}
//
// which does not match OpenAI's {"error":{...}} schema, so openai-go's
// unmarshal populates nothing: RawJSON(), Code, Message and Type all come back
// EMPTY and only StatusCode survives. The user was therefore shown
// `failed to process events: POST "...": 404 Not Found` — technically true and
// completely useless, since it looks like the app or the network is broken when
// in fact the key is fine and only that one model is off-limits.
//
// Anything added here must be derivable from the status alone. Do not reach for
// the response body: it is already gone by this point.
func explainAPIStatus(status int, modelName string) string {
	if modelName == "" {
		modelName = "this model"
	}
	// Kept SHORT and with the status code inline, deliberately. This lands in the
	// one-line status bar, which clampToWidth truncates at the terminal width, so
	// every word spent here is a word of raw detail pushed off the right edge.
	// Leading with the diagnosis and the number means the useful part survives on
	// a narrow terminal and only the trailing URL is ever lost — and that is still
	// in the log.
	switch status {
	case 404:
		return fmt.Sprintf("%s isn't enabled for your account (HTTP 404 — your key is fine). "+
			"Pick another with /models.", modelName)
	case 410:
		return fmt.Sprintf("%s has been retired by the provider (HTTP 410). "+
			"Pick another with /models.", modelName)
	case 401:
		return "Your API key was rejected (HTTP 401). Add or replace it with /connect."
	case 403:
		return fmt.Sprintf("Your key isn't allowed to use %s (HTTP 403 — usually needs a paid tier). "+
			"Pick another with /models.", modelName)
	}
	return ""
}

func (o *openaiClient) shouldRetry(attempts int, err error, contentEmitted bool) (bool, int64, error) {
	var apierr *openai.Error
	if !errors.As(err, &apierr) {
		// GORILLA OVERRIDE: not an API error — this is a transport/stream
		// failure (dropped SSE connection, unexpected EOF, connection reset,
		// read timeout). These are common when a big model is slow to its
		// first token (an idle proxy drops the stream) and on flaky mobile
		// links — the "SSE existential crisis" reported by users on 550B-class
		// models. Retry them too, but ONLY before any visible content has been
		// streamed: a retry restarts the stream from scratch, so retrying mid-
		// answer would duplicate output.
		if contentEmitted {
			// A retry restarts the stream from scratch; retrying mid-answer
			// would duplicate output. Never safe once content has streamed.
			return false, 0, err
		}
		// GORILLA OVERRIDE: two recoverable classes reach here as plain
		// (non-*openai.Error) stream errors:
		//   1. server-busy — NVIDIA NIM's "ResourceExhausted: ... request
		//      limit reached", "overloaded", "rate limit", 503, etc. These
		//      arrive in-band on the SSE stream, so they never surface as an
		//      HTTP status we can match below. Back off LONGER: hammering a
		//      congested endpoint makes it worse and burns scarce satellite
		//      bandwidth.
		//   2. transport blips — dropped SSE, unexpected EOF, reset, timeout
		//      (the "SSE existential crisis" on slow 550B models / flaky
		//      links). Back off SHORT and try again.
		// GORILLA OVERRIDE (2026-08-18): a first-byte timeout that happens TWICE
		// is not a transient link problem, it is a server that will not answer.
		//
		// config.FirstByteTimeout bounds one attempt. Retrying it five times
		// multiplies the bound back into something unbounded-feeling: measured
		// against a live black-holed model with a 20s timeout, the run was still
		// going at 123 seconds. At the 120s default that is twelve minutes of
		// silence, and every attempt re-uploads the whole conversation.
		//
		// This is the same trap as the retry storm in uploadbudget.go, one layer
		// down: a limit is only a limit if nothing above it multiplies it. So the
		// first-byte timeout gets exactly one retry — enough to ride out a real
		// dropout, not enough to sit on a dead model.
		if isFirstByteTimeout(err) && attempts > 1 {
			// The wording matters and the first version of it was wrong. It said
			// the model "is not actually being served". Measured 40 minutes
			// later, the same black-holed model answered in 12 seconds: these
			// endpoints COLD-START, so silence means "not warm yet or queued",
			// not "broken". Telling a user to abandon a model that would have
			// worked is worse than saying nothing.
			return false, 0, fmt.Errorf(
				"the server took the request and sent nothing back, twice. Models on "+
					"shared endpoints are often idle and have to start up, which can take "+
					"minutes — so this usually means 'not ready yet' rather than 'broken'. "+
					"Either try again shortly, pick a model that is already warm with "+
					"/models, or raise the wait. (waited %s each time; "+
					"GORILLA_OPENCODE_FIRST_BYTE_TIMEOUT changes that): %w",
				config.FirstByteTimeout(), err)
		}

		busy := isServerBusyError(err)
		if !busy && !isTransientStreamError(err) {
			return false, 0, err
		}
		if attempts > maxRetries {
			return false, 0, fmt.Errorf("still failing after %d retries — the provider is busy/rate-limiting or the connection is unstable; wait a moment, lower the request pace in /context, or switch to a smaller model: %w", maxRetries, err)
		}
		baseMs, capMs := 500, 6000 // transport: 0.5,1,2,4,6...
		if busy {
			baseMs, capMs = 2000, 20000 // server-busy: 2,4,8,16,20...
		}
		retryMs := baseMs * (1 << (attempts - 1))
		if retryMs > capMs {
			retryMs = capMs
		}
		retryMs += int(float64(retryMs) * 0.2) // jitter
		return true, int64(retryMs), nil
	}

	// Retry on rate-limit (429) and server-side errors (500/503) and the
	// "overloaded" 529 some providers use.
	if apierr.StatusCode != 429 && apierr.StatusCode != 500 && apierr.StatusCode != 503 && apierr.StatusCode != 529 {
		if plain := explainAPIStatus(apierr.StatusCode, o.providerOptions.model.Name); plain != "" {
			logging.Error("provider rejected the request", "status", apierr.StatusCode, "err", err)
			// GORILLA FIX: explain AND show, never explain INSTEAD of showing.
			//
			// The plain sentence goes FIRST because the status bar truncates the
			// tail, so the part that says what to do is the part that always
			// survives. The raw error is appended, not discarded — a translation
			// that swallows the URL and status code hides exactly what you need
			// when the translation itself turns out to be wrong.
			//
			// %w, not %s: the *openai.Error stays reachable through errors.As, so
			// anything upstream that inspects the status still can.
			return false, 0, fmt.Errorf("%s  ⟨%w⟩", plain, err)
		}
		return false, 0, err
	}

	if attempts > maxRetries {
		return false, 0, fmt.Errorf("still failing after %d retries (HTTP %d) — the provider is rate-limiting or unavailable; wait a moment or switch model", maxRetries, apierr.StatusCode)
	}

	// GORILLA OVERRIDE: the old schedule was 2s·2^(n-1) = 2,4,8,16,32,
	// 64,128,256s — a brief 429 (common on NIM's low concurrency limit)
	// turned into 8+ minutes of "retrying". Cap the backoff so a
	// transient rate-limit recovers in seconds. Honour a server-sent
	// Retry-After but cap that too.
	retryMs := 500 * (1 << (attempts - 1)) // 0.5,1,2,4,8,16...
	if retryMs > 6000 {
		retryMs = 6000
	}
	retryMs += int(float64(retryMs) * 0.2) // jitter
	if vals := apierr.Response.Header.Values("Retry-After"); len(vals) > 0 {
		var secs int
		if _, err := fmt.Sscanf(vals[0], "%d", &secs); err == nil {
			retryMs = secs * 1000
			if retryMs > 15000 {
				retryMs = 15000
			}
		}
	}
	return true, int64(retryMs), nil
}

// isServerBusyError reports whether err signals a transient server-side
// capacity/rate condition that is worth retrying after a longer back-off —
// most importantly NVIDIA NIM's in-band stream error
//
//	"ResourceExhausted: Worker local total request limit reached (N/...)"
//
// which arrives on the SSE stream (not as an HTTP status), plus the usual
// rate-limit / overloaded phrasings from other providers. Matched on message
// text because these do not surface as a typed *openai.Error.
func isServerBusyError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"resourceexhausted", "resource exhausted",
		"request limit reached", "total request limit",
		"too many requests", "rate limit", "rate-limit", "ratelimit",
		"overloaded", "server is busy", "service unavailable",
		"try again later", "please retry", "temporarily unavailable",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

// isTransientStreamError reports whether err is a recoverable transport-level
// failure of the token stream (dropped connection, truncated body, timeout) as
// opposed to a genuine API/application error. A clean io.EOF is NOT included —
// that is a normal end-of-stream and is treated as success before we ever get
// here.
func isTransientStreamError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, s := range []string{
		"connection reset", "reset by peer", "broken pipe",
		"unexpected eof", "connection closed", "use of closed",
		"timeout", "stream error", "goaway", "server closed",
	} {
		if strings.Contains(msg, s) {
			return true
		}
	}
	return false
}

func (o *openaiClient) toolCalls(completion openai.ChatCompletion) []message.ToolCall {
	var toolCalls []message.ToolCall

	if len(completion.Choices) > 0 && len(completion.Choices[0].Message.ToolCalls) > 0 {
		for _, call := range completion.Choices[0].Message.ToolCalls {
			toolCall := message.ToolCall{
				ID:       call.ID,
				Name:     call.Function.Name,
				Input:    call.Function.Arguments,
				Type:     "function",
				Finished: true,
			}
			toolCalls = append(toolCalls, toolCall)
		}
	}

	return toolCalls
}

func (o *openaiClient) usage(completion openai.ChatCompletion) TokenUsage {
	cachedTokens := completion.Usage.PromptTokensDetails.CachedTokens
	inputTokens := completion.Usage.PromptTokens - cachedTokens

	return TokenUsage{
		InputTokens:         inputTokens,
		OutputTokens:        completion.Usage.CompletionTokens,
		CacheCreationTokens: 0, // OpenAI doesn't provide this directly
		CacheReadTokens:     cachedTokens,
	}
}

func WithOpenAIBaseURL(baseURL string) OpenAIOption {
	return func(options *openaiOptions) {
		options.baseURL = baseURL
	}
}

func WithOpenAIExtraHeaders(headers map[string]string) OpenAIOption {
	return func(options *openaiOptions) {
		options.extraHeaders = headers
	}
}

func WithOpenAIDisableCache() OpenAIOption {
	return func(options *openaiOptions) {
		options.disableCache = true
	}
}

func WithReasoningEffort(effort string) OpenAIOption {
	return func(options *openaiOptions) {
		defaultReasoningEffort := "medium"
		switch effort {
		case "low", "medium", "high":
			defaultReasoningEffort = effort
		default:
			logging.Warn("Invalid reasoning effort, using default: medium")
		}
		options.reasoningEffort = defaultReasoningEffort
	}
}
