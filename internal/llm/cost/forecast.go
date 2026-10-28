package cost

// Forecasts for the paid-key warning and the status bar (owner, 2026-10-28):
// what a message costs before it is sent, so a person on an API key can see
// the bill coming instead of reading it afterwards.
//
// These are ESTIMATES at list price with no cache discount, which is the most
// a message can cost: Gorilla OpenCode caches the repeated part, so a message
// sent within five minutes of the last one usually costs less. Every place
// that shows them says "about" or "up to".

const (
	// ShortConversationTokens is a conversation at its start: the system
	// prompt and tool descriptions alone were 19.4K tokens on the MacBook Air
	// (measured 2026-10-28).
	ShortConversationTokens = 20_000
	// LongConversationTokens is a long working session.
	LongConversationTokens = 200_000
	// TypicalReplyTokens is the output assumed for one reply.
	TypicalReplyTokens = 500
	// WorkingCallsPerMinute is how many calls the AI makes on its own while it
	// reads files and runs commands (estimate).
	WorkingCallsPerMinute = 4
)

// cacheWritePremium is what Anthropic charges to write the prompt cache: 1.25x
// the input price for the 5-minute cache, on every model in prices.json. The
// first message, and the first after a pause, pays it on the whole context.
// Without it the "up to" figure was below a measured bill: 4.5 cents
// forecast, 4.85 cents billed for the Air's first "comms check" (2026-10-28).
const cacheWritePremium = 1.25

// MessageCost is the most one message costs at the given list rates (dollars
// per million tokens) when contextTokens are sent again with it.
func MessageCost(in, out float64, contextTokens int64) float64 {
	return float64(contextTokens)*in*cacheWritePremium/1e6 + TypicalReplyTokens*out/1e6
}

// WorkingHourCost is the most an hour of the AI working on its own costs in a
// long conversation.
func WorkingHourCost(in, out float64) float64 {
	return MessageCost(in, out, LongConversationTokens) * WorkingCallsPerMinute * 60
}
