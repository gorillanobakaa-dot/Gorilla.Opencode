package auth

import "testing"

func TestClassifyProbe(t *testing.T) {
	if r := ClassifyProbe(404, `{"error":{"message":"Requested entity was not found."}}`); !r.NotFound || r.OK {
		t.Fatalf("404 must be NotFound, not OK: %+v", r)
	}
	if r := ClassifyProbe(200, "{}"); !r.OK || r.NotFound {
		t.Fatalf("200 must be OK: %+v", r)
	}
	if r := ClassifyProbe(429, ""); !r.OK {
		t.Fatalf("429 keeps the model: %+v", r)
	}
	if r := ClassifyProbe(500, ""); r.OK || r.NotFound {
		t.Fatalf("500 is neither: %+v", r)
	}
}
