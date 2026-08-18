package session

import (
	"context"
	"database/sql"

	"github.com/google/uuid"
	"github.com/opencode-ai/opencode/internal/config"
	"github.com/opencode-ai/opencode/internal/db"
	"github.com/opencode-ai/opencode/internal/pubsub"
)

type Session struct {
	ID               string
	ParentSessionID  string
	Title            string
	MessageCount     int64
	PromptTokens     int64
	CompletionTokens int64
	SummaryMessageID string
	Cost             float64
	CreatedAt        int64
	UpdatedAt        int64
	// StartedIn is the primary working directory the session was created in.
	// Empty means unknown — a row written before this column existed.
	StartedIn string

	// TotalPromptTokens and TotalCompletionTokens include every helper session
	// spawned from this conversation. They are COMPUTED on read and never
	// persisted.
	//
	// GORILLA OVERRIDE (2026-08-17): helper spend was invisible — a run that
	// burned 507,935 tokens across 17 sessions showed as 44,688 in the status
	// bar. The first fix added those tokens into PromptTokens and was wiped
	// seconds later, every turn, because the agent loop ASSIGNS that field
	// (`sess.PromptTokens = usage.InputTokens`) rather than adding to it.
	// Observed: 522,261 followed by 44,688 on the next turn.
	//
	// A field nothing writes cannot be overwritten. These are filled by the
	// service on Get and before publishing an update, so the UI reads them
	// without knowing anything about helpers.
	TotalPromptTokens     int64
	TotalCompletionTokens int64
}

// TotalTokens is what the run actually cost, helpers included. Falls back to
// the session's own counters when the totals were not computed (a Session built
// in a test, or a row read through a path that does not aggregate).
func (s Session) TotalTokens() int64 {
	if s.TotalPromptTokens > 0 || s.TotalCompletionTokens > 0 {
		return s.TotalPromptTokens + s.TotalCompletionTokens
	}
	return s.PromptTokens + s.CompletionTokens
}

type Service interface {
	pubsub.Suscriber[Session]
	Create(ctx context.Context, title string) (Session, error)
	CreateTitleSession(ctx context.Context, parentSessionID string) (Session, error)
	CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (Session, error)
	Get(ctx context.Context, id string) (Session, error)
	List(ctx context.Context) ([]Session, error)
	// ListByDir returns sessions started in dir, plus any whose origin is
	// unknown. See the comment on ListByDir's implementation for why unknown
	// rows are included rather than hidden.
	ListByDir(ctx context.Context, dir string) ([]Session, error)
	Save(ctx context.Context, session Session) (Session, error)
	// ListResearchHelpers returns the helper sessions a research run spawned.
	// List deliberately hides them (they are not conversations); recovery is
	// the one caller that needs to see them.
	ListResearchHelpers(ctx context.Context) ([]Session, error)
	Delete(ctx context.Context, id string) error
}

type service struct {
	*pubsub.Broker[Session]
	q db.Querier
}

func (s *service) Create(ctx context.Context, title string) (Session, error) {
	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:        uuid.New().String(),
		Title:     title,
		StartedIn: config.WorkingDirectory(),
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	return session, nil
}

func (s *service) CreateTaskSession(ctx context.Context, toolCallID, parentSessionID, title string) (Session, error) {
	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:              toolCallID,
		ParentSessionID: sql.NullString{String: parentSessionID, Valid: true},
		Title:           title,
		StartedIn:       config.WorkingDirectory(),
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	return session, nil
}

func (s *service) CreateTitleSession(ctx context.Context, parentSessionID string) (Session, error) {
	dbSession, err := s.q.CreateSession(ctx, db.CreateSessionParams{
		ID:              "title-" + parentSessionID,
		ParentSessionID: sql.NullString{String: parentSessionID, Valid: true},
		Title:           "Generate a title",
		StartedIn:       config.WorkingDirectory(),
	})
	if err != nil {
		return Session{}, err
	}
	session := s.fromDBItem(dbSession)
	s.Publish(pubsub.CreatedEvent, session)
	return session, nil
}

func (s *service) Delete(ctx context.Context, id string) error {
	session, err := s.Get(ctx, id)
	if err != nil {
		return err
	}
	err = s.q.DeleteSession(ctx, session.ID)
	if err != nil {
		return err
	}
	s.Publish(pubsub.DeletedEvent, session)
	return nil
}

func (s *service) Get(ctx context.Context, id string) (Session, error) {
	dbSession, err := s.q.GetSessionByID(ctx, id)
	if err != nil {
		return Session{}, err
	}
	return s.withHelperTotals(ctx, s.fromDBItem(dbSession)), nil
}

// helperTotaler is satisfied by the generated *db.Queries via the hand-written
// SumSessionTokens. Type-asserted rather than added to db.Querier so the
// generated interface stays untouched by this feature.
type helperTotaler interface {
	SumSessionTokens(ctx context.Context, sessionID string) (int64, int64, error)
}

// withHelperTotals fills in what the conversation cost including its helpers.
// A failure here must never break the caller: the totals are a display
// improvement, and the session itself is still valid without them.
func (s *service) withHelperTotals(ctx context.Context, sess Session) Session {
	t, ok := s.q.(helperTotaler)
	if !ok || sess.ID == "" {
		return sess
	}
	prompt, completion, err := t.SumSessionTokens(ctx, sess.ID)
	if err != nil {
		return sess
	}
	sess.TotalPromptTokens, sess.TotalCompletionTokens = prompt, completion
	return sess
}

// researchHelperLister is satisfied by the generated *db.Queries via the
// hand-written ListResearchHelpers. Type-asserted rather than added to
// db.Querier so the generated interface stays untouched by this feature.
type researchHelperLister interface {
	ListResearchHelpers(ctx context.Context) ([]db.ResearchHelper, error)
}

// ListResearchHelpers returns every research helper session, newest first.
//
// Only the fields recovery reads are filled: the id (which encodes the run and
// the lane), the title, what the lane cost, and when it started. A store that
// cannot answer returns nothing rather than an error — a listing that fails
// tells someone their two hours are gone, which would not even be true.
func (s *service) ListResearchHelpers(ctx context.Context) ([]Session, error) {
	l, ok := s.q.(researchHelperLister)
	if !ok {
		return nil, nil
	}
	rows, err := l.ListResearchHelpers(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Session, 0, len(rows))
	for _, r := range rows {
		out = append(out, Session{
			ID:               r.ID,
			Title:            r.Title,
			PromptTokens:     r.PromptTokens,
			CompletionTokens: r.CompletionTokens,
			CreatedAt:        r.CreatedAt,
		})
	}
	return out, nil
}

func (s *service) Save(ctx context.Context, session Session) (Session, error) {
	dbSession, err := s.q.UpdateSession(ctx, db.UpdateSessionParams{
		ID:               session.ID,
		Title:            session.Title,
		PromptTokens:     session.PromptTokens,
		CompletionTokens: session.CompletionTokens,
		SummaryMessageID: sql.NullString{
			String: session.SummaryMessageID,
			Valid:  session.SummaryMessageID != "",
		},
		Cost: session.Cost,
	})
	if err != nil {
		return Session{}, err
	}
	session = s.fromDBItem(dbSession)
	// Publish WITH the helper totals, so every UI subscribed to session
	// updates sees the true cost of a run without asking for it. The saved row
	// is unchanged: the totals are computed, never written.
	session = s.withHelperTotals(ctx, session)
	s.Publish(pubsub.UpdatedEvent, session)
	return session, nil
}

func (s *service) List(ctx context.Context) ([]Session, error) {
	dbSessions, err := s.q.ListSessions(ctx)
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, len(dbSessions))
	for i, dbSession := range dbSessions {
		sessions[i] = s.fromDBItem(dbSession)
	}
	return sessions, nil
}

// ListByDir returns the sessions started in dir.
//
// GORILLA OVERRIDE: rows with an empty started_in are returned for EVERY dir,
// not hidden. Those are sessions written before the column existed. A session
// the user cannot find is indistinguishable from one that was deleted, and this
// project has already paid once for storage changes that made history vanish
// with no explanation (v0.1.85). Showing a few extra rows is the cheap failure.
func (s *service) ListByDir(ctx context.Context, dir string) ([]Session, error) {
	dbSessions, err := s.q.ListSessionsByDir(ctx, dir)
	if err != nil {
		return nil, err
	}
	sessions := make([]Session, len(dbSessions))
	for i, dbSession := range dbSessions {
		sessions[i] = s.fromDBItem(dbSession)
	}
	return sessions, nil
}

func (s service) fromDBItem(item db.Session) Session {
	return Session{
		ID:               item.ID,
		ParentSessionID:  item.ParentSessionID.String,
		Title:            item.Title,
		MessageCount:     item.MessageCount,
		PromptTokens:     item.PromptTokens,
		CompletionTokens: item.CompletionTokens,
		SummaryMessageID: item.SummaryMessageID.String,
		Cost:             item.Cost,
		CreatedAt:        item.CreatedAt,
		UpdatedAt:        item.UpdatedAt,
		StartedIn:        item.StartedIn,
	}
}

func NewService(q db.Querier) Service {
	broker := pubsub.NewBroker[Session]()
	return &service{
		broker,
		q,
	}
}
