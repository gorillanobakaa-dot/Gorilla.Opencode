package config

import "fmt"

// GORILLA OVERRIDE: a personal shortlist of models, kept across providers.
//
// WHY THIS EXISTS
//
// The picker can now offer hundreds of models - 333 from OpenRouter alone, 128
// from NVIDIA NIM - with names like "inclusionai/ling-3.0-tiny:free". A list
// that long is not a choice, it is a wall, and there is no search in the picker.
//
// The deeper cost is bandwidth, and it is the reason this is not a nicety. To
// choose between unfamiliar model names you would have to go and read about
// them: a search per name, then a product page, and vendor model pages are
// heavy. On the connection this project is built for - single digit KB/s - that
// research is not slow, it is impossible. The curated descriptions already in
// the picker exist to spare people that, and a shortlist is what makes the
// saving durable: you decide once, and never scroll the catalogue again.
//
// Stored as model IDs rather than indexes or names, so a refreshed catalogue,
// a reordered list or a renamed model cannot silently point a bookmark at
// something else. An ID that no longer resolves is shown as unavailable rather
// than dropped - a bookmark vanishing without explanation is the same silent
// failure this project keeps hunting.
//
// No cap. There are 14 free tool-capable models on OpenRouter today, so a limit
// of 100 would be a number nobody could reach and someone would have to
// maintain. The list is self-limiting: people bookmark what they use.

// BookmarkedModels returns the saved shortlist, oldest first. Order is the order
// they were added, which is the only order the user authored.
func BookmarkedModels() []string {
	if cfg == nil {
		return nil
	}
	out := make([]string, len(cfg.BookmarkedModels))
	copy(out, cfg.BookmarkedModels)
	return out
}

// IsBookmarked reports whether a model id is on the shortlist.
func IsBookmarked(id string) bool {
	if cfg == nil {
		return false
	}
	for _, b := range cfg.BookmarkedModels {
		if b == id {
			return true
		}
	}
	return false
}

// ToggleBookmark adds or removes a model, returning the new state. Deselection
// matters as much as selection here: "this one did not do what it claimed" is
// exactly as useful a judgement as "this one is good", and a list you can only
// add to becomes another wall.
func ToggleBookmark(id string) (nowBookmarked bool, err error) {
	if cfg == nil {
		return false, fmt.Errorf("config not loaded")
	}
	if id == "" {
		return false, fmt.Errorf("empty model id")
	}

	remove := IsBookmarked(id)
	apply := func(c *Config) {
		var next []string
		for _, b := range c.BookmarkedModels {
			if b != id {
				next = append(next, b)
			}
		}
		if !remove {
			next = append(next, id)
		}
		c.BookmarkedModels = next
	}

	apply(cfg)
	if err := updateCfgFile(apply); err != nil {
		return !remove, err
	}
	return !remove, nil
}
