// Command seed creates demo users, vaults, environments and secrets through the HTTP API,
// so it works against any deployment. It is idempotent: existing data is left alone.
//
// New accounts must verify their email before they can log in. Against a local API running
// with APP_ENV=development and no SMTP, pass --log with the API's log file and the seed reads
// the verification links from it.
//
// Against a real deployment neither works, and registering through the API would send
// verification and invitation mail to addresses that do not exist; the bounces would damage the
// sending domain's reputation. Pass --database-url there instead: the demo accounts are written
// straight to the database, already verified and already in the organization, so no mail is
// sent at all. Everything else still goes through the API.
package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

type demoUser struct {
	Email   string
	Name    string
	OrgRole string
}

type demoSecret struct {
	Key          string
	Value        string
	Description  string
	RotationDays int // 0 = no rotation schedule
	ExpiresIn    time.Duration
}

type demoVault struct {
	Name         string
	Description  string
	Environments map[string][]demoSecret
	envOrder     []string
	// Members maps teammate email to vault role; the owner is added automatically.
	Members map[string]string
}

// owner creates and owns every demo vault; the others are teammates invited to the owner's
// organization with an organization role.
var (
	owner     = demoUser{Email: "salaar@demo.dev", Name: "Salaar Butt"}
	teammates = []demoUser{
		{Email: "priya@demo.dev", Name: "Priya Raman", OrgRole: "admin"},
		{Email: "marcus@demo.dev", Name: "Marcus Chen", OrgRole: "developer"},
		{Email: "ops-bot@demo.dev", Name: "Ops Bot", OrgRole: "viewer"},
	}
)

func demoVaults() []demoVault {
	return []demoVault{
		{
			Name:        "payments-api",
			Description: "Stripe, database, and queue credentials for the payments service",
			envOrder:    []string{"development", "staging", "production"},
			Members:     map[string]string{"priya@demo.dev": "admin", "marcus@demo.dev": "developer", "ops-bot@demo.dev": "viewer"},
			Environments: map[string][]demoSecret{
				"development": {
					{Key: "DATABASE_URL", Value: "postgres://payments:demo-dev-password@localhost:5432/payments", Description: "Local Postgres"},
				},
				"staging": {
					{Key: "DATABASE_URL", Value: "postgres://payments:demo-staging-password@staging-db.internal:5432/payments", Description: "Staging Postgres"},
					{Key: "STRIPE_SECRET_KEY", Value: "sk_test_demo_51HxStagingKeyNotReal", Description: "Stripe test key"},
				},
				"production": {
					{Key: "DATABASE_URL", Value: "postgres://payments:demo-prod-password@prod-db.internal:5432/payments", Description: "Primary Postgres", RotationDays: 90},
					{Key: "STRIPE_SECRET_KEY", Value: "sk_live_demo_51HxProductionKeyNotReal", Description: "Stripe API key", RotationDays: 90},
					{Key: "REDIS_URL", Value: "redis://:demo-redis-password@cache.internal:6379/0", Description: "Session cache"},
					{Key: "JWT_SIGNING_KEY", Value: "demo-signing-key-9f2c1e7a4b8d", Description: "Access token signing", RotationDays: 30},
					{Key: "SENTRY_DSN", Value: "https://demo-public-key@o0.ingest.sentry.io/0", Description: "Error reporting"},
				},
			},
		},
		{
			Name:        "web-frontend",
			Description: "Public build-time keys and analytics tokens",
			envOrder:    []string{"staging", "production"},
			Environments: map[string][]demoSecret{
				"staging": {
					{Key: "NEXT_PUBLIC_API_URL", Value: "https://api.staging.demo.dev", Description: "API origin"},
				},
				"production": {
					{Key: "NEXT_PUBLIC_API_URL", Value: "https://api.demo.dev", Description: "API origin"},
					{Key: "ANALYTICS_WRITE_KEY", Value: "demo-analytics-write-key-3c9e", Description: "Analytics ingest"},
				},
			},
		},
		{
			Name:        "infra-terraform",
			Description: "State backend and provider credentials for infrastructure",
			envOrder:    []string{"production"},
			Members:     map[string]string{"marcus@demo.dev": "oncall"},
			Environments: map[string][]demoSecret{
				"production": {
					{Key: "TF_STATE_BUCKET", Value: "demo-terraform-state", Description: "Remote state"},
					{Key: "CLOUDFLARE_API_TOKEN", Value: "demo-cloudflare-token-7d1a", Description: "DNS automation", ExpiresIn: 20 * 24 * time.Hour},
				},
			},
		},
		{
			Name:        "ml-inference",
			Description: "Model registry and GPU cluster credentials",
			envOrder:    []string{"production"},
			Environments: map[string][]demoSecret{
				"production": {
					{Key: "HF_TOKEN", Value: "hf_demoTokenNotReal1234", Description: "Model registry pull", ExpiresIn: 5 * 24 * time.Hour, RotationDays: 60},
					{Key: "AWS_ACCESS_KEY_ID", Value: "AKIADEMONOTREAL0000", Description: "Artifact bucket access"},
					{Key: "S3_BUCKET", Value: "demo-model-checkpoints", Description: "Checkpoint storage"},
				},
			},
		},
	}
}

func main() {
	apiURL := flag.String("api-url", envOr("SEED_API_URL", "http://localhost:8080"), "API base URL")
	logPath := flag.String("log", os.Getenv("SEED_API_LOG"), "API log file to read development verification links from")
	dbURL := flag.String("database-url", os.Getenv("SEED_DATABASE_URL"), "create the demo accounts directly in this database, verified and in the organization, instead of registering them through the API (sends no email)")
	password := flag.String("password", envOr("SEED_PASSWORD", "Demo-Passw0rd!2026"), "password for every demo account")
	flag.Parse()

	s := &seeder{
		api:      strings.TrimRight(*apiURL, "/"),
		logPath:  *logPath,
		password: *password,
		http:     &http.Client{Timeout: 15 * time.Second},
	}

	if *dbURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, *dbURL)
		if err != nil {
			fmt.Fprintf(os.Stderr, "seed: connect to the database: %v\n", err)
			os.Exit(1)
		}
		defer pool.Close()
		s.db = pool
	}

	if err := s.run(); err != nil {
		fmt.Fprintf(os.Stderr, "seed: %v\n", err)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

type seeder struct {
	api      string
	logPath  string
	password string
	http     *http.Client
	// db, when set, provisions the demo accounts directly instead of registering them.
	db *pgxpool.Pool
}

func (s *seeder) run() error {
	if _, err := s.call(http.MethodGet, "/health", "", nil, nil); err != nil {
		return fmt.Errorf("API not reachable at %s: %w", s.api, err)
	}

	if s.db != nil {
		if err := s.provisionAccounts(context.Background()); err != nil {
			return err
		}
	}

	ownerToken, err := s.ensureUser(owner)
	if err != nil {
		return err
	}
	teammateTokens := map[string]string{}
	for _, u := range teammates {
		token, err := s.ensureUser(u)
		if err != nil {
			return err
		}
		teammateTokens[u.Email] = token
	}
	// With --database-url the teammates are already in the organization, and inviting them
	// would email addresses that do not exist.
	if s.db == nil {
		if err := s.ensureTeam(ownerToken, teammateTokens); err != nil {
			return err
		}
	}

	var vaults []struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if _, err := s.call(http.MethodGet, "/vaults", ownerToken, nil, &vaults); err != nil {
		return fmt.Errorf("list vaults: %w", err)
	}
	vaultIDs := map[string]string{}
	for _, v := range vaults {
		vaultIDs[v.Name] = v.ID
	}

	created := 0
	for _, dv := range demoVaults() {
		vaultID, ok := vaultIDs[dv.Name]
		if !ok {
			var v struct {
				ID string `json:"id"`
			}
			body := map[string]any{"name": dv.Name, "description": dv.Description}
			if _, err := s.call(http.MethodPost, "/vaults", ownerToken, body, &v); err != nil {
				return fmt.Errorf("create vault %s: %w", dv.Name, err)
			}
			vaultID = v.ID
			fmt.Printf("created vault %s\n", dv.Name)
		}

		if err := s.ensureVaultMembers(ownerToken, vaultID, dv.Members); err != nil {
			return err
		}

		var envs []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}
		if _, err := s.call(http.MethodGet, "/vaults/"+vaultID+"/envs", ownerToken, nil, &envs); err != nil {
			return fmt.Errorf("list environments of %s: %w", dv.Name, err)
		}
		envIDs := map[string]string{}
		for _, e := range envs {
			envIDs[e.Name] = e.ID
		}

		for _, envName := range dv.envOrder {
			envID, ok := envIDs[envName]
			if !ok {
				var e struct {
					ID string `json:"id"`
				}
				if _, err := s.call(http.MethodPost, "/vaults/"+vaultID+"/envs", ownerToken, map[string]any{"name": envName}, &e); err != nil {
					return fmt.Errorf("create environment %s/%s: %w", dv.Name, envName, err)
				}
				envID = e.ID
			}

			var secrets []struct {
				KeyName string `json:"key_name"`
			}
			if _, err := s.call(http.MethodGet, "/envs/"+envID+"/secrets", ownerToken, nil, &secrets); err != nil {
				return fmt.Errorf("list secrets of %s/%s: %w", dv.Name, envName, err)
			}
			existing := map[string]bool{}
			for _, sec := range secrets {
				existing[sec.KeyName] = true
			}

			for _, sec := range dv.Environments[envName] {
				if existing[sec.Key] {
					continue
				}
				body := map[string]any{"key_name": sec.Key, "value": sec.Value, "description": sec.Description}
				if sec.RotationDays > 0 {
					body["rotation_interval_days"] = sec.RotationDays
				}
				if sec.ExpiresIn > 0 {
					body["expires_at"] = time.Now().Add(sec.ExpiresIn).UTC().Format(time.RFC3339)
				}
				if _, err := s.call(http.MethodPost, "/envs/"+envID+"/secrets", ownerToken, body, nil); err != nil {
					return fmt.Errorf("create secret %s/%s/%s: %w", dv.Name, envName, sec.Key, err)
				}
				created++
			}
		}
	}

	fmt.Printf("seed complete: %d new secrets; log in as %s with the seed password\n", created, owner.Email)
	return nil
}

// ensureTeam invites each teammate into the owner's organization and accepts the invitation as
// them, using the development invite link from the API log. Existing members are left alone.
func (s *seeder) ensureTeam(ownerToken string, teammateTokens map[string]string) error {
	var orgs []struct {
		ID   string `json:"id"`
		Role string `json:"role"`
	}
	if _, err := s.call(http.MethodGet, "/orgs", ownerToken, nil, &orgs); err != nil {
		return fmt.Errorf("list organizations: %w", err)
	}
	var orgID string
	for _, o := range orgs {
		if o.Role == "owner" {
			orgID = o.ID
			break
		}
	}
	if orgID == "" {
		return fmt.Errorf("%s does not own an organization", owner.Email)
	}

	var members []struct {
		Email string `json:"email"`
	}
	if _, err := s.call(http.MethodGet, "/orgs/"+orgID+"/members", ownerToken, nil, &members); err != nil {
		return fmt.Errorf("list organization members: %w", err)
	}
	inOrg := map[string]bool{}
	for _, m := range members {
		inOrg[m.Email] = true
	}

	for _, u := range teammates {
		if inOrg[u.Email] {
			continue
		}
		if _, err := s.call(http.MethodPost, "/orgs/"+orgID+"/invites", ownerToken, map[string]any{"email": u.Email, "role": u.OrgRole}, nil); err != nil {
			return fmt.Errorf("invite %s: %w", u.Email, err)
		}
		token, err := s.findLinkToken(u.Email, "/invite")
		if err != nil {
			return fmt.Errorf("%s was invited but no development invite link was found (%v); accept the emailed invitation and run the seed again", u.Email, err)
		}
		if _, err := s.call(http.MethodPost, "/invites/accept", teammateTokens[u.Email], map[string]any{"token": token}, nil); err != nil {
			return fmt.Errorf("accept invitation for %s: %w", u.Email, err)
		}
		fmt.Printf("%s joined the organization as %s\n", u.Email, u.OrgRole)
	}
	return nil
}

// provisionAccounts writes the demo accounts straight to the database: verified, sharing one
// organization, with the same password as the API-driven path. It replaces registration and
// invitation, both of which send mail, and is idempotent so a nightly reset can call it.
func (s *seeder) provisionAccounts(ctx context.Context) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(s.password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash the demo password: %w", err)
	}

	ownerID, err := s.upsertUser(ctx, owner, string(hash))
	if err != nil {
		return err
	}
	orgID, err := s.ensureOwnedOrg(ctx, ownerID, owner.Name+"'s Organization")
	if err != nil {
		return err
	}
	fmt.Printf("provisioned %s as owner\n", owner.Email)

	for _, u := range teammates {
		id, err := s.upsertUser(ctx, u, string(hash))
		if err != nil {
			return err
		}
		if _, err := s.db.Exec(ctx, `
			INSERT INTO user_organizations (user_id, organization_id, role, created_at)
			VALUES ($1, $2, $3, now())
			ON CONFLICT (user_id, organization_id) DO UPDATE SET role = EXCLUDED.role
		`, id, orgID, u.OrgRole); err != nil {
			return fmt.Errorf("add %s to the organization: %w", u.Email, err)
		}
		fmt.Printf("provisioned %s as %s\n", u.Email, u.OrgRole)
	}
	return nil
}

// upsertUser creates the account if it is missing and makes sure it is verified and uses the
// seed password, so a half-finished run or a changed password does not leave it unusable.
func (s *seeder) upsertUser(ctx context.Context, u demoUser, passwordHash string) (string, error) {
	var id string
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (id, email, password_hash, name, email_verified, created_at, updated_at)
		VALUES ($1, lower($2), $3, $4, true, now(), now())
		ON CONFLICT (lower(email)) DO UPDATE
			SET password_hash = EXCLUDED.password_hash, email_verified = true, updated_at = now()
		RETURNING id
	`, uuid.New(), u.Email, passwordHash, u.Name).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("create %s: %w", u.Email, err)
	}
	return id, nil
}

// ensureOwnedOrg returns the organization the user already owns, or creates one, matching what
// registration does for a new account.
func (s *seeder) ensureOwnedOrg(ctx context.Context, userID, name string) (string, error) {
	var orgID string
	err := s.db.QueryRow(ctx, `
		SELECT organization_id FROM user_organizations WHERE user_id = $1 AND role = 'owner' LIMIT 1
	`, userID).Scan(&orgID)
	if err == nil {
		return orgID, nil
	}

	orgID = uuid.New().String()
	if _, err := s.db.Exec(ctx, `
		INSERT INTO organizations (id, name, created_at, updated_at) VALUES ($1, $2, now(), now())
	`, orgID, name); err != nil {
		return "", fmt.Errorf("create the demo organization: %w", err)
	}
	if _, err := s.db.Exec(ctx, `
		INSERT INTO user_organizations (user_id, organization_id, role, created_at)
		VALUES ($1, $2, 'owner', now())
	`, userID, orgID); err != nil {
		return "", fmt.Errorf("make %s the owner: %w", name, err)
	}
	return orgID, nil
}

// ensureVaultMembers adds teammates to a vault with the given roles.
func (s *seeder) ensureVaultMembers(ownerToken, vaultID string, roles map[string]string) error {
	var members []struct {
		Email string `json:"email"`
	}
	if _, err := s.call(http.MethodGet, "/vaults/"+vaultID+"/members", ownerToken, nil, &members); err != nil {
		return fmt.Errorf("list vault members: %w", err)
	}
	existing := map[string]bool{}
	for _, m := range members {
		existing[m.Email] = true
	}
	for email, role := range roles {
		if existing[email] {
			continue
		}
		if _, err := s.call(http.MethodPost, "/vaults/"+vaultID+"/members", ownerToken, map[string]any{"email": email, "role": role}, nil); err != nil {
			return fmt.Errorf("add %s to vault: %w", email, err)
		}
	}
	return nil
}

// ensureUser registers the user if needed, verifies the email when a development link is
// available, and returns an access token.
func (s *seeder) ensureUser(u demoUser) (string, error) {
	// Already provisioned directly, and verified: registering would only spend the endpoint's
	// rate limit on a request that can answer nothing but 409.
	if s.db != nil {
		token, _, err := s.login(u.Email)
		if err != nil {
			return "", fmt.Errorf("log in as %s: %w", u.Email, err)
		}
		return token, nil
	}

	status, err := s.call(http.MethodPost, "/auth/register", "", map[string]any{
		"email": u.Email, "password": s.password, "name": u.Name,
	}, nil)
	switch {
	case err == nil:
		fmt.Printf("registered %s\n", u.Email)
	case status == http.StatusConflict:
		// already registered
	default:
		return "", fmt.Errorf("register %s: %w", u.Email, err)
	}

	token, status, err := s.login(u.Email)
	if err == nil {
		return token, nil
	}
	if status != http.StatusForbidden {
		return "", fmt.Errorf("log in as %s: %w", u.Email, err)
	}

	// Email not verified yet: look for the development verification link in the API log.
	verification, findErr := s.findLinkToken(u.Email, "/verify-email")
	if findErr != nil {
		return "", fmt.Errorf("%s is not verified and no development verification link was found (%v); verify the account, or run the API with APP_ENV=development and pass --log", u.Email, findErr)
	}
	if _, err := s.call(http.MethodPost, "/auth/verify-email", "", map[string]any{"token": verification}, nil); err != nil {
		return "", fmt.Errorf("verify %s: %w", u.Email, err)
	}
	fmt.Printf("verified %s\n", u.Email)

	token, _, err = s.login(u.Email)
	if err != nil {
		return "", fmt.Errorf("log in as %s after verification: %w", u.Email, err)
	}
	return token, nil
}

func (s *seeder) login(email string) (string, int, error) {
	var tokens struct {
		AccessToken string `json:"access_token"`
	}
	status, err := s.call(http.MethodPost, "/auth/login", "", map[string]any{"email": email, "password": s.password}, &tokens)
	return tokens.AccessToken, status, err
}

// findLinkToken returns the token from the newest development link with the given path logged
// for the address, waiting briefly because emails are sent asynchronously.
func (s *seeder) findLinkToken(email, path string) (string, error) {
	if s.logPath == "" {
		return "", errors.New("no API log file given")
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		token, err := scanLogForToken(s.logPath, email, path)
		if err == nil || time.Now().After(deadline) {
			return token, err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func scanLogForToken(logPath, email, linkPath string) (string, error) {
	f, err := os.Open(logPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var token string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if !bytes.Contains(line, []byte(`"link"`)) {
			continue
		}
		var entry struct {
			To   string `json:"to"`
			Link string `json:"link"`
		}
		if json.Unmarshal(line, &entry) != nil || !strings.EqualFold(entry.To, email) || !strings.Contains(entry.Link, linkPath+"?") {
			continue
		}
		if u, err := url.Parse(entry.Link); err == nil && u.Query().Get("token") != "" {
			token = u.Query().Get("token")
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	if token == "" {
		return "", fmt.Errorf("no %s link for %s in %s", linkPath, email, logPath)
	}
	return token, nil
}

// call sends a JSON request and decodes a JSON response into out. It returns the status code
// and an error for any non-2xx response.
func (s *seeder) call(method, path, token string, body, out any) (int, error) {
	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return 0, err
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequest(method, s.api+path, reader)
	if err != nil {
		return 0, err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, err
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		var apiErr struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &apiErr)
		return resp.StatusCode, fmt.Errorf("%s %s: %d %s", method, path, resp.StatusCode, apiErr.Message)
	}

	if out != nil && len(data) > 0 {
		if err := json.Unmarshal(data, out); err != nil {
			return resp.StatusCode, fmt.Errorf("decode %s %s: %w", method, path, err)
		}
	}
	return resp.StatusCode, nil
}
