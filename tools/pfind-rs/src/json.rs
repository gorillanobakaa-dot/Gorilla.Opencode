// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Just enough JSON to read presets.json: one object whose values pfind uses
// as strings. Anything that is not valid JSON is an error the caller reports
// ("pfind: ignoring FILE: ..."), as pfind.py's json.load did.

use std::collections::HashMap;

pub enum Val {
    Str(String),
    Null,
    Other(String), // number, bool, array or object, as written
}

struct P<'a> {
    s: &'a [u8],
    i: usize,
}

impl P<'_> {
    fn ws(&mut self) {
        while self.i < self.s.len() && matches!(self.s[self.i], b' ' | b'\t' | b'\n' | b'\r') {
            self.i += 1;
        }
    }
    fn err<T>(&self, what: &str) -> Result<T, String> {
        Err(format!("{} at byte {}", what, self.i))
    }
    fn string(&mut self) -> Result<String, String> {
        if self.s.get(self.i) != Some(&b'"') {
            return self.err("Expecting string");
        }
        self.i += 1;
        let mut out: Vec<u8> = Vec::new();
        loop {
            let Some(&c) = self.s.get(self.i) else { return self.err("Unterminated string") };
            self.i += 1;
            match c {
                b'"' => break,
                b'\\' => {
                    let Some(&e) = self.s.get(self.i) else { return self.err("Unterminated string") };
                    self.i += 1;
                    match e {
                        b'"' => out.push(b'"'),
                        b'\\' => out.push(b'\\'),
                        b'/' => out.push(b'/'),
                        b'b' => out.push(8),
                        b'f' => out.push(12),
                        b'n' => out.push(b'\n'),
                        b'r' => out.push(b'\r'),
                        b't' => out.push(b'\t'),
                        b'u' => {
                            let hex = std::str::from_utf8(self.s.get(self.i..self.i + 4).unwrap_or(b"")).unwrap_or("");
                            let mut cp = u32::from_str_radix(hex, 16).map_err(|_| "Invalid \\uXXXX escape".to_string())?;
                            self.i += 4;
                            if (0xD800..0xDC00).contains(&cp) && self.s.get(self.i..self.i + 2) == Some(b"\\u") {
                                let lo = std::str::from_utf8(self.s.get(self.i + 2..self.i + 6).unwrap_or(b"")).unwrap_or("");
                                if let Ok(lo) = u32::from_str_radix(lo, 16) {
                                    cp = 0x10000 + ((cp - 0xD800) << 10) + (lo - 0xDC00);
                                    self.i += 6;
                                }
                            }
                            let ch = char::from_u32(cp).unwrap_or('\u{fffd}');
                            out.extend_from_slice(ch.to_string().as_bytes());
                        }
                        _ => return self.err("Invalid \\escape"),
                    }
                }
                c if c < 0x20 => return self.err("Invalid control character"),
                c => out.push(c),
            }
        }
        String::from_utf8(out).map_err(|_| "invalid UTF-8".to_string())
    }
    fn skip_value(&mut self) -> Result<(), String> {
        self.ws();
        match self.s.get(self.i) {
            Some(b'"') => self.string().map(|_| ()),
            Some(b'{') | Some(b'[') => {
                let close = if self.s[self.i] == b'{' { b'}' } else { b']' };
                self.i += 1;
                self.ws();
                if self.s.get(self.i) == Some(&close) {
                    self.i += 1;
                    return Ok(());
                }
                loop {
                    if close == b'}' {
                        self.ws();
                        self.string()?;
                        self.ws();
                        if self.s.get(self.i) != Some(&b':') {
                            return self.err("Expecting ':' delimiter");
                        }
                        self.i += 1;
                    }
                    self.skip_value()?;
                    self.ws();
                    match self.s.get(self.i) {
                        Some(b',') => self.i += 1,
                        Some(c) if *c == close => {
                            self.i += 1;
                            return Ok(());
                        }
                        _ => return self.err("Expecting ',' delimiter"),
                    }
                }
            }
            Some(_) => {
                let start = self.i;
                while self.i < self.s.len() && !matches!(self.s[self.i], b',' | b'}' | b']' | b' ' | b'\t' | b'\n' | b'\r') {
                    self.i += 1;
                }
                let tok = std::str::from_utf8(&self.s[start..self.i]).unwrap_or("");
                if tok == "true" || tok == "false" || tok == "null" || tok.parse::<f64>().is_ok() {
                    Ok(())
                } else {
                    self.i = start;
                    self.err("Expecting value")
                }
            }
            None => self.err("Expecting value"),
        }
    }
}

/// Parse a top-level JSON object. Values other than strings are kept as text.
pub fn parse_object(text: &str) -> Result<HashMap<String, Val>, String> {
    let text = text.strip_prefix('\u{feff}').unwrap_or(text);
    let mut p = P { s: text.as_bytes(), i: 0 };
    let mut out = HashMap::new();
    p.ws();
    if p.s.get(p.i) != Some(&b'{') {
        // valid JSON that is not an object: pfind.py's data.get() then failed
        p.skip_value()?;
        return Err("presets must be a JSON object".into());
    }
    p.i += 1;
    p.ws();
    if p.s.get(p.i) == Some(&b'}') {
        p.i += 1;
    } else {
        loop {
            p.ws();
            let key = p.string()?;
            p.ws();
            if p.s.get(p.i) != Some(&b':') {
                return p.err("Expecting ':' delimiter");
            }
            p.i += 1;
            p.ws();
            let start = p.i;
            let val = if p.s.get(p.i) == Some(&b'"') {
                Val::Str(p.string()?)
            } else {
                p.skip_value()?;
                let raw = std::str::from_utf8(&p.s[start..p.i]).unwrap_or("").trim().to_string();
                if raw == "null" { Val::Null } else { Val::Other(raw) }
            };
            out.insert(key, val);
            p.ws();
            match p.s.get(p.i) {
                Some(b',') => p.i += 1,
                Some(b'}') => {
                    p.i += 1;
                    break;
                }
                _ => return p.err("Expecting ',' delimiter"),
            }
        }
    }
    p.ws();
    if p.i != p.s.len() {
        return p.err("Extra data");
    }
    Ok(out)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn reads_presets() {
        let m = parse_object(r#"{"brain": "~/notes", "n": 3, "x": null, "o": {"a": [1, 2]}, "u": "é"}"#).unwrap();
        assert!(matches!(m.get("brain"), Some(Val::Str(s)) if s == "~/notes"));
        assert!(matches!(m.get("n"), Some(Val::Other(s)) if s == "3"));
        assert!(matches!(m.get("x"), Some(Val::Null)));
        assert!(matches!(m.get("u"), Some(Val::Str(s)) if s == "é"));
        assert!(parse_object("{\"a\": }").is_err());
        assert!(parse_object("{} x").is_err());
    }
}
