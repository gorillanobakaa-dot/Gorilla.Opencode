package models

// GORILLA CULL (2026-08-21): the hand-written OpenAI model list that lived here
// is gone. Its models are now FETCHED from OpenAI itself — see
// catalogue_fetch.go, which explains why (sixteen entries across three providers
// were dead on the day this was written, and a dead entry does not fail politely).
//
// The old list is kept, not deleted, in
// /home/gorilla/Agents.Work.Trash/gorilla-opencode-provider-cull-26-08-21-11-58/models/hardcoded-openai.go
// — it still records the curated descriptions and prices, which are worth
// harvesting into metadata/ if anyone wants them back on the fetched entries.
//
// Only the provider identity stays here; that is not a claim about the world and
// cannot go stale.

const (
	ProviderOpenAI ModelProvider = "openai"
)
