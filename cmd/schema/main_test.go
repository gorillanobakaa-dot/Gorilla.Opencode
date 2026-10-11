package main

// GORILLA FIX (2026-10-10): the schema generator must give the same bytes on
// every run, and the committed file must be what it gives. Before this, the
// model enum followed Go's random map order, so neither held.

import (
	"bytes"
	"encoding/json"
	"os"
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

func TestTheCommittedSchemaMatchesItsGenerator(t *testing.T) {
	committed, err := os.ReadFile("../../opencode-schema.json")
	if err != nil {
		t.Skipf("no committed schema: %v", err)
	}
	if !bytes.Equal(bytes.ReplaceAll(committed, []byte("\r\n"), []byte("\n")), render(t)) {
		t.Fatal("opencode-schema.json is not what cmd/schema generates: run `go run ./cmd/schema > opencode-schema.json` and commit it")
	}
}
