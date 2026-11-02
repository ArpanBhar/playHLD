package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"playhld/simulation/engine"
)

func TestNativeSimulationFixture(t *testing.T) {
	input, err := os.ReadFile("../../../schemas/v3/fixtures/basic-simulation.json")
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(http.MethodPost, "/api/simulations", bytes.NewReader(input))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewServer(true).ServeHTTP(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("unexpected status %d: %s", response.Code, response.Body.String())
	}
	var result engine.Result
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Summary.Generated != 15_000 || result.Summary.Failed != 0 || result.Summary.Succeeded != 15_000 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
}

func TestNativeSimulationIsOptIn(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/simulations", nil)
	response := httptest.NewRecorder()
	NewServer(false).ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", response.Code)
	}
}

func TestUnknownConfigFieldIsRejected(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/simulations", bytes.NewBufferString(`{"architecture":{"schemaVersion":2,"nodes":[],"edges":[]},"workload":{"schemaVersion":1,"seed":"1","entryNodeId":"x","durationUs":1,"arrivalRateRps":1},"snapshotIntervalUs":1,"clientScore":100}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewServer(true).ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d: %s", response.Code, response.Body.String())
	}
}

func TestErrorCarriesRequestID(t *testing.T) {
	request := httptest.NewRequest(http.MethodPost, "/api/simulations", bytes.NewBufferString(`{}`))
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	NewServer(true).ServeHTTP(response, request)
	var body struct {
		Error struct {
			RequestID string `json:"requestId"`
		} `json:"error"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body.Error.RequestID == "" || body.Error.RequestID != response.Header().Get("X-Request-ID") {
		t.Fatalf("missing request ID: %s", response.Body.String())
	}
	if response.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Fatal("missing nosniff header")
	}
}
