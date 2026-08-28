// Package apitest runs the real API router against a throwaway database for integration tests.
package apitest

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/app"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/email"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/testutil"
	"go.uber.org/zap"
)

// TestPassword is the password given to every user created through the harness.
const TestPassword = "Integration-Test-Passw0rd!"

// Server is a running API backed by its own database.
type Server struct {
	*httptest.Server
	Pool  *pgxpool.Pool
	Email *EmailRecorder
	t     *testing.T
}

// User is an account created through the API with a valid access token.
type User struct {
	ID    uuid.UUID
	Email string
	Name  string
	OrgID uuid.UUID
	Token string
}

// Options adjust the test server.
type Options struct {
	// RateLimits keeps the production rate limits enabled.
	RateLimits bool
}

// New starts the API on a fresh, migrated database.
func New(t *testing.T) *Server {
	return NewWithOptions(t, Options{})
}

// NewWithOptions starts the API with options.
func NewWithOptions(t *testing.T, opts Options) *Server {
	t.Helper()
	pool := testutil.NewDatabase(t)

	kek, _ := hex.DecodeString("7f3a9c1e5b2d8f406a1c3e5b7d9f02468ace13579bdf02468ace13579bdf0246")
	recorder := &EmailRecorder{}
	handler, err := app.New(pool, app.Config{
		MasterKEK: kek,
		JWTSecret: "integration-test-jwt-secret-5f8e2a9c4b7d1e3f",
		Email:     recorder,
		Logger:    zap.NewNop(),
		SLogger:   slog.New(slog.NewTextHandler(io.Discard, nil)),
		// Tests make many requests from one address; TestRateLimits builds its own server.
		DisableRateLimits: !opts.RateLimits,
	})
	if err != nil {
		t.Fatal(err)
	}

	// Every request made through the harness is checked against docs/openapi.yaml.
	srv := httptest.NewServer(contractMiddleware(t, handler))
	t.Cleanup(srv.Close)
	return &Server{Server: srv, Pool: pool, Email: recorder, t: t}
}

// Response is a completed request with its body read.
type Response struct {
	Status int
	Header http.Header
	Body   []byte
}

// Decode unmarshals the response body into out, failing the test on error.
func (r Response) Decode(t *testing.T, out any) {
	t.Helper()
	if err := json.Unmarshal(r.Body, out); err != nil {
		t.Fatalf("decode response (%d %s): %v", r.Status, r.Body, err)
	}
}

// Do sends a request with an optional bearer token and JSON body.
func (s *Server) Do(method, path, token string, body any) Response {
	s.t.Helper()
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			s.t.Fatal(err)
		}
		reader = bytes.NewReader(payload)
	}
	req, err := http.NewRequest(method, s.URL+path, reader)
	if err != nil {
		s.t.Fatal(err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := s.Client().Do(req)
	if err != nil {
		s.t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		s.t.Fatal(err)
	}
	return Response{Status: resp.StatusCode, Header: resp.Header, Body: data}
}

// MustDo is Do that fails the test unless the response has the wanted status.
func (s *Server) MustDo(want int, method, path, token string, body any) Response {
	s.t.Helper()
	resp := s.Do(method, path, token, body)
	if resp.Status != want {
		s.t.Fatalf("%s %s: want status %d, got %d: %s", method, path, want, resp.Status, resp.Body)
	}
	return resp
}

// CreateUser registers a user through the API, verifies them with the emailed token and logs in.
// Registration also creates the user's own organization, where they are the owner.
func (s *Server) CreateUser(name string) User {
	s.t.Helper()
	email := strings.ToLower(strings.ReplaceAll(name, " ", ".")) + "+" + uuid.NewString()[:8] + "@example.test"

	var registered struct {
		UserID uuid.UUID `json:"user_id"`
	}
	s.MustDo(http.StatusCreated, http.MethodPost, "/auth/register", "", map[string]string{
		"email": email, "password": TestPassword, "name": name,
	}).Decode(s.t, &registered)

	token := s.Email.WaitForToken(s.t, email)
	s.MustDo(http.StatusOK, http.MethodPost, "/auth/verify-email", "", map[string]string{"token": token})

	user := User{ID: registered.UserID, Email: email, Name: name}
	user.Token = s.Login(email, TestPassword)

	if err := s.Pool.QueryRow(context.Background(),
		`SELECT organization_id FROM user_organizations WHERE user_id = $1`, user.ID).Scan(&user.OrgID); err != nil {
		s.t.Fatalf("find organization for %s: %v", email, err)
	}
	return user
}

// Login returns an access token for the credentials.
func (s *Server) Login(email, password string) string {
	s.t.Helper()
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	s.MustDo(http.StatusOK, http.MethodPost, "/auth/login", "", map[string]string{
		"email": email, "password": password,
	}).Decode(s.t, &tokens)
	return tokens.AccessToken
}

// AddToOrg makes the user a member of the organization with the given role.
func (s *Server) AddToOrg(user User, orgID uuid.UUID, role string) {
	s.t.Helper()
	if _, err := s.Pool.Exec(context.Background(),
		`INSERT INTO user_organizations (user_id, organization_id, role) VALUES ($1, $2, $3)
		 ON CONFLICT (user_id, organization_id) DO UPDATE SET role = EXCLUDED.role`,
		user.ID, orgID, role); err != nil {
		s.t.Fatalf("add %s to organization: %v", user.Email, err)
	}
}

// EmailRecorder is an email service that keeps sent tokens in memory.
type EmailRecorder struct {
	mu     sync.Mutex
	tokens map[string][]string
}

func (r *EmailRecorder) record(to, token string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.tokens == nil {
		r.tokens = map[string][]string{}
	}
	key := strings.ToLower(to)
	r.tokens[key] = append(r.tokens[key], token)
}

// SendVerificationEmail records the verification token.
func (r *EmailRecorder) SendVerificationEmail(_ context.Context, to, _, token string) error {
	r.record(to, token)
	return nil
}

// SendInviteEmail records the invite token.
func (r *EmailRecorder) SendInviteEmail(_ context.Context, invite email.Invite) error {
	r.record(invite.To, invite.Token)
	return nil
}

// WaitForToken returns the newest token emailed to the address, waiting for asynchronous sends.
func (r *EmailRecorder) WaitForToken(t *testing.T, to string) string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		r.mu.Lock()
		list := r.tokens[strings.ToLower(to)]
		r.mu.Unlock()
		if len(list) > 0 {
			return list[len(list)-1]
		}
		if time.Now().After(deadline) {
			t.Fatalf("no email sent to %s", to)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
