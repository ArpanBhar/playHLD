package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"systemforge/simulation/model"
)

type workspaceInput struct {
	Name string `json:"name"`
}
type projectInput struct {
	WorkspaceID  string          `json:"workspaceId"`
	Name         string          `json:"name"`
	Description  string          `json:"description"`
	Architecture json.RawMessage `json:"architecture"`
}
type draftInput struct {
	Architecture     json.RawMessage `json:"architecture"`
	ExpectedRevision int64           `json:"expectedRevision"`
}
type versionInput struct {
	ExpectedRevision int64 `json:"expectedRevision"`
}

func validName(name string) bool {
	return len(strings.TrimSpace(name)) > 0 && len([]rune(strings.TrimSpace(name))) <= 120
}

func workspaceRole(ctx context.Context, db *pgxpool.Pool, userID, workspaceID string) (string, error) {
	var role string
	err := db.QueryRow(ctx, `SELECT role FROM workspace_members WHERE workspace_id=$1 AND user_id=$2 AND status='active'`, workspaceID, userID).Scan(&role)
	return role, err
}
func projectRole(ctx context.Context, db *pgxpool.Pool, userID, projectID string) (string, error) {
	var role string
	err := db.QueryRow(ctx, `SELECT m.role FROM projects p JOIN workspace_members m ON m.workspace_id=p.workspace_id WHERE p.id=$1 AND m.user_id=$2 AND m.status='active'`, projectID, userID).Scan(&role)
	return role, err
}
func requireProject(w http.ResponseWriter, r *http.Request, db *pgxpool.Pool, userID, projectID string, write bool) bool {
	if !uuidPattern.MatchString(projectID) {
		writeError(w, 404, "NOT_FOUND", "project was not found")
		return false
	}
	role, err := projectRole(r.Context(), db, userID, projectID)
	if err != nil {
		writeError(w, 404, "NOT_FOUND", "project was not found")
		return false
	}
	if write && role == "viewer" {
		writeError(w, 403, "FORBIDDEN", "project is read only")
		return false
	}
	return true
}

func registerProjectRoutes(mux *http.ServeMux, db *pgxpool.Pool) {
	mux.HandleFunc("POST /api/workspaces", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, 403, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		var input workspaceInput
		if !readStrict(w, r, 4096, &input) {
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if !validName(input.Name) {
			writeError(w, 422, "INVALID_NAME", "name must contain 1–120 characters")
			return
		}
		tx, err := db.Begin(r.Context())
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer tx.Rollback(r.Context())
		var id string
		err = tx.QueryRow(r.Context(), `INSERT INTO workspaces(owner_id,name) VALUES($1,$2) RETURNING id::text`, user.ID, input.Name).Scan(&id)
		if err == nil {
			_, err = tx.Exec(r.Context(), `INSERT INTO workspace_members(workspace_id,user_id,role) VALUES($1,$2,'owner')`, id, user.ID)
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 201, map[string]any{"id": id, "name": input.Name, "role": "owner"})
	})
	mux.HandleFunc("GET /api/workspaces", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		rows, err := db.Query(r.Context(), `SELECT w.id::text,w.name,m.role FROM workspaces w JOIN workspace_members m ON m.workspace_id=w.id WHERE m.user_id=$1 AND m.status='active' ORDER BY w.created_at DESC,w.id DESC LIMIT 100`, user.ID)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, name, role string
			if err = rows.Scan(&id, &name, &role); err != nil {
				writeDatabaseError(w)
				return
			}
			items = append(items, map[string]any{"id": id, "name": name, "role": role})
		}
		if rows.Err() != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("POST /api/projects", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, 403, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		var input projectInput
		if !readStrict(w, r, model.MaxPayloadBytes, &input) {
			return
		}
		input.Name = strings.TrimSpace(input.Name)
		if !uuidPattern.MatchString(input.WorkspaceID) || !validName(input.Name) || len(input.Description) > 2000 {
			writeError(w, 422, "INVALID_PROJECT", "workspace, name or description is invalid")
			return
		}
		role, err := workspaceRole(r.Context(), db, user.ID, input.WorkspaceID)
		if err != nil {
			writeError(w, 404, "NOT_FOUND", "workspace was not found")
			return
		}
		if role == "viewer" {
			writeError(w, 403, "FORBIDDEN", "workspace is read only")
			return
		}
		graph, err := model.ParseArchitecture(input.Architecture)
		if err != nil {
			writeError(w, 422, "INVALID_ARCHITECTURE", err.Error())
			return
		}
		graphJSON, _ := json.Marshal(graph)
		tx, err := db.Begin(r.Context())
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer tx.Rollback(r.Context())
		var projectID, branchID string
		err = tx.QueryRow(r.Context(), `INSERT INTO projects(workspace_id,name,description) VALUES($1,$2,$3) RETURNING id::text`, input.WorkspaceID, input.Name, input.Description).Scan(&projectID)
		if err == nil {
			err = tx.QueryRow(r.Context(), `INSERT INTO architecture_branches(project_id,name,draft_json) VALUES($1,'main',$2) RETURNING id::text`, projectID, graphJSON).Scan(&branchID)
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 201, map[string]any{"id": projectID, "workspaceId": input.WorkspaceID, "name": input.Name, "description": input.Description, "branch": map[string]any{"id": branchID, "revision": 0, "architecture": graph}})
	})
	mux.HandleFunc("GET /api/projects", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		rows, err := db.Query(r.Context(), `SELECT p.id::text,p.workspace_id::text,p.name,p.description,p.updated_at,m.role FROM projects p JOIN workspace_members m ON m.workspace_id=p.workspace_id WHERE m.user_id=$1 AND m.status='active' ORDER BY p.updated_at DESC,p.id DESC LIMIT 100`, user.ID)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, wid, name, description, role string
			var updated time.Time
			if err = rows.Scan(&id, &wid, &name, &description, &updated, &role); err != nil {
				writeDatabaseError(w)
				return
			}
			items = append(items, map[string]any{"id": id, "workspaceId": wid, "name": name, "description": description, "updatedAt": updated, "role": role})
		}
		if rows.Err() != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/projects/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !requireProject(w, r, db, user.ID, id, false) {
			return
		}
		var workspaceID, name, description, branchID string
		var revision int64
		var graph json.RawMessage
		err := db.QueryRow(r.Context(), `SELECT p.workspace_id::text,p.name,p.description,b.id::text,b.revision,b.draft_json FROM projects p JOIN architecture_branches b ON b.project_id=p.id AND b.name='main' WHERE p.id=$1`, id).Scan(&workspaceID, &name, &description, &branchID, &revision, &graph)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"id": id, "workspaceId": workspaceID, "name": name, "description": description, "branch": map[string]any{"id": branchID, "revision": revision, "architecture": graph}})
	})
	mux.HandleFunc("PUT /api/projects/{id}/draft", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, 403, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !requireProject(w, r, db, user.ID, id, true) {
			return
		}
		var input draftInput
		if !readStrict(w, r, model.MaxPayloadBytes, &input) {
			return
		}
		if input.ExpectedRevision < 0 {
			writeError(w, 422, "INVALID_REVISION", "expected revision must be nonnegative")
			return
		}
		graph, err := model.ParseArchitecture(input.Architecture)
		if err != nil {
			writeError(w, 422, "INVALID_ARCHITECTURE", err.Error())
			return
		}
		graphJSON, _ := json.Marshal(graph)
		var revision int64
		err = db.QueryRow(r.Context(), `UPDATE architecture_branches SET draft_json=$1,revision=revision+1 WHERE project_id=$2 AND name='main' AND revision=$3 RETURNING revision`, graphJSON, id, input.ExpectedRevision).Scan(&revision)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 409, "REVISION_CONFLICT", "draft changed; reload before saving")
			return
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		_, _ = db.Exec(r.Context(), `UPDATE projects SET updated_at=now() WHERE id=$1`, id)
		writeJSON(w, 200, map[string]any{"id": id, "revision": revision, "architecture": graph})
	})
	mux.HandleFunc("POST /api/projects/{id}/versions", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, 403, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !requireProject(w, r, db, user.ID, id, true) {
			return
		}
		var input versionInput
		if !readStrict(w, r, 4096, &input) {
			return
		}
		tx, err := db.Begin(r.Context())
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer tx.Rollback(r.Context())
		var branchID string
		var revision int64
		var graph []byte
		err = tx.QueryRow(r.Context(), `SELECT id::text,revision,draft_json FROM architecture_branches WHERE project_id=$1 AND name='main' FOR UPDATE`, id).Scan(&branchID, &revision, &graph)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		if revision != input.ExpectedRevision {
			writeError(w, 409, "REVISION_CONFLICT", "draft changed; reload before saving version")
			return
		}
		var existing bool
		err = tx.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM architecture_versions WHERE branch_id=$1 AND branch_revision=$2)`, branchID, revision).Scan(&existing)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		if existing {
			writeError(w, 409, "VERSION_EXISTS", "this revision already has a version")
			return
		}
		var parentID *string
		var parent string
		err = tx.QueryRow(r.Context(), `SELECT id::text FROM architecture_versions WHERE branch_id=$1 ORDER BY branch_revision DESC LIMIT 1`, branchID).Scan(&parent)
		if err == nil {
			parentID = &parent
		} else if !errors.Is(err, pgx.ErrNoRows) {
			writeDatabaseError(w)
			return
		}
		hash := sha256.Sum256(graph)
		var versionID string
		var createdAt time.Time
		err = tx.QueryRow(r.Context(), `INSERT INTO architecture_versions(project_id,branch_id,parent_id,branch_revision,graph_json,graph_hash,author_id) VALUES($1,$2,$3,$4,$5,$6,$7) RETURNING id::text,created_at`, id, branchID, parentID, revision, graph, hex.EncodeToString(hash[:]), user.ID).Scan(&versionID, &createdAt)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		if err = tx.Commit(r.Context()); err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 201, map[string]any{"id": versionID, "projectId": id, "branchId": branchID, "revision": revision, "parentId": parentID, "graphHash": hex.EncodeToString(hash[:]), "createdAt": createdAt})
	})
	mux.HandleFunc("GET /api/projects/{id}/versions", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !requireProject(w, r, db, user.ID, id, false) {
			return
		}
		rows, err := db.Query(r.Context(), `SELECT id::text,branch_revision,graph_hash,created_at FROM architecture_versions WHERE project_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, id)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var versionID, hash string
			var revision int64
			var createdAt time.Time
			if err = rows.Scan(&versionID, &revision, &hash, &createdAt); err != nil {
				writeDatabaseError(w)
				return
			}
			items = append(items, map[string]any{"id": versionID, "revision": revision, "graphHash": hash, "createdAt": createdAt})
		}
		if rows.Err() != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/projects/{id}/versions/{versionId}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !requireProject(w, r, db, user.ID, id, false) {
			return
		}
		versionID := r.PathValue("versionId")
		if !uuidPattern.MatchString(versionID) {
			writeError(w, 404, "NOT_FOUND", "version was not found")
			return
		}
		var revision int64
		var graph json.RawMessage
		var hash string
		var createdAt time.Time
		err := db.QueryRow(r.Context(), `SELECT branch_revision,graph_json,graph_hash,created_at FROM architecture_versions WHERE project_id=$1 AND id=$2`, id, versionID).Scan(&revision, &graph, &hash, &createdAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, 404, "NOT_FOUND", "version was not found")
			return
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"id": versionID, "projectId": id, "revision": revision, "architecture": graph, "graphHash": hash, "createdAt": createdAt})
	})
}
