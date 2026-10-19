package tools

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// Owner rule 2026-10-18: telemetry is cut out of everything we ship. Measured
// 2026-10-19: pfind's --brain semantic opened Chroma with its defaults, which
// load a usage-reporting client (it wrote ~/.cache/chroma/telemetry_user_id on
// 2026-08-25) and download the embedding model from S3 when it is missing.

// Every Chroma client and collection pfind opens goes through the cut-out
// settings and the local-only model. Runs everywhere, chromadb or not.
func TestPfindOpensChromaOnlyWithTheDoorsCut(t *testing.T) {
	src, err := os.ReadFile("pfind.py")
	if err != nil {
		t.Fatal(err)
	}
	s := string(src)
	for _, m := range regexp.MustCompile(`PersistentClient\([^\n]*`).FindAllString(s, -1) {
		if !strings.Contains(m, "settings=Settings(**settings)") {
			t.Errorf("a Chroma client opened without the cut-out settings: %s", m)
		}
	}
	for _, m := range regexp.MustCompile(`get_collection\([^\n]*`).FindAllString(s, -1) {
		if !strings.Contains(m, "embedding_function=LocalModelOnly()") {
			t.Errorf("a collection opened with Chroma's downloading model: %s", m)
		}
	}
	if strings.Count(s, "import chromadb") != 1 {
		t.Error("chromadb must be imported only inside _cut_chroma_doors")
	}
	// pfind.py is embedded in the program: the shipped binary must not carry a
	// telemetry vendor's name, not even in the line that blocks it (2026-10-19:
	// the artifact scan of 0.1.148's first build found exactly that one line).
	if strings.Contains(strings.ToLower(s), "post"+"hog") {
		t.Error("pfind.py names the usage-reporting vendor; the shipped binary would carry it")
	}
}

const chromaDriver = `
import importlib.util, json, os, sys
from pathlib import Path
spec = importlib.util.spec_from_file_location("pfind", sys.argv[1])
pfind = importlib.util.module_from_spec(spec)
sys.argv = ["pfind"]
spec.loader.exec_module(pfind)
store = Path(os.environ["PFIND_TEST_STORE"])
chromadb, settings, LocalModelOnly = pfind._cut_chroma_doors()
from chromadb.config import Settings
c = chromadb.PersistentClient(path=str(store), settings=Settings(**settings))
col = c.create_collection("notes", embedding_function=LocalModelOnly())
col.add(ids=["1"], embeddings=[[0.1] * 384], documents=["d"])
del c, col
pfind.BRAIN_CHROMA = store
out = pfind.semantic_brain("anything", "notes", 1)
print(json.dumps({
    "result": out,
    "client_imported": any(n == "posthog" or n.startswith("posthog.") for n in sys.modules),  # telemetry-ok: the check
    "chroma_client_loaded": "chromadb.telemetry.product.posthog" in sys.modules,  # telemetry-ok: the check
}))
`

// The real chromadb, in a throwaway home: its usage client never loads, no telemetry
// id is written, and a missing model is refused instead of downloaded.
func TestPfindSemanticSendsNothingAndDownloadsNothing(t *testing.T) {
	python := os.Getenv("PFIND_CHROMA_PYTHON")
	if python == "" {
		p, _, err := findPythonExe()
		if err != nil {
			t.Skip("no python interpreter")
		}
		python = p
	}
	if exec.Command(python, "-c", "import chromadb").Run() != nil {
		t.Skipf("%s has no chromadb (set PFIND_CHROMA_PYTHON to one that does)", python)
	}
	home := t.TempDir()
	driver := filepath.Join(t.TempDir(), "driver.py")
	if err := os.WriteFile(driver, []byte(chromaDriver), 0o644); err != nil {
		t.Fatal(err)
	}
	src, _ := filepath.Abs("pfind.py")
	cmd := exec.Command(python, driver, src)
	// A dead local proxy: if a regression tries the network, it fails here and
	// nothing leaves the machine.
	cmd.Env = append(os.Environ(), "HOME="+home, "USERPROFILE="+home,
		"PFIND_TEST_STORE="+filepath.Join(home, "store"), "ANONYMIZED_TELEMETRY=True",
		"HTTP_PROXY=http://127.0.0.1:9", "HTTPS_PROXY=http://127.0.0.1:9", "ALL_PROXY=http://127.0.0.1:9",
		"NO_PROXY=", "no_proxy=")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("driver failed: %v\n%s", err, stderr.String())
	}
	got := string(out)
	if !strings.Contains(got, `"client_imported": false`) || !strings.Contains(got, `"chroma_client_loaded": false`) {
		t.Errorf("Chroma's usage-reporting client was reachable: %s", got)
	}
	if !strings.Contains(stderr.String(), "pfind never downloads") {
		t.Errorf("a missing model must be refused, not fetched; stderr: %s", stderr.String())
	}
	if _, err := os.Stat(filepath.Join(home, ".cache", "chroma", "telemetry_user_id")); err == nil {
		t.Error("telemetry_user_id was written")
	}
	if _, err := os.Stat(filepath.Join(home, ".cache", "chroma", "onnx_models", "all-MiniLM-L6-v2", "onnx.tar.gz")); err == nil {
		t.Error("the embedding model was downloaded")
	}
}
