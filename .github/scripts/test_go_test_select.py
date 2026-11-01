"""Tests for go_test_select.select (pure function, no git or go needed). Run: python -m unittest discover -s .github/scripts"""
import os
import sys
import unittest

sys.path.insert(0, os.path.dirname(__file__))
from go_test_select import select  # noqa: E402

ROOT = os.path.abspath("repo")
M = "example.com/m"


def pkg(path, imports=(), test=(), xtest=(), embed=()):
    d = ROOT if path == "" else os.path.join(ROOT, *path.split("/"))
    return {"ImportPath": M + ("/" + path if path else ""), "Dir": d, "Imports": [M + "/" + i for i in imports],
            "TestImports": [M + "/" + i for i in test], "XTestImports": [M + "/" + i for i in xtest],
            "EmbedFiles": list(embed)}


PKGS = [pkg(""), pkg("a"), pkg("b", imports=["a"]), pkg("c", imports=["b"]), pkg("d", test=["a"]),
        pkg("e", xtest=["d"]), pkg("f"), pkg("g/h", embed=["prompts/x.md"]), pkg("g")]


class Select(unittest.TestCase):
    def sel(self, *files):
        return select(list(files), PKGS, ROOT)

    def names(self, *files):
        mode, p = self.sel(*files)
        self.assertEqual(mode, "some")
        return [x.replace(M + "/", "") for x in p]

    def test_leaf_package_and_reverse_closure(self):
        self.assertEqual(self.names("a/a.go"), ["a", "b", "c", "d", "e"])

    def test_untouched_branch_not_selected(self):
        self.assertEqual(self.names("f/f.go"), ["f"])

    def test_test_and_xtest_imports_count(self):
        self.assertEqual(self.names("d/d_test.go"), ["d", "e"])

    def test_nested_package_owns_its_files(self):
        self.assertEqual(self.names("g/h/h.go"), ["g/h"])

    def test_non_go_file_in_package_tree(self):
        self.assertEqual(self.names("a/testdata/x.json"), ["a", "b", "c", "d", "e"])

    def test_embedded_file(self):
        self.assertEqual(self.names("g/h/prompts/x.md"), ["g/h"])

    def test_root_package_go_file(self):
        mode, p = self.sel("main.go")
        self.assertEqual((mode, p), ("some", [M]))

    def test_docs_select_nothing(self):
        for f in ("README.md", "docs/x.md", "ReleaseNotes/1.md", "packaging/nfpm.yaml", ".github/CODEOWNERS"):
            self.assertEqual(self.sel(f), ("none", []), f)

    def test_run_everything_files(self):
        for f in ("go.mod", "go.sum", ".github/workflows/ci.yml", ".github/scripts/go-test-annotate.sh",
                  ".github/scripts/go_test_select.py"):
            self.assertEqual(self.sel(f), ("all", []), f)

    def test_go_file_in_unknown_dir_runs_everything(self):
        self.assertEqual(self.sel("gone/old.go"), ("all", []))

    def test_mixed_docs_and_code(self):
        self.assertEqual(self.names("README.md", "f/f.go"), ["f"])

    def test_empty_diff(self):
        self.assertEqual(self.sel(), ("none", []))


if __name__ == "__main__":
    unittest.main()
