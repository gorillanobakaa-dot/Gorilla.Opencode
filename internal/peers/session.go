package peers

// GORILLA (2026-10-10): this process's place among its peers — its name, its
// endpoint, its register entry, and what it has received.

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// Options configures Start.
type Options struct {
	// Dir is the register folder, normally DefaultDir(config.StateBase()).
	Dir string
	// Folder returns the working folder now. Read on every register write, so a
	// /cd is reflected the next time the entry is written.
	Folder func() string
	// Version is the program version shown to peers.
	Version string
	// PID is this process; 0 means os.Getpid().
	PID int
	// Tag lets a test run two sessions inside one process. Never set in the
	// program itself.
	Tag string
}

// Session is this process's messaging state.
type Session struct {
	reg      Registry
	key      string
	endpoint string
	pid      int
	version  string
	folder   func() string
	started  time.Time
	srv      *Server
	bg       sync.WaitGroup

	mu       sync.Mutex
	name     string
	busy     bool
	closed   bool
	inbox    []Message
	recent   []Message
	idleSubs map[string]Entry // sender -> where its idle notice goes
	awaiting map[string]bool  // sessions this one asked for an idle notice
	rate     map[string][]time.Time
	notify   func(Message)
}

var tagRe = regexp.MustCompile(`^[a-z0-9]{1,16}$`)

// Start opens this session's endpoint, picks a free name and writes the
// register entry. The endpoint is opened first: a session that cannot be
// reached must not take a name.
func Start(o Options) (*Session, error) {
	if o.PID <= 0 {
		o.PID = os.Getpid()
	}
	if o.Folder == nil {
		wd, _ := os.Getwd()
		o.Folder = func() string { return wd }
	}
	key := strconv.Itoa(o.PID)
	if o.Tag != "" {
		if !tagRe.MatchString(o.Tag) {
			return nil, fmt.Errorf("invalid tag %q", o.Tag)
		}
		key += "-" + o.Tag
	}
	reg := Registry{Dir: o.Dir}
	if err := reg.ensure(); err != nil {
		return nil, fmt.Errorf("the register folder: %w", err)
	}
	s := &Session{
		reg:      reg,
		key:      key,
		endpoint: endpointFor(o.Dir, key),
		pid:      o.PID,
		version:  cleanLine(o.Version, 64),
		folder:   o.Folder,
		started:  time.Now().UTC(),
		idleSubs: map[string]Entry{},
		awaiting: map[string]bool{},
		rate:     map[string][]time.Time{},
	}
	srv, err := Listen(s.endpoint, s.handle)
	if err != nil {
		return nil, fmt.Errorf("could not open the local endpoint %s: %w", s.endpoint, err)
	}
	s.srv = srv

	if err := s.claimName(baseName(s.folder())); err != nil {
		srv.Close()
		reg.remove(key)
		return nil, err
	}
	return s, nil
}

// claimName takes the first free name from base and writes the entry. Two
// sessions starting in the same folder at the same moment can both see the name
// as free; after writing, the later starter (by start time, then key) yields
// and takes the next one.
func (s *Session) claimName(base string) error {
	for attempt := 0; attempt < 3; attempt++ {
		others, err := s.Peers()
		if err != nil {
			return err
		}
		taken := map[string]bool{}
		for _, e := range others {
			taken[strings.ToLower(e.Name)] = true
		}
		s.mu.Lock()
		s.name = uniqueName(base, taken)
		err = s.writeLocked()
		name := s.name
		s.mu.Unlock()
		if err != nil {
			return err
		}
		others, err = s.Peers()
		if err != nil {
			return err
		}
		clash := false
		for _, e := range others {
			if strings.EqualFold(e.Name, name) &&
				(e.Started.Before(s.started) || (e.Started.Equal(s.started) && e.key < s.key)) {
				clash = true
			}
		}
		if !clash {
			return nil
		}
	}
	return nil
}

// writeLocked writes this session's entry. Callers hold s.mu.
func (s *Session) writeLocked() error {
	return s.reg.write(Entry{
		Name:     s.name,
		Folder:   cleanLine(s.folder(), maxFolderLen),
		PID:      s.pid,
		Endpoint: s.endpoint,
		Version:  s.version,
		Started:  s.started,
		Busy:     s.busy,
		key:      s.key,
	})
}

// Close removes the entry first, so nobody new looks this session up, then
// stops the endpoint and waits for idle notices still being sent.
func (s *Session) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	s.reg.remove(s.key)
	s.srv.Close()
	s.bg.Wait()
	s.reg.remove(s.key)
}

// Name is this session's name.
func (s *Session) Name() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.name
}

// Folder is this session's working folder as peers see it.
func (s *Session) Folder() string { return cleanLine(s.folder(), maxFolderLen) }

// Busy reports what this session last told its peers.
func (s *Session) Busy() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.busy
}

// Endpoint is the pipe name or socket path this session answers on.
func (s *Session) Endpoint() string { return s.endpoint }

// Identity is how this session introduces itself.
func (s *Session) Identity() Identity {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.identityLocked()
}

func (s *Session) identityLocked() Identity {
	return Identity{Name: s.name, PID: s.pid, Folder: cleanLine(s.folder(), maxFolderLen)}
}

// Rename gives this session a new name, refused if another running session
// already has it.
func (s *Session) Rename(n string) (string, error) {
	n = strings.TrimSpace(n)
	if !ValidName(n) {
		return "", fmt.Errorf("%q cannot be a session name: use 1 to %d letters, digits, dots, dashes or underscores, with no spaces",
			n, maxNameLen)
	}
	others, err := s.Peers()
	if err != nil {
		return "", err
	}
	for _, e := range others {
		if strings.EqualFold(e.Name, n) {
			return "", fmt.Errorf("another running session (folder %s) is already called %s", e.Folder, e.Name)
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	old := s.name
	s.name = n
	if err := s.writeLocked(); err != nil {
		s.name = old
		return "", err
	}
	return n, nil
}

// Peers lists the other live sessions.
func (s *Session) Peers() ([]Entry, error) {
	all, err := s.reg.List()
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(all))
	for _, e := range all {
		if e.key != s.key {
			out = append(out, e)
		}
	}
	return out, nil
}

// Lookup finds a peer by name: exact first, then ignoring case.
func (s *Session) Lookup(name string) (Entry, error) {
	name = strings.TrimSpace(name)
	if strings.EqualFold(name, s.Name()) {
		return Entry{}, fmt.Errorf("%s is this session's own name", name)
	}
	others, err := s.Peers()
	if err != nil {
		return Entry{}, err
	}
	for _, e := range others {
		if e.Name == name {
			return e, nil
		}
	}
	for _, e := range others {
		if strings.EqualFold(e.Name, name) {
			return e, nil
		}
	}
	if len(others) == 0 {
		return Entry{}, fmt.Errorf("no session called %q is running; no other Gorilla OpenCode session is running on this computer", name)
	}
	names := make([]string, 0, len(others))
	for _, e := range others {
		names = append(names, e.Name)
	}
	return Entry{}, fmt.Errorf("no session called %q is running. Running now: %s", name, strings.Join(names, ", "))
}

// SetBusy records a change between busy and idle. Becoming idle sends each
// waiting idle notice exactly once and forgets the subscription.
func (s *Session) SetBusy(b bool) {
	s.mu.Lock()
	if s.closed || s.busy == b {
		s.mu.Unlock()
		return
	}
	s.busy = b
	_ = s.writeLocked()
	var targets []Entry
	if !b {
		for k, e := range s.idleSubs {
			targets = append(targets, e)
			delete(s.idleSubs, k)
		}
	}
	me := s.identityLocked()
	s.mu.Unlock()
	for _, e := range targets {
		s.sendIdleAsync(e, me)
	}
}

// sendIdleAsync sends one idle notice without holding up the caller, which is
// the end of a turn.
func (s *Session) sendIdleAsync(target Entry, me Identity) {
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		_, _ = call(target.Endpoint, target.PID, Request{Type: TypeIdleNotice, From: me})
	}()
}

// Send delivers text to the session called to. With notifyWhenIdle it also asks
// that session for one notice when its current turn ends. The returned sentence
// says what happened, for the person or the AI that asked.
func (s *Session) Send(to, text string, notifyWhenIdle bool) (string, error) {
	if err := checkText(text); err != nil {
		return "", err
	}
	target, err := s.Lookup(to)
	if err != nil {
		return "", err
	}
	me := s.Identity()
	rep, err := call(target.Endpoint, target.PID, Request{Type: TypeMessage, From: me, Text: text})
	if err != nil {
		return "", s.explainCallError(target, err)
	}
	if !rep.OK {
		return "", fmt.Errorf("session %s refused the message: %s", target.Name, rep.Error)
	}
	state := "idle"
	if target.Busy {
		state = "busy with a turn"
	}
	out := fmt.Sprintf("Delivered to session %s (folder %s), which is %s. Its AI sees the message at the start "+
		"of its next turn, marked as text from another program and not as its user's instruction.",
		target.Name, target.Folder, state)
	if !notifyWhenIdle {
		return out, nil
	}
	// Recorded BEFORE asking: an idle session answers at once, and its notice
	// can arrive before the reply to this request does.
	k := senderKey(target.Name, target.PID)
	s.mu.Lock()
	s.awaiting[k] = true
	s.mu.Unlock()
	rep, err = call(target.Endpoint, target.PID, Request{Type: TypeNotifyWhenIdle, From: me})
	if err != nil || !rep.OK {
		s.mu.Lock()
		delete(s.awaiting, k)
		s.mu.Unlock()
		why := rep.Error
		if err != nil {
			why = err.Error()
		}
		return out + " Asking it for an idle notice failed: " + why, nil
	}
	return out + " It will send one notice when its current turn ends, or at once if it is idle.", nil
}

func (s *Session) explainCallError(target Entry, err error) error {
	if errors.Is(err, errNotRunning) {
		if !pidAlive(target.PID) {
			s.reg.remove(target.key)
			return fmt.Errorf("session %s is no longer running; its entry has been removed", target.Name)
		}
		return fmt.Errorf("session %s is not answering: its process is running but its endpoint is not open", target.Name)
	}
	return fmt.Errorf("could not reach session %s: %v", target.Name, err)
}

// findEntry looks up the register entry for a sender, by name and process.
func (s *Session) findEntry(from Identity) (Entry, bool) {
	others, err := s.Peers()
	if err != nil {
		return Entry{}, false
	}
	for _, e := range others {
		if e.PID == from.PID && strings.EqualFold(e.Name, from.Name) {
			return e, true
		}
	}
	return Entry{}, false
}

func refuse(why string) Reply { return Reply{Error: why} }

// allowLocked applies the per-sender rate limit. Callers hold s.mu.
func (s *Session) allowLocked(k string, now time.Time) bool {
	cutoff := now.Add(-time.Minute)
	for key, ts := range s.rate {
		kept := ts[:0]
		for _, t := range ts {
			if t.After(cutoff) {
				kept = append(kept, t)
			}
		}
		if len(kept) == 0 {
			delete(s.rate, key)
		} else {
			s.rate[key] = kept
		}
	}
	if len(s.rate[k]) >= MaxPerMinute {
		return false
	}
	s.rate[k] = append(s.rate[k], now)
	return true
}

func (s *Session) pushRecentLocked(m Message) {
	s.recent = append(s.recent, m)
	if len(s.recent) > maxRecent {
		s.recent = append([]Message(nil), s.recent[len(s.recent)-maxRecent:]...)
	}
}

// handle answers one request. It only ever queues text, records a
// subscription, or refuses; it has no way to reach a permission, a setting or a
// tool, and that is the design rather than an omission.
func (s *Session) handle(req Request, peerPID int) Reply {
	if peerPID != 0 && req.From.PID != peerPID {
		return refuse("the sender's process number does not match the connection")
	}
	from := Identity{
		Name:   cleanName(req.From.Name),
		PID:    req.From.PID,
		Folder: cleanLine(req.From.Folder, maxFolderLen),
	}
	if from.Name == "" || from.PID <= 0 {
		return refuse("the request does not say which session sent it")
	}
	now := time.Now()

	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return refuse("this session is closing")
	}
	if !s.allowLocked(senderKey(from.Name, from.PID), now) {
		s.mu.Unlock()
		return refuse(fmt.Sprintf("too many requests: at most %d a minute from one session", MaxPerMinute))
	}
	switch req.Type {
	case TypeMessage:
		if err := checkText(req.Text); err != nil {
			s.mu.Unlock()
			return refuse(err.Error())
		}
		if len(s.inbox) >= maxInbox {
			s.mu.Unlock()
			return refuse("this session's inbox is full: its AI has not started a turn since the earlier messages arrived")
		}
		m := Message{From: from, At: now, Text: sanitizeText(req.Text), Kind: KindMessage}
		s.inbox = append(s.inbox, m)
		s.pushRecentLocked(m)
		fn := s.notify
		s.mu.Unlock()
		if fn != nil {
			go fn(m)
		}
		return Reply{OK: true}

	case TypeNotifyWhenIdle:
		s.mu.Unlock()
		// The notice goes to the endpoint in the register, never to an address
		// the request supplies.
		target, ok := s.findEntry(from)
		if !ok {
			return refuse("your session is not in the register, so there is nowhere to send the notice")
		}
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return refuse("this session is closing")
		}
		if !s.busy {
			me := s.identityLocked()
			s.mu.Unlock()
			s.sendIdleAsync(target, me)
			return Reply{OK: true}
		}
		s.idleSubs[senderKey(from.Name, from.PID)] = target
		s.mu.Unlock()
		return Reply{OK: true}

	case TypeIdleNotice:
		k := senderKey(from.Name, from.PID)
		if !s.awaiting[k] {
			s.mu.Unlock()
			return refuse("this session did not ask that session for an idle notice")
		}
		delete(s.awaiting, k)
		m := Message{From: from, At: now, Kind: KindIdle}
		if len(s.inbox) < maxInbox {
			s.inbox = append(s.inbox, m)
		}
		s.pushRecentLocked(m)
		fn := s.notify
		s.mu.Unlock()
		if fn != nil {
			go fn(m)
		}
		return Reply{OK: true}
	}
	s.mu.Unlock()
	return refuse(fmt.Sprintf("unknown request type %q", req.Type))
}

// TakeForAI returns and clears what has arrived since the AI's last turn.
func (s *Session) TakeForAI() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := s.inbox
	s.inbox = nil
	return out
}

// Requeue puts messages back at the front of the inbox, for a turn that took
// them and then failed before the AI saw them.
func (s *Session) Requeue(msgs []Message) {
	if len(msgs) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.inbox = append(append([]Message(nil), msgs...), s.inbox...)
	if len(s.inbox) > maxInbox {
		s.inbox = s.inbox[:maxInbox]
	}
}

// Pending is how many messages wait for the AI's next turn.
func (s *Session) Pending() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.inbox)
}

// Recent is what arrived lately, oldest first, for the /peers page.
func (s *Session) Recent() []Message {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]Message(nil), s.recent...)
}

// SetNotify sets what is called, on its own goroutine, when something arrives.
func (s *Session) SetNotify(fn func(Message)) {
	s.mu.Lock()
	s.notify = fn
	s.mu.Unlock()
}

// active is this process's session, or nil when messaging is not running.
var active atomic.Pointer[Session]

// SetActive records this process's session (nil to clear).
func SetActive(s *Session) { active.Store(s) }

// Active returns this process's session, or nil.
func Active() *Session { return active.Load() }
