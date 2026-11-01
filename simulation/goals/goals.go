package goals

import (
	"fmt"
	"math"

	"systemforge/simulation/engine"
	"systemforge/simulation/model"
)

type Rule struct {
	ID       string  `json:"id"`
	Label    string  `json:"label"`
	Metric   string  `json:"metric"`
	Operator string  `json:"operator"`
	Target   float64 `json:"target"`
	NodeID   string  `json:"nodeId,omitempty"`
}

type Outcome struct {
	ID          string   `json:"id"`
	Label       string   `json:"label"`
	Status      string   `json:"status"`
	Observed    *float64 `json:"observed"`
	Target      float64  `json:"target"`
	Unit        string   `json:"unit"`
	Explanation string   `json:"explanation"`
}

func Evaluate(result engine.Result, workload model.Workload, rules []Rule) ([]Outcome, error) {
	if len(rules) > 30 {
		return nil, fmt.Errorf("too many goal rules")
	}
	results := make([]Outcome, 0, len(rules))
	for _, rule := range rules {
		if rule.ID == "" || math.IsNaN(rule.Target) || math.IsInf(rule.Target, 0) || rule.Target < 0 || rule.Operator != "gte" && rule.Operator != "lte" {
			return nil, fmt.Errorf("invalid goal rule %q", rule.ID)
		}
		out := Outcome{ID: rule.ID, Label: rule.Label, Status: "not_measurable", Target: rule.Target}
		var value float64
		measurable := true
		switch rule.Metric {
		case "success_fraction":
			out.Unit = "%"
			if result.Summary.Generated == 0 {
				measurable = false
			} else {
				value = float64(result.Summary.Succeeded) * 100 / float64(result.Summary.Generated)
			}
		case "throughput_rps":
			out.Unit = "req/s"
			if workload.DurationUs <= 0 {
				measurable = false
			} else {
				value = float64(result.Summary.Succeeded) * 1_000_000 / float64(workload.DurationUs)
			}
		case "p95_latency_us":
			out.Unit = "ms"
			if result.Summary.P95LatencyUs == nil {
				measurable = false
			} else {
				value = float64(*result.Summary.P95LatencyUs) / 1000
			}
		case "p99_latency_us":
			out.Unit = "ms"
			if result.Summary.P99LatencyUs == nil {
				measurable = false
			} else {
				value = float64(*result.Summary.P99LatencyUs) / 1000
			}
		case "cache_hit_ratio":
			out.Unit = "%"
			if len(result.Snapshots) == 0 {
				measurable = false
			} else {
				metrics, ok := result.Snapshots[len(result.Snapshots)-1].NodeMetrics[rule.NodeID]
				if !ok || metrics.CacheLookups == 0 {
					measurable = false
				} else {
					value = float64(metrics.CacheHits) * 100 / float64(metrics.CacheLookups)
				}
			}
		case "max_queue_depth":
			out.Unit = "requests"
			if len(result.Snapshots) == 0 {
				measurable = false
			} else {
				metrics, ok := result.Snapshots[len(result.Snapshots)-1].NodeMetrics[rule.NodeID]
				if !ok {
					measurable = false
				} else {
					value = float64(metrics.MaxQueueDepth)
				}
			}
		default:
			return nil, fmt.Errorf("unknown goal metric %q", rule.Metric)
		}
		if measurable {
			out.Observed = &value
			met := rule.Operator == "gte" && value >= rule.Target || rule.Operator == "lte" && value <= rule.Target
			if result.Status != "completed" || result.Summary.Unfinished > 0 {
				met = false
				out.Explanation = "Run had unfinished work; goal cannot pass."
			} else if met {
				out.Explanation = "Target met by the simulated result."
			} else {
				out.Explanation = "Observed metric did not reach the target."
			}
			if met {
				out.Status = "met"
			} else {
				out.Status = "unmet"
			}
		} else {
			out.Explanation = "Run did not produce enough data to measure this goal."
		}
		results = append(results, out)
	}
	return results, nil
}
