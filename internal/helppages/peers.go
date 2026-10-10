package helppages

// GORILLA (2026-10-10): the /peers page. Live: it lists the sessions running
// at the moment it opens, this session's own name, and what has arrived lately.
// Written for someone who has never opened a terminal, so it explains what a
// session is before it uses the word, and it says plainly what a message can
// and cannot do.

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/peers"
)

// PeersState is everything the page shows, so a test can give it any state.
type PeersState struct {
	// Running is true when this session takes part in messaging now.
	Running bool
	// SwitchedOn is the /context row's current value, which applies from the
	// next start.
	SwitchedOn bool
	Self       string
	Folder     string
	Others     []peers.Entry
	ListError  string
	Recent     []peers.Message
	Pending    int
}

// peersRowName is the /context row the page tells the reader to look for.
const peersRowName = "Session messaging — /peers"

// Peers is the live /peers page.
func Peers() Page {
	st := PeersState{SwitchedOn: config.LoadoutEnabled(config.PeersComponentID)}
	if s := peers.Active(); s != nil {
		st.Running = true
		st.Self = s.Name()
		st.Folder = s.Folder()
		st.Recent = s.Recent()
		st.Pending = s.Pending()
		if list, err := s.Peers(); err != nil {
			st.ListError = err.Error()
		} else {
			st.Others = list
		}
	}
	return PeersPage(st)
}

// recentShown caps how much of each received message the page repeats.
const recentShown = 200

// PeersPage builds the /peers page from its inputs.
func PeersPage(st PeersState) Page {
	var b builder
	b.p("You can run Gorilla OpenCode more than once at the same time, for example one window for " +
		"each project. Each running copy is called a session. Sessions that you started on this " +
		"computer can see each other's names and send each other short text messages. Nothing goes " +
		"over the internet, and nobody else's sessions can take part.")
	b.blank()

	b.h("This session")
	if st.Running {
		b.p(fmt.Sprintf("This session is called %s. Its folder is %s.", st.Self, st.Folder))
	} else {
		b.p(peers.NotRunning)
	}
	b.blank()

	if st.Running {
		b.h("Other sessions running now")
		switch {
		case st.ListError != "":
			b.p("The list could not be read: " + st.ListError)
		case len(st.Others) == 0:
			b.p("None running. Start Gorilla OpenCode in another window and it appears here.")
		default:
			for _, e := range st.Others {
				state := "idle (waiting for its person)"
				if e.Busy {
					state = "busy (its AI is answering)"
				}
				b.p(fmt.Sprintf("  * %s: folder %s, %s, version %s", e.Name, e.Folder, state, e.Version))
			}
		}
		b.blank()

		if len(st.Recent) > 0 {
			b.h("Received lately")
			for _, m := range st.Recent {
				at := m.At.Local().Format("15:04")
				if m.Kind == peers.KindIdle {
					b.p(fmt.Sprintf("  * %s from %s: finished its turn and is idle (you asked to be told).", at, m.From.Name))
					continue
				}
				text := strings.Join(strings.Fields(m.Text), " ")
				if utf8.RuneCountInString(text) > recentShown {
					text = string([]rune(text)[:recentShown]) + " …"
				}
				b.p(fmt.Sprintf("  * %s from %s: %s", at, m.From.Name, text))
			}
			switch st.Pending {
			case 0:
				b.n("The AI has already been given all of these.")
			case 1:
				b.n("1 of these waits for the AI's next turn.")
			default:
				b.n(fmt.Sprintf("%d of these wait for the AI's next turn.", st.Pending))
			}
			b.blank()
		}
	}

	b.h("Renaming this session")
	b.p("A new session is named after its folder. If another session already has that name, a number " +
		"is added (proj-2, proj-3). To choose a name yourself, type:")
	b.code("  /peers name NEWNAME")
	b.p(fmt.Sprintf("A name may use letters, digits, dots, dashes and underscores, up to %d characters, and "+
		"no other running session may have it already.", 40))
	b.blank()

	b.h("Sending a message")
	b.p("Type /message, then the name of the other session, then your text:")
	b.code("  /message NAME your text")
	b.p("You can also ask the AI in ordinary words, for example \"tell the session called website that " +
		"the release is done\". The AI asks your permission before it sends. It can also ask the other " +
		"session to tell it, once, when that session's current answer is finished.")
	b.blank()

	b.h("What a message does")
	b.p("  * It appears in the other window at once, as a notice its person can read.")
	b.p("  * The other AI reads it at the start of its next answer, inside a marked block that says it " +
		"came from another program and is not its person's instruction.")
	b.p(fmt.Sprintf("  * It can be at most %d KB long, and one session can send at most %d a minute.",
		peers.MaxTextBytes/1024, peers.MaxPerMinute))
	b.blank()

	b.h("What a message cannot do")
	b.p("  * Answer a permission question, switch on /yolo, or change a setting. Only the person at that " +
		"window can do those.")
	b.p("  * Make the other AI start working. It waits until its own person types something.")
	b.p("  * Show the other conversation. Sessions learn each other's name, folder, busy or idle, and " +
		"version, and nothing else.")
	b.p("  * Reach another computer or another account on this one. On Windows the connection point " +
		"admits only your own account; on Linux and macOS it sits in a folder only your account can open.")
	b.p("  * Reach Claude Code or any other program. Only Gorilla OpenCode sessions take part.")
	b.blank()

	b.h("Switching it off")
	b.p("Open /context and switch off the row \"" + peersRowName + "\". From the next start this " +
		"session opens no connection point, is listed nowhere, and the AI has no tool to send or list " +
		"messages.")
	if st.SwitchedOn {
		b.n("The row is ON now.")
	} else {
		b.n("The row is OFF now; it takes effect the next time the program starts.")
	}
	return Page{Name: "peers", Title: "Sessions talking to each other (/peers)", Lines: b.lines}
}
