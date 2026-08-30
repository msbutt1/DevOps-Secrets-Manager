package http

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/http/clientip"
	"github.com/msbutt1/DevOps-Secrets-Manager/apps/api/internal/logging"
)

func TestRequestLoggingRecoversPanics(t *testing.T) {
	var out bytes.Buffer
	resolver, err := clientip.NewResolver(nil)
	if err != nil {
		t.Fatal(err)
	}
	panicking := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logging.SetUserID(r.Context(), [16]byte{1})
		panic("boom")
	})
	handler := requestLogging(logging.New(&out, 0), resolver)(panicking)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest("POST", "/secrets/123/reveal?x=1", nil))

	if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "internal_error") {
		t.Fatalf("want 500 JSON error, got %d %s", rec.Code, rec.Body)
	}
	id := rec.Header().Get(RequestIDHeader)

	lines := strings.Split(strings.TrimSpace(out.String()), "\n")
	if len(lines) != 2 {
		t.Fatalf("want panic and request lines, got %q", out.String())
	}
	var panicLine, requestLine map[string]any
	_ = json.Unmarshal([]byte(lines[0]), &panicLine)
	_ = json.Unmarshal([]byte(lines[1]), &requestLine)
	if panicLine["panic"] != "boom" || panicLine["request_id"] != id || panicLine["level"] != "ERROR" {
		t.Errorf("unexpected panic log: %v", panicLine)
	}
	if requestLine["status"] != float64(500) || requestLine["path"] != "/secrets/123/reveal" || requestLine["request_id"] != id {
		t.Errorf("unexpected request log: %v", requestLine)
	}
	if requestLine["user_id"] == nil {
		t.Errorf("user set by inner middleware missing from request log: %v", requestLine)
	}
}
