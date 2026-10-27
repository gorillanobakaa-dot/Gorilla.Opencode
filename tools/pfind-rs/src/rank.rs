// SPDX-License-Identifier: AGPL-3.0-or-later
//
// pfind's ranking: name matching (exact, substring, path-inside-root, fuzzy
// with difflib's SequenceMatcher ratio, ported exactly so the same names pass
// the same 0.6 bar), weighted Reciprocal Rank Fusion of the signals, and the
// eza sort vocabulary for --sort/--sortr.

use crate::meta;
use crate::util;
use std::cmp::Ordering;
use std::collections::HashMap;
use std::path::PathBuf;

/// Python 3.7+ re.escape.
pub fn re_escape(s: &str) -> String {
    let mut o = String::with_capacity(s.len() * 2);
    for ch in s.chars() {
        if "()[]{}?*+-|^$\\.&~# \t\n\r\x0b\x0c".contains(ch) {
            o.push('\\');
        }
        o.push(ch);
    }
    o
}

pub fn build_loose_regex(snippet: &str) -> String {
    snippet.split_whitespace().map(re_escape).filter(|p| !p.is_empty()).collect::<Vec<_>>().join(r"\s+")
}

/// An exact multi-line snippet that also matches CRLF files.
pub fn build_exact_regex(snippet: &str) -> String {
    snippet.replace("\r\n", "\n").split('\n').map(re_escape).collect::<Vec<_>>().join(r"\r?\n")
}

/// ripgrep's precedence for names: -s over -i over smart case; an escape such
/// as \S does not count as a capital.
pub fn smart_insensitive(pattern: &str, ignore_case: bool, case_sensitive: bool) -> bool {
    if case_sensitive {
        return false;
    }
    if ignore_case {
        return true;
    }
    let mut bare = String::new();
    let mut it = pattern.chars().peekable();
    while let Some(c) = it.next() {
        if c == '\\' {
            if it.next().is_some() {
                continue;
            }
            bare.push(c);
        } else {
            bare.push(c);
        }
    }
    bare == bare.to_lowercase()
}

/// difflib.SequenceMatcher(None, a, b).ratio(), autojunk included.
pub fn seq_ratio(a: &[char], b: &[char]) -> f64 {
    let (la, lb) = (a.len(), b.len());
    if la + lb == 0 {
        return 1.0;
    }
    let mut b2j: HashMap<char, Vec<usize>> = HashMap::new();
    for (i, c) in b.iter().enumerate() {
        b2j.entry(*c).or_default().push(i);
    }
    if lb >= 200 {
        let ntest = lb / 100 + 1;
        b2j.retain(|_, v| v.len() <= ntest);
    }
    let longest = |alo: usize, ahi: usize, blo: usize, bhi: usize| -> (usize, usize, usize) {
        let (mut besti, mut bestj, mut bestsize) = (alo, blo, 0usize);
        let mut j2len: HashMap<usize, usize> = HashMap::new();
        for i in alo..ahi {
            let mut newj2len: HashMap<usize, usize> = HashMap::new();
            if let Some(js) = b2j.get(&a[i]) {
                for &j in js {
                    if j < blo {
                        continue;
                    }
                    if j >= bhi {
                        break;
                    }
                    let k = if j > 0 { j2len.get(&(j - 1)).copied().unwrap_or(0) } else { 0 } + 1;
                    newj2len.insert(j, k);
                    if k > bestsize {
                        besti = i + 1 - k;
                        bestj = j + 1 - k;
                        bestsize = k;
                    }
                }
            }
            j2len = newj2len;
        }
        while besti > alo && bestj > blo && a[besti - 1] == b[bestj - 1] {
            besti -= 1;
            bestj -= 1;
            bestsize += 1;
        }
        while besti + bestsize < ahi && bestj + bestsize < bhi && a[besti + bestsize] == b[bestj + bestsize] {
            bestsize += 1;
        }
        (besti, bestj, bestsize)
    };
    let mut queue = vec![(0usize, la, 0usize, lb)];
    let mut matches = 0usize;
    while let Some((alo, ahi, blo, bhi)) = queue.pop() {
        let (i, j, k) = longest(alo, ahi, blo, bhi);
        if k > 0 {
            matches += k;
            if alo < i && blo < j {
                queue.push((alo, i, blo, j));
            }
            if i + k < ahi && j + k < bhi {
                queue.push((i + k, ahi, j + k, bhi));
            }
        }
    }
    2.0 * matches as f64 / (la + lb) as f64
}

fn is_subsequence(q: &[char], b: &[char]) -> bool {
    let mut it = b.iter();
    q.iter().all(|c| it.any(|x| x == c))
}

/// The part of a path below its search root (pfind.py _inside).
pub fn inside(fp: &str, roots: &[PathBuf]) -> String {
    let parts: Vec<&str> = fp.split(['/', '\\']).filter(|s| !s.is_empty()).collect();
    for r in roots {
        let rs = r.to_string_lossy();
        let rparts: Vec<&str> = rs.split(['/', '\\']).filter(|s| !s.is_empty()).collect();
        if rparts.len() > parts.len() {
            continue;
        }
        let same = rparts.iter().zip(parts.iter()).all(|(x, y)| {
            if cfg!(windows) {
                x.to_lowercase() == y.to_lowercase()
            } else {
                x == y
            }
        });
        if !same {
            continue;
        }
        let rel = parts[rparts.len()..].join(&util::SEP.to_string());
        return if rel.is_empty() { util::basename(fp).to_string() } else { rel };
    }
    fp.to_string()
}

pub fn match_names(query: &str, files: &[String], regex: bool, ignore_case: bool, fuzzy: bool, roots: &[PathBuf],
                   case_sensitive: bool) -> Result<Vec<(String, f64)>, String> {
    let insensitive = fuzzy || smart_insensitive(query, ignore_case, case_sensitive);
    let rx = if regex {
        Some(regex::RegexBuilder::new(query).case_insensitive(insensitive).build().map_err(|e| e.to_string())?)
    } else {
        None
    };
    let q = if insensitive { query.to_lowercase() } else { query.to_string() };
    let qc: Vec<char> = q.chars().collect();
    let mut scored = Vec::new();
    for fp in files {
        let base = util::basename(fp);
        let rel = inside(fp, roots);
        let b = if insensitive { base.to_lowercase() } else { base.to_string() };
        let p = if insensitive { rel.to_lowercase() } else { rel.clone() };
        let mut score = 0.0;
        if let Some(rx) = &rx {
            if rx.is_match(base) {
                score = 3.0;
            } else if rx.is_match(&rel) {
                score = 1.5;
            }
        } else if q == b || format!("{}{}", q, util::suffix(base).to_lowercase()) == b {
            score = 5.0;
        } else if b.contains(&q) {
            score = 3.0;
        } else if p.contains(&q) {
            score = 1.5;
        } else if fuzzy {
            let bc: Vec<char> = b.chars().collect();
            let r = seq_ratio(&qc, &bc);
            if r >= 0.6 || is_subsequence(&qc, &bc) {
                score = 0.5 + r;
            }
        }
        if score > 0.0 {
            scored.push((fp.clone(), score));
        }
    }
    scored.sort_by(|x, y| y.1.partial_cmp(&x.1).unwrap_or(Ordering::Equal).then_with(|| x.0.cmp(&y.0)));
    Ok(scored)
}

pub fn weight(label: &str) -> f64 {
    match label {
        "name" => 2.0,
        "semantic" => 1.5,
        "content" => 1.0,
        "git" => 0.8,
        "recency" => 0.5,
        _ => 1.0,
    }
}

pub type Fused = Vec<(String, f64, Vec<String>)>;

pub fn rrf_fuse(rankings: &[(&str, Vec<String>)], k: i64) -> Fused {
    let mut order: Vec<String> = Vec::new();
    let mut fused: HashMap<String, (f64, Vec<String>)> = HashMap::new();
    for (label, ranked) in rankings {
        let w = weight(label);
        for (i, path) in ranked.iter().enumerate() {
            let rank = (i + 1) as f64;
            let e = fused.entry(path.clone()).or_insert_with(|| {
                order.push(path.clone());
                (0.0, Vec::new())
            });
            e.0 += w / (k as f64 + rank);
            e.1.push(label.to_string());
        }
    }
    let mut out: Fused = order
        .into_iter()
        .map(|p| {
            let (s, l) = fused.remove(&p).unwrap();
            (p, s, l)
        })
        .collect();
    out.sort_by(|x, y| y.1.partial_cmp(&x.1).unwrap_or(Ordering::Equal).then_with(|| x.0.cmp(&y.0)));
    out
}

#[derive(PartialEq, PartialOrd)]
enum Key {
    F(f64),
    S(String),
    T(f64, String),
    TS(String, String),
}

fn cmp_key(a: &Key, b: &Key) -> Ordering {
    a.partial_cmp(b).unwrap_or(Ordering::Equal)
}

pub fn sort_files(files: Vec<String>, sort_by: Option<&str>, reverse: bool, dirs_first: bool, dirs_last: bool) -> Vec<String> {
    let mut list = files;
    if let Some(field) = sort_by.filter(|f| *f != "none") {
        let invert = field == "newest";
        let key = |path: &str| -> Key {
            let base = util::basename(path);
            let st = meta::stat(path, false).ok();
            let t = |f: fn(&meta::Stat) -> f64| st.as_ref().map(f).unwrap_or(0.0);
            match field {
                "modified" | "date" | "time" | "age" | "old" | "newest" => Key::F(t(|s| s.mtime)),
                "accessed" => Key::F(t(|s| s.atime)),
                "changed" => Key::F(t(|s| s.ctime)),
                "created" => Key::F(t(|s| s.birthtime)),
                "size" => Key::F(t(|s| s.size as f64)),
                "inode" => Key::F(st.as_ref().map(|s| meta::ino_nlink(path, s).0 as f64).unwrap_or(0.0)),
                "type" => {
                    let rank = match &st {
                        None => 9.0,
                        Some(s) if s.is_dir() => 0.0,
                        Some(s) if s.is_link() => 1.0,
                        Some(s) if s.is_reg() => 2.0,
                        _ => 3.0,
                    };
                    Key::T(rank, base.to_lowercase())
                }
                "ext" => Key::TS(util::splitext_ext(base).to_lowercase(), base.to_lowercase()),
                "Ext" => Key::TS(util::splitext_ext(base).to_string(), base.to_string()),
                "Name" => Key::S(base.to_string()),
                ".name" => Key::S(base.trim_start_matches('.').to_lowercase()),
                ".Name" => Key::S(base.trim_start_matches('.').to_string()),
                "path" => Key::S(path.to_string()),
                _ => Key::S(base.to_lowercase()),
            }
        };
        let mut keyed: Vec<(Key, String)> = list.into_iter().map(|p| (key(&p), p)).collect();
        if reverse != invert {
            keyed.sort_by(|a, b| cmp_key(&b.0, &a.0));
        } else {
            keyed.sort_by(|a, b| cmp_key(&a.0, &b.0));
        }
        list = keyed.into_iter().map(|(_, p)| p).collect();
    }
    if dirs_first || dirs_last {
        let (d, f): (Vec<String>, Vec<String>) = list.into_iter().partition(|p| util::is_dir(p));
        return if dirs_first { [d, f].concat() } else { [f, d].concat() };
    }
    list
}

#[cfg(test)]
mod tests {
    use super::*;
    fn r(a: &str, b: &str) -> f64 {
        seq_ratio(&a.chars().collect::<Vec<_>>(), &b.chars().collect::<Vec<_>>())
    }
    #[test]
    fn ratio_matches_difflib() {
        // values printed by Python 3.12 difflib.SequenceMatcher(None, a, b).ratio()
        assert!((r("abcd", "bcde") - 0.75).abs() < 1e-12);
        assert!((r("readme", "readme.md") - 0.8).abs() < 1e-12);
        assert!((r("confgi", "config.go") - 0.6666666666666666).abs() < 1e-12);
        assert_eq!(r("", ""), 1.0);
    }
    #[test]
    fn rrf_order_and_scores() {
        let f = rrf_fuse(&[("name", vec!["b".into(), "a".into()]), ("content", vec!["a".into()])], 60);
        assert_eq!(f[0].0, "a");
        assert!((f[0].1 - (2.0 / 62.0 + 1.0 / 61.0)).abs() < 1e-15);
        assert_eq!(f[0].2, vec!["name".to_string(), "content".to_string()]);
    }
    #[test]
    fn escape_like_python() {
        assert_eq!(re_escape("a b.c"), r"a\ b\.c");
        assert_eq!(build_exact_regex("x(\r\ny"), r"x\(\r?\ny");
        assert_eq!(build_loose_regex("  foo   bar() "), r"foo\s+bar\(\)");
    }
    #[test]
    fn smart_case_ignores_escapes() {
        assert!(smart_insensitive(r"\Sfoo", false, false));
        assert!(!smart_insensitive("Foo", false, false));
        assert!(smart_insensitive("Foo", true, false));
        assert!(!smart_insensitive("foo", true, true));
    }
}
