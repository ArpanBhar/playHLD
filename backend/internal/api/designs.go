package api

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"systemforge/simulation/model"
)

var uuidPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

type designInput struct {
	Name         string          `json:"name"`
	Architecture json.RawMessage `json:"architecture"`
}
type updateInput struct {
	Name            string          `json:"name"`
	Architecture    json.RawMessage `json:"architecture"`
	ExpectedVersion int64           `json:"expectedVersion"`
}
type designResponse struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	Architecture json.RawMessage `json:"architecture"`
	Version      int64           `json:"version"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}
type designSummary struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Version   int64     `json:"version"`
	UpdatedAt time.Time `json:"updatedAt"`
}

func validatedDesign(w http.ResponseWriter, name string, raw json.RawMessage) (string, []byte, bool) {
	name = strings.TrimSpace(name)
	if utf8.RuneCountInString(name) < 1 || utf8.RuneCountInString(name) > 120 {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_NAME", "name must contain 1–120 characters")
		return "", nil, false
	}
	graph, err := model.ParseArchitecture(raw)
	if err != nil {
		writeError(w, http.StatusUnprocessableEntity, "INVALID_ARCHITECTURE", err.Error())
		return "", nil, false
	}
	data, err := json.Marshal(graph)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL", "could not encode graph")
		return "", nil, false
	}
	return name, data, true
}

func writeDatabaseError(w http.ResponseWriter) {
	writeError(w, http.StatusInternalServerError, "DATABASE_ERROR", "could not complete request")
}

func registerDesignRoutes(mux *http.ServeMux, db *pgxpool.Pool) {
	mux.HandleFunc("POST /api/designs", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		var input designInput
		if !readStrict(w, r, model.MaxPayloadBytes, &input) {
			return
		}
		name, graph, ok := validatedDesign(w, input.Name, input.Architecture)
		if !ok {
			return
		}
		var design designResponse
		design.Name = name
		design.Architecture = graph
		err := db.QueryRow(r.Context(), `INSERT INTO designs(owner_id,name,graph_json) VALUES($1,$2,$3) RETURNING id::text,version,updated_at`, user.ID, name, graph).Scan(&design.ID, &design.Version, &design.UpdatedAt)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, http.StatusCreated, design)
	})

	mux.HandleFunc("GET /api/designs", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		limit := 20
		if raw := r.URL.Query().Get("limit"); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil || value < 1 || value > 50 {
				writeError(w, http.StatusBadRequest, "INVALID_LIMIT", "limit must be 1–50")
				return
			}
			limit = value
		}
		cursor := r.URL.Query().Get("cursor")
		var rows pgx.Rows
		var err error
		if cursor == "" {
			rows, err = db.Query(r.Context(), `SELECT id::text,name,version,updated_at FROM designs WHERE owner_id=$1 ORDER BY updated_at DESC,id DESC LIMIT $2`, user.ID, limit+1)
		} else {
			decoded, decodeErr := base64.RawURLEncoding.DecodeString(cursor)
			parts := strings.SplitN(string(decoded), "|", 2)
			if decodeErr != nil || len(parts) != 2 || !uuidPattern.MatchString(parts[1]) {
				writeError(w, http.StatusBadRequest, "INVALID_CURSOR", "cursor is invalid")
				return
			}
			instant, timeErr := time.Parse(time.RFC3339Nano, parts[0])
			if timeErr != nil {
				writeError(w, http.StatusBadRequest, "INVALID_CURSOR", "cursor is invalid")
				return
			}
			rows, err = db.Query(r.Context(), `SELECT id::text,name,version,updated_at FROM designs WHERE owner_id=$1 AND (updated_at,id)<($2,$3) ORDER BY updated_at DESC,id DESC LIMIT $4`, user.ID, instant, parts[1], limit+1)
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer rows.Close()
		items := make([]designSummary, 0, limit+1)
		for rows.Next() {
			var item designSummary
			if err := rows.Scan(&item.ID, &item.Name, &item.Version, &item.UpdatedAt); err != nil {
				writeDatabaseError(w)
				return
			}
			items = append(items, item)
		}
		if rows.Err() != nil {
			writeDatabaseError(w)
			return
		}
		next := ""
		if len(items) > limit {
			items = items[:limit]
			last := items[len(items)-1]
			next = base64.RawURLEncoding.EncodeToString([]byte(last.UpdatedAt.Format(time.RFC3339Nano) + "|" + last.ID))
		}
		writeJSON(w, http.StatusOK, map[string]any{"items": items, "nextCursor": next})
	})

	mux.HandleFunc("GET /api/designs/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !uuidPattern.MatchString(id) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "design not found")
			return
		}
		var design designResponse
		err := db.QueryRow(r.Context(), `SELECT id::text,name,graph_json,version,updated_at FROM designs WHERE id=$1 AND owner_id=$2`, id, user.ID).Scan(&design.ID, &design.Name, &design.Architecture, &design.Version, &design.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "design not found")
			return
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, http.StatusOK, design)
	})

	mux.HandleFunc("PUT /api/designs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !uuidPattern.MatchString(id) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "design not found")
			return
		}
		var input updateInput
		if !readStrict(w, r, model.MaxPayloadBytes, &input) {
			return
		}
		if input.ExpectedVersion < 1 {
			writeError(w, http.StatusUnprocessableEntity, "INVALID_VERSION", "expectedVersion must be positive")
			return
		}
		name, graph, ok := validatedDesign(w, input.Name, input.Architecture)
		if !ok {
			return
		}
		var design designResponse
		design.ID = id
		design.Name = name
		design.Architecture = graph
		err := db.QueryRow(r.Context(), `UPDATE designs SET name=$1,graph_json=$2,version=version+1,updated_at=now() WHERE id=$3 AND owner_id=$4 AND version=$5 RETURNING version,updated_at`, name, graph, id, user.ID, input.ExpectedVersion).Scan(&design.Version, &design.UpdatedAt)
		if errors.Is(err, pgx.ErrNoRows) {
			var exists bool
			if queryErr := db.QueryRow(r.Context(), `SELECT EXISTS(SELECT 1 FROM designs WHERE id=$1 AND owner_id=$2)`, id, user.ID).Scan(&exists); queryErr != nil {
				writeDatabaseError(w)
				return
			}
			if !exists {
				writeError(w, http.StatusNotFound, "NOT_FOUND", "design not found")
			} else {
				writeError(w, http.StatusConflict, "VERSION_CONFLICT", "reload design before saving")
			}
			return
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, http.StatusOK, design)
	})

	mux.HandleFunc("DELETE /api/designs/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, http.StatusForbidden, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		id := r.PathValue("id")
		if !uuidPattern.MatchString(id) {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "design not found")
			return
		}
		command, err := db.Exec(r.Context(), `DELETE FROM designs WHERE id=$1 AND owner_id=$2`, id, user.ID)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		if command.RowsAffected() == 0 {
			writeError(w, http.StatusNotFound, "NOT_FOUND", "design not found")
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
