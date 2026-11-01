package challenges

import (
	"context"
	"testing"

	"systemforge/simulation/engine"
	"systemforge/simulation/goals"
	"systemforge/simulation/model"
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
