package http_test

import (
	"context"
	"net/http"
	"strconv"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/apitest"
)

func TestRepeatedFailedLoginsLockTheAccount(t *testing.T) {
	api := apitest.New(t)
	user := api.CreateUser("Lock Target")
	wrong := map[string]string{"email": user.Email, "password": "wrong-password"}
	right := map[string]string{"email": user.Email, "password": apitest.TestPassword}

	for i := 0; i < 5; i++ {
		api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", wrong)
	}

	// Locked: even the right password is refused, without being checked
	resp := api.MustDo(http.StatusTooManyRequests, "POST", "/auth/login", "", right)
	retry, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || retry < 1 || retry > 60 {
		t.Fatalf("expected Retry-After of up to a minute for the first lock, got %q", resp.Header.Get("Retry-After"))
	}
	var body struct {
		Error string `json:"error"`
	}
	resp.Decode(t, &body)
	if body.Error != "account_locked" {
		t.Fatalf("unexpected error %q", body.Error)
	}

	var page auditPage
	api.MustDo(http.StatusOK, "GET", "/audit?action=login.locked", user.Token, nil).Decode(t, &page)
	if page.Total != 1 {
		t.Fatalf("expected one login.locked event, got %d", page.Total)
	}

	// When the lock expires the right password works and the counter resets
	ctx := context.Background()
	if _, err := api.Pool.Exec(ctx, `UPDATE users SET login_locked_until = now() - interval '1 second' WHERE email = $1`, user.Email); err != nil {
		t.Fatal(err)
	}
	api.MustDo(http.StatusOK, "POST", "/auth/login", "", right)
	var attempts int
	if err := api.Pool.QueryRow(ctx, `SELECT failed_login_attempts FROM users WHERE email = $1`, user.Email).Scan(&attempts); err != nil {
		t.Fatal(err)
	}
	if attempts != 0 {
		t.Fatalf("counter not reset, still %d", attempts)
	}

	// Each further failure past the threshold doubles the wait
	for i := 0; i < 5; i++ {
		api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", wrong)
	}
	if _, err := api.Pool.Exec(ctx, `UPDATE users SET login_locked_until = now() - interval '1 second' WHERE email = $1`, user.Email); err != nil {
		t.Fatal(err)
	}
	api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", wrong)
	var minutes float64
	if err := api.Pool.QueryRow(ctx, `SELECT EXTRACT(EPOCH FROM login_locked_until - now()) / 60 FROM users WHERE email = $1`, user.Email).Scan(&minutes); err != nil {
		t.Fatal(err)
	}
	if minutes < 1.5 || minutes > 2.1 {
		t.Fatalf("sixth failure should lock for about 2 minutes, got %.2f", minutes)
	}

	// Unknown accounts are never locked and keep answering 401
	for i := 0; i < 7; i++ {
		api.MustDo(http.StatusUnauthorized, "POST", "/auth/login", "", map[string]string{"email": "ghost@example.test", "password": "x"})
	}
}
