package models

import "time"

// PickerOrder is the list of endpoint-served models a person should be shown,
// in the order they should be shown.
//
// Two kinds are left out, and counted so the picker can say so:
//
//   - models the provider answered "retired" for (404/410). Providers go on
//     listing them; on 2026-10-05 NVIDIA listed 80 models and a third of the
//     picker was rows that could not be used.
//   - entries that are not chat models at all: embedders, safety filters,
//     document parsers. They are in the provider's list and can never hold a
//     conversation.
//
// The rest are ordered by what was FOUND: models seen answering first, then
// those never tested, then those that failed a test in a way that may pass.
func PickerOrder(ids []ModelID) (shown []ModelID, retired, notChat int) {
	var live []ModelID
	for _, id := range ids {
		if chatTier(string(id)) >= 9 {
			notChat++
			continue
		}
		if v, ok := ProbeVerdictFor(id); ok && v.Outcome == ProbeGone {
			retired++
			continue
		}
		live = append(live, id)
	}
	return candidateOrder(sortedIDs(live)), retired, notChat
}

// useEvidenceAge is how old a "works" record may be before real use renews it.
const useEvidenceAge = 12 * time.Hour

// NoteAnsweredInUse records that a model served by an endpoint has just
// answered a real request with a tool call. Use is better evidence than a
// test: a model labelled "provider error 500" by one test question was chosen
// anyway and worked, and its row went on saying it had failed.
func NoteAnsweredInUse(id ModelID, cacheDir string) {
	if LocalEndpointFor(id) == "" {
		return
	}
	if v, ok := ProbeVerdictFor(id); ok && v.Usable() && time.Since(v.At) < useEvidenceAge {
		return
	}
	setProbeVerdict(id, ProbeVerdict{Outcome: ProbeWorks, Status: 200, Detail: "in use", At: time.Now().UTC()})
	_ = saveProbeVerdicts(cacheDir)
}
