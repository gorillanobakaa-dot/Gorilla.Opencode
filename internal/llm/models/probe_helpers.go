package models

// PreferredOnSameEndpoint names the model to fall back on when id, served by a
// configured endpoint, has been found dead: the endpoint's own best candidate
// (see candidateOrder). "" when id is not an endpoint model or nothing else is
// registered there.
func PreferredOnSameEndpoint(id ModelID) ModelID {
	r, ok := localRoute[id]
	if !ok {
		return ""
	}
	var ids []ModelID
	for other, o := range localRoute {
		if o.Endpoint == r.Endpoint && other != id {
			ids = append(ids, other)
		}
	}
	return preferredChatModel(sortedIDs(ids))
}

// PreferredOnEndpoint is the default model for a named endpoint.
func PreferredOnEndpoint(name string) ModelID {
	var ids []ModelID
	for id, r := range localRoute {
		if r.Endpoint == name {
			ids = append(ids, id)
		}
	}
	return preferredChatModel(sortedIDs(ids))
}

func sortedIDs(ids []ModelID) []ModelID {
	for i := 1; i < len(ids); i++ {
		for j := i; j > 0 && ids[j] < ids[j-1]; j-- {
			ids[j], ids[j-1] = ids[j-1], ids[j]
		}
	}
	return ids
}

// EndpointWasProbed reports whether any model of a named endpoint has a
// recorded answer.
func EndpointWasProbed(endpoint string) bool {
	for id, r := range localRoute {
		if r.Endpoint != endpoint {
			continue
		}
		if _, ok := ProbeVerdictFor(id); ok {
			return true
		}
	}
	return false
}

// EndpointHasModels reports whether any registered model is routed through the
// named endpoint.
func EndpointHasModels(endpoint string) bool {
	for _, r := range localRoute {
		if r.Endpoint == endpoint {
			return true
		}
	}
	return false
}
