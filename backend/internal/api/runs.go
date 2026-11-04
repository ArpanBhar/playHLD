package api

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"playhld/backend/internal/storage"
	"playhld/simulation/engine"
	"playhld/simulation/model"
)

type runInput struct {
	VersionID string         `json:"versionId"`
	Workload  model.Workload `json:"workload"`
}

func registerRunRoutes(mux *http.ServeMux, db *storage.DB) {
	mux.HandleFunc("POST /api/projects/{id}/runs", func(w http.ResponseWriter, r *http.Request) {
		if !sameOrigin(r) {
			writeError(w, 403, "ORIGIN_DENIED", "origin is not allowed")
			return
		}
		user, ok := requireActor(w, r, db, true)
		if !ok {
			return
		}
		projectID := r.PathValue("id")
		if !requireProject(w, r, db, user.ID, projectID, true) {
			return
		}
		var input runInput
		if !readStrict(w, r, 16_384, &input) {
			return
		}
		if !uuidPattern.MatchString(input.VersionID) {
			writeError(w, 422, "INVALID_VERSION", "version ID is invalid")
			return
		}
		var graphData []byte
		err := db.QueryRow(r.Context(), `SELECT graph_json FROM architecture_versions WHERE id=$1 AND project_id=$2`, input.VersionID, projectID).Scan(&graphData)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, 404, "NOT_FOUND", "version was not found")
			return
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		graph, err := model.ParseArchitecture(graphData)
		if err != nil {
			writeError(w, 422, "INVALID_VERSION", "stored version is invalid")
			return
		}
		config := model.SimulationConfig{Architecture: graph, Workload: input.Workload, SnapshotIntervalUs: 1_000_000}
		sim, err := engine.NewEngine(config)
		if err != nil {
			writeError(w, 422, "INVALID_SIMULATION", err.Error())
			return
		}
		configData, _ := json.Marshal(config)
		hash := sha256.Sum256(configData)
		configHash := hex.EncodeToString(hash[:])
		lookup := func() (string, json.RawMessage, time.Time, error) {
			var id string
			var result json.RawMessage
			var createdAt time.Time
			err := db.QueryRow(r.Context(), `SELECT id,result_json,created_at FROM simulation_runs WHERE project_id=$1 AND version_id=$2 AND config_hash=$3 AND engine_version=$4 AND provenance='server_native'`, projectID, input.VersionID, configHash, engine.EngineVersion).Scan(&id, &result, &createdAt)
			return id, result, createdAt, err
		}
		if existingID, existingResult, createdAt, err := lookup(); err == nil {
			writeJSON(w, 200, map[string]any{"id": existingID, "projectId": projectID, "versionId": input.VersionID, "engineVersion": engine.EngineVersion, "configHash": configHash, "provenance": "server_native", "workload": input.Workload, "result": existingResult, "createdAt": createdAt})
			return
		} else if !errors.Is(err, sql.ErrNoRows) {
			writeDatabaseError(w)
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
		workloadData, _ := json.Marshal(input.Workload)
		resultData, _ := json.Marshal(result)
		summaryData, _ := json.Marshal(result.Summary)
		var runID string
		var createdAt time.Time
		err = db.QueryRow(r.Context(), `INSERT INTO simulation_runs(project_id,version_id,engine_version,config_hash,provenance,summary_json,workload_json,result_json) VALUES($1,$2,$3,$4,'server_native',$5,$6,$7) ON CONFLICT DO NOTHING RETURNING id,created_at`, projectID, input.VersionID, engine.EngineVersion, configHash, summaryData, workloadData, resultData).Scan(&runID, &createdAt)
		if errors.Is(err, sql.ErrNoRows) {
			existingID, existingResult, existingAt, lookupErr := lookup()
			if lookupErr != nil {
				writeDatabaseError(w)
				return
			}
			writeJSON(w, 200, map[string]any{"id": existingID, "projectId": projectID, "versionId": input.VersionID, "engineVersion": engine.EngineVersion, "configHash": configHash, "provenance": "server_native", "workload": input.Workload, "result": existingResult, "createdAt": existingAt})
			return
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 201, map[string]any{"id": runID, "projectId": projectID, "versionId": input.VersionID, "engineVersion": engine.EngineVersion, "configHash": configHash, "provenance": "server_native", "workload": input.Workload, "result": result, "createdAt": createdAt})
	})
	mux.HandleFunc("GET /api/projects/{id}/runs", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		projectID := r.PathValue("id")
		if !requireProject(w, r, db, user.ID, projectID, false) {
			return
		}
		rows, err := db.Query(r.Context(), `SELECT id,version_id,engine_version,config_hash,provenance,summary_json,workload_json,created_at FROM simulation_runs WHERE project_id=$1 ORDER BY created_at DESC,id DESC LIMIT 100`, projectID)
		if err != nil {
			writeDatabaseError(w)
			return
		}
		defer rows.Close()
		items := make([]map[string]any, 0)
		for rows.Next() {
			var id, versionID, engineVersion, configHash, provenance string
			var summary, workload json.RawMessage
			var createdAt time.Time
			if err = rows.Scan(&id, &versionID, &engineVersion, &configHash, &provenance, &summary, &workload, &createdAt); err != nil {
				writeDatabaseError(w)
				return
			}
			items = append(items, map[string]any{"id": id, "versionId": versionID, "engineVersion": engineVersion, "configHash": configHash, "provenance": provenance, "summary": summary, "workload": workload, "createdAt": createdAt})
		}
		if rows.Err() != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"items": items})
	})
	mux.HandleFunc("GET /api/projects/{id}/runs/{runId}", func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireActor(w, r, db, false)
		if !ok {
			return
		}
		projectID := r.PathValue("id")
		if !requireProject(w, r, db, user.ID, projectID, false) {
			return
		}
		runID := r.PathValue("runId")
		if !uuidPattern.MatchString(runID) {
			writeError(w, 404, "NOT_FOUND", "run was not found")
			return
		}
		var versionID, engineVersion, configHash, provenance string
		var workload, result json.RawMessage
		var createdAt time.Time
		err := db.QueryRow(r.Context(), `SELECT version_id,engine_version,config_hash,provenance,workload_json,result_json,created_at FROM simulation_runs WHERE id=$1 AND project_id=$2`, runID, projectID).Scan(&versionID, &engineVersion, &configHash, &provenance, &workload, &result, &createdAt)
		if errors.Is(err, sql.ErrNoRows) {
			writeError(w, 404, "NOT_FOUND", "run was not found")
			return
		}
		if err != nil {
			writeDatabaseError(w)
			return
		}
		writeJSON(w, 200, map[string]any{"id": runID, "projectId": projectID, "versionId": versionID, "engineVersion": engineVersion, "configHash": configHash, "provenance": provenance, "workload": workload, "result": result, "createdAt": createdAt})
	})
}
