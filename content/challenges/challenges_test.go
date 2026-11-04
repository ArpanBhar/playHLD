package challenges

import (
	"context"
	"testing"

	"playhld/simulation/engine"
	"playhld/simulation/goals"
	"playhld/simulation/model"
)

func TestProblemsStartEmptyAndHaveFeasibleBudgetedSolution(t *testing.T) {
	all, err := All()
	if err != nil {
		t.Fatal(err)
	}
	if len(all) < 3 {
		t.Fatalf("expected multiple problems, got %d", len(all))
	}
	for _, problem := range all {
		t.Run(problem.ID, func(t *testing.T) {
			if len(problem.Starter.Nodes) != 2 || len(problem.Starter.Edges) != 0 || problem.Starter.Nodes[0].Kind != "source" || problem.Starter.Nodes[1].Kind != "api" {
				t.Fatalf("starter must be client and server only: %+v", problem.Starter)
			}
			graph := problem.Solution.Architecture
			if problem.Solution.Explanation == "" {
				t.Fatal("solution needs an explanation")
			}
			if err := ValidateGraph(problem, graph); err != nil {
				t.Fatal(err)
			}
			sim, err := engine.NewEngine(model.SimulationConfig{Architecture: graph, Workload: problem.Workload, SnapshotIntervalUs: 1_000_000})
			if err != nil {
				t.Fatal(err)
			}
			result, err := sim.Run(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			outcomes, err := goals.Evaluate(result, problem.Workload, problem.Goals)
			if err != nil {
				t.Fatal(err)
			}
			for _, outcome := range outcomes {
				if outcome.Status != "met" {
					t.Fatalf("published solution failed %s: %+v, summary %+v", outcome.ID, outcomes, result.Summary)
				}
			}
			if problem.ID == "background-queue" {
				traversals := result.Snapshots[len(result.Snapshots)-1].EdgeTraversals
				if traversals["solution-3"] == 0 || traversals["solution-worker-b"] == 0 {
					t.Fatalf("queue did not dispatch jobs to both visible workers: %+v", traversals)
				}
			}
			overBudget := problem.Starter
			overBudget.Nodes = append([]model.Node(nil), problem.Starter.Nodes...)
			overBudget.Nodes[1].Config.ProcessingSlots = problem.ResourceBudget.MaxProcessingSlotsPerNode + 1
			if err := ValidateGraph(problem, overBudget); err == nil {
				t.Fatal("over-budget graph accepted")
			}
			tampered := problem.Starter
			tampered.Nodes = append([]model.Node(nil), problem.Starter.Nodes...)
			tampered.Nodes[1].Config.MeanServiceTimeUs = 1
			if err := ValidateGraph(problem, tampered); err == nil {
				t.Fatal("faster imported server accepted")
			}
		})
	}
}

func TestDifferentTopologyCanMeetSameProblem(t *testing.T) {
	problem, found, err := Get("service-capacity")
	if err != nil || !found {
		t.Fatal("problem unavailable", err)
	}
	graph := problem.Starter
	graph.Nodes = append([]model.Node(nil), graph.Nodes...)
	graph.Nodes[1].Config.ProcessingSlots = 2
	second := graph.Nodes[1]
	second.ID = "api-alt"
	second.Position.Y = 320
	lbSpec := problem.ComponentSpecs["load_balancer"]
	lb := model.Node{ID: "balancer", Kind: "load_balancer", Position: model.Position{X: 280, Y: 170}, Config: model.NodeConfig{ProcessingSlots: 1, MeanServiceTimeUs: lbSpec.MeanServiceTimeUs, QueueLimit: lbSpec.QueueLimit, RoutingPolicy: "round_robin"}}
	graph.Nodes = append(graph.Nodes, lb, second)
	graph.Edges = []model.Edge{{ID: "entry", Source: "source", Target: "balancer", Route: "always"}, {ID: "first", Source: "balancer", Target: "api", Route: "always"}, {ID: "second", Source: "balancer", Target: "api-alt", Route: "always"}}
	if err := ValidateGraph(problem, graph); err != nil {
		t.Fatal(err)
	}
	sim, err := engine.NewEngine(model.SimulationConfig{Architecture: graph, Workload: problem.Workload, SnapshotIntervalUs: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	result, err := sim.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	outcomes, err := goals.Evaluate(result, problem.Workload, problem.Goals)
	if err != nil {
		t.Fatal(err)
	}
	for _, outcome := range outcomes {
		if outcome.Status != "met" {
			t.Fatalf("alternate topology failed: %+v", outcomes)
		}
	}
}

func TestFunctionalURLRequestsRequirePersistenceAndMeasureCache(t *testing.T) {
	problem, found, err := Get("design-url-shortener")
	if err != nil || !found {
		t.Fatal("URL problem unavailable", err)
	}
	run := func(graph model.Architecture) engine.Result {
		t.Helper()
		sim, err := engine.NewEngine(model.SimulationConfig{Architecture: graph, Workload: problem.Workload, SnapshotIntervalUs: 1_000_000})
		if err != nil {
			t.Fatal(err)
		}
		result, err := sim.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	apiOnly := problem.Starter
	apiOnly.Edges = []model.Edge{{ID: "entry", Source: "source", Target: "api", Route: "always"}}
	broken := run(apiOnly)
	if broken.Summary.Correct != 0 || broken.Summary.Incorrect == 0 || broken.Summary.Succeeded != 0 {
		t.Fatalf("API-only architecture produced functional success: %+v", broken.Summary)
	}
	working := run(problem.Solution.Architecture)
	if working.Summary.Correct != working.Summary.Generated || working.Summary.Incorrect != 0 || len(working.FunctionalTrace) == 0 {
		t.Fatalf("working architecture did not return correct outputs: %+v", working.Summary)
	}
	final := working.Snapshots[len(working.Snapshots)-1]
	cache := final.NodeMetrics["cache"]
	if cache.CacheLookups == 0 || cache.CacheHits == 0 || cache.CacheHits >= cache.CacheLookups {
		t.Fatalf("cache needs measured hits and misses: %+v", cache)
	}
	if final.EdgeTraversals["a-read"] == 0 || final.EdgeTraversals["a-write"] == 0 || final.EdgeTraversals["cache-miss"] == 0 {
		t.Fatalf("read, write and cache-miss paths were not all traversed: %+v", final.EdgeTraversals)
	}
	tiny := problem.Solution.Architecture
	tiny.Nodes = append([]model.Node(nil), tiny.Nodes...)
	for i := range tiny.Nodes {
		if tiny.Nodes[i].ID == "cache" {
			tiny.Nodes[i].Config.CacheCapacity = 1
		}
	}
	tinyResult := run(tiny)
	tinyCache := tinyResult.Snapshots[len(tinyResult.Snapshots)-1].NodeMetrics["cache"]
	if tinyCache.CacheHits >= cache.CacheHits || tinyResult.Summary.Correct != tinyResult.Summary.Generated {
		t.Fatalf("shrinking cache should lower hit count without changing correctness: normal=%+v tiny=%+v", cache, tinyCache)
	}
	first, err := engine.NewEngine(model.SimulationConfig{Architecture: problem.Solution.Architecture, Workload: problem.Workload, SnapshotIntervalUs: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Run(context.Background()); err != nil {
		t.Fatal(err)
	}
	state, err := first.ExportFunctionalState()
	if err != nil {
		t.Fatal(err)
	}
	second, err := engine.NewEngine(model.SimulationConfig{Architecture: problem.Solution.Architecture, Workload: problem.Workload, SnapshotIntervalUs: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	if err := second.ImportFunctionalState(state); err != nil {
		t.Fatal(err)
	}
	warm, err := second.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	warmCache := warm.Snapshots[len(warm.Snapshots)-1].NodeMetrics["cache"]
	if warmCache.CacheHits <= cache.CacheHits || warm.Summary.Correct != warm.Summary.Generated {
		t.Fatalf("live state transfer should preserve warm cache and correctness: cold=%+v warm=%+v summary=%+v trace=%+v", cache, warmCache, warm.Summary, warm.FunctionalTrace)
	}
}

func TestAsyncJudgeSeparatesAcknowledgmentAndJobCompletion(t *testing.T) {
	problem, found, err := Get("design-online-judge")
	if err != nil || !found {
		t.Fatal("judge problem unavailable", err)
	}
	run := func(graph model.Architecture) engine.Result {
		t.Helper()
		sim, err := engine.NewEngine(model.SimulationConfig{Architecture: graph, Workload: problem.Workload, SnapshotIntervalUs: 1_000_000})
		if err != nil {
			t.Fatal(err)
		}
		result, err := sim.Run(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	working := run(problem.Solution.Architecture)
	if working.Summary.JobsEnqueued != working.Summary.Generated || working.Summary.JobsCompleted != working.Summary.Generated || working.Summary.Correct != working.Summary.Generated {
		t.Fatalf("judge solution did not acknowledge and persist all jobs: %+v", working.Summary)
	}
	if working.Summary.P95LatencyUs == nil || *working.Summary.P95LatencyUs >= 20_000 {
		t.Fatalf("acknowledgment waited for 20 ms worker service: %+v", working.Summary)
	}
	slow := problem.Solution.Architecture
	slow.Nodes = append([]model.Node(nil), slow.Nodes...)
	for i := range slow.Nodes {
		if slow.Nodes[i].Kind == "worker" {
			slow.Nodes[i].Config.ProcessingSlots = 1
			slow.Nodes[i].Config.MaxInFlight = 1
		}
	}
	congested := run(slow)
	queue := congested.Snapshots[len(congested.Snapshots)-1].NodeMetrics["mq"]
	if queue.MaxQueueDepth == 0 || congested.Summary.JobsCompleted >= working.Summary.JobsCompleted || congested.Summary.JobsUnfinished == 0 {
		t.Fatalf("worker bottleneck did not produce queue backlog and unfinished jobs: summary=%+v queue=%+v", congested.Summary, queue)
	}
}

func TestComponentFailuresPropagateToClientAndAsyncJobs(t *testing.T) {
	url, found, err := Get("design-url-shortener")
	if err != nil || !found {
		t.Fatal("URL problem unavailable", err)
	}
	urlGraph := url.Solution.Architecture
	urlGraph.Nodes = append([]model.Node(nil), urlGraph.Nodes...)
	for i := range urlGraph.Nodes {
		if urlGraph.Nodes[i].ID == "db" {
			urlGraph.Nodes[i].Config.FailureRate = 1
		}
	}
	urlSim, err := engine.NewEngine(model.SimulationConfig{Architecture: urlGraph, Workload: url.Workload, SnapshotIntervalUs: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	urlResult, err := urlSim.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if urlResult.Summary.Failed == 0 || urlResult.Summary.Correct == urlResult.Summary.Generated || urlResult.Snapshots[len(urlResult.Snapshots)-1].NodeMetrics["db"].Failures == 0 {
		t.Fatalf("datastore failures did not reach URL results: %+v", urlResult.Summary)
	}
	judge, found, err := Get("design-online-judge")
	if err != nil || !found {
		t.Fatal("judge problem unavailable", err)
	}
	judgeGraph := judge.Solution.Architecture
	judgeGraph.Nodes = append([]model.Node(nil), judgeGraph.Nodes...)
	for i := range judgeGraph.Nodes {
		if judgeGraph.Nodes[i].Kind == "worker" {
			judgeGraph.Nodes[i].Config.FailureRate = 1
		}
	}
	judgeSim, err := engine.NewEngine(model.SimulationConfig{Architecture: judgeGraph, Workload: judge.Workload, SnapshotIntervalUs: 1_000_000})
	if err != nil {
		t.Fatal(err)
	}
	judgeResult, err := judgeSim.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if judgeResult.Summary.Succeeded != judgeResult.Summary.Generated || judgeResult.Summary.JobsFailed != judgeResult.Summary.Generated || judgeResult.Summary.JobsCompleted != 0 {
		t.Fatalf("worker failures should preserve client acknowledgments but fail jobs: %+v", judgeResult.Summary)
	}
}
