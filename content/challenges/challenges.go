package challenges

import (
	"embed"
	"encoding/json"
	"fmt"
	"sort"

	"playhld/simulation/goals"
	"playhld/simulation/model"
)

//go:embed *.json
var files embed.FS

type Challenge struct {
	ID             string                   `json:"id"`
	Version        int                      `json:"version"`
	Title          string                   `json:"title"`
	Difficulty     string                   `json:"difficulty"`
	Scenario       string                   `json:"scenario"`
	Starter        model.Architecture       `json:"starter"`
	Workload       model.Workload           `json:"workload"`
	Goals          []goals.Rule             `json:"goals"`
	Hints          []string                 `json:"hints"`
	AllowedKinds   []string                 `json:"allowedKinds"`
	ResourceBudget ResourceBudget           `json:"resourceBudget"`
	ComponentSpecs map[string]ComponentSpec `json:"componentSpecs"`
	Solution       Solution                 `json:"solution"`
}

type ResourceBudget struct {
	MaxNodes                  int            `json:"maxNodes"`
	MaxSlots                  int            `json:"maxSlots"`
	MaxProcessingSlotsPerNode int            `json:"maxProcessingSlotsPerNode"`
	MaxByKind                 map[string]int `json:"maxByKind"`
}

type Solution struct {
	Explanation  string             `json:"explanation"`
	Architecture model.Architecture `json:"architecture"`
}

type ComponentSpec struct {
	MeanServiceTimeUs int64   `json:"meanServiceTimeUs"`
	QueueLimit        int     `json:"queueLimit"`
	HitRate           float64 `json:"hitRate,omitempty"`
	CacheCapacity     int     `json:"cacheCapacity,omitempty"`
	CacheTTLUs        int64   `json:"cacheTtlUs,omitempty"`
}

// ValidateGraph applies the published resource and component rules to every attempt.
// It is also called before native verification, so imported JSON cannot evade the UI.
func ValidateGraph(challenge Challenge, graph model.Architecture) error {
	budget := challenge.ResourceBudget
	if len(graph.Nodes) > budget.MaxNodes {
		return fmt.Errorf("resource budget exceeded: at most %d components", budget.MaxNodes)
	}
	allowed := make(map[string]bool, len(challenge.AllowedKinds))
	for _, kind := range challenge.AllowedKinds {
		allowed[kind] = true
	}
	counts := map[string]int{}
	slots := 0
	for _, node := range graph.Nodes {
		if !allowed[node.Kind] {
			return fmt.Errorf("%s is not allowed in this problem", node.Kind)
		}
		counts[node.Kind]++
		if limit, ok := budget.MaxByKind[node.Kind]; ok && counts[node.Kind] > limit {
			return fmt.Errorf("resource budget exceeded: at most %d %s components", limit, node.Kind)
		}
		if node.Kind == "source" {
			if node.ID != challenge.Workload.EntryNodeID {
				return fmt.Errorf("the client must remain the published entry node")
			}
			continue
		}
		if node.Config.ProcessingSlots < 1 {
			return fmt.Errorf("each component needs positive processing slots")
		}
		spec, ok := challenge.ComponentSpecs[node.Kind]
		if !ok {
			return fmt.Errorf("component specification missing for %s", node.Kind)
		}
		if node.Config.MeanServiceTimeUs != spec.MeanServiceTimeUs || node.Config.QueueLimit != spec.QueueLimit || node.Config.HitRate != spec.HitRate || node.Config.FailureRate != 0 || node.Config.CacheCapacity != spec.CacheCapacity || node.Config.CacheTTLUs != spec.CacheTTLUs || node.Config.ReadServiceTimeUs != 0 || node.Config.WriteServiceTimeUs != 0 || node.Config.ServiceTimeModel == "exponential" {
			return fmt.Errorf("%s must use this problem's fixed component performance", node.Kind)
		}
		if node.Config.ProcessingSlots > budget.MaxProcessingSlotsPerNode {
			return fmt.Errorf("resource budget exceeded: at most %d processing slots per component", budget.MaxProcessingSlotsPerNode)
		}
		slots += node.Config.ProcessingSlots
	}
	if counts["source"] != 1 || counts["api"] < 1 {
		return fmt.Errorf("keep one client and at least one API server")
	}
	if slots > budget.MaxSlots {
		return fmt.Errorf("resource budget exceeded: %d of %d processing slots", slots, budget.MaxSlots)
	}
	return nil
}

func All() ([]Challenge, error) {
	entries, err := files.ReadDir(".")
	if err != nil {
		return nil, err
	}
	items := make([]Challenge, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		data, err := files.ReadFile(entry.Name())
		if err != nil {
			return nil, err
		}
		var challenge Challenge
		if err := json.Unmarshal(data, &challenge); err != nil {
			return nil, fmt.Errorf("%s: %w", entry.Name(), err)
		}
		if challenge.ID == "" || challenge.Version < 1 {
			return nil, fmt.Errorf("%s: missing identity", entry.Name())
		}
		items = append(items, challenge)
	}
	rank := map[string]int{"Beginner": 0, "Intermediate": 1, "Advanced": 2}
	sort.Slice(items, func(i, j int) bool {
		if rank[items[i].Difficulty] != rank[items[j].Difficulty] {
			return rank[items[i].Difficulty] < rank[items[j].Difficulty]
		}
		return items[i].Title < items[j].Title
	})
	return items, nil
}

func Get(id string) (Challenge, bool, error) {
	all, err := All()
	if err != nil {
		return Challenge{}, false, err
	}
	for _, challenge := range all {
		if challenge.ID == id {
			return challenge, true, nil
		}
	}
	return Challenge{}, false, nil
}
