package main

// GORILLA FIX (2026-10-10): the schema generator must give the same bytes on
// every run, and the committed file must be what it gives. Before this, the
// model enum followed Go's random map order, so neither held.

import (
	"bytes"
	"encoding/json"
	"testing"
)

func render(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	if err := enc.Encode(generateSchema()); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestTheSchemaIsTheSameOnEveryRun(t *testing.T) {
	first := render(t)
	for i := 0; i < 5; i++ {
		if !bytes.Equal(render(t), first) {
			t.Fatal("two runs of the schema generator gave different output")
		}
	}
}
