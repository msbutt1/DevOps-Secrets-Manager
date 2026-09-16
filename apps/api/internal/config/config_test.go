package config

import "testing"

func TestServerPort(t *testing.T) {
	t.Run("defaults to 8080", func(t *testing.T) {
		t.Setenv("APP_SERVER_PORT", "")
		t.Setenv("PORT", "")
		if got := ServerPort(); got != 8080 {
			t.Errorf("got %d", got)
		}
	})

	t.Run("uses PORT, as platforms set it", func(t *testing.T) {
		t.Setenv("APP_SERVER_PORT", "")
		t.Setenv("PORT", "10000")
		if got := ServerPort(); got != 10000 {
			t.Errorf("got %d", got)
		}
	})

	t.Run("APP_SERVER_PORT wins over PORT", func(t *testing.T) {
		t.Setenv("APP_SERVER_PORT", "9000")
		t.Setenv("PORT", "10000")
		if got := ServerPort(); got != 9000 {
			t.Errorf("got %d", got)
		}
	})

	t.Run("nonsense falls through", func(t *testing.T) {
		t.Setenv("APP_SERVER_PORT", "http")
		t.Setenv("PORT", "70000")
		if got := ServerPort(); got != 8080 {
			t.Errorf("got %d", got)
		}
	})
}

func TestVersion(t *testing.T) {
	t.Run("a build version wins", func(t *testing.T) {
		t.Setenv("RENDER_GIT_COMMIT", "0123456789abcdef")
		if got := Version("v1.0.0"); got != "v1.0.0" {
			t.Errorf("Version = %q, want v1.0.0", got)
		}
	})
	t.Run("the platform's commit is used when the build says dev", func(t *testing.T) {
		t.Setenv("VERSION", "")
		t.Setenv("RENDER_GIT_COMMIT", "0123456789abcdef")
		if got := Version("dev"); got != "0123456789ab" {
			t.Errorf("Version = %q, want the commit shortened to 12", got)
		}
	})
	t.Run("VERSION is preferred over the commit", func(t *testing.T) {
		t.Setenv("VERSION", "v1.1.0")
		t.Setenv("RENDER_GIT_COMMIT", "0123456789abcdef")
		if got := Version("dev"); got != "v1.1.0" {
			t.Errorf("Version = %q, want v1.1.0", got)
		}
	})
	t.Run("dev when nothing says otherwise", func(t *testing.T) {
		t.Setenv("VERSION", "")
		t.Setenv("RENDER_GIT_COMMIT", "")
		t.Setenv("FLY_MACHINE_VERSION", "")
		t.Setenv("KOYEB_GIT_SHA", "")
		if got := Version(""); got != "dev" {
			t.Errorf("Version = %q, want dev", got)
		}
	})
}
