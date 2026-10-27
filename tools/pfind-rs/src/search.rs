// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Content search on ripgrep's own crates: grep-regex builds the matcher with
// ripgrep's case rules (-s over -i over smart case), grep-searcher reads the
// files (memory maps, binary detection, encodings, context, multi-line), and
// the `ignore` walker hands every core a file. The sink records exactly what
// `rg --json` reported to pfind.py: one record per match/context event, its
// line number, text, absolute byte offset and submatches.

use crate::archive;
use crate::args::Args;
use crate::util;
use crate::walk::{self, WalkSpec};
use grep_matcher::Matcher;
use grep_regex::{RegexMatcher, RegexMatcherBuilder};
use grep_searcher::{BinaryDetection, Encoding, MmapChoice, Searcher, SearcherBuilder, Sink, SinkContext, SinkMatch};
use std::collections::{HashMap, HashSet};
use std::path::Path;
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::Mutex;
use std::time::{Duration, Instant};

#[derive(Clone)]
pub struct LineData {
    pub lineno: u64,
    pub text: String,
    pub is_match: bool,
    pub col: u64,
}

#[derive(Clone, Default)]
pub struct Hit {
    pub count: u64,
    /// (line, text, column); column is meaningful only with --column
    pub samples: Vec<(u64, String, u64)>,
    pub terms: HashSet<String>,
    pub lines_data: Vec<LineData>,
    pub offsets: HashMap<u64, u64>,
}

pub type Hits = HashMap<String, Hit>;

/// One display line: CRLF-safe, multi-line blocks folded, long lines cut.
pub fn sample(text: &str) -> String {
    let t = text.replace("\r\n", "\n").replace('\r', "\n");
    let t = t.trim_matches('\n');
    let mut s = if t.contains('\n') {
        t.split('\n').map(|x| x.trim()).filter(|x| !x.is_empty()).collect::<Vec<_>>().join(" \u{21b5} ")
    } else {
        t.to_string()
    };
    s = s.trim().to_string();
    if util::char_len(&s) > 220 {
        format!("{}\u{2026}", util::take_chars(&s, 220))
    } else {
        s
    }
}

#[allow(dead_code)] // regex: kept for parity with pfind.py rg_content signature
pub struct Query<'a> {
    pub pattern: &'a str,
    pub regex: bool,
    pub fixed: bool,
    pub multiline: bool,
    pub ctx_before: i64,
    pub ctx_after: i64,
}

fn lim(text: &Option<String>) -> Option<usize> {
    text.as_deref().and_then(walk::parse_size).map(|n| n as usize)
}

pub fn build_matcher(a: &Args, q: &Query) -> Result<RegexMatcher, String> {
    let mut b = RegexMatcherBuilder::new();
    if a.case_sensitive {
        b.case_insensitive(false);
    } else if a.ignore_case && !a.smart_case {
        b.case_insensitive(true);
    } else {
        b.case_smart(true);
    }
    b.fixed_strings(q.fixed)
        .word(a.word_regexp)
        .whole_line(a.line_regexp)
        .unicode(!a.no_unicode)
        .crlf(a.crlf);
    if q.multiline {
        b.multi_line(true);
        if a.multiline_dotall || a.exact || a.loose {
            b.dot_matches_new_line(true);
        }
    } else {
        b.line_terminator(Some(if a.null_data { b'\0' } else { b'\n' }));
    }
    if let Some(n) = lim(&a.dfa_size_limit) {
        b.dfa_size_limit(n);
    }
    if let Some(n) = lim(&a.regex_size_limit) {
        b.size_limit(n);
    }
    b.build(q.pattern).map_err(|e| e.to_string())
}

struct Collect<'m> {
    matcher: &'m RegexMatcher,
    hit: Hit,
    max_per_file: usize,
    column: bool,
    only_matching: bool,
    invert: bool,
}

fn utf8_or_empty(b: &[u8]) -> String {
    match std::str::from_utf8(b) {
        Ok(s) => s.to_string(),
        Err(_) => String::new(),
    }
}

fn strip_nl(s: &str) -> &str {
    s.trim_end_matches('\n')
}

impl Sink for Collect<'_> {
    type Error = std::io::Error;

    fn matched(&mut self, _s: &Searcher, m: &SinkMatch<'_>) -> Result<bool, Self::Error> {
        let bytes = m.bytes();
        let lineno = m.line_number().unwrap_or(0);
        self.hit.offsets.insert(lineno, m.absolute_byte_offset());
        self.hit.count += 1;
        let mut subs: Vec<(usize, &[u8])> = Vec::new();
        if !self.invert {
            let _ = self.matcher.find_iter(bytes, |mm| {
                subs.push((mm.start(), &bytes[mm.start()..mm.end()]));
                true
            });
        }
        let mut col = 0;
        if self.column {
            if let Some((st, _)) = subs.first() {
                col = *st as u64 + 1;
            }
        }
        for (_, t) in &subs {
            if let Ok(s) = std::str::from_utf8(t) {
                if !s.is_empty() {
                    self.hit.terms.insert(util::take_chars(&s.to_lowercase(), 40).to_string());
                }
            }
        }
        let mut text = strip_nl(&utf8_or_empty(bytes)).to_string();
        if self.only_matching {
            let parts: Vec<String> = subs
                .iter()
                .filter_map(|(_, t)| std::str::from_utf8(t).ok())
                .filter(|s| !s.is_empty())
                .map(|s| s.to_string())
                .collect();
            if !parts.is_empty() {
                text = parts.join(" ");
            }
        }
        if self.hit.samples.len() < self.max_per_file {
            self.hit.samples.push((lineno, sample(&text), col));
        }
        self.hit.lines_data.push(LineData { lineno, text, is_match: true, col });
        Ok(true)
    }

    fn context(&mut self, _s: &Searcher, c: &SinkContext<'_>) -> Result<bool, Self::Error> {
        let lineno = c.line_number().unwrap_or(0);
        self.hit.offsets.insert(lineno, c.absolute_byte_offset());
        let text = strip_nl(&utf8_or_empty(c.bytes())).to_string();
        self.hit.lines_data.push(LineData { lineno, text, is_match: false, col: 0 });
        Ok(true)
    }
}

fn searcher_builder(a: &Args, q: &Query) -> SearcherBuilder {
    let mut sb = SearcherBuilder::new();
    sb.line_number(true)
        .multi_line(q.multiline)
        .invert_match(a.invert_match)
        .before_context(q.ctx_before.max(0) as usize)
        .after_context(q.ctx_after.max(0) as usize)
        .passthru(a.passthru)
        .stop_on_nonmatch(a.stop_on_nonmatch);
    if a.max_count > 0 {
        sb.max_matches(Some(a.max_count as u64));
    }
    if a.crlf {
        sb.line_terminator(grep_matcher::LineTerminator::crlf());
    } else if a.null_data {
        sb.line_terminator(grep_matcher::LineTerminator::byte(b'\0'));
    }
    if let Some(enc) = &a.encoding {
        if enc != "auto" && enc != "none" {
            match Encoding::new(enc) {
                Ok(e) => {
                    sb.encoding(Some(e));
                }
                Err(e) => crate::errln!("pfind: {}", e),
            }
        } else if enc == "none" {
            sb.bom_sniffing(false);
        }
    }
    if !a.no_mmap {
        // SAFETY (as in ripgrep): a file truncated while mapped can fault; the
        // searcher only maps when it is a single-file read and falls back otherwise.
        sb.memory_map(unsafe { MmapChoice::auto() });
    }
    sb
}

fn binary_mode(a: &Args, explicit: bool) -> BinaryDetection {
    if a.text || a.null_data {
        BinaryDetection::none()
    } else if a.binary || explicit {
        BinaryDetection::convert(b'\0')
    } else {
        BinaryDetection::quit(b'\0')
    }
}

fn pre_matches(a: &Args, path: &Path) -> bool {
    match &a.pre_glob {
        None => true,
        Some(g) => match globset::Glob::new(g) {
            Ok(gl) => {
                let m = gl.compile_matcher();
                m.is_match(path) || path.file_name().map(|f| m.is_match(f)).unwrap_or(false)
            }
            Err(_) => false,
        },
    }
}

fn search_one(a: &Args, searcher: &mut Searcher, sink: &mut Collect, path: &Path) -> Result<(), String> {
    let ps = path.to_string_lossy();
    if let Some(pre) = a.pre.as_deref() {
        if pre_matches(a, path) {
            let f = std::fs::File::open(path).map_err(|e| e.to_string())?;
            let out = std::process::Command::new(pre)
                .arg(path)
                .stdin(f)
                .stderr(std::process::Stdio::piped())
                .output()
                .map_err(|e| format!("preprocessor command could not start: '{}': {}", pre, e))?;
            return searcher.search_slice(sink.matcher, &out.stdout, &mut *sink).map_err(|e| e.to_string());
        }
    }
    if a.search_zip && archive::is_archive(&ps) {
        let data = archive::read_archive(&ps).map_err(|e| e.to_string())?;
        return searcher.search_slice(sink.matcher, &data, &mut *sink).map_err(|e| e.to_string());
    }
    searcher.search_path(sink.matcher, path, &mut *sink).map_err(|e| e.to_string())
}

/// The rg --json content search pfind.py ran, natively and on every core.
pub fn content(spec: &WalkSpec, q: &Query, max_per_file: usize, timeout_s: i64) -> Hits {
    let a = spec.args;
    let (ext, tg, keep) = walk::split_type_filter(&spec.ext_filter, &spec.type_globs, &a.globs, &a.iglobs);
    let globs = walk::common_globs(&ext, &a.exclude, &tg, &a.globs);
    let matcher = match build_matcher(a, q) {
        Ok(m) => m,
        Err(e) => {
            if !a.no_messages {
                crate::errln!("pfind: rg: {}", e.lines().next().unwrap_or(""));
            }
            return Hits::new();
        }
    };
    let wb = match walk::builder(spec, &globs) {
        Ok(b) => b,
        Err(e) => {
            if !a.no_messages {
                crate::errln!("pfind: rg: {}", e);
            }
            return Hits::new();
        }
    };
    let sb = searcher_builder(a, q);
    let hits: Mutex<Hits> = Mutex::new(Hits::new());
    let deadline = Instant::now() + Duration::from_secs(timeout_s.max(1) as u64);
    let timed_out = AtomicBool::new(false);
    let quiet = a.no_messages;
    wb.build_parallel().run(|| {
        let mut searcher = sb.build();
        let matcher = matcher.clone();
        let hits = &hits;
        let keep = &keep;
        let sb = &sb;
        let timed_out = &timed_out;
        let _ = sb;
        Box::new(move |res| {
            if Instant::now() > deadline {
                timed_out.store(true, Ordering::Relaxed);
                return ignore::WalkState::Quit;
            }
            let ent = match res {
                Ok(e) => e,
                Err(err) => {
                    if !quiet {
                        crate::errln!("pfind: {}", err);
                    }
                    return ignore::WalkState::Continue;
                }
            };
            let ft = match ent.file_type() {
                Some(t) => t,
                None => return ignore::WalkState::Continue,
            };
            let explicit = ent.depth() == 0;
            if ft.is_dir() || !(ft.is_file() || explicit) {
                return ignore::WalkState::Continue;
            }
            let path = ent.path();
            let ps = path.to_string_lossy().into_owned();
            if let Some(k) = keep {
                if !k(&ps) {
                    return ignore::WalkState::Continue;
                }
            }
            searcher.set_binary_detection(binary_mode(a, explicit));
            let mut sink = Collect {
                matcher: &matcher,
                hit: Hit::default(),
                max_per_file,
                column: a.column,
                only_matching: a.only_matching,
                invert: a.invert_match,
            };
            if let Err(e) = search_one(a, &mut searcher, &mut sink, path) {
                if !quiet {
                    crate::errln!("pfind: {}: {}", ps, e);
                }
            }
            if sink.hit.count > 0 || !sink.hit.lines_data.is_empty() {
                hits.lock().unwrap().insert(ps, sink.hit);
            }
            ignore::WalkState::Continue
        })
    });
    if timed_out.load(Ordering::Relaxed) && !quiet {
        crate::errln!("pfind: content search stopped at the {} s timeout; results are PARTIAL", timeout_s);
    }
    hits.into_inner().unwrap()
}
