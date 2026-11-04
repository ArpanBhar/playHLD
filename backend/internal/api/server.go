package api

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"playhld/backend/internal/storage"
	"playhld/simulation/engine"
	"playhld/simulation/model"
)

type errorEnvelope struct {
	Error struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"requestId,omitempty"`
	} `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	var body errorEnvelope
	body.Error.Code = code
	body.Error.Message = message
	body.Error.RequestID = w.Header().Get("X-Request-ID")
	writeJSON(w, status, body)
}

type Options struct {
	DB                     *storage.DB
	SecureCookies          bool
	EnableNativeSimulation bool
}

// NewServer returns the current bootstrap API. Native simulation is disabled by
// default in production until identity and job admission limits are in place.
func NewServer(enableNativeSimulation bool) http.Handler {
	return NewServerWithOptions(Options{EnableNativeSimulation: enableNativeSimulation})
}

func NewServerWithOptions(options Options) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /api/catalog", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"catalogVersion": "0.3.0",
			"engineVersion":  engine.EngineVersion,
			"components": []map[string]string{
				{"kind": "source", "label": "Source"},
				{"kind": "load_balancer", "label": "Load balancer"},
				{"kind": "api", "label": "API service"},
				{"kind": "worker", "label": "Worker"},
				{"kind": "cache", "label": "Cache"},
				{"kind": "cdn", "label": "CDN"},
				{"kind": "database", "label": "Database"},
				{"kind": "object_storage", "label": "Object storage"},
				{"kind": "message_queue", "label": "Message queue"},
			},
		})
	})
	registerChallengeRoutes(mux)
	if options.DB != nil {
		registerAuthRoutes(mux, options.DB, options.SecureCookies)
		registerDesignRoutes(mux, options.DB)
		registerPracticeAttemptRoutes(mux, options.DB)
		registerProjectRoutes(mux, options.DB)
		registerRunRoutes(mux, options.DB)
		mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
			if err := options.DB.Ping(r.Context()); err != nil {
				writeError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database is unavailable")
				return
			}
			writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
		})
	}
	if options.EnableNativeSimulation {
		mux.HandleFunc("POST /api/simulations", func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get("Content-Type") != "application/json" {
				writeError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "send application/json")
				return
			}
			r.Body = http.MaxBytesReader(w, r.Body, model.MaxPayloadBytes)
			data, err := io.ReadAll(r.Body)
			if err != nil {
				var maxErr *http.MaxBytesError
				if errors.As(err, &maxErr) {
					writeError(w, http.StatusRequestEntityTooLarge, "PAYLOAD_TOO_LARGE", "simulation config exceeds 1 MiB")
				} else {
					writeError(w, http.StatusBadRequest, "READ_FAILED", "could not read request")
				}
				return
			}
			decoder := json.NewDecoder(bytes.NewReader(data))
			decoder.DisallowUnknownFields()
			var config model.SimulationConfig
			if err = decoder.Decode(&config); err != nil {
				writeError(w, http.StatusBadRequest, "INVALID_JSON", err.Error())
				return
			}
			if decoder.Decode(new(any)) != io.EOF {
				writeError(w, http.StatusBadRequest, "INVALID_JSON", "multiple JSON values")
				return
			}
			sim, err := engine.NewEngine(config)
			if err != nil {
				writeError(w, http.StatusUnprocessableEntity, "INVALID_SIMULATION", err.Error())
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
			defer cancel()
			result, err := sim.Run(ctx)
			if err != nil {
				if errors.Is(err, context.DeadlineExceeded) {
					writeError(w, http.StatusServiceUnavailable, "SIMULATION_TIMEOUT", "simulation exceeded its time budget")
				} else {
					writeError(w, http.StatusUnprocessableEntity, "SIMULATION_FAILED", err.Error())
				}
				return
			}
			writeJSON(w, http.StatusOK, result)
		})
	}
	return withRequestID(mux)
}

func withRequestID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		bytes := make([]byte, 12)
		if _, err := rand.Read(bytes); err != nil {
			http.Error(w, "request ID unavailable", http.StatusInternalServerError)
			return
		}
		w.Header().Set("X-Request-ID", hex.EncodeToString(bytes))
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}
