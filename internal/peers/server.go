package peers

// GORILLA (2026-10-10): the wire. One JSON object per line, one request per
// connection:
//
//	-> {"type":"message","from":{"name":"proj","pid":1234,"folder":"C:\\work\\proj"},"text":"..."}
//	<- {"ok":true}            or   {"ok":false,"error":"..."}
//
// Every read and write runs against a deadline, so a peer that connects and
// says nothing holds a goroutine for seconds, not for ever. After the reply the
// server waits for the client to close its end (or for the deadline): closing
// first could discard a reply the client has not read yet.

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"
)

const (
	maxRequestBytes = 128 * 1024 // a 16 KB text JSON-escaped at worst is ~96 KB
	maxReplyBytes   = 4 * 1024
	ioTimeout       = 5 * time.Second
	dialTimeout     = 3 * time.Second
)

var (
	errClosed     = errors.New("the endpoint is closed")
	errTimeout    = errors.New("timed out")
	errNotRunning = errors.New("no session is listening there")
)

// conn is one connection, on either platform.
type conn interface {
	io.ReadWriteCloser
	SetDeadline(t time.Time) error
	// PeerPID is the process on the other end, or 0 where the platform cannot
	// say (Unix sockets here; the 0700 folder is the gate there).
	PeerPID() int
}

// listener accepts connections on one endpoint.
type listener interface {
	Accept() (conn, error)
	// Close stops Accept. free releases what Close leaves for Accept to see;
	// it is called once Accept can no longer be running.
	Close() error
	free()
}

// Server answers requests on one endpoint.
type Server struct {
	endpoint string
	ln       listener
	handle   func(req Request, peerPID int) Reply
	closed   atomic.Bool
	wg       sync.WaitGroup
	once     sync.Once
}

// Listen opens endpoint and serves it until Close.
func Listen(endpoint string, handle func(req Request, peerPID int) Reply) (*Server, error) {
	ln, err := listen(endpoint)
	if err != nil {
		return nil, err
	}
	s := &Server{endpoint: endpoint, ln: ln, handle: handle}
	s.wg.Add(1)
	go s.serve()
	return s, nil
}

// Endpoint is the pipe name or socket path being served.
func (s *Server) Endpoint() string { return s.endpoint }

// Close stops the server and waits for connections in progress to finish.
func (s *Server) Close() {
	s.once.Do(func() {
		s.closed.Store(true)
		_ = s.ln.Close()
		s.wg.Wait()
		s.ln.free()
	})
}

func (s *Server) serve() {
	defer s.wg.Done()
	for {
		c, err := s.ln.Accept()
		if err != nil {
			if s.closed.Load() || errors.Is(err, errClosed) || errors.Is(err, net.ErrClosed) {
				return
			}
			// One failed instance must not end the server; the pause stops a
			// persistent fault from spinning a core.
			time.Sleep(50 * time.Millisecond)
			continue
		}
		s.wg.Add(1)
		go func() {
			defer s.wg.Done()
			s.serveConn(c)
		}()
	}
}

func (s *Server) serveConn(c conn) {
	defer c.Close()
	_ = c.SetDeadline(time.Now().Add(ioTimeout))
	var reply Reply
	line, err := readLine(c, maxRequestBytes)
	if err != nil {
		reply = Reply{Error: "could not read the request: " + err.Error()}
	} else {
		var req Request
		dec := json.NewDecoder(bytes.NewReader(line))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&req); err != nil {
			reply = Reply{Error: "the request is not valid JSON for this protocol"}
		} else {
			reply = s.handle(req, c.PeerPID())
		}
	}
	if err := writeJSONLine(c, reply); err != nil {
		return
	}
	// Wait for the client to close its end, or for the deadline.
	var sink [64]byte
	for {
		if _, err := c.Read(sink[:]); err != nil {
			return
		}
	}
}

// readLine reads one newline-terminated line of at most max bytes.
func readLine(r io.Reader, max int) ([]byte, error) {
	br := bufio.NewReaderSize(io.LimitReader(r, int64(max)+1), 4096)
	line, err := br.ReadBytes('\n')
	if len(line) > max {
		return nil, fmt.Errorf("it is larger than %d bytes", max)
	}
	if err != nil {
		if errors.Is(err, io.EOF) && len(bytes.TrimSpace(line)) > 0 {
			return line, nil
		}
		return nil, err
	}
	return line, nil
}

// writeJSONLine writes v as one line.
func writeJSONLine(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	data = append(data, '\n')
	_, err = w.Write(data)
	return err
}

// call sends one request to endpoint and returns the reply. wantPID is the
// process the register says owns that endpoint; where the platform can name
// the process on the other end and it is a different one, the call is refused
// before anything is sent.
func call(endpoint string, wantPID int, req Request) (Reply, error) {
	c, err := dial(endpoint, time.Now().Add(dialTimeout))
	if err != nil {
		return Reply{}, err
	}
	defer c.Close()
	if got := c.PeerPID(); got != 0 && wantPID != 0 && got != wantPID {
		return Reply{}, fmt.Errorf("refused: the program answering there is process %d, not the session that registered it (process %d)", got, wantPID)
	}
	_ = c.SetDeadline(time.Now().Add(ioTimeout))
	if err := writeJSONLine(c, req); err != nil {
		return Reply{}, err
	}
	line, err := readLine(c, maxReplyBytes)
	if err != nil {
		return Reply{}, fmt.Errorf("no answer: %w", err)
	}
	var rep Reply
	if err := json.Unmarshal(line, &rep); err != nil {
		return Reply{}, errors.New("the other session's answer was not valid JSON")
	}
	return rep, nil
}
