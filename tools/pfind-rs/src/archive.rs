// SPDX-License-Identifier: AGPL-3.0-or-later
//
// -z/--search-zip, in-process (no gzip/xz programs needed, which ripgrep's -z
// requires and Windows does not have): .gz/.tgz, .bz2, .xz/.lzma, .zst
// streams, and the members of .zip and .tar (plain or compressed), read in
// archive order and concatenated, as pfind.py's built-in engine did.
//
// GORILLA OVERRIDE (2026-08-20, carried from pfind.py): a decompression budget
// shared across the whole archive (default 256 MB, PFIND_MAX_DECOMPRESSED_MB,
// 0 = no cap). A 42 KB zip can expand to gigabytes; when the budget runs out
// the search says so on stderr, naming the file: results are PARTIAL.

use std::fs::File;
use std::io::{self, BufReader, Read};

pub const MAX_DECOMPRESSED_BYTES: u64 = 256 * 1024 * 1024;

pub fn max_decompressed() -> u64 {
    if let Ok(raw) = std::env::var("PFIND_MAX_DECOMPRESSED_MB") {
        let raw = raw.trim();
        if !raw.is_empty() {
            if let Ok(mb) = raw.parse::<f64>() {
                if mb <= 0.0 {
                    return u64::MAX;
                }
                return (mb * 1024.0 * 1024.0) as u64;
            }
        }
    }
    MAX_DECOMPRESSED_BYTES
}

pub struct Budget {
    pub limit: u64,
    pub left: u64,
    pub blown: bool,
}

impl Budget {
    pub fn new(limit: u64) -> Budget {
        Budget { limit, left: limit, blown: false }
    }
    /// Read until EOF or until the budget is spent; only bytes that arrived are billed.
    pub fn read_capped<R: Read>(&mut self, mut r: R, out: &mut Vec<u8>) -> io::Result<()> {
        let mut chunk = vec![0u8; 1 << 20];
        loop {
            let room = self.left.min(chunk.len() as u64) as usize;
            if room == 0 {
                self.blown = true;
                return Ok(());
            }
            let n = r.read(&mut chunk[..room])?;
            if n == 0 {
                return Ok(());
            }
            self.left -= n as u64;
            out.extend_from_slice(&chunk[..n]);
        }
    }
}

#[derive(PartialEq)]
enum Stream {
    Gz,
    Bz2,
    Xz,
    Lzma,
    Zst,
    Plain,
}

fn stream_kind(low: &str) -> Option<Stream> {
    if low.ends_with(".gz") || low.ends_with(".tgz") {
        Some(Stream::Gz)
    } else if low.ends_with(".bz2") || low.ends_with(".tbz2") {
        Some(Stream::Bz2)
    } else if low.ends_with(".xz") || low.ends_with(".txz") {
        Some(Stream::Xz)
    } else if low.ends_with(".lzma") {
        Some(Stream::Lzma)
    } else if low.ends_with(".zst") || low.ends_with(".zstd") {
        Some(Stream::Zst)
    } else {
        None
    }
}

/// True when -z should open this file as an archive or compressed stream.
pub fn is_archive(path: &str) -> bool {
    let low = path.to_lowercase();
    stream_kind(&low).is_some() || low.ends_with(".zip") || low.ends_with(".tar")
}

fn open_stream(path: &str, kind: &Stream) -> io::Result<Box<dyn Read>> {
    let f = BufReader::new(File::open(path)?);
    Ok(match kind {
        Stream::Gz => Box::new(flate2::read::MultiGzDecoder::new(f)),
        Stream::Bz2 => Box::new(bzip2::read::MultiBzDecoder::new(f)),
        Stream::Zst => Box::new(ruzstd::decoding::StreamingDecoder::new(f).map_err(|e| io::Error::other(e.to_string()))?),
        Stream::Xz | Stream::Lzma => {
            // lzma-rs decodes into a writer: cap the writer so a bomb stops at
            // the budget (+1 byte, so the reader below sees it was exceeded).
            let mut src = f;
            let mut w = CappedWriter { buf: Vec::new(), limit: max_decompressed().saturating_add(1) };
            let res = if *kind == Stream::Xz {
                lzma_rs::xz_decompress(&mut src, &mut w)
            } else {
                lzma_rs::lzma_decompress(&mut src, &mut w)
            };
            if let Err(e) = res {
                if (w.buf.len() as u64) < w.limit {
                    return Err(io::Error::other(format!("{:?}", e)));
                }
            }
            Box::new(io::Cursor::new(w.buf))
        }
        Stream::Plain => Box::new(f),
    })
}

struct CappedWriter {
    buf: Vec<u8>,
    limit: u64,
}

impl io::Write for CappedWriter {
    fn write(&mut self, data: &[u8]) -> io::Result<usize> {
        let room = self.limit.saturating_sub(self.buf.len() as u64) as usize;
        if room == 0 {
            return Err(io::Error::other("decompression budget"));
        }
        let n = room.min(data.len());
        self.buf.extend_from_slice(&data[..n]);
        Ok(n)
    }
    fn flush(&mut self) -> io::Result<()> {
        Ok(())
    }
}

/// Decompressed bytes of `path` for searching. Err = could not read it (the
/// caller prints "pfind: PATH: error" unless --no-messages).
pub fn read_archive(path: &str) -> io::Result<Vec<u8>> {
    let low = path.to_lowercase();
    let mut budget = Budget::new(max_decompressed());
    let mut out = Vec::new();
    let is_tar_name = low.ends_with(".tar")
        || low.ends_with(".tgz")
        || low.ends_with(".tbz2")
        || low.ends_with(".txz")
        || [".tar.gz", ".tar.bz2", ".tar.xz", ".tar.zst", ".tar.lzma"].iter().any(|e| low.ends_with(e));
    if low.ends_with(".zip") {
        let mut z = crate::zipread::Zip::open(path)?;
        for i in 0..z.members.len() {
            if z.members[i].name.ends_with('/') {
                continue;
            }
            let member = z.member(i)?;
            budget.read_capped(member, &mut out)?;
            if budget.blown {
                break;
            }
        }
    } else if is_tar_name {
        let kind = stream_kind(&low).unwrap_or(Stream::Plain);
        let mut t = tar::Archive::new(open_stream(path, &kind)?);
        for ent in t.entries()? {
            let ent = ent?;
            if !ent.header().entry_type().is_file() {
                continue;
            }
            budget.read_capped(ent, &mut out)?;
            if budget.blown {
                break;
            }
        }
    } else if let Some(kind) = stream_kind(&low) {
        budget.read_capped(open_stream(path, &kind)?, &mut out)?;
    } else {
        File::open(path)?.read_to_end(&mut out)?;
    }
    if budget.blown {
        crate::errln!(
            "pfind: {} exceeded the {} MB decompression budget - results are PARTIAL. Raise it with PFIND_MAX_DECOMPRESSED_MB if you meant it.",
            path,
            budget.limit / (1024 * 1024)
        );
    }
    Ok(out)
}
