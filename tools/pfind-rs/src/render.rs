// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Every output format of pfind.py 3.3.0, ported line for line so the text an
// agent or a person reads is the same: ranked results with samples and
// context, --json, the long table, grid, oneline, tree, folder overview,
// --code summary, vimgrep and the count/files modes.

use crate::args::Args;
use crate::meta::{self, GitStatusCache};
use crate::rank::Fused;
use crate::search::{Hit, Hits, LineData};
use crate::util::{self, c, commas};
use crate::{errln, outln};
use std::collections::HashMap;
use std::path::{Path, PathBuf};

pub struct Ctx<'a> {
    pub a: &'a Args,
    pub use_color: bool,
    pub colors: HashMap<&'static str, String>,
    pub host: String,
    pub roots: Vec<PathBuf>,
    pub git: Option<GitStatusCache>,
    pub dir_sizes: HashMap<String, u64>,
    pub repo_cache: HashMap<(String, bool), Option<String>>,
}

impl Ctx<'_> {
    pub fn cc(&self, kind: &str, s: &str) -> String {
        match self.colors.get(kind) {
            Some(code) if !code.is_empty() => c(code, s, self.use_color),
            _ => s.to_string(),
        }
    }
    fn git_status(&self, path: &str) -> Option<&str> {
        if self.a.git {
            self.git.as_ref().and_then(|g| g.get(path))
        } else {
            None
        }
    }
    pub fn link(&self, path: &str, line: u64, text: &str) -> String {
        if !self.a.hyperlink {
            return text.to_string();
        }
        let abs = util::abspath_str(path);
        let target = match self.a.hyperlink_format.as_str() {
            "vscode" => {
                let mut t = format!("vscode://file{}", abs);
                if line > 0 {
                    t += &format!(":{}", line);
                }
                t
            }
            "grep+" => {
                let mut t = format!("file://{}", abs);
                if line > 0 {
                    t += &format!("#L{}", line);
                }
                t
            }
            _ => {
                let mut t = format!("file://{}{}", self.host, abs);
                if line > 0 {
                    t += &format!("#{}", line);
                }
                t
            }
        };
        format!("\x1b]8;;{}\x1b\\{}\x1b]8;;\x1b\\", target, text)
    }
    fn display_path(&self, path: &str) -> String {
        let p = match self.a.absolute.as_str() {
            "follow" => std::fs::canonicalize(path)
                .map(|p| util::strip_verbatim(p).to_string_lossy().into_owned())
                .unwrap_or_else(|_| util::abspath_str(path)),
            "on" => util::abspath_str(path),
            _ => path.to_string(),
        };
        meta::short_nix(&p, self.a.short_nix)
    }
    fn icon(&self, path: &str, is_dir: bool) -> String {
        if self.a.icons {
            format!("{} ", crate::data::icon_for(path, is_dir))
        } else {
            String::new()
        }
    }
}

const COLOR_NAMES: &[(&str, u32)] = &[("black", 0), ("red", 1), ("green", 2), ("yellow", 3), ("blue", 4), ("magenta", 5), ("cyan", 6), ("white", 7)];

fn colour_code(value: &str, bg: bool) -> Option<String> {
    let v = value.trim().to_lowercase();
    let base = if bg { 40 } else { 30 };
    if let Some((_, n)) = COLOR_NAMES.iter().find(|(k, _)| *k == v) {
        return Some((base + n).to_string());
    }
    if v.starts_with("0x") && v.len() == 8 {
        let r = u8::from_str_radix(&v[2..4], 16).ok()?;
        let g = u8::from_str_radix(&v[4..6], 16).ok()?;
        let b = u8::from_str_radix(&v[6..8], 16).ok()?;
        return Some(format!("{};2;{};{};{}", base + 8, r, g, b));
    }
    if !v.is_empty() && v.chars().all(|c| c.is_ascii_digit()) {
        if let Ok(n) = v.parse::<u32>() {
            if n <= 255 {
                return Some(format!("{};5;{}", base + 8, v));
            }
        }
    }
    None
}

/// --colors type:attribute:value (ripgrep's grammar), resolved to ANSI codes.
pub fn parse_color_specs(specs: &[String]) -> HashMap<&'static str, String> {
    let mut m: HashMap<&'static str, String> = HashMap::new();
    m.insert("path", "1;32".into());
    m.insert("line", "1;33".into());
    m.insert("column", "1;33".into());
    m.insert("match", "1;31".into());
    for raw in specs {
        let parts: Vec<&str> = raw.split(':').collect();
        if parts.len() < 2 {
            errln!("pfind: unrecognised --colors spec {:?} (want type:attribute:value)", raw);
            continue;
        }
        let ctype = parts[0].trim().to_lowercase();
        let key: &'static str = match ctype.as_str() {
            "path" => "path",
            "line" => "line",
            "column" => "column",
            "match" => "match",
            _ => {
                errln!("pfind: unknown --colors type {:?} (want path|line|column|match)", ctype);
                continue;
            }
        };
        if parts.len() == 2 && parts[1].trim().to_lowercase() == "none" {
            m.insert(key, String::new());
            continue;
        }
        if parts.len() < 3 {
            errln!("pfind: unrecognised --colors spec {:?} (want type:attribute:value)", raw);
            continue;
        }
        let attr = parts[1].trim().to_lowercase();
        let value = parts[2..].join(":");
        let code = match attr.as_str() {
            "style" => match value.trim().to_lowercase().as_str() {
                "bold" | "intense" => Some("1".to_string()),
                "nobold" | "nointense" => Some("22".to_string()),
                "underline" => Some("4".to_string()),
                "nounderline" => Some("24".to_string()),
                _ => None,
            },
            "fg" | "bg" => colour_code(&value, attr == "bg"),
            _ => None,
        };
        match code {
            None => errln!("pfind: unrecognised --colors value in {:?}", raw),
            Some(code) => {
                if attr == "style" {
                    let mut existing: Vec<String> = m[key].split(';').filter(|x| !x.is_empty()).map(|s| s.to_string()).collect();
                    existing.push(code);
                    m.insert(key, existing.join(";"));
                } else {
                    m.insert(key, code);
                }
            }
        }
    }
    m
}

/// A tree of a folder too large to draw: folders only, with file counts.
pub fn render_folder_overview(all_files: &[String], roots: &[PathBuf], limit: i64) {
    let sep = util::SEP;
    let seps = sep.to_string();
    let mut direct: Vec<(String, i64)> = Vec::new();
    let mut idx: HashMap<String, usize> = HashMap::new();
    for f in all_files {
        let norm = f.replace('/', &seps);
        let d = match norm.rfind(sep) {
            Some(i) => norm[..i].to_string(),
            None => String::new(),
        };
        match idx.get(&d) {
            Some(&i) => direct[i].1 += 1,
            None => {
                idx.insert(d.clone(), direct.len());
                direct.push((d, 1));
            }
        }
    }
    let cap = limit.max(200) as usize;
    for root in roots {
        let rootstr = util::normpath(root).to_string_lossy().trim_end_matches(sep).to_string();
        let prefix = format!("{}{}", rootstr, sep);
        let mut total: Vec<(String, i64)> = Vec::new();
        let mut tidx: HashMap<String, usize> = HashMap::new();
        let (mut here, mut n) = (0i64, 0i64);
        for (d, k) in &direct {
            if *d == rootstr {
                here += k;
                n += k;
                continue;
            }
            let Some(rest) = d.strip_prefix(&prefix) else { continue };
            n += k;
            let parts: Vec<&str> = rest.split(sep).collect();
            for i in 1..=parts.len() {
                let key = parts[..i].join(&seps);
                match tidx.get(&key) {
                    Some(&j) => total[j].1 += k,
                    None => {
                        tidx.insert(key.clone(), total.len());
                        total.push((key, *k));
                    }
                }
            }
        }
        let mut by_depth: HashMap<usize, i64> = HashMap::new();
        for (rel, _) in &total {
            *by_depth.entry(rel.matches(sep).count() + 1).or_default() += 1;
        }
        let mut depth = 1usize;
        let mut running = *by_depth.get(&1).unwrap_or(&0);
        let deepest = by_depth.keys().max().copied().unwrap_or(1);
        let budget = limit.max(150);
        while depth < deepest && running + by_depth.get(&(depth + 1)).copied().unwrap_or(0) <= budget {
            depth += 1;
            running += by_depth.get(&depth).copied().unwrap_or(0);
        }
        let mut shown: Vec<(String, i64)> = total.iter().filter(|(rel, _)| rel.matches(sep).count() + 1 <= depth).cloned().collect();
        let mut left_out = 0usize;
        if shown.len() > cap {
            shown.sort_by(|a, b| b.1.cmp(&a.1));
            left_out = shown.len() - cap;
            shown.truncate(cap);
        }
        shown.sort_by(|a, b| {
            let ka: Vec<String> = a.0.split(sep).map(|x| x.to_lowercase()).collect();
            let kb: Vec<String> = b.0.split(sep).map(|x| x.to_lowercase()).collect();
            ka.cmp(&kb)
        });
        outln!(
            "{}: {} files in {} folders. That is too many to list one by one, so this shows FOLDERS ONLY, {} level(s) deep, each with the number of files inside it (sub-folders included). Hidden and git-ignored files are not counted. This is NOT the whole tree.",
            rootstr, commas(n), commas(total.len() as i64), depth
        );
        for (rel, cnt) in &shown {
            let parts: Vec<&str> = rel.split(sep).collect();
            outln!("{}{}{}  ({} files)", "    ".repeat(parts.len() - 1), parts[parts.len() - 1], sep, commas(*cnt));
        }
        if left_out > 0 {
            outln!("\u{2026} and {} smaller folder(s) at this depth not shown.", commas(left_out as i64));
        }
        if here > 0 {
            outln!("({} file(s) sit directly in {}, not listed here.)", commas(here), rootstr);
        }
        outln!("To see inside one folder, call find again with path set to that folder.");
    }
}

fn split_parts(p: &str) -> Vec<String> {
    let mut out = Vec::new();
    let path = Path::new(p);
    for comp in path.components() {
        out.push(comp.as_os_str().to_string_lossy().into_owned());
    }
    out
}

fn parent_str(p: &str) -> String {
    Path::new(p).parent().map(|x| x.to_string_lossy().into_owned()).unwrap_or_default()
}

/// os.path.commonpath over parent folders.
fn commonpath(paths: &[String]) -> Option<String> {
    let parents: Vec<Vec<String>> = paths.iter().map(|p| split_parts(&parent_str(p))).collect();
    let first = parents.first()?;
    let mut n = first.len();
    for p in &parents[1..] {
        let m = first.iter().zip(p.iter()).take_while(|(a, b)| {
            if cfg!(windows) { a.to_lowercase() == b.to_lowercase() } else { a == b }
        }).count();
        n = n.min(m);
    }
    if cfg!(windows) && n == 0 {
        return None; // different drives
    }
    let mut pb = PathBuf::new();
    for comp in &first[..n] {
        pb.push(comp);
    }
    Some(pb.to_string_lossy().into_owned())
}

#[derive(Default)]
struct Node {
    kids: std::collections::BTreeMap<String, Node>,
}

pub fn render_tree_view(ctx: &Ctx, paths: &[String], hits: &Hits) {
    if paths.is_empty() {
        return;
    }
    let a = ctx.a;
    let roots = &ctx.roots;
    let mut base: Option<String> = None;
    if roots.len() == 1 && roots[0].is_dir() && paths.iter().all(|p| Path::new(p).starts_with(&roots[0])) {
        base = Some(roots[0].to_string_lossy().into_owned());
    } else if let Some(cp) = commonpath(paths) {
        base = Some(cp);
    }
    if let Some(b) = &base {
        if b.is_empty() || b == "." {
            base = None;
        } else {
            outln!("{}", c("1;34", b, ctx.use_color));
        }
    }
    let mut tree = Node::default();
    for p in paths {
        let parts: Vec<String> = match &base {
            Some(b) => match Path::new(p).strip_prefix(b) {
                Ok(rel) => rel.components().map(|c| c.as_os_str().to_string_lossy().into_owned()).collect(),
                Err(_) => split_parts(p),
            },
            None => split_parts(p),
        };
        let mut cur = &mut tree;
        for part in parts {
            cur = cur.kids.entry(part).or_default();
        }
    }
    fn walk(ctx: &Ctx, node: &Node, prefix: &str, cur: &Path, depth: i64, hits: &Hits) {
        if let Some(l) = ctx.a.level {
            if depth >= l {
                return;
            }
        }
        let n = node.kids.len();
        for (i, (name, child)) in node.kids.iter().enumerate() {
            let last = i == n - 1;
            let connector = if last { "\u{2514}\u{2500}\u{2500} " } else { "\u{251c}\u{2500}\u{2500} " };
            let child_prefix = if last { "    " } else { "\u{2502}   " };
            let full = cur.join(name);
            let fs = full.to_string_lossy().into_owned();
            let is_dir = !child.kids.is_empty();
            let icon = ctx.icon(&fs, is_dir);
            let badge = match ctx.git_status(&fs) {
                Some(st) => format!(" {}", c("31;1", &format!("[{}]", st), ctx.use_color)),
                None => String::new(),
            };
            let mut dn = if is_dir { c("1;34", name, ctx.use_color) } else { c("1;32", name, ctx.use_color) };
            if ctx.a.hyperlink {
                dn = ctx.link(&fs, 0, &dn);
            }
            outln!("{}{}{}{}{}", prefix, connector, icon, dn, badge);
            if !is_dir {
                if let Some(h) = hits.get(&fs) {
                    for (ln, text, _) in &h.samples {
                        outln!("{}{}    {} {}", prefix, child_prefix, c("90", &format!("{}:", ln), ctx.use_color), text);
                    }
                }
            }
            walk(ctx, child, &format!("{}{}", prefix, child_prefix), &full, depth + 1, hits);
        }
    }
    let start = base.map(PathBuf::from).unwrap_or_default();
    let _ = a;
    walk(ctx, &tree, "", &start, 0, hits);
}

pub fn render_code_summary(files: &[String], use_color: bool, mode: &str) {
    let mode = if ["lines", "percent", "both"].contains(&mode) { mode } else { "both" };
    let mut order: Vec<&'static str> = Vec::new();
    let mut summary: HashMap<&'static str, meta::Loc> = HashMap::new();
    for fp in files {
        if let Some((lang, counts)) = meta::classify_file_loc(fp) {
            if !summary.contains_key(lang) {
                order.push(lang);
            }
            summary.entry(lang).or_default().add(&counts);
        }
    }
    if summary.is_empty() {
        errln!("pfind: no source code files identified for --code summary.");
        return;
    }
    outln!("\n{}", c("1;37", "Language Breakdown & Lines of Code Summary:", use_color));
    outln!("{}", "-".repeat(70));
    outln!("{:<24} {:>12} {:>10} {:>10} {:>10}", "Language", "Total Lines", "Code", "Comments", "Blanks");
    outln!("{}", "-".repeat(70));
    let cell = |value: i64, total: i64| -> String {
        let t = total.max(1);
        match mode {
            "percent" => format!("{:>9.1}%", value as f64 * 100.0 / t as f64),
            "both" => format!("{:>5} ({:>2}%)", commas(value), value * 100 / t),
            _ => format!("{:>10}", commas(value)),
        }
    };
    let mut rows: Vec<(&'static str, meta::Loc)> = order.iter().map(|l| (*l, summary[l])).collect();
    rows.sort_by(|x, y| y.1.code.cmp(&x.1.code));
    let mut grand = meta::Loc::default();
    for (lang, counts) in rows {
        grand.add(&counts);
        outln!("{} {:>12} {} {} {}",
            c("1;36", &format!("{:<24}", lang), use_color),
            commas(counts.lines),
            c("32", &cell(counts.code, counts.lines), use_color),
            c("33", &cell(counts.comments, counts.lines), use_color),
            c("90", &cell(counts.blanks, counts.lines), use_color));
    }
    outln!("{}", "=".repeat(70));
    outln!("{} {:>12} {} {} {}\n",
        c("1;37", &format!("{:<24}", "TOTAL"), use_color),
        commas(grand.lines),
        c("1;32", &cell(grand.code, grand.lines), use_color),
        c("1;33", &cell(grand.comments, grand.lines), use_color),
        c("1;90", &cell(grand.blanks, grand.lines), use_color));
}

fn scale_color(val: f64, kind: &str, use_color: bool, mode: &str) -> String {
    if !use_color {
        return String::new();
    }
    if mode == "gradient" {
        let frac = if kind == "size" {
            (val.max(1.0).log10() / 9.0).min(1.0)
        } else {
            (val / (86400.0 * 90.0)).min(1.0)
        };
        let ramp = [46, 82, 118, 154, 190, 226, 220, 214, 208, 202, 196];
        return format!("38;5;{}", ramp[(frac * (ramp.len() - 1) as f64) as usize]);
    }
    let s = if kind == "size" {
        if val < 1024.0 { "32" } else if val < 1048576.0 { "36" } else if val < 100.0 * 1048576.0 { "33" } else { "31" }
    } else if val < 3600.0 { "1;32" } else if val < 86400.0 { "32" } else if val < 86400.0 * 7.0 { "36" } else if val < 86400.0 * 30.0 { "33" } else { "90" };
    s.to_string()
}

pub fn loc_badge(counts: &meta::Loc, mode: &str) -> String {
    let total = counts.lines.max(1);
    match mode {
        "lines" => format!("[code:{} com:{}]", counts.code, counts.comments),
        "percent" => format!("[code:{}% com:{}%]", counts.code * 100 / total, counts.comments * 100 / total),
        _ => format!("[code:{} ({}%) com:{} ({}%)]", counts.code, counts.code * 100 / total, counts.comments, counts.comments * 100 / total),
    }
}

pub fn render_long_table(ctx: &mut Ctx, paths: &[String], hits: &Hits) {
    let a = ctx.a;
    let uc = ctx.use_color;
    let show_perms = !a.no_permissions;
    let show_size = !a.no_filesize;
    let show_user = !a.no_user;
    let show_time = !a.no_time;
    let smart_group = a.smart_group && !a.group;
    let quotes = !a.no_quotes;
    let repos_mode = a.git_repos || a.git_repos_no_status;
    if a.header {
        let mut h: Vec<String> = Vec::new();
        if a.inode { h.push(format!("{:<10}", "INODE")); }
        if a.blocks { h.push(format!("{:<10}", "BLOCKS")); }
        if a.links { h.push(format!("{:<6}", "LINKS")); }
        if a.flags { h.push(format!("{:<8}", "FLAGS")); }
        if show_perms { h.push(format!("{:<10}", "PERMISSIONS")); }
        h.push(format!("{:<3}", "GIT"));
        if show_user { h.push(format!("{:<8}", "USER")); }
        h.push(format!("{:<8}", "GROUP"));
        if show_size { h.push(format!("{:>9}", "SIZE")); }
        if show_time { h.push(format!("{:<12}", a.time.to_uppercase())); }
        if repos_mode { h.push(format!("{:<20}", "REPO")); }
        h.push("NAME".into());
        outln!("{}", c("1;37", &h.join(" "), uc));
    }
    for path in paths {
        let st = match meta::stat(path, a.dereference) {
            Ok(s) => s,
            Err(_) => {
                outln!("{}", path);
                continue;
            }
        };
        let is_dir = st.is_dir();
        let mut cols: Vec<String> = Vec::new();
        let (ino, nlink) = if a.inode || a.links { meta::ino_nlink(path, &st) } else { (0, 0) };
        if a.inode { cols.push(format!("{:<10}", ino)); }
        if a.blocks { cols.push(format!("{:<10}", st.blocks)); }
        if a.links { cols.push(format!("{:<6}", nlink)); }
        if a.flags { cols.push(format!("{:<8}", meta::file_flags(path))); }
        let mode_str = meta::format_permissions(st.mode, a.octal_permissions);
        let raw_size = if a.total_size && is_dir { meta::dir_total_size(path, &mut ctx.dir_sizes) } else { st.size };
        let size_str = if a.bytes { raw_size.to_string() } else { meta::format_size(raw_size, !a.metric_units) };
        let ts = meta::pick_timestamp(&st, &a.time);
        let time_str = meta::format_timestamp(ts, &a.time_style);
        let (user_s, mut group_s) = if a.numeric { (st.uid.to_string(), st.gid.to_string()) } else { meta::user_group(st.uid, st.gid) };
        if smart_group && group_s == user_s {
            group_s = String::new();
        }
        let icon = ctx.icon(path, is_dir);
        let git_badge = match ctx.git_status(path) {
            Some(g) => c("31;1", &format!("{:<3}", g), uc),
            None => "   ".to_string(),
        };
        let mut repo_col = String::new();
        if repos_mode {
            let summary = meta::repo_summary(path, !a.git_repos_no_status, &mut ctx.repo_cache);
            let cell = format!("{:<20}", summary.unwrap_or_else(|| "-".into()));
            repo_col = c("35", &cell, uc) + " ";
        }
        let mut lb = String::new();
        if let Some(mode) = &a.loc {
            if let Some((_, lc)) = meta::classify_file_loc(path) {
                lb = format!(" {}", loc_badge(&lc, mode));
            }
        }
        let shown = ctx.display_path(path);
        let name_text = meta::quote_name(&shown, quotes) + if a.classify { meta::classify_indicator(path, Some(&st)) } else { "" };
        let path_display = ctx.link(path, 0, &name_text);
        let (size_color, time_color) = if a.color_scale {
            (scale_color(raw_size as f64, "size", uc, &a.color_scale_mode), scale_color(util::now_secs() - ts, "age", uc, &a.color_scale_mode))
        } else {
            ("33".to_string(), "36".to_string())
        };
        let mut row = if cols.is_empty() { String::new() } else { cols.join(" ") + " " };
        if show_perms {
            row += &(c("90", &format!("{:<10}", mode_str), uc) + " ");
        }
        row += &(git_badge + " ");
        if show_user {
            row += &format!("{:<8} ", user_s);
        }
        row += &format!("{:<8} ", group_s);
        if show_size {
            row += &(c(&size_color, &format!("{:>9}", size_str), uc) + " ");
        }
        if show_time {
            row += &(c(&time_color, &format!("{:<12}", time_str), uc) + " ");
        }
        row += &repo_col;
        row += &format!("{}{}{}", icon, ctx.cc("path", &path_display), lb);
        if a.mounts {
            if let Some((dev, fs)) = meta::mount_info(path) {
                row += &c("90", &format!("  [{} {}]", dev, fs), uc);
            }
        }
        outln!("{}", row);
        if a.extended {
            for (xn, xs) in meta::xattrs(path) {
                outln!("{}", c("90", &format!("        {} ({} bytes)", xn, xs), uc));
            }
        }
        if let Some(h) = hits.get(path) {
            for (ln, text, _) in &h.samples {
                let ld = ctx.link(path, *ln, &format!("{}:", ln));
                outln!("    {} {}", c("90", &ld, uc), text);
            }
        }
    }
}

pub fn render_grid(ctx: &Ctx, paths: &[String]) {
    if paths.is_empty() {
        return;
    }
    let a = ctx.a;
    let width = match a.width {
        Some(w) if w != 0 => w.max(1) as usize,
        _ => terminal_width(),
    };
    let quotes = !a.no_quotes;
    let cells: Vec<(String, String)> = paths
        .iter()
        .map(|p| {
            let is_dir = util::is_dir(p);
            let icon = ctx.icon(p, is_dir);
            let mut name = meta::quote_name(util::basename(&meta::short_nix(p, a.short_nix)), quotes);
            if a.classify {
                name += meta::classify_indicator(p, None);
            }
            let plain = format!("{}{}", icon, name);
            let coloured = format!("{}{}", icon, if is_dir { c("1;34", &name, ctx.use_color) } else { name.clone() });
            (plain, coloured)
        })
        .collect();
    let col_width = cells.iter().map(|(p, _)| util::char_len(p)).max().unwrap_or(0) + 2;
    let num_cols = (width / col_width).max(1);
    let num_rows = cells.len().div_ceil(num_cols);
    let pad = |i: usize| -> String {
        let (plain, col) = &cells[i];
        format!("{}{}", col, " ".repeat(col_width.saturating_sub(util::char_len(plain))))
    };
    for r in 0..num_rows {
        let row: Vec<String> = if a.across {
            (r * num_cols..((r + 1) * num_cols).min(cells.len())).map(pad).collect()
        } else {
            (0..num_cols).map(|col| col * num_rows + r).filter(|i| *i < cells.len()).map(pad).collect()
        };
        if !row.is_empty() || a.across {
            outln!("{}", row.join("").trim_end());
        }
    }
}

fn terminal_width() -> usize {
    if let Ok(c) = std::env::var("COLUMNS") {
        if let Ok(n) = c.trim().parse::<usize>() {
            if n > 0 {
                return n;
            }
        }
    }
    #[cfg(windows)]
    unsafe {
        use windows_sys::Win32::System::Console::*;
        let h = GetStdHandle(STD_OUTPUT_HANDLE);
        let mut info: CONSOLE_SCREEN_BUFFER_INFO = std::mem::zeroed();
        if GetConsoleScreenBufferInfo(h, &mut info) != 0 {
            let w = (info.srWindow.Right - info.srWindow.Left + 1) as usize;
            if w > 0 {
                return w;
            }
        }
    }
    #[cfg(unix)]
    unsafe {
        let mut ws: libc::winsize = std::mem::zeroed();
        if libc::ioctl(1, libc::TIOCGWINSZ, &mut ws) == 0 && ws.ws_col > 0 {
            return ws.ws_col as usize;
        }
    }
    80
}

pub fn render_oneline(ctx: &Ctx, paths: &[String]) {
    let a = ctx.a;
    let uc = ctx.use_color;
    for path in paths {
        let is_dir = util::is_dir(path);
        let icon = ctx.icon(path, is_dir);
        let shown = ctx.display_path(path) + if a.classify { meta::classify_indicator(path, None) } else { "" };
        if a.long {
            match meta::stat(path, a.dereference) {
                Ok(st) => {
                    let size = meta::format_size(st.size, !a.metric_units);
                    let dn = if is_dir { c("1;34", &shown, uc) } else { c("1;32", &shown, uc) };
                    outln!("{}{:<60} {}", icon, dn, c("33", &size, uc));
                }
                Err(_) => outln!("{}", shown),
            }
        } else {
            let dn = if is_dir { c("1;34", &shown, uc) } else { shown };
            if a.null {
                util::out(&format!("{}{}\0", icon, dn));
            } else {
                outln!("{}{}", icon, dn);
            }
        }
    }
}

pub fn render_vimgrep(hits: &Hits, limit: i64) {
    let mut items: Vec<(&String, &Hit)> = hits.iter().collect();
    items.sort_by(|a, b| a.0.cmp(b.0));
    if limit > 0 {
        items.truncate(limit as usize);
    }
    for (path, h) in items {
        if h.lines_data.is_empty() {
            for (ln, text, col) in &h.samples {
                outln!("{}:{}:{}:{}", path, ln, if *col > 0 { *col } else { 1 }, text);
            }
        } else {
            for ld in &h.lines_data {
                if ld.is_match {
                    outln!("{}:{}:{}:{}", path, ld.lineno, ld_col(ld), ld.text);
                }
            }
        }
    }
}

fn ld_col(ld: &LineData) -> u64 {
    // pfind.py: a 4-tuple (with --column) printed its column, else 1
    if ld.col > 0 { ld.col } else { 1 }
}

/// ripgrep-style $1 / ${name} replacement preview (regex) or literal replace.
pub fn replacement(text: &str, repl: &str, pattern: Option<&str>, is_regex: bool) -> String {
    if repl.is_empty() {
        return text.to_string();
    }
    match pattern {
        Some(p) => {
            if is_regex {
                match regex::Regex::new(p) {
                    Ok(rx) => rx.replace_all(text, repl).into_owned(),
                    Err(_) => text.to_string(),
                }
            } else {
                text.replace(p, repl)
            }
        }
        None => repl.to_string(),
    }
}

fn cap_lines_data<'h>(lines: &'h [LineData], a: &Args) -> (&'h [LineData], i64) {
    if a.max <= 0 || a.passthru {
        return (lines, 0);
    }
    let cap = a.max;
    let mut seen = 0;
    let mut end = lines.len();
    for (i, e) in lines.iter().enumerate() {
        if e.is_match {
            seen += 1;
            if seen > cap {
                end = i;
                break;
            }
        }
    }
    let total = lines.iter().filter(|e| e.is_match).count() as i64;
    (&lines[..end], if total > cap { total - cap } else { 0 })
}

fn prep_line_text(text: &str, a: &Args) -> String {
    let t = if a.trim { text.trim_start() } else { text };
    let limit = a.max_columns.max(0) as usize;
    let n = util::char_len(t);
    if limit > 0 && n > limit {
        if a.max_columns_preview {
            return format!("{} [\u{2026} omitted end of long line]", util::take_chars(t, limit));
        }
        return format!("[Omitted long line with {} chars]", n);
    }
    t.to_string()
}

fn line_prefix(path: &str, lineno: u64, h: Option<&Hit>, a: &Args, in_heading: bool) -> String {
    let mut parts: Vec<String> = Vec::new();
    if a.with_filename && in_heading && !a.no_filename {
        parts.push(path.to_string());
    }
    if a.byte_offset {
        if let Some(off) = h.and_then(|h| h.offsets.get(&lineno)) {
            parts.push(off.to_string());
        }
    }
    if parts.is_empty() { String::new() } else { parts.join(":") + ":" }
}

/// pfind.py render(): every non-JSON search output. Returns the exit code.
pub fn render(ctx: &mut Ctx, fused: &Fused, hits: &Hits, query_pattern: Option<&str>) -> i32 {
    let a = ctx.a;
    let uc = ctx.use_color;
    if fused.is_empty() {
        errln!("no matches.");
        return 0;
    }
    let mut paths: Vec<String> = fused.iter().map(|f| f.0.clone()).collect();
    let keep_links = a.show_symlinks;
    if a.only_dirs {
        paths.retain(|p| util::is_dir(p) || (keep_links && util::is_link(p)));
    } else if a.only_files {
        paths.retain(|p| util::is_file(p) || (keep_links && util::is_link(p)));
    }
    if a.no_symlinks {
        paths.retain(|p| !util::is_link(p));
    }
    let count_of = |p: &str| hits.get(p).map(|h| h.count).unwrap_or(0);
    if a.files_with_matches || a.files_without_match {
        let want = a.files_with_matches;
        for p in paths.iter().filter(|p| (count_of(p) > 0) == want) {
            if a.null { util::out(&format!("{}\0", p)); } else { outln!("{}", p); }
        }
        return 0;
    }
    if a.count_lines || a.count_matches {
        for p in &paths {
            match hits.get(p) {
                Some(h) => {
                    let n = if a.count_matches { h.count } else { h.samples.len() as u64 };
                    outln!("{}:{}", n, p);
                }
                None if a.include_zero => outln!("0:{}", p),
                None => {}
            }
        }
        return 0;
    }
    if a.quiet {
        return if paths.is_empty() { 1 } else { 0 };
    }
    let data: HashMap<&str, (f64, &Vec<String>)> = fused.iter().map(|(p, s, l)| (p.as_str(), (*s, l))).collect();
    let fused2: Vec<(String, f64, Vec<String>)> = paths.iter().filter_map(|p| data.get(p.as_str()).map(|(s, l)| (p.clone(), *s, (*l).clone()))).collect();
    if fused2.is_empty() {
        errln!("no matches.");
        return 0;
    }
    if a.vimgrep {
        render_vimgrep(hits, a.limit);
        return 0;
    }
    if a.grid {
        render_grid(ctx, &paths);
        return 0;
    }
    if a.oneline {
        render_oneline(ctx, &paths);
        return 0;
    }
    if a.tree {
        render_tree_view(ctx, &paths, hits);
        return 0;
    }
    if a.long {
        render_long_table(ctx, &paths, hits);
        return 0;
    }
    let ctx_mode = a.context > 0 || a.after_context > 0 || a.before_context > 0 || a.passthru;
    let match_sep = if a.field_match_separator.is_empty() { ":" } else { a.field_match_separator.as_str() };
    let ctx_sep = if a.field_context_separator.is_empty() { "-" } else { a.field_context_separator.as_str() };
    let ctx_divider = if a.no_context_separator { "" } else if a.context_separator.is_empty() { "--" } else { a.context_separator.as_str() };
    let mut shown = 0i64;
    for (path, score, labels) in &fused2 {
        if a.limit > 0 && shown >= a.limit {
            break;
        }
        shown += 1;
        let mut why: Vec<String> = Vec::new();
        if labels.iter().any(|l| l == "name") {
            why.push(c("36", "name", uc));
        }
        if labels.iter().any(|l| l == "content") {
            let n = count_of(path);
            why.push(c("33", &format!("{} hit{}", n, if n != 1 { "s" } else { "" }), uc));
        }
        if labels.iter().any(|l| l == "semantic") {
            why.push(c("35", "semantic", uc));
        }
        if labels.iter().any(|l| l == "git") {
            why.push(c("31", "dirty", uc));
        }
        let tag = why.join(",");
        let icon = ctx.icon(path, false);
        let badge = match ctx.git_status(path) {
            Some(g) => format!(" {}", c("31;1", &format!("[{}]", g), uc)),
            None => String::new(),
        };
        let mut lb = String::new();
        if let Some(mode) = &a.loc {
            if let Some((_, lc)) = meta::classify_file_loc(path) {
                lb = format!(" {}", c("90", &loc_badge(&lc, mode), uc));
            }
        }
        let shown_name = ctx.display_path(path) + if a.classify { meta::classify_indicator(path, None) } else { "" };
        let pd = ctx.link(path, 0, &shown_name);
        let header = format!("{}  [{}]  {}{}{}{}", c("90", &format!("{:6.4}", score), uc), tag, icon, ctx.cc("path", &pd), badge, lb);
        if a.files_only {
            if a.null { util::out(&format!("{}\0", path)); } else { outln!("{}", path); }
            continue;
        }
        let out_path = match &a.path_separator {
            Some(sep) if !sep.is_empty() => path.replace('/', sep).replace('\\', sep),
            _ => path.clone(),
        };
        let h = hits.get(path);
        if a.no_heading {
            if ctx_mode {
                for ld in h.map(|h| h.lines_data.as_slice()).unwrap_or(&[]) {
                    let sep = if ld.is_match { match_sep } else { ctx_sep };
                    let head = if a.no_filename { String::new() } else { format!("{}{}", out_path, sep) };
                    let lineno_disp = if a.column && ld.col > 0 && !a.no_line_number {
                        format!("{}{}{}{}{}", head, ld.lineno, sep, ld.col, sep)
                    } else if !a.no_line_number {
                        format!("{}{}{}", head, ld.lineno, sep)
                    } else {
                        head
                    };
                    let mut text = ld.text.clone();
                    if let (Some(r), true) = (&a.replace, ld.is_match) {
                        text = replacement(&text, r, query_pattern, a.regex);
                    }
                    let off = line_prefix(path, ld.lineno, h, a, false);
                    outln!("{}{} {}", off, lineno_disp, prep_line_text(&text, a));
                }
            } else {
                for (ln, text, col) in h.map(|h| h.samples.as_slice()).unwrap_or(&[]) {
                    let head = if a.no_filename { String::new() } else { format!("{}{}", out_path, match_sep) };
                    let lineno_disp = if a.column && *col > 0 && !a.no_line_number {
                        format!("{}{}{}{}{}", head, ln, match_sep, col, match_sep)
                    } else if !a.no_line_number {
                        format!("{}{}{}", head, ln, match_sep)
                    } else {
                        head
                    };
                    let mut t = text.clone();
                    if let Some(r) = &a.replace {
                        t = replacement(&t, r, query_pattern, a.regex);
                    }
                    let off = line_prefix(path, *ln, h, a, false);
                    outln!("{}{} {}", off, lineno_disp, prep_line_text(&t, a));
                }
            }
            continue;
        }
        outln!("{}", header);
        if a.names {
            continue;
        }
        if ctx_mode {
            let (lines, omitted) = cap_lines_data(h.map(|h| h.lines_data.as_slice()).unwrap_or(&[]), a);
            let mut last: Option<u64> = None;
            for ld in lines {
                if let Some(l) = last {
                    if ld.lineno > l + 1 && !ctx_divider.is_empty() {
                        outln!("    {}", c("90", ctx_divider, uc));
                    }
                }
                last = Some(ld.lineno);
                let sep = if ld.is_match { match_sep } else { ctx_sep };
                let lineno_disp = if a.no_line_number {
                    String::new()
                } else if a.column && ld.col > 0 {
                    ctx.link(path, ld.lineno, &format!("{}{}{}{}", ld.lineno, sep, ld.col, sep))
                } else {
                    ctx.link(path, ld.lineno, &format!("{}{}", ld.lineno, sep))
                };
                let mut text = ld.text.clone();
                if let (Some(r), true) = (&a.replace, ld.is_match) {
                    text = replacement(&text, r, query_pattern, a.regex);
                }
                let off = line_prefix(path, ld.lineno, h, a, true);
                let text = prep_line_text(&text, a);
                if ld.is_match {
                    outln!("    {}{} {}", off, ctx.cc("line", &lineno_disp), text);
                } else {
                    outln!("    {}{} {}", off, c("90", &lineno_disp, uc), text);
                }
            }
            if omitted > 0 {
                outln!("    {}", c("90", &format!("\u{2026} {} more match(es) in this file (raise --max)", omitted), uc));
            }
        } else {
            for (ln, text, col) in h.map(|h| h.samples.as_slice()).unwrap_or(&[]) {
                let lineno_disp = if a.no_line_number {
                    String::new()
                } else if a.column {
                    ctx.link(path, *ln, &format!("{}{}{}{}", ln, match_sep, col, match_sep))
                } else {
                    ctx.link(path, *ln, &format!("{}{}", ln, match_sep))
                };
                let mut t = text.clone();
                if let Some(r) = &a.replace {
                    t = replacement(&t, r, query_pattern, a.regex);
                }
                let off = line_prefix(path, *ln, h, a, true);
                outln!("    {}{} {}", off, c("90", &lineno_disp, uc), prep_line_text(&t, a));
            }
        }
    }
    if a.count {
        let extra = if a.limit > 0 && fused2.len() as i64 > shown { format!(", showing {}", shown) } else { String::new() };
        errln!("\n--- {} file(s) matched{} ---", fused2.len(), extra);
    }
    0
}

/// Machine-readable results for agents: the same object pfind.py printed with
/// json.dumps(indent=1, ensure_ascii=False).
pub fn render_json(fused: &Fused, hits: &Hits, limit: i64) {
    let take = if limit > 0 { (limit as usize).min(fused.len()) } else { fused.len() };
    let mut s = String::new();
    s.push_str("{\n \"engine\": \"ripgrep\",\n");
    s.push_str(&format!(" \"matched\": {},\n", fused.len()));
    if take == 0 {
        s.push_str(" \"results\": []\n}");
        outln!("{}", s);
        return;
    }
    s.push_str(" \"results\": [\n");
    for (i, (path, score, labels)) in fused[..take].iter().enumerate() {
        let h = hits.get(path);
        s.push_str("  {\n");
        s.push_str(&format!("   \"path\": {},\n", util::json_str(path)));
        s.push_str(&format!("   \"score\": {},\n", util::py_float_repr(util::py_round(*score, 5))));
        if labels.is_empty() {
            s.push_str("   \"why\": [],\n");
        } else {
            s.push_str("   \"why\": [\n");
            let w: Vec<String> = labels.iter().map(|l| format!("    {}", util::json_str(l))).collect();
            s.push_str(&w.join(",\n"));
            s.push_str("\n   ],\n");
        }
        s.push_str(&format!("   \"hits\": {},\n", h.map(|h| h.count).unwrap_or(0)));
        let samples = h.map(|h| h.samples.as_slice()).unwrap_or(&[]);
        if samples.is_empty() {
            s.push_str("   \"samples\": []\n");
        } else {
            s.push_str("   \"samples\": [\n");
            let parts: Vec<String> = samples
                .iter()
                .map(|(ln, t, _)| format!("    {{\n     \"line\": {},\n     \"text\": {}\n    }}", ln, util::json_str(t)))
                .collect();
            s.push_str(&parts.join(",\n"));
            s.push_str("\n   ]\n");
        }
        s.push_str(if i + 1 < take { "  },\n" } else { "  }\n" });
    }
    s.push_str(" ]\n}");
    outln!("{}", s);
}
