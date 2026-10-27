// SPDX-License-Identifier: AGPL-3.0-or-later
//
// Local time and strftime for the --time-style formats, without a date crate:
// the operating system converts a timestamp to local time (localtime_r on
// Unix, SystemTimeToTzSpecificLocalTime on Windows, which is what Python's
// datetime.fromtimestamp uses underneath), and the codes Python's strftime
// accepts on every platform are formatted here. An unknown code returns None,
// as Python raises ValueError, and the caller falls back to "%Y-%m-%d %H:%M".

pub struct Tm {
    pub year: i64,
    pub mon: u32,  // 1..12
    pub day: u32,  // 1..31
    pub hour: u32,
    pub min: u32,
    pub sec: u32,
    pub micros: u32,
    pub wday: u32, // 0 = Sunday
    pub yday: u32, // 1..366
    pub offset_s: i64,
}

fn is_leap(y: i64) -> bool {
    (y % 4 == 0 && y % 100 != 0) || y % 400 == 0
}

fn yday(y: i64, m: u32, d: u32) -> u32 {
    const CUM: [u32; 12] = [0, 31, 59, 90, 120, 151, 181, 212, 243, 273, 304, 334];
    CUM[(m - 1) as usize] + d + if m > 2 && is_leap(y) { 1 } else { 0 }
}

/// days since 1970-01-01 for a civil date (Hinnant's algorithm)
fn days_from_civil(y: i64, m: u32, d: u32) -> i64 {
    let y = if m <= 2 { y - 1 } else { y };
    let era = if y >= 0 { y } else { y - 399 } / 400;
    let yoe = y - era * 400;
    let mp = (m as i64 + 9) % 12;
    let doy = (153 * mp + 2) / 5 + d as i64 - 1;
    let doe = yoe * 365 + yoe / 4 - yoe / 100 + doy;
    era * 146097 + doe - 719468
}

#[cfg(unix)]
pub fn local(ts: f64) -> Tm {
    let secs = ts.floor() as libc::time_t;
    let micros = ((ts - ts.floor()) * 1e6) as u32;
    let mut tm: libc::tm = unsafe { std::mem::zeroed() };
    unsafe {
        libc::localtime_r(&secs, &mut tm);
    }
    Tm {
        year: tm.tm_year as i64 + 1900,
        mon: tm.tm_mon as u32 + 1,
        day: tm.tm_mday as u32,
        hour: tm.tm_hour as u32,
        min: tm.tm_min as u32,
        sec: tm.tm_sec as u32,
        micros: micros.min(999_999),
        wday: tm.tm_wday as u32,
        yday: tm.tm_yday as u32 + 1,
        offset_s: tm.tm_gmtoff as i64,
    }
}

#[cfg(windows)]
pub fn local(ts: f64) -> Tm {
    use windows_sys::Win32::Foundation::{FILETIME, SYSTEMTIME};
    use windows_sys::Win32::System::Time::{FileTimeToSystemTime, SystemTimeToTzSpecificLocalTime};
    let secs = ts.floor() as i64;
    let micros = (((ts - ts.floor()) * 1e6) as u32).min(999_999);
    let ticks = (secs + 11_644_473_600) as u64 * 10_000_000;
    let ft = FILETIME { dwLowDateTime: ticks as u32, dwHighDateTime: (ticks >> 32) as u32 };
    let mut utc: SYSTEMTIME = unsafe { std::mem::zeroed() };
    let mut loc: SYSTEMTIME = unsafe { std::mem::zeroed() };
    let ok = unsafe { FileTimeToSystemTime(&ft, &mut utc) != 0 && SystemTimeToTzSpecificLocalTime(std::ptr::null(), &utc, &mut loc) != 0 };
    if !ok {
        loc = utc;
    }
    let (y, m, d) = (loc.wYear as i64, loc.wMonth as u32, loc.wDay as u32);
    let local_secs = days_from_civil(y, m, d) * 86400 + loc.wHour as i64 * 3600 + loc.wMinute as i64 * 60 + loc.wSecond as i64;
    Tm {
        year: y,
        mon: m,
        day: d,
        hour: loc.wHour as u32,
        min: loc.wMinute as u32,
        sec: loc.wSecond as u32,
        micros,
        wday: loc.wDayOfWeek as u32,
        yday: yday(y, m, d),
        offset_s: local_secs - secs,
    }
}

const MON: [&str; 12] = ["January", "February", "March", "April", "May", "June", "July", "August", "September", "October", "November", "December"];
const DAY: [&str; 7] = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

pub fn strftime(t: &Tm, fmt: &str) -> Option<String> {
    let mut o = String::new();
    let mut it = fmt.chars();
    while let Some(ch) = it.next() {
        if ch != '%' {
            o.push(ch);
            continue;
        }
        let code = it.next()?;
        let h12 = if t.hour % 12 == 0 { 12 } else { t.hour % 12 };
        match code {
            'Y' => o += &t.year.to_string(),
            'y' => o += &format!("{:02}", t.year.rem_euclid(100)),
            'm' => o += &format!("{:02}", t.mon),
            'd' => o += &format!("{:02}", t.day),
            'H' => o += &format!("{:02}", t.hour),
            'I' => o += &format!("{:02}", h12),
            'M' => o += &format!("{:02}", t.min),
            'S' => o += &format!("{:02}", t.sec),
            'f' => o += &format!("{:06}", t.micros),
            'p' => o += if t.hour < 12 { "AM" } else { "PM" },
            'j' => o += &format!("{:03}", t.yday),
            'b' => o += &MON[(t.mon - 1) as usize][..3],
            'B' => o += MON[(t.mon - 1) as usize],
            'a' => o += &DAY[t.wday as usize][..3],
            'A' => o += DAY[t.wday as usize],
            'w' => o += &t.wday.to_string(),
            'z' => {
                let s = t.offset_s;
                let (sign, a) = if s < 0 { ('-', -s) } else { ('+', s) };
                o += &format!("{}{:02}{:02}", sign, a / 3600, (a % 3600) / 60);
            }
            '%' => o.push('%'),
            _ => return None,
        }
    }
    Some(o)
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn civil_days() {
        assert_eq!(days_from_civil(1970, 1, 1), 0);
        assert_eq!(days_from_civil(2020, 1, 1), 18262);
        assert_eq!(yday(2020, 3, 1), 61);
    }
    #[test]
    fn formats() {
        let t = Tm { year: 2026, mon: 10, day: 7, hour: 13, min: 5, sec: 9, micros: 42, wday: 3, yday: 280, offset_s: 3600 };
        assert_eq!(strftime(&t, "%Y-%m-%d %H:%M:%S.%f %z").unwrap(), "2026-10-07 13:05:09.000042 +0100");
        assert_eq!(strftime(&t, "%a %b %I%p %%").unwrap(), "Wed Oct 01PM %");
        assert!(strftime(&t, "%Q").is_none());
    }
}
