// SPDX-License-Identifier: AGPL-3.0-or-later
//
// File enumeration. `rg_walker` builds the same walk `rg --files` did for
// pfind.py (ripgrep's own `ignore` crate: .gitignore/.ignore/.rgignore,
// hidden files, -g/--iglob overrides, depth, size and filesystem limits), in
// parallel on every core. `py_walk` is pfind.py's os.walk listing, kept for
// the views that used it (--only-dirs listings), so they list the same thing.

use crate::args::Args;
use crate::data::EXCLUDE_GLOBS;
use crate::util::{self, fnmatch, fnmatchcase};
use ignore::overrides::OverrideBuilder;
use ignore::{WalkBuilder, WalkState};
use std::path::{Path, PathBuf};
use std::sync::Mutex;

pub type KeepFn = Box<dyn Fn(&str) -> bool + Send + Sync>;

/// pfind.py _split_type_filter: a type filter and a name glob mean BOTH.
pub fn split_type_filter(
    ext_filter: &[String],
    type_globs: &[String],
    globs: &[String],
    iglobs: &[String],
) -> (Vec<String>, Vec<String>, Option<KeepFn>) {
    let user_inc: Vec<&String> = globs.iter().chain(iglobs.iter()).filter(|x| !x.starts_with('!')).collect();
    let type_inc: Vec<&String> = type_globs.iter().filter(|x| !x.starts_with('!')).collect();
    let exts: Vec<String> = ext_filter
        .iter()
        .map(|e| if e.starts_with('.') { e.clone() } else { format!(".{}", e) })
        .collect();
    if user_inc.is_empty() || (type_inc.is_empty() && exts.is_empty()) {
        return (ext_filter.to_vec(), type_globs.to_vec(), None);
    }
    let mut pats: Vec<String> = type_inc.iter().map(|x| x.to_lowercase()).collect();
    pats.extend(exts.iter().map(|e| format!("*{}", e.to_lowercase())));
    let keep: KeepFn = Box::new(move |path: &str| {
        let norm = path.replace('\\', "/");
        let base = norm.rsplit('/').next().unwrap_or("").to_lowercase();
        pats.iter().any(|x| fnmatchcase(&base, x))
    });
    let negs = type_globs.iter().filter(|x| x.starts_with('!')).cloned().collect();
    (Vec::new(), negs, Some(keep))
}

/// The -g list pfind.py handed to ripgrep, in the same order.
pub fn common_globs(ext_filter: &[String], extra_excludes: &[String], type_globs: &[String], globs: &[String]) -> Vec<String> {
    let mut out = Vec::new();
    for g in EXCLUDE_GLOBS {
        out.push(format!("!{}", g));
    }
    for g in extra_excludes {
        out.push(format!("!{}", g));
    }
    for e in ext_filter {
        let e = if e.starts_with('.') { e.clone() } else { format!(".{}", e) };
        out.push(format!("*{}", e));
    }
    out.extend(type_globs.iter().cloned());
    out.extend(globs.iter().cloned());
    out
}

pub struct WalkSpec<'a> {
    pub roots: &'a [PathBuf],
    pub ext_filter: Vec<String>,
    pub type_globs: Vec<String>,
    pub hidden: bool,
    pub no_ignore: bool,
    pub max_depth: Option<i64>,
    pub args: &'a Args,
}

pub fn parse_size(text: &str) -> Option<u64> {
    let s = text.trim().to_uppercase();
    let (num, mult) = match s.chars().last() {
        Some('K') => (&s[..s.len() - 1], 1024u64),
        Some('M') => (&s[..s.len() - 1], 1024 * 1024),
        Some('G') => (&s[..s.len() - 1], 1024 * 1024 * 1024),
        _ => (&s[..], 1),
    };
    match num.parse::<f64>() {
        Ok(v) => Some((v * mult as f64) as u64),
        Err(_) => {
            crate::errln!("pfind: unrecognised size value {:?}; ignoring", text);
            None
        }
    }
}

/// Build the walker exactly as ripgrep's flags would.
pub fn builder(spec: &WalkSpec, globs: &[String]) -> Result<WalkBuilder, String> {
    let a = spec.args;
    let mut b = WalkBuilder::new(&spec.roots[0]);
    for r in &spec.roots[1..] {
        b.add(r);
    }
    let no_ignore = spec.no_ignore;
    let no_dot = no_ignore || a.no_ignore_dot;
    let no_vcs = no_ignore || a.no_ignore_vcs;
    let no_parent = no_ignore || a.no_ignore_parent;
    let no_global = no_ignore || a.no_ignore_vcs || a.no_ignore_global;
    let no_exclude = no_ignore || a.no_ignore_vcs || a.no_ignore_exclude;
    b.hidden(!spec.hidden)
        .ignore(!no_dot)
        .git_ignore(!no_vcs)
        .git_global(!no_global)
        .git_exclude(!no_exclude)
        .parents(!no_parent)
        .require_git(!a.no_require_git)
        .ignore_case_insensitive(a.ignore_file_case_insensitive)
        .follow_links(a.follow_symlinks)
        .same_file_system(a.one_file_system)
        .threads(a.workers.max(1) as usize);
    if !no_dot {
        b.add_custom_ignore_filename(".rgignore");
    }
    if let Some(d) = spec.max_depth {
        if d != 0 {
            b.max_depth(Some(d.max(0) as usize));
        }
    }
    if let Some(ms) = &a.max_filesize {
        if let Some(n) = parse_size(ms) {
            b.max_filesize(Some(n));
        }
    }
    for f in &a.ignore_files {
        if let Some(err) = b.add_ignore(f) {
            if !a.no_ignore_messages && !a.no_messages {
                crate::errln!("pfind: {}", err);
            }
        }
    }
    let cwd = std::env::current_dir().unwrap_or_else(|_| PathBuf::from("."));
    let mut ob = OverrideBuilder::new(&cwd);
    if a.glob_case_insensitive {
        ob.case_insensitive(true).map_err(|e| e.to_string())?;
    }
    for g in globs {
        ob.add(g).map_err(|e| e.to_string())?;
    }
    ob.case_insensitive(true).map_err(|e| e.to_string())?;
    for g in &a.iglobs {
        ob.add(g).map_err(|e| e.to_string())?;
    }
    let ov = ob.build().map_err(|e| e.to_string())?;
    b.overrides(ov);
    Ok(b)
}

/// Walk in parallel, call `visit` for every regular file (and explicit file
/// roots). Results come back sorted: root order, then path components.
pub fn walk_files<T: Send>(
    b: &WalkBuilder,
    nroots: usize,
    roots: &[PathBuf],
    quiet_errors: bool,
    visit: &(dyn Fn(&Path, bool) -> Option<T> + Sync),
) -> Vec<(String, T)> {
    let results: Mutex<Vec<(usize, String, T)>> = Mutex::new(Vec::new());
    let walker = b.build_parallel();
    walker.run(|| {
        let results = &results;
        Box::new(move |res| {
            let ent = match res {
                Ok(e) => e,
                Err(err) => {
                    if !quiet_errors {
                        let msg = err.to_string();
                        if !msg.is_empty() {
                            crate::errln!("pfind: {}", msg);
                        }
                    }
                    return WalkState::Continue;
                }
            };
            let ft = match ent.file_type() {
                Some(ft) => ft,
                None => return WalkState::Continue,
            };
            let explicit = ent.depth() == 0;
            if ft.is_dir() || !(ft.is_file() || explicit) {
                return WalkState::Continue;
            }
            let p = ent.path();
            if let Some(v) = visit(p, explicit) {
                let ps = p.to_string_lossy().into_owned();
                let idx = if nroots > 1 {
                    roots.iter().position(|r| p.starts_with(r)).unwrap_or(0)
                } else {
                    0
                };
                results.lock().unwrap().push((idx, ps, v));
            }
            WalkState::Continue
        })
    });
    let mut v = results.into_inner().unwrap();
    v.sort_by(|a, b| a.0.cmp(&b.0).then_with(|| cmp_components(&a.1, &b.1)));
    v.into_iter().map(|(_, p, t)| (p, t)).collect()
}

fn cmp_components(a: &str, b: &str) -> std::cmp::Ordering {
    let ia = a.split(['/', '\\']);
    let ib = b.split(['/', '\\']);
    ia.cmp(ib)
}

/// rg --files: the list pfind.py got back from ripgrep.
pub fn rg_list_files(spec: &WalkSpec) -> Vec<String> {
    let a = spec.args;
    let (ext, tg, keep) = split_type_filter(&spec.ext_filter, &spec.type_globs, &a.globs, &a.iglobs);
    let globs = common_globs(&ext, &a.exclude, &tg, &a.globs);
    let b = match builder(spec, &globs) {
        Ok(b) => b,
        Err(e) => {
            if !a.no_messages {
                crate::errln!("pfind: rg --files: {}", e);
            }
            return Vec::new();
        }
    };
    let quiet = a.no_messages;
    let out = walk_files(&b, spec.roots.len(), spec.roots, quiet, &|_p, _e| Some(()));
    out.into_iter()
        .map(|(p, _)| p)
        .filter(|p| keep.as_ref().map_or(true, |k| k(p)))
        .collect()
}

fn excluded(name: &str, patterns: &[String]) -> bool {
    patterns.iter().any(|g| fnmatch(name, g))
}

/// pfind.py _apply_user_globs (its pure-Python override mirror).
pub fn apply_user_globs(paths: Vec<String>, globs: &[String]) -> Vec<String> {
    if globs.is_empty() {
        return paths;
    }
    let includes: Vec<&str> = globs.iter().filter(|g| !g.starts_with('!')).map(|s| s.as_str()).collect();
    let excludes: Vec<&str> = globs.iter().filter(|g| g.starts_with('!')).map(|g| &g[1..]).collect();
    let gmatch = |path: &str, pattern: &str| -> bool {
        if !pattern.contains('/') {
            return fnmatch(util::basename(path), pattern) || fnmatch(path, pattern);
        }
        let norm = path.replace(util::SEP, "/");
        let pat = pattern.trim_start_matches('/');
        fnmatch(&norm, pat) || fnmatch(&norm, &format!("*/{}", pat))
    };
    paths
        .into_iter()
        .filter(|p| {
            if excludes.iter().any(|e| gmatch(p, e)) {
                return false;
            }
            !(!includes.is_empty() && !includes.iter().any(|g| gmatch(p, g)))
        })
        .collect()
}

#[cfg(unix)]
fn dev_of(p: &Path) -> Option<u64> {
    use std::os::unix::fs::MetadataExt;
    std::fs::metadata(p).ok().map(|m| m.dev())
}
#[cfg(not(unix))]
fn dev_of(p: &Path) -> Option<u64> {
    // Windows: the volume a path lives on, by its root.
    let s = util::abspath(p).to_string_lossy().to_lowercase();
    let root: String = s.chars().take_while(|c| *c != '\\').collect();
    let mut h: u64 = 1469598103934665603;
    for b in root.bytes() {
        h = (h ^ b as u64).wrapping_mul(1099511628211);
    }
    Some(h)
}

/// pfind.py py_fallback_files (os.walk). Used for the --only-dirs listing.
pub fn py_walk(spec: &WalkSpec, include_dirs: bool, include_hidden: bool) -> Vec<String> {
    let a = spec.args;
    let mut patterns: Vec<String> = EXCLUDE_GLOBS.iter().map(|s| s.to_string()).collect();
    patterns.extend(a.exclude.iter().cloned());
    let exts: Vec<String> = spec
        .ext_filter
        .iter()
        .filter(|e| !e.is_empty())
        .map(|e| if e.starts_with('.') { e.to_lowercase() } else { format!(".{}", e).to_lowercase() })
        .collect();
    let max_bytes = a.max_filesize.as_deref().and_then(parse_size);
    let mut files = Vec::new();
    for root in spec.roots {
        let root_s = root.to_string_lossy().into_owned();
        let root_dev = if a.one_file_system { dev_of(root) } else { None };
        if a.treat_dirs_as_files && root.is_dir() {
            files.push(root_s);
            continue;
        }
        if !root.is_dir() {
            continue;
        }
        walk_dir(root, &root_s, 0, spec.max_depth, include_dirs, include_hidden, &patterns, &exts, max_bytes,
                 root_dev, a.follow_symlinks, &mut files);
    }
    if !spec.type_globs.is_empty() {
        files = apply_user_globs(files, &spec.type_globs);
    }
    if !a.globs.is_empty() {
        files = apply_user_globs(files, &a.globs);
    }
    if !a.iglobs.is_empty() {
        let ig: Vec<String> = a.iglobs.iter().map(|g| g.to_lowercase()).collect();
        let lowered: Vec<String> = files.iter().map(|f| f.to_lowercase()).collect();
        let keep: std::collections::HashSet<String> = apply_user_globs(lowered.clone(), &ig).into_iter().collect();
        files = files.into_iter().zip(lowered).filter(|(_, l)| keep.contains(l)).map(|(f, _)| f).collect();
    }
    if a.git_ignore {
        files = crate::meta::git_ignored_filter(files);
    }
    files
}

#[allow(clippy::too_many_arguments)]
fn walk_dir(dir: &Path, dirpath: &str, depth: i64, max_depth: Option<i64>, include_dirs: bool, include_hidden: bool,
            patterns: &[String], exts: &[String], max_bytes: Option<u64>, root_dev: Option<u64>, follow: bool,
            files: &mut Vec<String>) {
    let rd = match std::fs::read_dir(dir) {
        Ok(r) => r,
        Err(_) => return,
    };
    let mut dirnames: Vec<(String, bool)> = Vec::new();
    let mut filenames: Vec<String> = Vec::new();
    for ent in rd.flatten() {
        let name = ent.file_name().to_string_lossy().into_owned();
        let is_link = ent.file_type().map(|t| t.is_symlink()).unwrap_or(false);
        let is_d = ent.path().is_dir();
        if is_d {
            dirnames.push((name, is_link));
        } else {
            filenames.push(name);
        }
    }
    if !include_hidden {
        dirnames.retain(|(d, _)| !d.starts_with('.'));
        filenames.retain(|f| !f.starts_with('.'));
    }
    if let Some(md) = max_depth {
        if depth >= md {
            dirnames.clear();
        }
    }
    dirnames.retain(|(d, _)| !excluded(d, patterns));
    if let Some(rd) = root_dev {
        dirnames.retain(|(d, _)| dev_of(&dir.join(d)) == Some(rd));
    }
    if include_dirs && depth > 0 {
        files.push(dirpath.to_string());
    }
    for f in filenames {
        if excluded(&f, patterns) {
            continue;
        }
        let ext = util::suffix(&f).to_lowercase();
        if !exts.is_empty() && !exts.contains(&ext) {
            continue;
        }
        let fp = format!("{}{}{}", dirpath.trim_end_matches(util::SEP), util::SEP, f);
        if let Some(mb) = max_bytes {
            match std::fs::symlink_metadata(&fp) {
                Ok(m) if m.len() > mb => continue,
                Err(_) => continue,
                _ => {}
            }
        }
        files.push(fp);
    }
    for (d, is_link) in dirnames {
        if is_link && !follow {
            continue;
        }
        let sub = format!("{}{}{}", dirpath.trim_end_matches(util::SEP), util::SEP, d);
        walk_dir(&dir.join(&d), &sub, depth + 1, max_depth, include_dirs, include_hidden, patterns, exts, max_bytes,
                 root_dev, follow, files);
    }
}
