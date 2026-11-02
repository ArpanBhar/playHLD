package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"playhld/content/challenges"
	"playhld/simulation/engine"
	"playhld/simulation/goals"
	"playhld/simulation/model"
)

type practiceInput struct {
	Architecture json.RawMessage `json:"architecture"`
}

func registerChallengeRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/practice/challenges", func(w http.ResponseWriter, _ *http.Request) {
		items, err := challenges.All()
		if err != nil {
			writeError(w, 500, "CONTENT_ERROR", "challenges are unavailable")
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/practice/challenges/{id}", func(w http.ResponseWriter, r *http.Request) {
		item, found, err := challenges.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, 500, "CONTENT_ERROR", "challenge is unavailable")
			return
		}
		if !found {
			writeError(w, 404, "NOT_FOUND", "challenge was not found")
			return
		}
		writeJSON(w, 200, item)
	})
}

func registerPracticeAttemptRoutes(mux *http.ServeMux, db *pgxpool.Pool) {
	mux.HandleFunc("GET /api/practice/progress/{id}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		challenge, found, err := challenges.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, 500, "CONTENT_ERROR", "challenge is unavailable")
			return
		}
		if !found {
			writeError(w, 404, "NOT_FOUND", "challenge was not found")
			return
		}
		var count int
		if err := db.QueryRow(r.Context(), `SELECT count(*) FROM practice_attempts WHERE user_id=$1 AND challenge_id=$2`, user.ID, challenge.ID).Scan(&count); err != nil {
			writeDatabaseError(w)
			return
		}
		var id string
		var resultData []byte
		var version int
		var createdAt time.Time
		err = db.QueryRow(r.Context(), `SELECT id::text,challenge_version,result_json,created_at FROM practice_attempts WHERE user_id=$1 AND challenge_id=$2 ORDER BY created_at DESC,id DESC LIMIT 1`, user.ID, challenge.ID).Scan(&id, &version, &resultData, &createdAt)
		if errors.Is(err, pgx.ErrNoRows) {
			writeJSON(w, 200, map[string]any{"challengeId": challenge.ID, "attemptCount": 0, "latest": nil})
			return
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		var result engine.Result
		if err := json.Unmarshal(resultData, &result); err != nil {
			writeDatabaseError(w)
			return
		}
		versioned, found, err := challenges.Get(challenge.ID)
		if err != nil || !found || versioned.Version != version {
			writeJSON(w, 200, map[string]any{"challengeId": challenge.ID, "attemptCount": count, "latest": map[string]any{"id": id, "challengeVersion": version, "createdAt": createdAt, "goals": nil}})
			return
		}
		outcomes, err := goals.Evaluate(result, versioned.Workload, versioned.Goals)
		if err != nil {
			writeError(w, 500, "CONTENT_ERROR", "challenge goals are invalid")
			return
		}
		completed := true
		for _, outcome := range outcomes {
			if outcome.Status != "met" {
				completed = false
			}
		}
		writeJSON(w, 200, map[string]any{"challengeId": challenge.ID, "attemptCount": count, "latest": map[string]any{"id": id, "challengeVersion": version, "createdAt": createdAt, "goals": outcomes, "completed": completed}})
	})
	mux.HandleFunc("POST /api/practice/challenges/{id}/attempts", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, 403, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		challenge, found, err := challenges.Get(r.PathValue("id"))
		if err != nil {
			writeError(w, 500, "CONTENT_ERROR", "challenge is unavailable")
			return
		}
		if !found {
			writeError(w, 404, "NOT_FOUND", "challenge was not found")
			return
		}
		var input practiceInput
		if !readStrict(w, r, model.MaxPayloadBytes, &input) {
			return
		}
		graph, err := model.ParseArchitecture(input.Architecture)
		if err != nil {
			writeError(w, 422, "INVALID_ARCHITECTURE", err.Error())
			return
		}
		if err := challenges.ValidateGraph(challenge, graph); err != nil {
			writeError(w, 422, "RESOURCE_LIMIT", err.Error())
			return
		}
		config := model.SimulationConfig{Architecture: graph, Workload: challenge.Workload, SnapshotIntervalUs: 1_000_000}
		sim, err := engine.NewEngine(config)
		if err != nil {
			writeError(w, 422, "INVALID_SIMULATION", err.Error())
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
		defer cancel()
		result, err := sim.Run(ctx)
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				writeError(w, 503, "SIMULATION_TIMEOUT", "simulation exceeded its time budget")
			} else {
				writeError(w, 422, "SIMULATION_FAILED", err.Error())
			}
			return
		}
		outcomes, err := goals.Evaluate(result, challenge.Workload, challenge.Goals)
		if err != nil {
			writeError(w, 500, "CONTENT_ERROR", "challenge goals are invalid")
			return
		}
		completed := true
		for _, outcome := range outcomes {
			if outcome.Status != "met" {
				completed = false
			}
		}
		graphJSON, _ := json.Marshal(graph)
		resultJSON, _ := json.Marshal(result)
		challengeJSON, _ := json.Marshal(challenge)
		tx, err := db.Begin(ctx)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer tx.Rollback(ctx)
		_, err = tx.Exec(ctx, `INSERT INTO challenges(id,version,content_json,published_at) VALUES($1,$2,$3,now()) ON CONFLICT(id,version) DO NOTHING`, challenge.ID, challenge.Version, challengeJSON)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		var id string
		err = tx.QueryRow(ctx, `INSERT INTO practice_attempts(user_id,challenge_id,challenge_version,architecture_json,result_json,engine_version,provenance) VALUES($1,$2,$3,$4,$5,$6,'server_native') RETURNING id::text`, user.ID, challenge.ID, challenge.Version, graphJSON, resultJSON, engine.EngineVersion).Scan(&id)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		if err = tx.Commit(ctx); err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 201, map[string]any{"id": id, "challengeId": challenge.ID, "challengeVersion": challenge.Version, "result": result, "goals": outcomes, "completed": completed, "provenance": "server_native"})
	})
	mux.HandleFunc("GET /api/practice/attempts", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		rows, err := db.Query(r.Context(), `SELECT id::text,challenge_id,challenge_version,engine_version,created_at,result_json FROM practice_attempts WHERE user_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, user.ID)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, challengeID, engineVersion string
			var version int
			var createdAt time.Time
			var result json.RawMessage
			if err := rows.Scan(&id, &challengeID, &version, &engineVersion, &createdAt, &result); err != nil {
				writeDatabaseError(w)
				return
			}
			items = append(items, map[string]any{"id": id, "challengeId": challengeID, "challengeVersion": version, "engineVersion": engineVersion, "createdAt": createdAt, "result": result, "provenance": "server_native"})
		}
		if rows.Err() != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
}
