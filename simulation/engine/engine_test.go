package engine

import (
	"context"
	"encoding/json"
	"math"
	"reflect"
	"testing"

	"systemforge/simulation/model"
)

func fixture(rate, duration, drain, service int64, queue int) model.SimulationConfig {
	return model.SimulationConfig{
		Architecture: model.Architecture{SchemaVersion: 3, Nodes: []model.Node{
			{ID: "source", Kind: "source", Position: model.Position{X: 0, Y: 0}},
			{ID: "api", Kind: "api", Position: model.Position{X: 200, Y: 0}, Config: model.NodeConfig{ProcessingSlots: 1, MeanServiceTimeUs: service, QueueLimit: queue}},
		}, Edges: []model.Edge{{ID: "e1", Source: "source", Target: "api", Route: "always"}}},
		Workload:           model.Workload{SchemaVersion: 1, Seed: "42", EntryNodeID: "source", DurationUs: duration, DrainTimeoutUs: drain, ArrivalRateRps: rate},
		SnapshotIntervalUs: 1_000_000,
	}
}

func TestNoOverloadTimeline(t *testing.T) {
	e, err := NewEngine(fixture(1, 2_000_000, 1_000_000, 500_000, 2))
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Generated != 2 || result.Summary.Succeeded != 2 || result.Summary.Failed != 0 || result.Summary.Unfinished != 0 {
		t.Fatalf("unexpected accounting: %+v", result.Summary)
	}
	if result.Summary.P95LatencyUs == nil || *result.Summary.P95LatencyUs != 500_000 {
		t.Fatalf("unexpected p95: %+v", result.Summary.P95LatencyUs)
	}
	if len(result.Snapshots) < 2 || result.Snapshots[0].OfferedRps != 1 || result.Snapshots[0].ThroughputRps != 1 {
		t.Fatalf("bad first window: %+v", result.Snapshots)
	}
	if u := result.Snapshots[0].NodeMetrics["api"].ProcessingUtilization; u == nil || *u != 0.5 {
		t.Fatalf("bad utilization: %v", u)
	}
	if result.Snapshots[0].EdgeTraversals["e1"] != 1 || result.Snapshots[len(result.Snapshots)-1].EdgeTraversals["e1"] != result.Summary.Generated {
		t.Fatalf("edge traversal counts do not match generated requests: %+v", result.Snapshots)
	}
}

func TestInFlightLimitDoesNotAddProcessingCapacity(t *testing.T) {
	run := func(slots, inFlight int) Snapshot {
		t.Helper()
		cfg := fixture(30, 10_000_000, 1_000_000, 100_000, 20)
		cfg.Architecture.Nodes[1].Config.ProcessingSlots = slots
		cfg.Architecture.Nodes[1].Config.MaxInFlight = inFlight
		sim, err := NewEngine(cfg)
		if err != nil {
			t.Fatal(err)
		}
		result, err := sim.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return result.Snapshots[5]
	}
	limited := run(2, 2)
	buffered := run(2, 22)
	moreProcessing := run(4, 24)
	limitedBusy := *limited.NodeMetrics["api"].ProcessingUtilization
	bufferedBusy := *buffered.NodeMetrics["api"].ProcessingUtilization
	moreBusy := *moreProcessing.NodeMetrics["api"].ProcessingUtilization
	if math.Abs(limitedBusy-bufferedBusy) > 0.05 || math.Abs(limited.ThroughputRps-buffered.ThroughputRps) > 1 {
		t.Fatalf("raising in-flight limit changed processing capacity: limited=%+v buffered=%+v", limited, buffered)
	}
	if buffered.NodeMetrics["api"].QueueDepth == 0 {
		t.Fatal("higher in-flight limit did not admit waiting requests")
	}
	if moreProcessing.ThroughputRps <= buffered.ThroughputRps || moreBusy >= bufferedBusy {
		t.Fatalf("provisioning slots did not change throughput and processing utilization: buffered=%+v more=%+v", buffered, moreProcessing)
	}
}

func TestLiveBatchStopsAtOneSnapshot(t *testing.T) {
	e, err := NewEngine(fixture(2, 3_000_000, 0, 100_000, 2))
	if err != nil {
		t.Fatal(err)
	}
	frames := 0
	for {
		out, err := e.RunBatchUntilSnapshot(10_000)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Snapshots) > 1 {
			t.Fatalf("live batch emitted %d snapshots", len(out.Snapshots))
		}
		frames += len(out.Snapshots)
		if out.Done {
			if out.Result == nil || out.Result.Summary.Generated != 6 {
				t.Fatalf("bad live result: %+v", out.Result)
			}
			break
		}
	}
	if frames < 2 {
		t.Fatalf("expected incremental snapshots, got %d", frames)
	}
}

func TestApplyArchitecturePreservesInflightRequests(t *testing.T) {
	cfg := fixture(2, 3_000_000, 2_000_000, 750_000, 10)
	e, err := NewEngine(cfg)
	if err != nil {
		t.Fatal(err)
	}
	for {
		out, err := e.RunBatchUntilSnapshot(10_000)
		if err != nil {
			t.Fatal(err)
		}
		if len(out.Snapshots) > 0 {
			break
		}
	}
	updated := cfg.Architecture
	updated.Nodes = append(updated.Nodes, model.Node{ID: "added", Kind: "api", Position: model.Position{X: 400, Y: 0}, Config: model.NodeConfig{ProcessingSlots: 2, MeanServiceTimeUs: 100_000, QueueLimit: 10}})
	if err := e.ApplyArchitecture(updated); err == nil {
		t.Fatal("unconnected component should not be applied")
	}
	updated.Edges = append(updated.Edges, model.Edge{ID: "e2", Source: "api", Target: "added", Route: "always"})
	if err := e.ApplyArchitecture(updated); err != nil {
		t.Fatal(err)
	}
	result, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Generated != 6 || result.Summary.Succeeded != 6 || result.Summary.Failed != 0 {
		t.Fatalf("live topology update lost requests: %+v", result.Summary)
	}
	last := result.Snapshots[len(result.Snapshots)-1]
	if last.EdgeTraversals["e2"] == 0 || last.NodeMetrics["added"].Completions == 0 {
		t.Fatalf("new component did not receive live traffic: %+v", last)
	}
}

func TestPiecewiseTrafficCountsAndReset(t *testing.T) {
	cfg := fixture(1, 4_000_000, 1_000_000, 1, 0)
	cfg.Workload.Segments = []model.ArrivalSegment{{StartUs: 0, EndUs: 2_000_000, RateRps: 1}, {StartUs: 2_000_000, EndUs: 4_000_000, RateRps: 3}}
	e, err := NewEngine(cfg)
	if err != nil {
		t.Fatal(err)
	}
	first, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Summary.Generated != 8 || first.Summary.Succeeded != 8 {
		t.Fatalf("unexpected piecewise accounting: %+v", first.Summary)
	}
	if len(first.Snapshots) < 4 {
		t.Fatalf("expected four snapshots, got %d", len(first.Snapshots))
	}
	for i, want := range []float64{1, 1, 3, 3} {
		if got := first.Snapshots[i].OfferedRps; got != want {
			t.Fatalf("window %d offered %v, want %v", i, got, want)
		}
	}
	if err := e.Reset(); err != nil {
		t.Fatal(err)
	}
	second, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("reset changed piecewise result")
	}
}

func TestSeededPoissonAndExponentialReplay(t *testing.T) {
	cfg := fixture(20, 1_000_000, 1_000_000, 20_000, 100)
	cfg.Workload.ArrivalModel = "poisson"
	cfg.Architecture.Nodes[1].Config.ServiceTimeModel = "exponential"
	e, err := NewEngine(cfg)
	if err != nil {
		t.Fatal(err)
	}
	first, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Reset(); err != nil {
		t.Fatal(err)
	}
	second, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("seeded stochastic replay changed")
	}
	if first.Summary.Generated != 19 || first.Summary.Succeeded != 19 || first.Summary.P95LatencyUs == nil || *first.Summary.P95LatencyUs != 99598 {
		t.Fatalf("unexpected stochastic result: %+v", first.Summary)
	}
}

func TestArrivalStreamIndependentOfReadAndServiceDraws(t *testing.T) {
	base := fixture(20, 1_000_000, 1_000_000, 20_000, 100)
	base.Workload.ArrivalModel = "poisson"
	firstEngine, err := NewEngine(base)
	if err != nil {
		t.Fatal(err)
	}
	first, err := firstEngine.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	one := 1.0
	changed := base
	changed.Workload.ReadFraction = &one
	changed.Architecture.Nodes = append([]model.Node(nil), base.Architecture.Nodes...)
	changed.Architecture.Nodes[1].Config.ServiceTimeModel = "exponential"
	secondEngine, err := NewEngine(changed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := secondEngine.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Summary.Generated != second.Summary.Generated {
		t.Fatalf("arrival count changed: %d vs %d", first.Summary.Generated, second.Summary.Generated)
	}
	if len(first.Snapshots) != len(second.Snapshots) {
		t.Fatal("snapshot count changed")
	}
	for i := range first.Snapshots {
		if first.Snapshots[i].Offered != second.Snapshots[i].Offered {
			t.Fatalf("arrival count changed at snapshot %d", i)
		}
	}
}

func TestOverloadRejectsAndPreservesAccounting(t *testing.T) {
	e, err := NewEngine(fixture(4, 2_000_000, 1_000_000, 1_000_000, 1))
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Generated != 8 || result.Summary.Succeeded != 3 || result.Summary.Failed != 5 || result.Summary.Unfinished != 0 {
		t.Fatalf("unexpected accounting: %+v", result.Summary)
	}
	if result.Snapshots[0].NodeMetrics["api"].MaxQueueDepth != 1 {
		t.Fatalf("queue did not fill: %+v", result.Snapshots[0])
	}
}

func TestDeterministicReset(t *testing.T) {
	e, err := NewEngine(fixture(3, 2_000_000, 1_000_000, 400_000, 2))
	if err != nil {
		t.Fatal(err)
	}
	a, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if err := e.Reset(); err != nil {
		t.Fatal(err)
	}
	b, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(a, b) {
		aa, _ := json.Marshal(a)
		bb, _ := json.Marshal(b)
		t.Fatalf("reset differed: %s\n%s", aa, bb)
	}
}

func TestLimitAndCancellation(t *testing.T) {
	c := fixture(100_000, 300_000_000, 0, 1, 0)
	if _, err := NewEngine(c); err == nil {
		t.Fatal("expected generated request limit")
	}
	e, err := NewEngine(fixture(1, 2_000_000, 0, 1, 0))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := e.Run(ctx); err != context.Canceled {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestURLShortenerRoutingAndCache(t *testing.T) {
	service := model.NodeConfig{ProcessingSlots: 10, MeanServiceTimeUs: 100, QueueLimit: 100}
	cacheConfig := service
	cacheConfig.HitRate = 0.8
	c := model.SimulationConfig{
		Architecture: model.Architecture{SchemaVersion: 3, Nodes: []model.Node{
			{ID: "source", Kind: "source"},
			{ID: "lb", Kind: "load_balancer", Config: service},
			{ID: "api-a", Kind: "api", Config: service},
			{ID: "api-b", Kind: "api", Config: service},
			{ID: "cache", Kind: "cache", Config: cacheConfig},
			{ID: "database", Kind: "database", Config: service},
		}, Edges: []model.Edge{
			{ID: "1", Source: "source", Target: "lb", Route: "always"},
			{ID: "2", Source: "lb", Target: "api-a", Route: "always"},
			{ID: "3", Source: "lb", Target: "api-b", Route: "always"},
			{ID: "4", Source: "api-a", Target: "cache", Route: "always"},
			{ID: "5", Source: "api-b", Target: "cache", Route: "always"},
			{ID: "6", Source: "cache", Target: "database", Route: "cache_miss"},
		}},
		Workload:           model.Workload{SchemaVersion: 1, Seed: "42", EntryNodeID: "source", DurationUs: 1_000_000, DrainTimeoutUs: 100_000, ArrivalRateRps: 1_000},
		SnapshotIntervalUs: 500_000,
	}
	e, err := NewEngine(c)
	if err != nil {
		t.Fatal(err)
	}
	result, err := e.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if result.Summary.Generated != 1000 || result.Summary.Succeeded != 1000 || result.Summary.Failed != 0 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	last := result.Snapshots[len(result.Snapshots)-1]
	if last.NodeMetrics["api-a"].Completions != 500 || last.NodeMetrics["api-b"].Completions != 500 {
		t.Fatalf("round robin imbalance: %+v", last.NodeMetrics)
	}
	if last.NodeMetrics["cache"].CacheLookups != 1000 {
		t.Fatalf("missing lookups: %+v", last.NodeMetrics["cache"])
	}
	if last.NodeMetrics["database"].Completions < 150 || last.NodeMetrics["database"].Completions > 250 {
		t.Fatalf("unexpected backing-store traffic: %+v", last.NodeMetrics["database"])
	}
	if last.NodeMetrics["cache"].CacheHits+last.NodeMetrics["database"].Completions != 1000 {
		t.Fatal("hit and miss accounting does not reconcile")
	}
}
