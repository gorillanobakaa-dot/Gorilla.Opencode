package plain

// GORILLA (2026-10-10): session messaging in plain mode — /peers,
// /peers name NEW, /message NAME TEXT, and a printed line when something
// arrives. The page is the same one the full interface shows
// (internal/helppages), printed as ordinary text.

import (
	"fmt"
	"strings"

	"github.com/opencode-ai/opencode/internal/helppages"
	"github.com/opencode-ai/opencode/internal/peers"
)

// attachPeers prints each arrival as one block. It is written in a single call
// so it cannot interleave with itself; it may land between two lines of a reply
// that is streaming, which is the price of plain mode having no screen to place
// it on.
func (s *Session) attachPeers() {
	ps := peers.Active()
	if ps == nil {
		return
	}
	out := s.out
	ps.SetNotify(func(m peers.Message) {
		fmt.Fprint(out, "\n--- from another session ---\n"+peers.HumanNotice(m)+"\n----------------------------\n")
	})
}

func (s *Session) peersCommand(args string) {
	sub, rest := peers.SplitFirst(args)
	switch {
	case sub == "":
		fmt.Fprintln(s.out)
		fmt.Fprintln(s.out, helppages.Peers().PlainText(plainPageWidth))
	case strings.EqualFold(sub, "name"):
		ps := peers.Active()
		if ps == nil {
			fmt.Fprintln(s.out, peers.NotRunning)
			return
		}
		if rest == "" {
			fmt.Fprintln(s.out, "Give the new name: /peers name NEWNAME (letters, digits, dots, dashes, underscores).")
			return
		}
		got, err := ps.Rename(rest)
		if err != nil {
			fmt.Fprintf(s.out, "error: %v\n", err)
			return
		}
		fmt.Fprintf(s.out, "This session is now called %s.\n", got)
	default:
		fmt.Fprintln(s.out, "/peers takes nothing (the page), or: /peers name NEWNAME")
	}
}

func (s *Session) messageCommand(args string) {
	to, text := peers.SplitFirst(args)
	if to == "" || text == "" {
		fmt.Fprintln(s.out, "Type /message, a session name, then the text: /message NAME your text. /peers lists the names.")
		return
	}
	ps := peers.Active()
	if ps == nil {
		fmt.Fprintln(s.out, peers.NotRunning)
		return
	}
	res, err := ps.Send(to, text, false)
	if err != nil {
		fmt.Fprintf(s.out, "not sent: %v\n", err)
		return
	}
	fmt.Fprintln(s.out, res)
}
