package forensics

import (
	"bufio"
	"fmt"
	"io"
	"math"
	"strings"
)

// entropyOf returns the Shannon entropy of up to entropyCap bytes of r, in
// bits per byte, formatted for a section table. A section bigger than the cap
// is measured on its first entropyCap bytes and marked with "~".
func entropyOf(r io.Reader, size uint64) string {
	var hist [256]uint64
	buf := make([]byte, 64<<10)
	var n uint64
	lr := io.LimitReader(r, entropyCap)
	for {
		k, err := lr.Read(buf)
		for _, c := range buf[:k] {
			hist[c]++
		}
		n += uint64(k)
		if err != nil {
			break
		}
	}
	if n == 0 {
		return "-"
	}
	mark := ""
	if size > n {
		mark = "~"
	}
	return fmt.Sprintf("%s%.2f", mark, entropy(hist[:], n))
}

func entropy(hist []uint64, n uint64) float64 {
	var e float64
	for _, c := range hist {
		if c == 0 {
			continue
		}
		p := float64(c) / float64(n)
		e -= p * math.Log2(p)
	}
	return e
}

// foundString is one printable run found in a file.
type foundString struct {
	Off  int64
	Wide bool // UTF-16LE
	Text string
}

// scanStrings finds printable ASCII runs, and UTF-16LE runs of ASCII
// characters (how Windows stores most of its text), of at least min
// characters, in one pass over at most limit bytes. keep is called for each
// and returns false to stop collecting; counting still runs to the end.
func scanStrings(r io.Reader, min int, limit int64, keep func(foundString) bool) (total int, scanned int64) {
	br := bufio.NewReaderSize(io.LimitReader(r, limit), 256<<10)
	var (
		asc      []byte
		ascStart int64
		// Two UTF-16LE decoders, one per byte phase, so a wide string that
		// starts on an odd offset is found as well as one on an even offset.
		// For decoder p a byte at an offset of parity p is a character's low
		// byte; the byte after it is the high byte and must be zero.
		wide      [2][]byte
		wideStart [2]int64
		low       [2]byte
		haveLow   [2]bool
		collect   = true
		off       int64
	)
	emit := func(s foundString) {
		total++
		if collect && !keep(s) {
			collect = false
		}
	}
	flushASCII := func() {
		if len(asc) >= min {
			emit(foundString{Off: ascStart, Text: string(asc)})
		}
		asc = asc[:0]
	}
	flushWide := func(p int) {
		if len(wide[p]) >= min {
			emit(foundString{Off: wideStart[p], Wide: true, Text: string(wide[p])})
		}
		wide[p] = wide[p][:0]
		haveLow[p] = false
	}
	for {
		c, err := br.ReadByte()
		if err != nil {
			break
		}
		printable := c == '\t' || (c >= 0x20 && c < 0x7f)
		if printable {
			if len(asc) == 0 {
				ascStart = off
			}
			if len(asc) < maxStringLen {
				asc = append(asc, c)
			}
		} else {
			flushASCII()
		}
		for p := 0; p < 2; p++ {
			if int(off&1) == p { // low byte
				if printable {
					low[p], haveLow[p] = c, true
				} else {
					flushWide(p)
				}
				continue
			}
			// high byte
			if haveLow[p] && c == 0 {
				if len(wide[p]) == 0 {
					wideStart[p] = off - 1
				}
				if len(wide[p]) < maxStringLen {
					wide[p] = append(wide[p], low[p])
				}
				haveLow[p] = false
			} else {
				flushWide(p)
			}
		}
		off++
	}
	flushASCII()
	flushWide(0)
	flushWide(1)
	return total, off
}

func writeStrings(b *strings.Builder, r io.Reader, size int64, o Options) {
	min := o.MinLen
	if min <= 0 {
		min = 6
	}
	if min < 3 {
		min = 3
	}
	lf := strings.ToLower(o.Filter)
	var shown []foundString
	matched := 0
	total, scanned := scanStrings(r, min, stringScanCap, func(s foundString) bool {
		if lf != "" && !strings.Contains(strings.ToLower(s.Text), lf) {
			return true
		}
		matched++
		if len(shown) < maxStrings {
			shown = append(shown, s)
		}
		return true
	})
	if lf != "" {
		fmt.Fprintf(b, "strings (min %d chars, ASCII + UTF-16LE) matching %q: %d of %d\n", min, o.Filter, matched, total)
	} else {
		fmt.Fprintf(b, "strings (min %d chars, ASCII + UTF-16LE): %d\n", min, total)
	}
	b.WriteString("offset, a=ASCII w=UTF-16LE, text\n")
	for _, s := range shown {
		k := "a"
		if s.Wide {
			k = "w"
		}
		fmt.Fprintf(b, "  0x%x %s %s\n", s.Off, k, clean(s.Text))
	}
	if matched > len(shown) {
		fmt.Fprintf(b, "  ... %d more not shown (capped at %d; use filter to narrow)\n", matched-len(shown), maxStrings)
	}
	if scanned < size {
		fmt.Fprintf(b, "scan capped: only the first %d of %d bytes were read\n", scanned, size)
	}
}
