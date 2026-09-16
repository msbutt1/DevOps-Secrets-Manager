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
