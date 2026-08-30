// Package logging builds the API's JSON logger and carries per-request fields (request ID,
// user ID) through contexts so every log line written while handling a request can be tied
// back to it.
package logging

import (
	"context"
	"io"
	"log/slog"

	"github.com/google/uuid"
)

// New returns a JSON logger that adds request fields found in the context.
func New(w io.Writer, level slog.Level) *slog.Logger {
	return slog.New(NewHandler(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})))
}

// NewHandler wraps h so records logged with a request context carry request_id and user_id.
func NewHandler(h slog.Handler) slog.Handler {
	return contextHandler{h}
}

type contextHandler struct{ slog.Handler }

func (h contextHandler) Handle(ctx context.Context, record slog.Record) error {
	if f := fieldsFrom(ctx); f != nil {
		record.AddAttrs(slog.String("request_id", f.requestID))
		if f.userID != uuid.Nil {
			record.AddAttrs(slog.String("user_id", f.userID.String()))
		}
	}
	return h.Handler.Handle(ctx, record)
}

func (h contextHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return contextHandler{h.Handler.WithAttrs(attrs)}
}

func (h contextHandler) WithGroup(name string) slog.Handler {
	return contextHandler{h.Handler.WithGroup(name)}
}

// fields is shared by pointer so middleware deeper in the chain (authentication) can fill in
// the user for the access log line written by middleware further out.
type fields struct {
	requestID string
	userID    uuid.UUID
}

type contextKey struct{}

// WithRequest returns a context carrying the request ID.
func WithRequest(ctx context.Context, requestID string) context.Context {
	return context.WithValue(ctx, contextKey{}, &fields{requestID: requestID})
}

// SetUserID records the authenticated user for the request, if the context has request fields.
func SetUserID(ctx context.Context, userID uuid.UUID) {
	if f := fieldsFrom(ctx); f != nil {
		f.userID = userID
	}
}

// RequestID returns the request ID from the context, or "".
func RequestID(ctx context.Context) string {
	if f := fieldsFrom(ctx); f != nil {
		return f.requestID
	}
	return ""
}

// UserID returns the authenticated user recorded for the request, or uuid.Nil.
func UserID(ctx context.Context) uuid.UUID {
	if f := fieldsFrom(ctx); f != nil {
		return f.userID
	}
	return uuid.Nil
}

func fieldsFrom(ctx context.Context) *fields {
	f, _ := ctx.Value(contextKey{}).(*fields)
	return f
}
