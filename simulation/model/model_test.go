package model

import "testing"

func TestParseArchitectureRejectsUnknownFieldsAndExtraJSON(t *testing.T) {
	for _, input := range []string{
		`{"schemaVersion":3,"nodes":[],"edges":[],"secret":true}`,
		`{"schemaVersion":3,"nodes":[],"edges":[]} {"x":1}`,
		`{"schemaVersion":3,"nodes":[{"id":"api","kind":"api","position":{"x":0,"y":0},"config":{"concurrency":3,"processingSlots":3,"meanServiceTimeUs":2000,"queueLimit":10}}],"edges":[]}`,
	} {
		if _, err := ParseArchitecture([]byte(input)); err == nil {
			t.Fatalf("expected rejection: %s", input)
		}
	}
}

func TestInFlightLimitCannotExceedProcessingAndQueueCapacity(t *testing.T) {
	a := Architecture{SchemaVersion: 3, Nodes: []Node{{ID: "api", Kind: "api", Config: NodeConfig{ProcessingSlots: 2, MaxInFlight: 13, MeanServiceTimeUs: 100, QueueLimit: 10}}}}
	if issues := ValidateArchitecture(a); len(issues) == 0 {
		t.Fatal("in-flight limit exceeding processing slots plus queue was accepted")
	}
}

func TestInvalidGraphDoesNotPassExecutableValidation(t *testing.T) {
	c := SimulationConfig{Architecture: Architecture{SchemaVersion: 3, Nodes: []Node{{ID: "source", Kind: "source"}}, Edges: []Edge{{ID: "e", Source: "source", Target: "missing", Route: "always"}}}, Workload: Workload{SchemaVersion: 1, Seed: "1", EntryNodeID: "source", DurationUs: 1_000_000, ArrivalRateRps: 1}, SnapshotIntervalUs: 1_000_000}
	if len(ValidateSimulation(c)) == 0 {
		t.Fatal("dangling target accepted")
	}
}

func TestCacheRequiresMissRoute(t *testing.T) {
	c := SimulationConfig{
		Architecture: Architecture{SchemaVersion: 3, Nodes: []Node{
			{ID: "source", Kind: "source"},
			{ID: "cache", Kind: "cache", Config: NodeConfig{ProcessingSlots: 1, MeanServiceTimeUs: 100, QueueLimit: 1, HitRate: 0.8}},
		}, Edges: []Edge{{ID: "to-cache", Source: "source", Target: "cache", Route: "always"}}},
		Workload: Workload{SchemaVersion: 1, Seed: "42", EntryNodeID: "source", DurationUs: 1_000_000, ArrivalRateRps: 1}, SnapshotIntervalUs: 100_000,
	}
	issues := ValidateSimulation(c)
	found := false
	for _, issue := range issues {
		if issue.Code == "MISSING_ROUTE" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected missing cache route, got %+v", issues)
	}
}

func TestTrafficSegmentsMustCoverDuration(t *testing.T) {
	base := Workload{SchemaVersion: 1, Seed: "1", EntryNodeID: "source", DurationUs: 4_000_000, ArrivalRateRps: 1}
	valid := base
	valid.Segments = []ArrivalSegment{{StartUs: 0, EndUs: 2_000_000, RateRps: 1}, {StartUs: 2_000_000, EndUs: 4_000_000, RateRps: 3}}
	if got := ValidateWorkload(valid); len(got) != 0 {
		t.Fatalf("valid segments rejected: %+v", got)
	}
	for _, segments := range [][]ArrivalSegment{
		{{StartUs: 0, EndUs: 2_000_000, RateRps: 1}, {StartUs: 3_000_000, EndUs: 4_000_000, RateRps: 3}},
		{{StartUs: 0, EndUs: 2_000_000, RateRps: 1}, {StartUs: 1_000_000, EndUs: 4_000_000, RateRps: 3}},
		{{StartUs: 0, EndUs: 2_000_000, RateRps: 1}},
		{{StartUs: 0, EndUs: 4_000_000, RateRps: 0}},
	} {
		invalid := base
		invalid.Segments = segments
		if got := ValidateWorkload(invalid); len(got) == 0 {
			t.Fatalf("invalid segments accepted: %+v", segments)
		}
	}
}

func TestUnknownStochasticModelsAreRejected(t *testing.T) {
	w := Workload{SchemaVersion: 1, Seed: "1", EntryNodeID: "source", DurationUs: 1_000_000, ArrivalRateRps: 1, ArrivalModel: "random"}
	if len(ValidateWorkload(w)) == 0 {
		t.Fatal("unknown arrival model accepted")
	}
	a := Architecture{SchemaVersion: 3, Nodes: []Node{{ID: "api", Kind: "api", Config: NodeConfig{ProcessingSlots: 1, MeanServiceTimeUs: 1, ServiceTimeModel: "random"}}}}
	if len(ValidateArchitecture(a)) == 0 {
		t.Fatal("unknown service model accepted")
	}
}
