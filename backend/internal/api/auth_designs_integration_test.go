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

	"playhld/backend/internal/storage"
)

func TestPrivateDesignLifecycle(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL integration")
	}
	ctx := context.Background()
	db, err := storage.Open(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := storage.Migrate(ctx, db); err != nil {
		t.Fatal(err)
	}
	server := NewServerWithOptions(Options{DB: db})
	call := func(method, path string, body any, cookies []*http.Cookie, csrf string) *httptest.ResponseRecorder {
		var data []byte
		if body != nil {
			data, _ = json.Marshal(body)
		}
		request := httptest.NewRequest(method, path, bytes.NewReader(data))
		if body != nil {
			request.Header.Set("Content-Type", "application/json")
		}
		for _, cookie := range cookies {
			request.AddCookie(cookie)
		}
		if csrf != "" {
			request.Header.Set("X-CSRF-Token", csrf)
		}
		response := httptest.NewRecorder()
		server.ServeHTTP(response, request)
		return response
	}
	register := func(suffix string) (string, []*http.Cookie, string) {
		email := fmt.Sprintf("test-%d-%s@example.test", time.Now().UnixNano(), suffix)
		response := call("POST", "/api/auth/register", map[string]string{"email": email, "password": "correct horse battery staple"}, nil, "")
		if response.Code != 201 {
			t.Fatalf("register: %d %s", response.Code, response.Body.String())
		}
		var user struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(response.Body.Bytes(), &user); err != nil {
			t.Fatal(err)
		}
		cookies := response.Result().Cookies()
		csrf := ""
		for _, cookie := range cookies {
			if cookie.Name == csrfCookieName {
				csrf = cookie.Value
			}
		}
		if csrf == "" {
			t.Fatal("csrf cookie missing")
		}
		t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID) })
		return user.ID, cookies, csrf
	}
	_, aliceCookies, aliceCSRF := register("alice")
	_, bobCookies, bobCSRF := register("bob")
	graph := json.RawMessage(`{"schemaVersion":2,"nodes":[{"id":"source","kind":"source","position":{"x":0,"y":0},"config":{"queueLimit":0}}],"edges":[]}`)
	create := call("POST", "/api/designs", map[string]any{"name": "First design", "architecture": graph}, aliceCookies, aliceCSRF)
	if create.Code != 201 {
		t.Fatalf("create: %d %s", create.Code, create.Body.String())
	}
	var design struct {
		ID      string `json:"id"`
		Version int64  `json:"version"`
	}
	if err := json.Unmarshal(create.Body.Bytes(), &design); err != nil {
		t.Fatal(err)
	}
	if design.ID == "" || design.Version != 1 {
		t.Fatalf("unexpected created design: %+v", design)
	}
	if got := call("GET", "/api/designs/"+design.ID, nil, bobCookies, ""); got.Code != 404 {
		t.Fatalf("bob could read alice design: %d", got.Code)
	}
	if got := call("GET", "/api/designs/"+design.ID, nil, aliceCookies, ""); got.Code != 200 {
		t.Fatalf("alice could not read design: %d", got.Code)
	}
	if got := call("GET", "/api/designs?limit=1", nil, aliceCookies, ""); got.Code != 200 {
		t.Fatalf("alice list failed: %d", got.Code)
	}
	updateBody := map[string]any{"name": "Updated design", "architecture": graph, "expectedVersion": 1}
	if got := call("PUT", "/api/designs/"+design.ID, updateBody, bobCookies, bobCSRF); got.Code != 404 {
		t.Fatalf("bob could update alice design: %d", got.Code)
	}
	if got := call("PUT", "/api/designs/"+design.ID, updateBody, aliceCookies, ""); got.Code != 403 {
		t.Fatalf("missing csrf accepted: %d", got.Code)
	}
	if got := call("PUT", "/api/designs/"+design.ID, updateBody, aliceCookies, aliceCSRF); got.Code != 200 {
		t.Fatalf("owner update failed: %d %s", got.Code, got.Body.String())
	}
	if got := call("PUT", "/api/designs/"+design.ID, updateBody, aliceCookies, aliceCSRF); got.Code != 409 {
		t.Fatalf("stale version accepted: %d", got.Code)
	}
	if got := call("GET", "/api/auth/me", nil, aliceCookies, ""); got.Code != 200 {
		t.Fatalf("session failed: %d", got.Code)
	}
	if got := call("POST", "/api/auth/logout", nil, aliceCookies, aliceCSRF); got.Code != 200 {
		t.Fatalf("logout failed: %d", got.Code)
	}
	if got := call("GET", "/api/auth/me", nil, aliceCookies, ""); got.Code != 401 {
		t.Fatalf("revoked session accepted: %d", got.Code)
	}
}
