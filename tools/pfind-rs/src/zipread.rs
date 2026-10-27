// SPDX-License-Identifier: AGPL-3.0-or-later
//
// A read-only .zip member reader for -z: the central directory (zip64
// included) in archive order, members stored (method 0) or deflated
// (method 8, through flate2's pure-Rust inflater). Written here instead of
// vendoring the `zip` crate, which pulled 20 more crates (proc-macros
// included) for the one thing pfind needs: read each member's bytes once.

use std::fs::File;
use std::io::{self, BufReader, Read, Seek, SeekFrom};

fn u16le(b: &[u8], o: usize) -> u64 {
    u16::from_le_bytes([b[o], b[o + 1]]) as u64
}
fn u32le(b: &[u8], o: usize) -> u64 {
    u32::from_le_bytes([b[o], b[o + 1], b[o + 2], b[o + 3]]) as u64
}
fn u64le(b: &[u8], o: usize) -> u64 {
    let mut a = [0u8; 8];
    a.copy_from_slice(&b[o..o + 8]);
    u64::from_le_bytes(a)
}
fn bad(msg: &str) -> io::Error {
    io::Error::new(io::ErrorKind::InvalidData, format!("File is not a zip file ({})", msg))
}

pub struct Member {
    pub name: String,
    method: u64,
    flags: u64,
    csize: u64,
    offset: u64,
}

pub struct Zip {
    f: BufReader<File>,
    pub members: Vec<Member>,
}

impl Zip {
    pub fn open(path: &str) -> io::Result<Zip> {
        let mut f = BufReader::new(File::open(path)?);
        let len = f.seek(SeekFrom::End(0))?;
        let tail_len = len.min(65_557);
        f.seek(SeekFrom::Start(len - tail_len))?;
        let mut tail = vec![0u8; tail_len as usize];
        f.read_exact(&mut tail)?;
        let pos = (0..tail.len().saturating_sub(21)).rev().find(|&i| tail[i..i + 4] == [0x50, 0x4b, 0x05, 0x06]).ok_or_else(|| bad("no end record"))?;
        let mut count = u16le(&tail, pos + 10);
        let mut cd_size = u32le(&tail, pos + 12);
        let mut cd_off = u32le(&tail, pos + 16);
        if (count == 0xFFFF || cd_off == 0xFFFF_FFFF) && pos >= 20 && tail[pos - 20..pos - 16] == [0x50, 0x4b, 0x06, 0x07] {
            let e64 = u64le(&tail, pos - 12);
            f.seek(SeekFrom::Start(e64))?;
            let mut rec = [0u8; 56];
            f.read_exact(&mut rec)?;
            if rec[..4] != [0x50, 0x4b, 0x06, 0x06] {
                return Err(bad("bad zip64 end record"));
            }
            count = u64le(&rec, 32);
            cd_size = u64le(&rec, 40);
            cd_off = u64le(&rec, 48);
        }
        if cd_size > len {
            return Err(bad("central directory larger than the file"));
        }
        f.seek(SeekFrom::Start(cd_off))?;
        let mut cd = vec![0u8; cd_size as usize];
        f.read_exact(&mut cd)?;
        let mut members = Vec::new();
        let mut i = 0usize;
        for _ in 0..count {
            if i + 46 > cd.len() || cd[i..i + 4] != [0x50, 0x4b, 0x01, 0x02] {
                return Err(bad("truncated central directory"));
            }
            let flags = u16le(&cd, i + 8);
            let method = u16le(&cd, i + 10);
            let mut csize = u32le(&cd, i + 20);
            let usize_ = u32le(&cd, i + 24);
            let nlen = u16le(&cd, i + 28) as usize;
            let elen = u16le(&cd, i + 30) as usize;
            let clen = u16le(&cd, i + 32) as usize;
            let mut offset = u32le(&cd, i + 42);
            if i + 46 + nlen + elen > cd.len() {
                return Err(bad("truncated central directory"));
            }
            let name = String::from_utf8_lossy(&cd[i + 46..i + 46 + nlen]).into_owned();
            // zip64 extra field: the values that overflowed 32 bits, in this order
            let extra = &cd[i + 46 + nlen..i + 46 + nlen + elen];
            let mut e = 0usize;
            while e + 4 <= extra.len() {
                let id = u16le(extra, e);
                let sz = u16le(extra, e + 2) as usize;
                if id == 1 {
                    let mut k = e + 4;
                    if usize_ == 0xFFFF_FFFF && k + 8 <= extra.len() {
                        k += 8;
                    }
                    if csize == 0xFFFF_FFFF && k + 8 <= extra.len() {
                        csize = u64le(extra, k);
                        k += 8;
                    }
                    if offset == 0xFFFF_FFFF && k + 8 <= extra.len() {
                        offset = u64le(extra, k);
                    }
                }
                e += 4 + sz;
            }
            members.push(Member { name, method, flags, csize, offset });
            i += 46 + nlen + elen + clen;
        }
        Ok(Zip { f, members })
    }

    /// A reader over one member's uncompressed bytes.
    pub fn member(&mut self, idx: usize) -> io::Result<Box<dyn Read + '_>> {
        let m = &self.members[idx];
        if m.flags & 1 != 0 {
            return Err(io::Error::other(format!("File {:?} is encrypted, password required for extraction", m.name)));
        }
        self.f.seek(SeekFrom::Start(m.offset))?;
        let mut lh = [0u8; 30];
        self.f.read_exact(&mut lh)?;
        if lh[..4] != [0x50, 0x4b, 0x03, 0x04] {
            return Err(bad("bad local header"));
        }
        let skip = u16le(&lh, 26) + u16le(&lh, 28);
        self.f.seek(SeekFrom::Current(skip as i64))?;
        let (method, csize) = (m.method, m.csize);
        let data = (&mut self.f).take(csize);
        match method {
            0 => Ok(Box::new(data)),
            8 => Ok(Box::new(flate2::read::DeflateDecoder::new(data))),
            other => Err(io::Error::other(format!("compression method {} is not supported", other))),
        }
    }
}
