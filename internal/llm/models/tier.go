package models

// GORILLA OVERRIDE (2026-11-21): provider/generation/tier classification for
// the model picker so newer flagship models (e.g. Claude Opus 5.5, Gemini 4
// Neon) appear at the top with launch year-month and distinct accent colors,
// visually separated by vendor (Anthropic vs Gemini) and divided cleanly from
// legacy generations (2.5, 3.1, older 4.x) with capability descriptors.

import (
	"strconv"
	"strings"
)

// TierAccent names the semantic highlight color used in the model picker for
// newer models.
type TierAccent string

const (
	AccentNone   TierAccent = ""
	AccentCyan   TierAccent = "cyan"
	AccentGreen  TierAccent = "green"
	AccentYellow TierAccent = "yellow"
	AccentRed    TierAccent = "red"
)

// TierInfo describes how a model is grouped, badged, and explained in the picker.
type TierInfo struct {
	Vendor     string     // "Anthropic", "Gemini", "OpenAI", "Other"
	IsLegacy   bool       // true -> grouped below the Legacy divider
	LaunchYM   string     // e.g. "2026-11" for newer models when known
	Descriptor string     // short capability/power tier explanation
	Accent     TierAccent // highlight color for newer models
}

// ModelTierInfo classifies a model by vendor, generation (newer vs legacy),
// launch year-month, capability descriptor, and picker accent color.
func ModelTierInfo(m Model) TierInfo {
	s := strings.ToLower(m.Name + " " + string(m.ID) + " " + m.APIModel)
	ver := 0.0
	if match := agVersionRe.FindString(strings.ToLower(m.Name)); match != "" {
		ver, _ = strconv.ParseFloat(match, 64)
	} else if match := agVersionRe.FindString(strings.ToLower(m.APIModel)); match != "" {
		ver, _ = strconv.ParseFloat(match, 64)
	}

	vendor := "Other"
	switch {
	case strings.Contains(s, "claude") || strings.Contains(s, "opus") || strings.Contains(s, "sonnet") || strings.Contains(s, "haiku") || m.Provider == ProviderAnthropic:
		vendor = "Anthropic"
	case strings.Contains(s, "gemini") || strings.Contains(s, "neon") || strings.Contains(s, "argon") || strings.Contains(s, "hydrogen") || strings.Contains(s, "carbon") || m.Provider == ProviderGemini || m.Provider == ProviderGeminiCA:
		vendor = "Gemini"
	case strings.Contains(s, "gpt") || strings.Contains(s, "o3") || strings.Contains(s, "o4") || strings.Contains(s, "codex") || m.Provider == ProviderOpenAI || m.Provider == ProviderChatGPT:
		vendor = "OpenAI"
	}

	switch vendor {
	case "Anthropic":
		switch {
		case strings.Contains(s, "opus") && ver >= 5.0:
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-10", Descriptor: "flagship max reasoning", Accent: AccentRed}
		case strings.Contains(s, "sonnet") && ver >= 5.0:
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-10", Descriptor: "balanced coding workhorse", Accent: AccentGreen}
		case strings.Contains(s, "haiku") && ver >= 5.0:
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-10", Descriptor: "fast lightweight", Accent: AccentYellow}
		case strings.Contains(s, "opus"):
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy high-tier Claude"}
		case strings.Contains(s, "sonnet"):
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy mid-tier Claude"}
		default:
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy fast Claude"}
		}

	case "Gemini":
		switch {
		case strings.Contains(s, "neon"):
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-11", Descriptor: "flagship next-gen", Accent: AccentCyan}
		case strings.Contains(s, "argon"), strings.Contains(s, "hydrogen"), strings.Contains(s, "carbon"), ver >= 4.0:
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-10", Descriptor: "next-gen multimodal", Accent: AccentCyan}
		case strings.Contains(s, "lite"):
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy lightweight, low cost"}
		case strings.Contains(s, "3.8") && strings.Contains(s, "flash"):
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-10", Descriptor: "fast high-speed workhorse", Accent: AccentGreen}
		case strings.Contains(s, "3.6") && strings.Contains(s, "flash"):
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-08", Descriptor: "fast everyday tier", Accent: AccentYellow}
		case strings.Contains(s, "3.1") && strings.Contains(s, "pro"):
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy 3.1 pro tier"}
		case strings.Contains(s, "2.5") && strings.Contains(s, "pro"):
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy 2.5 pro tier"}
		case ver > 0 && ver < 3.6:
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy Gemini generation"}
		default:
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-10", Descriptor: "current Gemini tier", Accent: AccentYellow}
		}

	case "OpenAI":
		switch {
		case strings.Contains(s, "gpt-oss"):
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy open-weights tier"}
		case strings.Contains(s, "gpt-5") || strings.Contains(s, "o3") || strings.Contains(s, "o4"):
			return TierInfo{Vendor: vendor, IsLegacy: false, LaunchYM: "2026-09", Descriptor: "current reasoning tier", Accent: AccentGreen}
		default:
			return TierInfo{Vendor: vendor, IsLegacy: true, Descriptor: "legacy GPT generation"}
		}
	}

	return TierInfo{Vendor: vendor, IsLegacy: false}
}
