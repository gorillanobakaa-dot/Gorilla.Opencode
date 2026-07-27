package tools

import (
	"path/filepath"

	"github.com/opencode-ai/opencode/internal/config"
)

// permissionScope returns the path a permission request should be scoped to for
// a file the agent is about to modify.
//
// When the file lies inside a workspace root, the request is scoped to that ROOT,
// so one "allow for this session" covers the whole workspace instead of prompting
// again in every sub-directory. A file outside every root is scoped to its own
// directory, which keeps the grant as narrow as where the file actually is.
//
// GORILLA OVERRIDE: this replaces four byte-identical copies of
//
//	rootDir := config.WorkingDirectory()
//	permissionPath := filepath.Dir(filePath)
//	if strings.HasPrefix(filePath, rootDir) {
//	    permissionPath = rootDir
//	}
//
// in edit.go (3x) and write.go (1x). Two things were wrong with it.
//
// First, strings.HasPrefix is not a path test — it ignores the component
// boundary. With a working directory of /tmp/foo it matched /tmp/foobar/x.go, so
// editing a file in an unrelated sibling directory was attributed to the
// workspace root. Granting "allow for this session" on what looked like a
// workspace edit then silently covered every later edit anywhere under /tmp/foo,
// and a file the user never considered part of their project widened the scope of
// a grant they had already given. config.RootFor uses filepath.Rel and rejects
// results that escape upward, so a sibling no longer matches.
//
// Second, it only knew about the primary working directory. Additional roots
// added with /add-dir now scope the same way, which is the point of registering
// one: without this, every file under an added root prompted per directory.
func permissionScope(filePath string) string {
	if root, ok := config.RootFor(filePath); ok {
		return root
	}
	return filepath.Dir(filePath)
}
