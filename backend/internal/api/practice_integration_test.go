package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"systemforge/backend/internal/storage"
	"systemforge/content/challenges"
	"systemforge/simulation/model"
)

func TestPracticeAttemptIsServerVerifiedAndPrivate(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration")
	}
	db, err := storage.Open(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(context.Background(), db); err != nil {
		t.Fatal(err)
	}
	server := NewServerWithOptions(Options{DB: db})
	call := func(method, path string, body any, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		r := httptest.NewRequest(method, path, bytes.NewReader(data))
		if body != nil {
			r.Header.Set("Content-Type", "application/json")
		}
		if csrf != "" {
			r.Header.Set("X-CSRF-Token", csrf)
		}
		for _, cookie := range cookies {
			r.AddCookie(cookie)
		}
		w := httptest.NewRecorder()
		server.ServeHTTP(w, r)
		return w
	}
	if got := call("GET", "/api/practice/challenges", nil, nil, ""); got.Code != 200 {
		t.Fatalf("list: %d %s", got.Code, got.Body.String())
	}
	challenge, found, err := challenges.Get("service-capacity")
	if err != nil || !found {
		t.Fatal("missing challenge", err)
	}
	if got := call("POST", "/api/practice/challenges/service-capacity/attempts", map[string]any{"architecture": challenge.Starter}, nil, ""); got.Code != 401 {
		t.Fatalf("anonymous submit: %d", got.Code)
	}
	email := fmt.Sprintf("practice-%d@example.test", time.Now().UnixNano())
	registration := call("POST", "/api/auth/register", map[string]any{"email": email, "password": "correct horse battery staple"}, nil, "")
	if registration.Code != 201 {
		t.Fatalf("register: %d %s", registration.Code, registration.Body.String())
	}
	var user struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(registration.Body.Bytes(), &user); err != nil {
		t.Fatal(err)
	}
	defer db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID)
	cookies := registration.Result().Cookies()
	var csrf string
	for _, cookie := range cookies {
		if cookie.Name == csrfCookieName {
			csrf = cookie.Value
		}
	}
	if got := call("POST", "/api/practice/challenges/service-capacity/attempts", map[string]any{"architecture": challenge.Starter}, cookies, ""); got.Code != 403 {
		t.Fatalf("missing CSRF: %d", got.Code)
	}
	graph := challenge.Starter
	graph.Nodes[1].Config.ProcessingSlots = 4
	graph.Edges = append(graph.Edges, model.Edge{ID: "direct", Source: "source", Target: "api", Route: "always"})
	attempt := call("POST", "/api/practice/challenges/service-capacity/attempts", map[string]any{"architecture": graph}, cookies, csrf)
	if attempt.Code != 201 {
		t.Fatalf("attempt: %d %s", attempt.Code, attempt.Body.String())
	}
	var saved struct {
		Completed  bool   `json:"completed"`
		Provenance string `json:"provenance"`
		Goals      []struct {
			Status string `json:"status"`
		} `json:"goals"`
	}
	if err := json.Unmarshal(attempt.Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if !saved.Completed || saved.Provenance != "server_native" || len(saved.Goals) != 3 {
		t.Fatalf("unexpected attempt: %+v", saved)
	}
	if got := call("GET", "/api/practice/attempts", nil, cookies, ""); got.Code != 200 || !bytes.Contains(got.Body.Bytes(), []byte("service-capacity")) {
		t.Fatalf("attempt list: %d %s", got.Code, got.Body.String())
	}
	progress := call("GET", "/api/practice/progress/service-capacity", nil, cookies, "")
	if progress.Code != 200 || !bytes.Contains(progress.Body.Bytes(), []byte(`"attemptCount":1`)) || !bytes.Contains(progress.Body.Bytes(), []byte(`"completed":true`)) {
		t.Fatalf("progress: %d %s", progress.Code, progress.Body.String())
	}
}
