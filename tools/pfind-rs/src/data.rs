// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Tables carried over unchanged from pfind.py 3.3.0: default excludes, the
// language types (ripgrep's default_types.rs subset pfind has always used),
// the comment syntax table for LOC counting and the Nerd Font icons. pfind.py
// wrote these itself (they are facts about file types, not copied code).

pub const EXCLUDE_GLOBS: &[&str] = &[
    ".git", ".hg", ".svn", "node_modules", "__pycache__",
    ".mozbuild", "venv", ".venv", "vector_env",
    // NOT "build"/"dist": real source lives in folders with those names.
    "obj-*",
    "chroma_db", "chroma_fx154",
    "*.sqlite3", "*.bin", "*.parquet", "*.pyc",
    "*.min.js", "*.map",
];

pub fn language_types() -> Vec<(String, Vec<String>)> {
    let t: &[(&str, &[&str])] = &[
        ("c", &[".c", ".h"]),
        ("cpp", &[".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".h", ".C", ".H"]),
        ("rust", &[".rs"]),
        ("python", &[".py", ".pyi", ".pyx"]),
        ("py", &[".py", ".pyi", ".pyx"]),
        ("go", &[".go"]),
        ("javascript", &[".js", ".jsx", ".mjs", ".cjs"]),
        ("js", &[".js", ".jsx", ".mjs", ".cjs"]),
        ("typescript", &[".ts", ".tsx", ".mts", ".cts"]),
        ("ts", &[".ts", ".tsx", ".mts", ".cts"]),
        ("html", &[".html", ".htm", ".xhtml"]),
        ("css", &[".css", ".scss", ".sass", ".less"]),
        ("json", &[".json", ".jsonc", ".jsonl"]),
        ("yaml", &[".yaml", ".yml"]),
        ("toml", &[".toml"]),
        ("markdown", &[".md", ".markdown", ".mdown", ".mkdn"]),
        ("md", &[".md", ".markdown"]),
        ("sh", &[".sh", ".bash", ".zsh", ".ksh"]),
        ("bash", &[".sh", ".bash", ".zsh"]),
        ("java", &[".java", ".jsp"]),
        ("kotlin", &[".kt", ".kts"]),
        ("swift", &[".swift"]),
        ("ruby", &[".rb", ".rake", ".gemspec"]),
        ("php", &[".php", ".phtml", ".php3", ".php4", ".php5"]),
        ("perl", &[".pl", ".pm", ".t"]),
        ("lua", &[".lua"]),
        ("zig", &[".zig"]),
        ("nim", &[".nim", ".nims"]),
        ("scala", &[".scala", ".sc"]),
        ("sql", &[".sql"]),
        ("xml", &[".xml", ".xsl", ".xslt", ".svg", ".plist"]),
        ("docker", &["Dockerfile", ".dockerignore"]),
        ("make", &["Makefile", "makefile", ".mk", ".mak"]),
        ("cmake", &["CMakeLists.txt", ".cmake"]),
        ("assembly", &[".s", ".S", ".asm"]),
        ("asm", &[".s", ".S", ".asm"]),
    ];
    t.iter()
        .map(|(k, v)| (k.to_string(), v.iter().map(|s| s.to_string()).collect()))
        .collect()
}

pub struct Syntax {
    pub name: &'static str,
    pub line: &'static [&'static str],
    pub block: &'static [(&'static str, &'static str)],
    pub exts: &'static [&'static str],
    pub files: &'static [&'static str],
}

pub static SYNTAX_TABLE: &[Syntax] = &[
    Syntax { name: "C/C++", line: &["//"], block: &[("/*", "*/")], exts: &[".c", ".h", ".cpp", ".cc", ".cxx", ".hpp", ".hh", ".hxx", ".C", ".H"], files: &[] },
    Syntax { name: "Rust", line: &["//"], block: &[("/*", "*/")], exts: &[".rs"], files: &[] },
    Syntax { name: "Python", line: &["#"], block: &[("\"\"\"", "\"\"\""), ("'''", "'''")], exts: &[".py", ".pyi", ".pyx", ".pyw"], files: &[] },
    Syntax { name: "Go", line: &["//"], block: &[("/*", "*/")], exts: &[".go"], files: &[] },
    Syntax { name: "JavaScript/TypeScript", line: &["//"], block: &[("/*", "*/")], exts: &[".js", ".jsx", ".mjs", ".cjs", ".ts", ".tsx", ".mts", ".cts"], files: &[] },
    Syntax { name: "Shell", line: &["#"], block: &[], exts: &[".sh", ".bash", ".zsh", ".ksh", ".ebuild", ".eclass"], files: &[] },
    Syntax { name: "HTML/XML", line: &[], block: &[("<!--", "-->")], exts: &[".html", ".htm", ".xhtml", ".xml", ".svg", ".plist"], files: &[] },
    Syntax { name: "CSS", line: &["//"], block: &[("/*", "*/")], exts: &[".css", ".scss", ".sass", ".less"], files: &[] },
    Syntax { name: "JSON", line: &["//"], block: &[("/*", "*/")], exts: &[".json", ".jsonc"], files: &[] },
    Syntax { name: "YAML/TOML", line: &["#"], block: &[], exts: &[".yaml", ".yml", ".toml"], files: &[] },
    Syntax { name: "Markdown", line: &[], block: &[("<!--", "-->")], exts: &[".md", ".markdown"], files: &[] },
    Syntax { name: "Java/Kotlin", line: &["//"], block: &[("/*", "*/")], exts: &[".java", ".kt", ".kts"], files: &[] },
    Syntax { name: "Swift", line: &["//"], block: &[("/*", "*/")], exts: &[".swift"], files: &[] },
    Syntax { name: "Ruby", line: &["#"], block: &[("=begin", "=end")], exts: &[".rb", ".rake", ".gemspec"], files: &[] },
    Syntax { name: "PHP", line: &["//", "#"], block: &[("/*", "*/")], exts: &[".php", ".phtml"], files: &[] },
    Syntax { name: "Perl", line: &["#"], block: &[("=pod", "=cut")], exts: &[".pl", ".pm", ".t"], files: &[] },
    Syntax { name: "Lua", line: &["--"], block: &[("--[[", "]]")], exts: &[".lua"], files: &[] },
    Syntax { name: "Zig", line: &["//"], block: &[], exts: &[".zig"], files: &[] },
    Syntax { name: "Nim", line: &["#"], block: &[("#[", "]#")], exts: &[".nim", ".nims"], files: &[] },
    Syntax { name: "SQL", line: &["--"], block: &[("/*", "*/")], exts: &[".sql"], files: &[] },
    Syntax { name: "Makefile", line: &["#"], block: &[], exts: &[".mk", ".mak"], files: &["Makefile", "makefile", "GNUmakefile"] },
    Syntax { name: "CMake", line: &["#"], block: &[("#[[", "]]")], exts: &[".cmake"], files: &["CMakeLists.txt"] },
    Syntax { name: "Assembly", line: &[";", "//", "#", "@"], block: &[("/*", "*/")], exts: &[".s", ".S", ".asm"], files: &[] },
];

/// pfind.py built EXT_TO_SYNTAX by iterating the table in order, so a later
/// entry wins for a shared lowercase extension (".s" -> Assembly; ".h" -> C/C++).
pub fn syntax_for(path: &str) -> Option<&'static Syntax> {
    let fname = crate::util::basename(path).to_lowercase();
    let ext = crate::util::splitext_ext(&fname).to_lowercase();
    let mut by_file = None;
    let mut by_ext = None;
    for syn in SYNTAX_TABLE {
        if syn.files.iter().any(|f| f.to_lowercase() == fname) {
            by_file = Some(syn);
        }
        if !ext.is_empty() && syn.exts.iter().any(|e| e.to_lowercase() == ext) {
            by_ext = Some(syn);
        }
    }
    by_file.or(by_ext)
}

pub fn icon_for(path: &str, is_dir: bool) -> &'static str {
    if is_dir {
        return "\u{e5ff}";
    }
    let fname = crate::util::basename(path).to_lowercase();
    if let Some(i) = icon_lookup(&fname) {
        return i;
    }
    let ext = crate::util::splitext_ext(&fname).to_lowercase();
    icon_lookup(&ext).unwrap_or("\u{f15b}")
}

fn icon_lookup(key: &str) -> Option<&'static str> {
    Some(match key {
        ".rs" => "\u{e68b}", ".py" => "\u{e606}", ".c" => "\u{e61e}", ".h" => "\u{e61e}",
        ".cpp" => "\u{e61d}", ".hpp" => "\u{e61d}", ".go" => "\u{e627}", ".js" => "\u{e74e}",
        ".ts" => "\u{e628}", ".tsx" => "\u{e7ba}", ".jsx" => "\u{e7ba}", ".html" => "\u{e736}",
        ".css" => "\u{e749}", ".scss" => "\u{e749}", ".json" => "\u{e60b}", ".yaml" => "\u{e6a8}",
        ".yml" => "\u{e6a8}", ".toml" => "\u{e6b2}", ".md" => "\u{e609}", ".sh" => "\u{e795}",
        ".bash" => "\u{e795}", ".zsh" => "\u{e795}", ".java" => "\u{e738}", ".kt" => "\u{e634}",
        ".swift" => "\u{e755}", ".rb" => "\u{e739}", ".php" => "\u{e73d}", ".lua" => "\u{e620}",
        ".zig" => "\u{e6a9}", ".nim" => "\u{e677}", ".sql" => "\u{e706}", ".xml" => "\u{e796}",
        ".tar" => "\u{f410}", ".gz" => "\u{f410}", ".zip" => "\u{f410}", ".xz" => "\u{f410}",
        ".zst" => "\u{f410}", ".png" => "\u{f1c5}", ".jpg" => "\u{f1c5}", ".jpeg" => "\u{f1c5}",
        ".svg" => "\u{f1c5}", ".pdf" => "\u{f1c1}", ".deb" => "\u{e77d}", ".lock" => "\u{f023}",
        "makefile" => "\u{e660}", "cmakelists.txt" => "\u{e660}", "dockerfile" => "\u{e650}",
        "license" => "\u{f02d}", "readme.md" => "\u{f02d}", ".gitignore" => "\u{e725}",
        "dir" => "\u{e5ff}", "file" => "\u{f15b}", "symlink" => "\u{f0c1}", "binary" => "\u{eae8}",
        _ => return None,
    })
}
