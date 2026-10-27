// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Command-line parsing that behaves like pfind.py's argparse parser: the same
// flag names, the same short forms, unique-prefix abbreviation of long flags,
// `--` ends the options, nargs="+" for --ext/--exclude, nargs="?" for
// --loc/--code/--absolute, -u counted, short flags bundled (-uu, -C2).
// Errors exit 2 with "pfind: error: ...", as argparse does.

use std::process;

#[derive(Clone, Copy, PartialEq)]
enum Kind {
    Flag,
    Count,
    Int,
    OptInt,
    Str,
    Append,
    Plus,
    Opt(&'static str), // nargs="?" with this const
}

struct Spec {
    flags: &'static [&'static str],
    dest: &'static str,
    kind: Kind,
    choices: &'static [&'static str],
}

const NONE: &[&str] = &[];
const ENGINES: &[&str] = &["default", "pcre2", "auto"];
const LOCMODES: &[&str] = &["lines", "percent", "both"];
const SCALE: &[&str] = &["fixed", "gradient"];
const TIMES: &[&str] = &["modified", "accessed", "changed", "created"];
const ABS: &[&str] = &["on", "off", "follow"];
const HLF: &[&str] = &["file", "vscode", "grep+"];
const COLORS: &[&str] = &["never", "auto", "always", "ansi"];
pub const SORT_FIELDS: &[&str] = &[
    "name", "Name", ".name", ".Name", "size", "ext", "Ext", "type", "inode", "date", "time",
    "modified", "changed", "accessed", "created", "age", "old", "newest", "path", "none",
];

macro_rules! s {
    ($f:expr, $d:expr, $k:expr) => {
        Spec { flags: $f, dest: $d, kind: $k, choices: NONE }
    };
    ($f:expr, $d:expr, $k:expr, $c:expr) => {
        Spec { flags: $f, dest: $d, kind: $k, choices: $c }
    };
}

use Kind::*;

static SPECS: &[Spec] = &[
    s!(&["-h", "--help"], "help", Flag),
    s!(&["--version"], "version", Flag),
    s!(&["--query-file"], "query_file", Str),
    s!(&["--doctor"], "doctor", Flag),
    s!(&["--json"], "json", Flag),
    s!(&["--brain"], "brain", Flag),
    s!(&["--work"], "work", Flag),
    s!(&["--src"], "src", Flag),
    s!(&["--all"], "all", Flag),
    s!(&["-r", "--regex"], "regex", Flag),
    s!(&["-F", "--fixed-strings"], "fixed_strings", Flag),
    s!(&["-i", "--ignore-case"], "ignore_case", Flag),
    s!(&["-s", "--case-sensitive"], "case_sensitive", Flag),
    s!(&["-S", "--smart-case"], "smart_case", Flag),
    s!(&["-w", "--word-regexp"], "word_regexp", Flag),
    s!(&["-x", "--exact"], "exact", Flag),
    s!(&["--loose"], "loose", Flag),
    s!(&["--fuzzy"], "fuzzy", Flag),
    s!(&["-v", "--invert-match"], "invert_match", Flag),
    s!(&["--pcre2"], "pcre2", Flag),
    s!(&["--engine"], "engine", Str, ENGINES),
    s!(&["-U", "--multiline"], "multiline", Flag),
    s!(&["--multiline-dotall"], "multiline_dotall", Flag),
    s!(&["--line-regexp"], "line_regexp", Flag),
    s!(&["-E", "--encoding"], "encoding", Str),
    s!(&["--no-unicode"], "no_unicode", Flag),
    s!(&["--crlf"], "crlf", Flag),
    s!(&["--null-data"], "null_data", Flag),
    s!(&["--stop-on-nonmatch"], "stop_on_nonmatch", Flag),
    s!(&["-C", "--context"], "context", Int),
    s!(&["-A", "--after-context"], "after_context", Int),
    s!(&["-B", "--before-context"], "before_context", Int),
    s!(&["--context-separator"], "context_separator", Str),
    s!(&["--no-context-separator"], "no_context_separator", Flag),
    s!(&["--field-context-separator"], "field_context_separator", Str),
    s!(&["--field-match-separator"], "field_match_separator", Str),
    s!(&["--path-separator"], "path_separator", Str),
    s!(&["-o", "--only-matching"], "only_matching", Flag),
    s!(&["--replace"], "replace", Str),
    s!(&["-m", "--max-count"], "max_count", Int),
    s!(&["-c", "--count-lines"], "count_lines", Flag),
    s!(&["--count-matches"], "count_matches", Flag),
    s!(&["--include-zero"], "include_zero", Flag),
    s!(&["-M", "--max-columns"], "max_columns", Int),
    s!(&["--max-columns-preview"], "max_columns_preview", Flag),
    s!(&["-b", "--byte-offset"], "byte_offset", Flag),
    s!(&["-H", "--with-filename"], "with_filename", Flag),
    s!(&["--no-filename"], "no_filename", Flag),
    s!(&["--trim"], "trim", Flag),
    s!(&["--passthru", "--passthrough"], "passthru", Flag),
    s!(&["--colors"], "color_specs", Append),
    s!(&["--hostname-bin"], "hostname_bin", Str),
    s!(&["-t", "--type"], "types", Append),
    s!(&["--type-not"], "types_not", Append),
    s!(&["--type-add"], "type_adds", Append),
    s!(&["--type-clear"], "type_clears", Append),
    s!(&["--type-list"], "type_list", Flag),
    s!(&["--pre"], "pre", Str),
    s!(&["--pre-glob"], "pre_glob", Str),
    s!(&["--ext"], "ext", Plus),
    s!(&["--exclude"], "exclude", Plus),
    s!(&["-g", "--glob"], "globs", Append),
    s!(&["--iglob"], "iglobs", Append),
    s!(&["--glob-case-insensitive"], "glob_case_insensitive", Flag),
    s!(&["--ignore-file"], "ignore_files", Append),
    s!(&["--ignore-file-case-insensitive"], "ignore_file_case_insensitive", Flag),
    s!(&["-e", "--regexp"], "patterns", Append),
    s!(&["-f", "--pattern-file"], "pattern_file", Str),
    s!(&["-z", "--search-zip"], "search_zip", Flag),
    s!(&["-a", "--text"], "text", Flag),
    s!(&["--binary"], "binary", Flag),
    s!(&["--hidden"], "hidden", Flag),
    s!(&["--no-ignore"], "no_ignore", Flag),
    s!(&["--no-ignore-dot"], "no_ignore_dot", Flag),
    s!(&["--no-ignore-global"], "no_ignore_global", Flag),
    s!(&["--no-ignore-parent"], "no_ignore_parent", Flag),
    s!(&["--no-ignore-vcs"], "no_ignore_vcs", Flag),
    s!(&["--no-ignore-exclude"], "no_ignore_exclude", Flag),
    s!(&["--no-ignore-messages"], "no_ignore_messages", Flag),
    s!(&["--no-require-git"], "no_require_git", Flag),
    s!(&["--git-ignore"], "git_ignore", Flag),
    s!(&["-u", "--unrestricted"], "unrestricted", Count),
    s!(&["-d", "--max-depth"], "max_depth", OptInt),
    s!(&["--max-filesize"], "max_filesize", Str),
    s!(&["--one-file-system"], "one_file_system", Flag),
    s!(&["--mmap"], "mmap", Flag),
    s!(&["--no-mmap"], "no_mmap", Flag),
    s!(&["--block-buffered"], "block_buffered", Flag),
    s!(&["--line-buffered"], "line_buffered", Flag),
    s!(&["--dfa-size-limit"], "dfa_size_limit", Str),
    s!(&["--regex-size-limit"], "regex_size_limit", Str),
    s!(&["--no-config"], "no_config", Flag),
    s!(&["-l", "--long"], "long", Flag),
    s!(&["-T", "--tree"], "tree", Flag),
    s!(&["--level"], "level", OptInt),
    s!(&["-R", "--recurse"], "recurse", Flag),
    s!(&["-G", "--grid"], "grid", Flag),
    s!(&["-1", "--oneline"], "oneline", Flag),
    s!(&["--across"], "across", Flag),
    s!(&["--width"], "width", OptInt),
    s!(&["--loc"], "loc", Opt("both"), LOCMODES),
    s!(&["--code"], "code", Opt("both"), LOCMODES),
    s!(&["--git"], "git", Flag),
    s!(&["--no-git"], "no_git", Flag),
    s!(&["--git-repos"], "git_repos", Flag),
    s!(&["--git-repos-no-status"], "git_repos_no_status", Flag),
    s!(&["--git-dirty"], "git_dirty", Flag),
    s!(&["--dirty-boost"], "dirty_boost", Flag),
    s!(&["--recency-boost"], "recency_boost", Flag),
    s!(&["--color-scale"], "color_scale", Flag),
    s!(&["--color-scale-mode"], "color_scale_mode", Str, SCALE),
    s!(&["--total-size"], "total_size", Flag),
    s!(&["--time"], "time", Str, TIMES),
    s!(&["--changed"], "changed", Flag),
    s!(&["--accessed"], "accessed", Flag),
    s!(&["--modified"], "modified", Flag),
    s!(&["--created"], "created", Flag),
    s!(&["--time-style"], "time_style", Str),
    s!(&["--octal-permissions"], "octal_permissions", Flag),
    s!(&["--blocks"], "blocks", Flag),
    s!(&["--inode"], "inode", Flag),
    s!(&["--links"], "links", Flag),
    s!(&["--metric-units"], "metric_units", Flag),
    s!(&["--binary-units"], "binary_units", Flag),
    s!(&["--bytes"], "bytes", Flag),
    s!(&["--header"], "header", Flag),
    s!(&["--no-permissions"], "no_permissions", Flag),
    s!(&["--no-filesize"], "no_filesize", Flag),
    s!(&["--no-user"], "no_user", Flag),
    s!(&["--no-time"], "no_time", Flag),
    s!(&["--group"], "group", Flag),
    s!(&["--smart-group"], "smart_group", Flag),
    s!(&["--numeric"], "numeric", Flag),
    s!(&["--extended"], "extended", Flag),
    s!(&["--flags"], "flags", Flag),
    s!(&["--mounts"], "mounts", Flag),
    s!(&["--dereference"], "dereference", Flag),
    s!(&["--classify"], "classify", Flag),
    s!(&["--absolute"], "absolute", Opt("on"), ABS),
    s!(&["--no-quotes"], "no_quotes", Flag),
    s!(&["--short-nix"], "short_nix", Flag),
    s!(&["--stdin"], "stdin", Flag),
    s!(&["--treat-dirs-as-files"], "treat_dirs_as_files", Flag),
    s!(&["--icons"], "icons", Flag),
    s!(&["--hyperlink"], "hyperlink", Flag),
    s!(&["--hyperlink-format"], "hyperlink_format", Str, HLF),
    s!(&["--dirs-first"], "dirs_first", Flag),
    s!(&["--dirs-last"], "dirs_last", Flag),
    s!(&["-D", "--only-dirs"], "only_dirs", Flag),
    s!(&["--only-files"], "only_files", Flag),
    s!(&["--show-symlinks"], "show_symlinks", Flag),
    s!(&["--no-symlinks"], "no_symlinks", Flag),
    s!(&["--follow-symlinks"], "follow_symlinks", Flag),
    s!(&["--almost-all"], "almost_all", Flag),
    s!(&["-I", "--ignore-glob"], "ignore_glob", Str),
    s!(&["--reverse"], "reverse", Flag),
    s!(&["--names"], "names", Flag),
    s!(&["--content"], "content", Flag),
    s!(&["--files"], "files", Flag),
    s!(&["--files-with-matches"], "files_with_matches", Flag),
    s!(&["--files-without-match"], "files_without_match", Flag),
    s!(&["--files-only"], "files_only", Flag),
    s!(&["-q", "--quiet"], "quiet", Flag),
    s!(&["--no-messages"], "no_messages", Flag),
    s!(&["--count"], "count", Flag),
    s!(&["--stats"], "stats", Flag),
    s!(&["--column"], "column", Flag),
    s!(&["-n", "--line-number"], "line_number", Flag),
    s!(&["-N", "--no-line-number"], "no_line_number", Flag),
    s!(&["--heading"], "heading", Flag),
    s!(&["--no-heading"], "no_heading", Flag),
    s!(&["--vimgrep"], "vimgrep", Flag),
    s!(&["-p", "--pretty"], "pretty", Flag),
    s!(&["-0", "--null"], "null", Flag),
    s!(&["--sort"], "sort", Str, SORT_FIELDS),
    s!(&["--sortr"], "sortr", Str, SORT_FIELDS),
    s!(&["--color"], "color", Str, COLORS),
    s!(&["--workers"], "workers", Int),
    s!(&["--timeout"], "timeout", Int),
    s!(&["--rrf-k"], "rrf_k", Int),
    s!(&["--max"], "max", Int),
    s!(&["--limit"], "limit", Int),
    s!(&["--collection"], "collection", Str),
    s!(&["--no-color"], "no_color", Flag),
];

#[derive(Default, Clone)]
pub struct Args {
    pub query: Option<String>,
    pub paths: Vec<String>,
    pub help: bool,
    pub version: bool,
    pub query_file: Option<String>,
    pub doctor: bool,
    pub json: bool,
    pub brain: bool,
    pub work: bool,
    pub src: bool,
    pub all: bool,
    pub regex: bool,
    pub fixed_strings: bool,
    pub ignore_case: bool,
    pub case_sensitive: bool,
    pub smart_case: bool,
    pub word_regexp: bool,
    pub exact: bool,
    pub loose: bool,
    pub fuzzy: bool,
    pub invert_match: bool,
    pub pcre2: bool,
    pub engine: String,
    pub multiline: bool,
    pub multiline_dotall: bool,
    pub line_regexp: bool,
    pub encoding: Option<String>,
    pub no_unicode: bool,
    pub crlf: bool,
    pub null_data: bool,
    pub stop_on_nonmatch: bool,
    pub context: i64,
    pub after_context: i64,
    pub before_context: i64,
    pub context_separator: String,
    pub no_context_separator: bool,
    pub field_context_separator: String,
    pub field_match_separator: String,
    pub path_separator: Option<String>,
    pub only_matching: bool,
    pub replace: Option<String>,
    pub max_count: i64,
    pub count_lines: bool,
    pub count_matches: bool,
    pub include_zero: bool,
    pub max_columns: i64,
    pub max_columns_preview: bool,
    pub byte_offset: bool,
    pub with_filename: bool,
    pub no_filename: bool,
    pub trim: bool,
    pub passthru: bool,
    pub color_specs: Vec<String>,
    pub hostname_bin: Option<String>,
    pub types: Vec<String>,
    pub types_not: Vec<String>,
    pub type_adds: Vec<String>,
    pub type_clears: Vec<String>,
    pub type_list: bool,
    pub pre: Option<String>,
    pub pre_glob: Option<String>,
    pub ext: Vec<String>,
    pub exclude: Vec<String>,
    pub globs: Vec<String>,
    pub iglobs: Vec<String>,
    pub glob_case_insensitive: bool,
    pub ignore_files: Vec<String>,
    pub ignore_file_case_insensitive: bool,
    pub patterns: Vec<String>,
    pub pattern_file: Option<String>,
    pub search_zip: bool,
    pub text: bool,
    pub binary: bool,
    pub hidden: bool,
    pub no_ignore: bool,
    pub no_ignore_dot: bool,
    pub no_ignore_global: bool,
    pub no_ignore_parent: bool,
    pub no_ignore_vcs: bool,
    pub no_ignore_exclude: bool,
    pub no_ignore_messages: bool,
    pub no_require_git: bool,
    pub git_ignore: bool,
    pub unrestricted: i64,
    pub max_depth: Option<i64>,
    pub max_filesize: Option<String>,
    pub one_file_system: bool,
    pub mmap: bool,
    pub no_mmap: bool,
    pub block_buffered: bool,
    pub line_buffered: bool,
    pub dfa_size_limit: Option<String>,
    pub regex_size_limit: Option<String>,
    pub no_config: bool,
    pub long: bool,
    pub tree: bool,
    pub level: Option<i64>,
    pub recurse: bool,
    pub grid: bool,
    pub oneline: bool,
    pub across: bool,
    pub width: Option<i64>,
    pub loc: Option<String>,
    pub code: Option<String>,
    pub git: bool,
    pub no_git: bool,
    pub git_repos: bool,
    pub git_repos_no_status: bool,
    pub git_dirty: bool,
    pub dirty_boost: bool,
    pub recency_boost: bool,
    pub color_scale: bool,
    pub color_scale_mode: String,
    pub total_size: bool,
    pub time: String,
    pub changed: bool,
    pub accessed: bool,
    pub modified: bool,
    pub created: bool,
    pub time_style: String,
    pub octal_permissions: bool,
    pub blocks: bool,
    pub inode: bool,
    pub links: bool,
    pub metric_units: bool,
    pub binary_units: bool,
    pub bytes: bool,
    pub header: bool,
    pub no_permissions: bool,
    pub no_filesize: bool,
    pub no_user: bool,
    pub no_time: bool,
    pub group: bool,
    pub smart_group: bool,
    pub numeric: bool,
    pub extended: bool,
    pub flags: bool,
    pub mounts: bool,
    pub dereference: bool,
    pub classify: bool,
    pub absolute: String,
    pub no_quotes: bool,
    pub short_nix: bool,
    pub stdin: bool,
    pub treat_dirs_as_files: bool,
    pub icons: bool,
    pub hyperlink: bool,
    pub hyperlink_format: String,
    pub dirs_first: bool,
    pub dirs_last: bool,
    pub only_dirs: bool,
    pub only_files: bool,
    pub show_symlinks: bool,
    pub no_symlinks: bool,
    pub follow_symlinks: bool,
    pub almost_all: bool,
    pub ignore_glob: Option<String>,
    pub reverse: bool,
    pub names: bool,
    pub content: bool,
    pub files: bool,
    pub files_with_matches: bool,
    pub files_without_match: bool,
    pub files_only: bool,
    pub quiet: bool,
    pub no_messages: bool,
    pub count: bool,
    pub stats: bool,
    pub column: bool,
    pub line_number: bool,
    pub no_line_number: bool,
    pub heading: bool,
    pub no_heading: bool,
    pub vimgrep: bool,
    pub pretty: bool,
    pub null: bool,
    pub sort: Option<String>,
    pub sortr: Option<String>,
    pub color: String,
    pub workers: i64,
    pub timeout: i64,
    pub rrf_k: i64,
    pub max: i64,
    pub limit: i64,
    pub collection: String,
    pub no_color: bool,
}

pub fn usage() -> &'static str {
    "usage: pfind [options] [query] [paths ...]"
}

pub fn fail(msg: &str) -> ! {
    eprintln!("{}\npfind: error: {}", usage(), msg);
    process::exit(2);
}

impl Args {
    fn defaults(cpus: i64) -> Args {
        Args {
            engine: "default".into(),
            context_separator: "--".into(),
            field_context_separator: "-".into(),
            field_match_separator: ":".into(),
            git: true,
            color_scale_mode: "fixed".into(),
            time: "modified".into(),
            time_style: "relative".into(),
            absolute: "off".into(),
            hyperlink_format: "file".into(),
            line_number: true,
            color: "auto".into(),
            workers: cpus,
            timeout: 600,
            rrf_k: 60,
            max: 5,
            limit: 40,
            collection: "core_memory".into(),
            ..Default::default()
        }
    }

    fn set_flag(&mut self, dest: &str) {
        macro_rules! flags {
            ($($f:ident),*) => {
                match dest {
                    $(stringify!($f) => self.$f = true,)*
                    "unrestricted" => self.unrestricted += 1,
                    _ => unreachable!("flag {}", dest),
                }
            };
        }
        flags!(help, version, doctor, json, brain, work, src, all, regex, fixed_strings, ignore_case,
            case_sensitive, smart_case, word_regexp, exact, loose, fuzzy, invert_match, pcre2,
            multiline, multiline_dotall, line_regexp, no_unicode, crlf, null_data, stop_on_nonmatch,
            no_context_separator, only_matching, count_lines, count_matches, include_zero,
            max_columns_preview, byte_offset, with_filename, no_filename, trim, passthru, type_list,
            glob_case_insensitive, ignore_file_case_insensitive, search_zip, text, binary, hidden,
            no_ignore, no_ignore_dot, no_ignore_global, no_ignore_parent, no_ignore_vcs,
            no_ignore_exclude, no_ignore_messages, no_require_git, git_ignore, one_file_system, mmap,
            no_mmap, block_buffered, line_buffered, no_config, long, tree, recurse, grid, oneline,
            across, git, no_git, git_repos, git_repos_no_status, git_dirty, dirty_boost,
            recency_boost, color_scale, total_size, changed, accessed, modified, created,
            octal_permissions, blocks, inode, links, metric_units, binary_units, bytes, header,
            no_permissions, no_filesize, no_user, no_time, group, smart_group, numeric, extended,
            flags, mounts, dereference, classify, no_quotes, short_nix, stdin, treat_dirs_as_files,
            icons, hyperlink, dirs_first, dirs_last, only_dirs, only_files, show_symlinks,
            no_symlinks, follow_symlinks, almost_all, reverse, names, content, files,
            files_with_matches, files_without_match, files_only, quiet, no_messages, count, stats,
            column, line_number, no_line_number, heading, no_heading, vimgrep, pretty, null, no_color)
    }

    fn set_value(&mut self, dest: &str, flag: &str, v: String) {
        let int = |v: &str| -> i64 {
            v.trim().parse::<i64>().unwrap_or_else(|_| {
                fail(&format!("argument {}: invalid int value: '{}'", flag, v))
            })
        };
        match dest {
            "query_file" => self.query_file = Some(v),
            "engine" => self.engine = v,
            "encoding" => self.encoding = Some(v),
            "context" => self.context = int(&v),
            "after_context" => self.after_context = int(&v),
            "before_context" => self.before_context = int(&v),
            "context_separator" => self.context_separator = v,
            "field_context_separator" => self.field_context_separator = v,
            "field_match_separator" => self.field_match_separator = v,
            "path_separator" => self.path_separator = Some(v),
            "replace" => self.replace = Some(v),
            "max_count" => self.max_count = int(&v),
            "max_columns" => self.max_columns = int(&v),
            "color_specs" => self.color_specs.push(v),
            "hostname_bin" => self.hostname_bin = Some(v),
            "types" => self.types.push(v),
            "types_not" => self.types_not.push(v),
            "type_adds" => self.type_adds.push(v),
            "type_clears" => self.type_clears.push(v),
            "pre" => self.pre = Some(v),
            "pre_glob" => self.pre_glob = Some(v),
            "ext" => self.ext.push(v),
            "exclude" => self.exclude.push(v),
            "globs" => self.globs.push(v),
            "iglobs" => self.iglobs.push(v),
            "ignore_files" => self.ignore_files.push(v),
            "patterns" => self.patterns.push(v),
            "pattern_file" => self.pattern_file = Some(v),
            "max_depth" => self.max_depth = Some(int(&v)),
            "max_filesize" => self.max_filesize = Some(v),
            "dfa_size_limit" => self.dfa_size_limit = Some(v),
            "regex_size_limit" => self.regex_size_limit = Some(v),
            "level" => self.level = Some(int(&v)),
            "width" => self.width = Some(int(&v)),
            "loc" => self.loc = Some(v),
            "code" => self.code = Some(v),
            "color_scale_mode" => self.color_scale_mode = v,
            "time" => self.time = v,
            "time_style" => self.time_style = v,
            "absolute" => self.absolute = v,
            "hyperlink_format" => self.hyperlink_format = v,
            "ignore_glob" => self.ignore_glob = Some(v),
            "sort" => self.sort = Some(v),
            "sortr" => self.sortr = Some(v),
            "color" => self.color = v,
            "workers" => self.workers = int(&v),
            "timeout" => self.timeout = int(&v),
            "rrf_k" => self.rrf_k = int(&v),
            "max" => self.max = int(&v),
            "limit" => self.limit = int(&v),
            "collection" => self.collection = v,
            _ => unreachable!("value {}", dest),
        }
    }
}

fn looks_like_option(tok: &str) -> bool {
    // argparse: a token starting with '-' is an option unless it is '-' itself
    // or a negative number (no option here looks like a number).
    if !tok.starts_with('-') || tok == "-" {
        return false;
    }
    let rest = &tok[1..];
    !(rest.parse::<f64>().is_ok())
}

fn find_long(name: &str) -> Result<&'static Spec, String> {
    for sp in SPECS {
        if sp.flags.contains(&name) {
            return Ok(sp);
        }
    }
    let hits: Vec<(&'static Spec, &'static str)> = SPECS
        .iter()
        .flat_map(|sp| sp.flags.iter().map(move |f| (sp, *f)))
        .filter(|(_, f)| f.starts_with("--") && f.starts_with(name))
        .collect();
    match hits.len() {
        0 => Err(format!("unrecognized arguments: {}", name)),
        1 => Ok(hits[0].0),
        _ => {
            let names: Vec<&str> = hits.iter().map(|(_, f)| *f).collect();
            Err(format!("ambiguous option: {} could match {}", name, names.join(", ")))
        }
    }
}

fn find_short(c: char) -> Option<&'static Spec> {
    let want = format!("-{}", c);
    SPECS.iter().find(|sp| sp.flags.contains(&want.as_str()))
}

fn check_choice(sp: &Spec, flag: &str, v: &str) {
    if !sp.choices.is_empty() && !sp.choices.contains(&v) {
        let quoted: Vec<String> = sp.choices.iter().map(|c| format!("'{}'", c)).collect();
        fail(&format!(
            "argument {}: invalid choice: '{}' (choose from {})",
            flag,
            v,
            quoted.join(", ")
        ));
    }
}

pub fn parse(argv: Vec<String>, cpus: i64) -> Args {
    let mut a = Args::defaults(cpus);
    let mut positionals: Vec<String> = Vec::new();
    let mut i = 0;
    let n = argv.len();
    while i < n {
        let tok = argv[i].clone();
        i += 1;
        if tok == "--" {
            positionals.extend(argv[i..].iter().cloned());
            break;
        }
        if !looks_like_option(&tok) {
            positionals.push(tok);
            continue;
        }
        if tok.starts_with("--") {
            let (name, inline) = match tok.find('=') {
                Some(p) => (&tok[..p], Some(tok[p + 1..].to_string())),
                None => (tok.as_str(), None),
            };
            let sp = find_long(name).unwrap_or_else(|e| fail(&e));
            let flag = sp.flags.last().copied().unwrap_or(name);
            apply(&mut a, sp, flag, inline, &argv, &mut i);
        } else {
            // bundle of short flags: -uu, -il, -C2, -tpy
            let chars: Vec<char> = tok[1..].chars().collect();
            let mut j = 0;
            while j < chars.len() {
                let sp = find_short(chars[j]).unwrap_or_else(|| {
                    fail(&format!("unrecognized arguments: {}", tok))
                });
                let flag = sp.flags[0];
                match sp.kind {
                    Flag | Count => {
                        a.set_flag(sp.dest);
                        j += 1;
                    }
                    _ => {
                        let rest: String = chars[j + 1..].iter().collect();
                        let inline = if rest.is_empty() { None } else { Some(rest) };
                        apply(&mut a, sp, flag, inline, &argv, &mut i);
                        break;
                    }
                }
            }
        }
    }
    let mut it = positionals.into_iter();
    a.query = it.next();
    a.paths = it.collect();
    a
}

fn apply(a: &mut Args, sp: &'static Spec, flag: &str, inline: Option<String>, argv: &[String], i: &mut usize) {
    match sp.kind {
        Flag | Count => {
            if inline.is_some() {
                fail(&format!("argument {}: ignored explicit argument '{}'", flag, inline.unwrap()));
            }
            a.set_flag(sp.dest);
        }
        Int | OptInt | Str | Append => {
            let v = match inline {
                Some(v) => v,
                None => {
                    if *i >= argv.len() || looks_like_option(&argv[*i]) {
                        fail(&format!("argument {}: expected one argument", flag));
                    }
                    *i += 1;
                    argv[*i - 1].clone()
                }
            };
            check_choice(sp, flag, &v);
            a.set_value(sp.dest, flag, v);
        }
        Plus => {
            let mut got = Vec::new();
            if let Some(v) = inline {
                got.push(v);
            } else {
                while *i < argv.len() && !looks_like_option(&argv[*i]) && argv[*i] != "--" {
                    got.push(argv[*i].clone());
                    *i += 1;
                }
            }
            if got.is_empty() {
                fail(&format!("argument {}: expected at least one argument", flag));
            }
            // nargs="+" replaces the list each time it is given, as argparse does
            if sp.dest == "ext" {
                a.ext.clear();
            } else {
                a.exclude.clear();
            }
            for v in got {
                a.set_value(sp.dest, flag, v);
            }
        }
        Opt(konst) => {
            let v = match inline {
                Some(v) => v,
                None => {
                    if *i < argv.len() && !looks_like_option(&argv[*i]) && argv[*i] != "--" {
                        *i += 1;
                        argv[*i - 1].clone()
                    } else {
                        konst.to_string()
                    }
                }
            };
            check_choice(sp, flag, &v);
            a.set_value(sp.dest, flag, v);
        }
    }
}

pub const HELP: &str = "usage: pfind [options] [query] [paths ...]

Unified hybrid ranked search & exploration. Windows, Linux and macOS.
pfind.exe 4.0.0: native, built on ripgrep's crates; no Python, no ripgrep install needed.

positional arguments:
  query                 what to find (literal substring, regex, or snippet); '-' reads it from stdin
  paths                 roots to search (default: cwd, or a preset)

The flags are pfind.py 3.3.0's, unchanged. Main ones:
  --query-file FILE     read the query (e.g. a multi-line snippet) from FILE
  --doctor              check this computer and say what is there
  --json                machine-readable ranked results (for agents)
  --brain --work --src --all   search the configured preset roots
  -r --regex  -F --fixed-strings  -i -s -S  -w  -x --exact  --loose  --fuzzy  -v
  -U --multiline  --multiline-dotall  --line-regexp  -E --encoding  --crlf  --null-data
  -C/-A/-B NUM  -o  --replace  -m NUM  -c  --count-matches  -M NUM  --max-columns-preview
  -t TYPE  --type-not  --type-add  --type-clear  --type-list  --ext  --exclude  -g  --iglob
  --hidden  --no-ignore*  -u/-uu/-uuu  -d NUM  --max-filesize  -z --search-zip  -a --text
  -l --long  -T --tree  --level  -G --grid  -1 --oneline  --loc  --code  --git-dirty
  --dirty-boost  --recency-boost  --sort/--sortr FIELD  --names  --content  --files
  --files-only  --files-with-matches  -q  --count  --stats  --column  --vimgrep  --limit N
  --max N  --rrf-k N  --workers N  --no-color

presets: --brain --work --src --all | engines: ripgrep crates + eza-style views + pfind RRF
first time here? run:  pfind --doctor
";
