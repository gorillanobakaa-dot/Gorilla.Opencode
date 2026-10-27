package tools

import (
	"fmt"
	"net/url"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// htmlToText renders an HTML page as plain text in the shape `lynx -dump`
// writes: the readable text, every link marked "[n]" where it stands, and a
// "References" list of the numbered absolute URLs at the end.
//
// GORILLA FIX (2026-10-27, job 09, gap #12): general web search drove lynx,
// and lynx has no Windows installer this program may use, so on Windows the
// web source was simply off. Producing lynx's own layout means the one parser
// that already serves every engine (parseLynxDump) reads this output as it
// is; nothing about result extraction is duplicated.
//
// Only what a text browser shows is kept: scripts, styles, templates, SVG and
// the head are dropped, block elements start a new line, and runs of
// whitespace inside a line collapse to one space. Links whose address is not
// http or https (javascript:, mailto:, #fragment) are not numbered.
func htmlToText(src string, base *url.URL) string {
	doc, err := html.Parse(strings.NewReader(src))
	if err != nil {
		return ""
	}
	r := &textRenderer{base: base}
	r.walk(doc)
	r.newline()

	var b strings.Builder
	for _, line := range r.lines {
		b.WriteString(line)
		b.WriteByte('\n')
	}
	if len(r.refs) > 0 {
		b.WriteString("\nReferences\n\n")
		for i, u := range r.refs {
			fmt.Fprintf(&b, "  %3d. %s\n", i+1, u)
		}
	}
	return b.String()
}

type textRenderer struct {
	base  *url.URL
	lines []string
	cur   strings.Builder
	space bool // a space is owed before the next word on this line
	refs  []string
}

// skipped elements never contribute text.
var skippedElements = map[atom.Atom]bool{
	atom.Head: true, atom.Script: true, atom.Style: true, atom.Noscript: true,
	atom.Template: true, atom.Svg: true, atom.Iframe: true, atom.Object: true,
	atom.Select: true,
}

// blockElements start and end on their own line.
var blockElements = map[atom.Atom]bool{
	atom.P: true, atom.Div: true, atom.Li: true, atom.Ul: true, atom.Ol: true,
	atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true,
	atom.Tr: true, atom.Table: true, atom.Section: true, atom.Article: true,
	atom.Header: true, atom.Footer: true, atom.Nav: true, atom.Main: true, atom.Aside: true,
	atom.Blockquote: true, atom.Pre: true, atom.Dl: true, atom.Dt: true, atom.Dd: true,
	atom.Form: true, atom.Fieldset: true, atom.Figure: true, atom.Figcaption: true,
	atom.Hr: true, atom.Address: true, atom.Body: true,
}

func (r *textRenderer) walk(n *html.Node) {
	switch n.Type {
	case html.TextNode:
		r.text(n.Data)
		return
	case html.ElementNode:
		if skippedElements[n.DataAtom] {
			return
		}
		if n.DataAtom == atom.Br {
			r.newline()
			return
		}
		if n.DataAtom == atom.Img {
			if alt := attr(n, "alt"); strings.TrimSpace(alt) != "" {
				r.text(alt)
			}
			return
		}
		block := blockElements[n.DataAtom]
		if block {
			r.newline()
		}
		if n.DataAtom == atom.A {
			if u := r.resolve(attr(n, "href")); u != "" {
				r.refs = append(r.refs, u)
				r.word(fmt.Sprintf("[%d]", len(r.refs)), false)
				r.space = false
			}
		}
		if n.DataAtom == atom.Td || n.DataAtom == atom.Th {
			r.space = true
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			r.walk(c)
		}
		if block {
			r.newline()
		}
		return
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		r.walk(c)
	}
}

// text adds running text, collapsing whitespace the way a browser does.
func (r *textRenderer) text(s string) {
	if s == "" {
		return
	}
	if isHTMLSpace(rune(s[0])) {
		r.space = true
	}
	words := strings.Fields(s)
	for i, w := range words {
		r.word(w, i > 0 || r.space)
	}
	if len(words) > 0 && isHTMLSpace(rune(s[len(s)-1])) {
		r.space = true
	}
}

func (r *textRenderer) word(w string, spaceBefore bool) {
	if r.cur.Len() > 0 && (spaceBefore || r.space) {
		r.cur.WriteByte(' ')
	}
	r.cur.WriteString(w)
	r.space = false
}

func (r *textRenderer) newline() {
	if line := strings.TrimSpace(r.cur.String()); line != "" {
		r.lines = append(r.lines, line)
	} else if len(r.lines) > 0 && r.lines[len(r.lines)-1] != "" {
		// One blank line between blocks, never a run of them.
		r.lines = append(r.lines, "")
	}
	r.cur.Reset()
	r.space = false
}

// resolve turns an href into an absolute http(s) URL, or "".
func (r *textRenderer) resolve(href string) string {
	href = strings.TrimSpace(href)
	if href == "" || strings.HasPrefix(href, "#") {
		return ""
	}
	u, err := url.Parse(href)
	if err != nil {
		return ""
	}
	if r.base != nil {
		u = r.base.ResolveReference(u)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	return u.String()
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if strings.EqualFold(a.Key, key) {
			return a.Val
		}
	}
	return ""
}

func isHTMLSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r' || r == '\f'
}
