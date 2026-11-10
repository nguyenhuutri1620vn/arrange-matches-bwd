package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"testing"

	"arrange-matches-bwd/internal/finance"
	"arrange-matches-bwd/internal/session"
	"arrange-matches-bwd/internal/storage"
)

type client struct {
	t    *testing.T
	http *http.Client
	base string
}

func newTestServer(t *testing.T) *client {
	t.Helper()
	store, err := storage.Open(context.Background(), ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	srv := httptest.NewServer(New(Config{
		Sessions:      session.New(store, nil),
		Finance:       finance.New(store, nil),
		AdminUsername: "host",
		AdminPassword: "test-password",
		Secret:        []byte("test-secret"),
	}))
	t.Cleanup(srv.Close)
	jar, _ := cookiejar.New(nil)
	return &client{t: t, http: &http.Client{Jar: jar}, base: srv.URL}
}

// do sends a JSON request and decodes the response into out (if not nil).
func (c *client) do(method, path string, body, out any) int {
	c.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			c.t.Fatal(err)
		}
	}
	req, err := http.NewRequest(method, c.base+path, &buf)
	if err != nil {
		c.t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer resp.Body.Close()
	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			c.t.Fatalf("%s %s: decode: %v", method, path, err)
		}
	}
	return resp.StatusCode
}

func (c *client) login() {
	c.t.Helper()
	if status := c.do("POST", "/api/login", map[string]string{"username": "host", "password": "test-password"}, nil); status != http.StatusOK {
		c.t.Fatalf("login status %d", status)
	}
}

func TestAPIRequiresLogin(t *testing.T) {
	c := newTestServer(t)
	for _, probe := range []struct{ method, path string }{
		{"GET", "/api/players"},
		{"GET", "/api/session/current"},
		{"POST", "/api/sessions"},
		{"GET", "/api/reports/summary"},
		{"DELETE", "/api/transactions/1"},
		{"GET", "/api/unknown"},
	} {
		if status := c.do(probe.method, probe.path, map[string]int{}, nil); status != http.StatusUnauthorized {
			t.Errorf("%s %s without login: status %d, want 401", probe.method, probe.path, status)
		}
	}
	if status := c.do("POST", "/api/login", map[string]string{"username": "host", "password": "wrong"}, nil); status != http.StatusUnauthorized {
		t.Fatalf("wrong password: status %d", status)
	}
	var me struct {
		Authenticated bool `json:"authenticated"`
	}
	c.do("GET", "/api/me", nil, &me)
	if me.Authenticated {
		t.Fatal("authenticated before login")
	}
	c.login()
	if status := c.do("GET", "/api/players", nil, nil); status != http.StatusOK {
		t.Fatalf("after login: status %d", status)
	}
	c.do("POST", "/api/logout", map[string]int{}, nil)
	if status := c.do("GET", "/api/players", nil, nil); status != http.StatusUnauthorized {
		t.Fatalf("after logout: status %d", status)
	}
}

func TestLoginThrottle(t *testing.T) {
	c := newTestServer(t)
	for i := 0; i < maxLoginFails; i++ {
		c.do("POST", "/api/login", map[string]string{"username": "host", "password": "wrong"}, nil)
	}
	if status := c.do("POST", "/api/login", map[string]string{"username": "host", "password": "test-password"}, nil); status != http.StatusTooManyRequests {
		t.Fatalf("status %d, want 429 after repeated failures", status)
	}
}

func TestSessionFlowOverHTTP(t *testing.T) {
	c := newTestServer(t)
	c.login()

	var state stateDTO
	if status := c.do("POST", "/api/sessions", map[string]int{"court_count": 1}, &state); status != http.StatusOK {
		t.Fatalf("create session: %d", status)
	}
	if state.Session.FeeMale != 70000 || state.Session.FeeFemale != 60000 {
		t.Fatalf("default fees not applied: %+v", state.Session)
	}
	sid := state.Session.ID
	for _, name := range []string{"An", "Bình", "Cường", "Dũng"} {
		var p playerDTO
		if status := c.do("POST", "/api/players", playerDTO{Name: name, Gender: "M", Level: "TB"}, &p); status != http.StatusOK {
			t.Fatalf("create player: %d", status)
		}
		if status := c.do("POST", "/api/sessions/"+itoa(sid)+"/players", map[string]int64{"player_id": p.ID}, &state); status != http.StatusOK {
			t.Fatalf("check in: %d", status)
		}
	}

	courtID := state.Courts[0].ID
	if status := c.do("POST", "/api/courts/"+itoa(courtID)+"/propose", nil, &state); status != http.StatusOK {
		t.Fatalf("propose: %d", status)
	}
	matchID := state.Courts[0].Match.ID
	if status := c.do("POST", "/api/matches/"+itoa(matchID)+"/confirm", map[string]bool{"force": false}, &state); status != http.StatusOK {
		t.Fatalf("confirm: %d", status)
	}
	if state.Courts[0].Status != "IN_USE" {
		t.Fatalf("court status %s", state.Courts[0].Status)
	}

	// A business error carries a code and a Vietnamese message.
	var errBody errorResponse
	if status := c.do("POST", "/api/courts/"+itoa(courtID)+"/propose", nil, &errBody); status != http.StatusConflict || errBody.Error.Code != "court_not_empty" || errBody.Error.Message == "" {
		t.Fatalf("propose on busy court: %d %+v", status, errBody)
	}

	if status := c.do("POST", "/api/matches/"+itoa(matchID)+"/finish", nil, &state); status != http.StatusOK {
		t.Fatalf("finish: %d", status)
	}
	if status := c.do("PUT", "/api/session-players/"+itoa(state.Players[0].ID)+"/payment", map[string]any{"is_paid": true, "fee_override": 50000}, &state); status != http.StatusOK {
		t.Fatalf("payment: %d", status)
	}
	if state.Fees.Collected != 50000 {
		t.Fatalf("collected %d", state.Fees.Collected)
	}
	if status := c.do("PUT", "/api/session-players/"+itoa(state.Players[0].ID)+"/payment", map[string]any{"fee_override": nil}, &state); status != http.StatusOK || state.Fees.Collected != 70000 {
		t.Fatalf("clearing override: %d collected %d", status, state.Fees.Collected)
	}
	if status := c.do("POST", "/api/sessions/"+itoa(sid)+"/end", nil, &state); status != http.StatusOK || state.Session.Status != "ENDED" {
		t.Fatalf("end: %d %s", status, state.Session.Status)
	}

	// Reloading reads everything back from the server.
	var current *stateDTO
	c.do("GET", "/api/session/current", nil, &current)
	if current != nil {
		t.Fatal("ended session still current")
	}
	var report monthReportDTO
	if status := c.do("GET", "/api/reports/months/"+state.Session.Date[:7], nil, &report); status != http.StatusOK {
		t.Fatalf("month report: %d", status)
	}
	if report.Income != 70000 || len(report.Sessions) != 1 || report.Sessions[0].PlayerCount != 4 {
		t.Fatalf("report: %+v", report)
	}
}

func itoa(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
