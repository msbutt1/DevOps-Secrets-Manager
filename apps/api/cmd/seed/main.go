// Command seed creates demo users, vaults, environments and secrets through the HTTP API,
// so it works against any deployment. It is idempotent: existing data is left alone.
//
// New accounts must verify their email before they can log in. Against a local API running
// with APP_ENV=development and no SMTP, pass --log with the API's log file and the seed reads
// the verification links from it. Elsewhere, verify the accounts by email and run it again.
package main

import (
	"bufio"
	"bytes"
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
)

type demoUser struct {
	Email string
	Name  string
}

type demoSecret struct {
	Key         string
	Value       string
	Description string
}

type demoVault struct {
	Name         string
	Description  string
	Environments map[string][]demoSecret
	envOrder     []string
}

// owner creates and owns every demo vault; the others are teammates.
var (
	owner     = demoUser{Email: "salaar@demo.dev", Name: "Salaar Butt"}
	teammates = []demoUser{
		{Email: "priya@demo.dev", Name: "Priya Raman"},
		{Email: "marcus@demo.dev", Name: "Marcus Chen"},
		{Email: "ops-bot@demo.dev", Name: "Ops Bot"},
	}
)

func demoVaults() []demoVault {
	return []demoVault{
		{
			Name:        "payments-api",
			Description: "Stripe, database, and queue credentials for the payments service",
			envOrder:    []string{"development", "staging", "production"},
			Environments: map[string][]demoSecret{
				"development": {
					{"DATABASE_URL", "postgres://payments:demo-dev-password@localhost:5432/payments", "Local Postgres"},
				},
				"staging": {
					{"DATABASE_URL", "postgres://payments:demo-staging-password@staging-db.internal:5432/payments", "Staging Postgres"},
					{"STRIPE_SECRET_KEY", "sk_test_demo_51HxStagingKeyNotReal", "Stripe test key"},
				},
				"production": {
					{"DATABASE_URL", "postgres://payments:demo-prod-password@prod-db.internal:5432/payments", "Primary Postgres"},
					{"STRIPE_SECRET_KEY", "sk_live_demo_51HxProductionKeyNotReal", "Stripe API key"},
					{"REDIS_URL", "redis://:demo-redis-password@cache.internal:6379/0", "Session cache"},
					{"JWT_SIGNING_KEY", "demo-signing-key-9f2c1e7a4b8d", "Access token signing"},
					{"SENTRY_DSN", "https://demo-public-key@o0.ingest.sentry.io/0", "Error reporting"},
				},
			},
		},
		{
			Name:        "web-frontend",
			Description: "Public build-time keys and analytics tokens",
			envOrder:    []string{"staging", "production"},
			Environments: map[string][]demoSecret{
				"staging": {
					{"NEXT_PUBLIC_API_URL", "https://api.staging.demo.dev", "API origin"},
				},
				"production": {
					{"NEXT_PUBLIC_API_URL", "https://api.demo.dev", "API origin"},
					{"ANALYTICS_WRITE_KEY", "demo-analytics-write-key-3c9e", "Analytics ingest"},
				},
			},
		},
		{
			Name:        "infra-terraform",
			Description: "State backend and provider credentials for infrastructure",
			envOrder:    []string{"production"},
			Environments: map[string][]demoSecret{
				"production": {
					{"TF_STATE_BUCKET", "demo-terraform-state", "Remote state"},
					{"CLOUDFLARE_API_TOKEN", "demo-cloudflare-token-7d1a", "DNS automation"},
				},
			},
		},
		{
			Name:        "ml-inference",
			Description: "Model registry and GPU cluster credentials",
			envOrder:    []string{"production"},
			Environments: map[string][]demoSecret{
				"production": {
					{"HF_TOKEN", "hf_demoTokenNotReal1234", "Model registry pull"},
					{"AWS_ACCESS_KEY_ID", "AKIADEMONOTREAL0000", "Artifact bucket access"},
					{"S3_BUCKET", "demo-model-checkpoints", "Checkpoint storage"},
				},
			},
		},
	}
}

func main() {
	apiURL := flag.String("api-url", envOr("SEED_API_URL", "http://localhost:8080"), "API base URL")
	logPath := flag.String("log", os.Getenv("SEED_API_LOG"), "API log file to read development verification links from")
	password := flag.String("password", envOr("SEED_PASSWORD", "Demo-Passw0rd!2026"), "password for every demo account")
	flag.Parse()

	s := &seeder{
		api:      strings.TrimRight(*apiURL, "/"),
		logPath:  *logPath,
		password: *password,
		http:     &http.Client{Timeout: 15 * time.Second},
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
}

func (s *seeder) run() error {
	if _, err := s.call(http.MethodGet, "/health", "", nil, nil); err != nil {
		return fmt.Errorf("API not reachable at %s: %w", s.api, err)
	}

	ownerToken, err := s.ensureUser(owner)
	if err != nil {
		return err
	}
	for _, u := range teammates {
		if _, err := s.ensureUser(u); err != nil {
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

// ensureUser registers the user if needed, verifies the email when a development link is
// available, and returns an access token.
func (s *seeder) ensureUser(u demoUser) (string, error) {
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
	verification, findErr := s.findVerificationToken(u.Email)
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

// findVerificationToken returns the token from the newest development verification link
// logged for the address, waiting briefly because the email is sent asynchronously.
func (s *seeder) findVerificationToken(email string) (string, error) {
	if s.logPath == "" {
		return "", errors.New("no API log file given")
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		token, err := scanLogForToken(s.logPath, email)
		if err == nil || time.Now().After(deadline) {
			return token, err
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func scanLogForToken(path, email string) (string, error) {
	f, err := os.Open(path)
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
		if json.Unmarshal(line, &entry) != nil || !strings.EqualFold(entry.To, email) || !strings.Contains(entry.Link, "/verify-email") {
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
		return "", fmt.Errorf("no verification link for %s in %s", email, path)
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
