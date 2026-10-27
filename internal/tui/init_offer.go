package tui

import (
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/opencode-ai/opencode/internal/tui/util"
)

// GORILLA FIX (2026-10-25): the start-up "Initialize Project" offer must not
// take keys typed for the chat.
//
// Measured on the real exe (Fieldkit chat_paste_probe.py, key trace): the
// program read every key of a paste made right after start-up, but the first
// 35 never reached the chat box. Init opens this offer ~10 ms after launch, on
// any project without the init flag file, and an open offer takes every key: it
// swallowed the paste until the "n" of "Screens" answered it No. A "y", or an
// Enter (Yes is preselected), would have run the initializer, which sends a
// prompt to the model - from keys meant to be a half-typed prompt.
//
// So a key is an answer only when it can be one. Keys that arrive before the
// offer, or before anyone could have read it (offerGrace), were typed for the
// chat. A key after that waits offerAnswerSettle: if another key follows, they
// are a paste or typing, not an answer. In both cases the offer is put away
// without an answer (the project is not marked, so it is offered again next
// launch) and every key goes to the chat box, in order, where the editor's
// paste rule makes an Enter inside the burst a line break.
//
// GORILLA FIX (2026-10-27, job 07): a held key is an answer only if the offer
// has a use for it. Typing at ~30 ms a key (the acceptance runner, and a fast
// typist) let every key settle on its own, so "What is in the folder Folder?"
// reached the model as "t is in the folder Folder?": the offer took "W" and "a"
// (no use for either, dropped) and "h" (its move-left key) before two keys came
// close enough to count as typing. A key the offer cannot use now puts it away
// and goes to the chat; a letter the offer only moves on (h, l) goes to the
// offer but is kept, and handed to the chat ahead of the next key if typing
// follows. Only y, n, q, Enter and Esc answer it, as before.
const (
	// offerGrace: a person needs ~250 ms to react to something that has just
	// appeared; a key sooner than this was on its way before the offer was seen.
	offerGrace = 500 * time.Millisecond
	// offerAnswerSettle: how long an answer waits for more keys. Pasted keys are
	// microseconds apart; a person cannot press two keys within it.
	offerAnswerSettle = 30 * time.Millisecond
)

type initOfferGate struct {
	armed   bool // the open offer is Init's, not one /init asked for
	shownAt time.Time
	typed   bool         // a key has arrived since the program started
	held    []tea.KeyMsg // a possible answer, waiting to see whether more keys follow
	seq     int
	passing bool // the held answer is being handed to the offer
	// moved: letters the offer took as moves (h, l), handed to the chat if a
	// key the offer has no use for follows.
	moved []tea.KeyMsg
}

// offerKeyUse is what the open offer does with a key (dialog/init.go).
type offerKeyUse int

const (
	offerNoUse      offerKeyUse = iota // typing for the chat
	offerMoveLetter                    // h, l: moves the selection, also a letter
	offerMove                          // tab, left, right
	offerAnswer                        // y, n, q, enter, esc
)

func offerUseOf(k tea.KeyMsg) offerKeyUse {
	switch k.String() {
	case "y", "n", "q", "enter", "esc":
		return offerAnswer
	case "h", "l":
		return offerMoveLetter
	case "tab", "left", "right":
		return offerMove
	}
	return offerNoUse
}

// offerKeySettledMsg says the held key with this sequence number was followed
// by silence, so it was pressed as an answer.
type offerKeySettledMsg struct{ seq int }

func (a appModel) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now()
}

// openStartupOffer handles Init's ShowInitDialogMsg.
func (a appModel) openStartupOffer(show bool) appModel {
	if a.offerGate.typed {
		show = false // typing has begun; the offer would take the next key
	}
	a.showInitDialog = show
	a.offerGate = initOfferGate{armed: show, shownAt: a.clock(), typed: a.offerGate.typed}
	return a
}

// gateOfferKey sees every key first. handled says the key was taken here.
func (a appModel) gateOfferKey(k tea.KeyMsg) (next appModel, cmd tea.Cmd, handled bool) {
	g := &a.offerGate
	g.typed = true
	if !a.showInitDialog || !g.armed || g.passing {
		return a, nil, false
	}
	if a.clock().Sub(g.shownAt) < offerGrace || len(g.held) > 0 {
		return a.withdrawOffer(k)
	}
	g.held = append(g.held, k)
	g.seq++
	seq := g.seq
	return a, tea.Tick(offerAnswerSettle, func(time.Time) tea.Msg { return offerKeySettledMsg{seq} }), true
}

// withdrawOffer puts the offer away unanswered and hands the letters it took as
// moves, the held keys, then k, to the chat as if the offer had never been open.
func (a appModel) withdrawOffer(k tea.KeyMsg) (appModel, tea.Cmd, bool) {
	keys := append(append(append([]tea.KeyMsg{}, a.offerGate.moved...), a.offerGate.held...), k)
	a.showInitDialog = false
	a.offerGate.armed = false
	a.offerGate.held = nil
	a.offerGate.moved = nil
	a.offerGate.seq++ // a timer still on its way finds nothing to answer
	cmds := []tea.Cmd{util.ReportInfo("Initialize-project offer put away while you type; /init brings it back.")}
	for _, key := range keys {
		m, cmd := a.update(key)
		cmds = append(cmds, cmd)
		if next, ok := m.(appModel); ok {
			a = next
		}
	}
	return a, tea.Batch(cmds...), true
}

// answerOffer hands a held key that was followed by silence to the offer.
func (a appModel) answerOffer(msg offerKeySettledMsg) (tea.Model, tea.Cmd) {
	g := &a.offerGate
	if msg.seq != g.seq || len(g.held) == 0 || !a.showInitDialog {
		return a, nil
	}
	k := g.held[0]
	g.held = nil
	switch offerUseOf(k) {
	case offerNoUse:
		next, cmd, _ := a.withdrawOffer(k)
		return next, cmd
	case offerMoveLetter:
		g.moved = append(g.moved, k)
	default:
		g.moved = nil
	}
	g.passing = true
	m, cmd := a.update(k)
	if next, ok := m.(appModel); ok {
		next.offerGate.passing = false
		return next, cmd
	}
	return m, cmd
}
