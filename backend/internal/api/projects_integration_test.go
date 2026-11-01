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
)

func TestProjectDraftAndImmutableVersions(t *testing.T) {
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
	register := func(suffix string) ([]*http.Cookie, string) {
		resp := call("POST", "/api/auth/register", map[string]string{"email": fmt.Sprintf("project-%s-%d@example.test", suffix, time.Now().UnixNano()), "password": "correct horse battery staple"}, nil, "")
		if resp.Code != 201 {
			t.Fatalf("register: %d %s", resp.Code, resp.Body.String())
		}
		var user struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(resp.Body.Bytes(), &user); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = db.Exec(context.Background(), `DELETE FROM users WHERE id=$1`, user.ID) })
		cookies := resp.Result().Cookies()
		var csrf string
		for _, cookie := range cookies {
			if cookie.Name == csrfCookieName {
				csrf = cookie.Value
			}
		}
		return cookies, csrf
	}
	alice, aliceCSRF := register("alice")
	bob, _ := register("bob")
	workspace := call("POST", "/api/workspaces", map[string]string{"name": "My lab"}, alice, aliceCSRF)
	if workspace.Code != 201 {
		t.Fatalf("workspace: %d %s", workspace.Code, workspace.Body.String())
	}
	var ws struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(workspace.Body.Bytes(), &ws)
	challenge, _, _ := challenges.Get("service-capacity")
	project := call("POST", "/api/projects", map[string]any{"workspaceId": ws.ID, "name": "Capacity lab", "architecture": challenge.Starter}, alice, aliceCSRF)
	if project.Code != 201 {
		t.Fatalf("project: %d %s", project.Code, project.Body.String())
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(project.Body.Bytes(), &created)
	if got := call("GET", "/api/projects/"+created.ID, nil, bob, ""); got.Code != 404 {
		t.Fatalf("other user read project: %d", got.Code)
	}
	first := call("POST", "/api/projects/"+created.ID+"/versions", map[string]int{"expectedRevision": 0}, alice, aliceCSRF)
	if first.Code != 201 {
		t.Fatalf("first version: %d %s", first.Code, first.Body.String())
	}
	var firstVersion struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(first.Body.Bytes(), &firstVersion)
	if got := call("POST", "/api/projects/"+created.ID+"/versions", map[string]int{"expectedRevision": 0}, alice, aliceCSRF); got.Code != 409 {
		t.Fatalf("duplicate version: %d", got.Code)
	}
	graph := challenge.Starter
	graph.Nodes[1].Config.ProcessingSlots = 2
	update := call("PUT", "/api/projects/"+created.ID+"/draft", map[string]any{"architecture": graph, "expectedRevision": 0}, alice, aliceCSRF)
	if update.Code != 200 {
		t.Fatalf("draft update: %d %s", update.Code, update.Body.String())
	}
	if got := call("PUT", "/api/projects/"+created.ID+"/draft", map[string]any{"architecture": graph, "expectedRevision": 0}, alice, aliceCSRF); got.Code != 409 {
		t.Fatalf("stale draft accepted: %d", got.Code)
	}
	second := call("POST", "/api/projects/"+created.ID+"/versions", map[string]int{"expectedRevision": 1}, alice, aliceCSRF)
	if second.Code != 201 {
		t.Fatalf("second version: %d %s", second.Code, second.Body.String())
	}
	var secondVersion struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(second.Body.Bytes(), &secondVersion)
	run := call("POST", "/api/projects/"+created.ID+"/runs", map[string]any{"versionId": secondVersion.ID, "workload": challenge.Workload}, alice, aliceCSRF)
	if run.Code != 201 || !bytes.Contains(run.Body.Bytes(), []byte(`"server_native"`)) {
		t.Fatalf("saved run: %d %s", run.Code, run.Body.String())
	}
	var savedRun struct {
		ID string `json:"id"`
	}
	_ = json.Unmarshal(run.Body.Bytes(), &savedRun)
	repeated := call("POST", "/api/projects/"+created.ID+"/runs", map[string]any{"versionId": secondVersion.ID, "workload": challenge.Workload}, alice, aliceCSRF)
	if repeated.Code != 200 || !bytes.Contains(repeated.Body.Bytes(), []byte(savedRun.ID)) {
		t.Fatalf("duplicate run did not return existing result: %d %s", repeated.Code, repeated.Body.String())
	}
	if got := call("GET", "/api/projects/"+created.ID+"/runs/"+savedRun.ID, nil, bob, ""); got.Code != 404 {
		t.Fatalf("other user read run: %d", got.Code)
	}
	if got := call("GET", "/api/projects/"+created.ID+"/runs", nil, alice, ""); got.Code != 200 || !bytes.Contains(got.Body.Bytes(), []byte(savedRun.ID)) {
		t.Fatalf("run list: %d %s", got.Code, got.Body.String())
	}
	old := call("GET", "/api/projects/"+created.ID+"/versions/"+firstVersion.ID, nil, alice, "")
	if old.Code != 200 {
		t.Fatalf("old version: %d", old.Code)
	}
	var previous struct {
		Architecture struct {
			Nodes []struct {
				Config struct {
					ProcessingSlots int `json:"processingSlots"`
				} `json:"config"`
			} `json:"nodes"`
		} `json:"architecture"`
	}
	if err := json.Unmarshal(old.Body.Bytes(), &previous); err != nil {
		t.Fatal(err)
	}
	if previous.Architecture.Nodes[1].Config.ProcessingSlots != 1 {
		t.Fatalf("old version mutated: %+v", previous)
	}
	if got := call("GET", "/api/projects/"+created.ID+"/versions", nil, alice, ""); got.Code != 200 {
		t.Fatalf("versions list: %d", got.Code)
	}
}
