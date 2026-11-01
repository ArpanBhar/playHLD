package model

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"regexp"
	"sort"
	"strconv"
)

const (
	ArchitectureSchemaVersion       = 3
	WorkloadSchemaVersion           = 1
	MaxPayloadBytes                 = 1 << 20
	MaxNodes                        = 100
	MaxEdges                        = 300
	MaxDurationUs             int64 = 300_000_000
	MaxDrainUs                int64 = 30_000_000
)

var idPattern = regexp.MustCompile(`^[A-Za-z0-9_-]{1,80}$`)

type Position struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
type NodeConfig struct {
	ProcessingSlots    int     `json:"processingSlots,omitempty"`
	MaxInFlight        int     `json:"maxInFlight,omitempty"`
	MeanServiceTimeUs  int64   `json:"meanServiceTimeUs,omitempty"`
	ReadServiceTimeUs  int64   `json:"readServiceTimeUs,omitempty"`
	WriteServiceTimeUs int64   `json:"writeServiceTimeUs,omitempty"`
	HitRate            float64 `json:"hitRate,omitempty"`
	RoutingPolicy      string  `json:"routingPolicy,omitempty"`
	QueueLimit         int     `json:"queueLimit"`
	ServiceTimeModel   string  `json:"serviceTimeModel,omitempty"`
}
type Node struct {
	ID       string     `json:"id"`
	Kind     string     `json:"kind"`
	Label    string     `json:"label,omitempty"`
	Position Position   `json:"position"`
	Config   NodeConfig `json:"config"`
}
type Edge struct {
	ID      string  `json:"id"`
	Source  string  `json:"source"`
	Target  string  `json:"target"`
	Route   string  `json:"route"`
	Weight  float64 `json:"weight,omitempty"`
	DelayUs int64   `json:"delayUs,omitempty"`
}
type Architecture struct {
	SchemaVersion int    `json:"schemaVersion"`
	Nodes         []Node `json:"nodes"`
	Edges         []Edge `json:"edges"`
}
type Workload struct {
	SchemaVersion  int              `json:"schemaVersion"`
	Seed           string           `json:"seed"`
	EntryNodeID    string           `json:"entryNodeId"`
	DurationUs     int64            `json:"durationUs"`
	DrainTimeoutUs int64            `json:"drainTimeoutUs"`
	ArrivalRateRps int64            `json:"arrivalRateRps"`
	ReadFraction   *float64         `json:"readFraction,omitempty"`
	Segments       []ArrivalSegment `json:"segments,omitempty"`
	ArrivalModel   string           `json:"arrivalModel,omitempty"`
}
type ArrivalSegment struct {
	StartUs int64 `json:"startUs"`
	EndUs   int64 `json:"endUs"`
	RateRps int64 `json:"rateRps"`
}
type SimulationConfig struct {
	Architecture       Architecture `json:"architecture"`
	Workload           Workload     `json:"workload"`
	SnapshotIntervalUs int64        `json:"snapshotIntervalUs"`
}
type ValidationIssue struct {
	Code    string `json:"code"`
	Path    string `json:"path"`
	Message string `json:"message"`
}

func ParseArchitecture(data []byte) (Architecture, error) {
	var a Architecture
	if len(data) > MaxPayloadBytes {
		return a, errors.New("architecture exceeds payload limit")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err := d.Decode(&a); err != nil {
		return a, err
	}
	if d.Decode(new(any)) != io.EOF {
		return a, errors.New("multiple JSON values")
	}
	if issues := ValidateArchitecture(a); len(issues) > 0 {
		return a, fmt.Errorf("invalid architecture: %s at %s", issues[0].Code, issues[0].Path)
	}
	return a, nil
}

func ValidateArchitecture(a Architecture) []ValidationIssue {
	issues := make([]ValidationIssue, 0)
	add := func(code, path, msg string) { issues = append(issues, ValidationIssue{code, path, msg}) }
	if a.SchemaVersion != ArchitectureSchemaVersion {
		add("UNSUPPORTED_SCHEMA", "schemaVersion", "expected architecture version 3")
	}
	if len(a.Nodes) > MaxNodes {
		add("LIMIT_EXCEEDED", "nodes", "too many nodes")
	}
	if len(a.Edges) > MaxEdges {
		add("LIMIT_EXCEEDED", "edges", "too many edges")
	}
	nodes := map[string]Node{}
	for i, n := range a.Nodes {
		p := fmt.Sprintf("nodes[%d]", i)
		if !idPattern.MatchString(n.ID) {
			add("INVALID_ID", p+".id", "use 1–80 letters, numbers, _ or -")
		}
		if _, ok := nodes[n.ID]; ok {
			add("DUPLICATE_ID", p+".id", "node ID is repeated")
		}
		nodes[n.ID] = n
		if n.Kind != "source" && n.Kind != "api" && n.Kind != "load_balancer" && n.Kind != "cache" && n.Kind != "database" && n.Kind != "message_queue" && n.Kind != "worker" && n.Kind != "object_storage" && n.Kind != "cdn" {
			add("UNSUPPORTED_KIND", p+".kind", "component kind is unsupported")
		}
		if len(n.Label) > 120 {
			add("LIMIT_EXCEEDED", p+".label", "label is too long")
		}
		if math.IsNaN(n.Position.X) || math.IsInf(n.Position.X, 0) || math.IsNaN(n.Position.Y) || math.IsInf(n.Position.Y, 0) || math.Abs(n.Position.X) > 100000 || math.Abs(n.Position.Y) > 100000 {
			add("INVALID_POSITION", p+".position", "coordinates must be finite and within bounds")
		}
		if n.Kind == "source" {
			if n.Config.ProcessingSlots != 0 || n.Config.MaxInFlight != 0 || n.Config.MeanServiceTimeUs != 0 || n.Config.QueueLimit != 0 || n.Config.HitRate != 0 || n.Config.RoutingPolicy != "" {
				add("INVALID_CONFIG", p+".config", "traffic sources do not have processing capacity")
			}
		} else {
			if n.Config.ServiceTimeModel != "" && n.Config.ServiceTimeModel != "constant" && n.Config.ServiceTimeModel != "exponential" {
				add("UNSUPPORTED_MODEL", p+".config.serviceTimeModel", "service model must be constant or exponential")
			}
			if n.Config.ProcessingSlots < 1 || n.Config.ProcessingSlots > 1000 {
				add("OUT_OF_RANGE", p+".config.processingSlots", "processing slots must be 1–1000 for this instance")
			}
			if n.Config.MaxInFlight != 0 && (n.Config.MaxInFlight < n.Config.ProcessingSlots || n.Config.MaxInFlight > 100000 || n.Config.MaxInFlight > n.Config.ProcessingSlots+n.Config.QueueLimit) {
				add("OUT_OF_RANGE", p+".config.maxInFlight", "max in-flight requests must be between processing slots and processing slots plus queue limit")
			}
			if n.Config.MeanServiceTimeUs < 1 || n.Config.MeanServiceTimeUs > MaxDurationUs {
				add("OUT_OF_RANGE", p+".config.meanServiceTimeUs", "service time must be positive")
			}
			if n.Config.QueueLimit < 0 || n.Config.QueueLimit > 100000 {
				add("OUT_OF_RANGE", p+".config.queueLimit", "queue limit must be 0–100000")
			}
			if n.Config.ReadServiceTimeUs < 0 || n.Config.ReadServiceTimeUs > MaxDurationUs || n.Config.WriteServiceTimeUs < 0 || n.Config.WriteServiceTimeUs > MaxDurationUs {
				add("OUT_OF_RANGE", p+".config.readServiceTimeUs", "class service times must be within range")
			}
			if math.IsNaN(n.Config.HitRate) || math.IsInf(n.Config.HitRate, 0) || n.Config.HitRate < 0 || n.Config.HitRate > 1 {
				add("OUT_OF_RANGE", p+".config.hitRate", "hit rate must be between zero and one")
			}
			if n.Config.RoutingPolicy != "" && n.Config.RoutingPolicy != "round_robin" && n.Config.RoutingPolicy != "weighted" {
				add("UNSUPPORTED_POLICY", p+".config.routingPolicy", "routing policy must be round_robin or weighted")
			}
			if n.Kind != "load_balancer" && n.Config.RoutingPolicy != "" {
				add("INVALID_CONFIG", p+".config.routingPolicy", "only load balancers choose a routing policy")
			}
			if n.Kind != "cache" && n.Kind != "cdn" && n.Config.HitRate != 0 {
				add("INVALID_CONFIG", p+".config.hitRate", "only caches and CDNs have a hit rate")
			}
			if n.Kind != "database" && n.Kind != "object_storage" && (n.Config.ReadServiceTimeUs != 0 || n.Config.WriteServiceTimeUs != 0) {
				add("INVALID_CONFIG", p+".config.readServiceTimeUs", "read/write overrides require a data store")
			}
		}
	}
	edges := map[string]bool{}
	for i, e := range a.Edges {
		p := fmt.Sprintf("edges[%d]", i)
		if !idPattern.MatchString(e.ID) {
			add("INVALID_ID", p+".id", "invalid edge ID")
		}
		if edges[e.ID] {
			add("DUPLICATE_ID", p+".id", "edge ID is repeated")
		}
		edges[e.ID] = true
		if _, ok := nodes[e.Source]; !ok {
			add("UNKNOWN_NODE", p+".source", "source node does not exist")
		}
		if _, ok := nodes[e.Target]; !ok {
			add("UNKNOWN_NODE", p+".target", "target node does not exist")
		}
		if e.Route != "always" && e.Route != "cache_hit" && e.Route != "cache_miss" && e.Route != "weighted" {
			add("UNSUPPORTED_ROUTE", p+".route", "route type is unsupported")
		}
		if math.IsNaN(e.Weight) || math.IsInf(e.Weight, 0) || e.Weight < 0 || e.Weight > 1 {
			add("OUT_OF_RANGE", p+".weight", "weight must be between zero and one")
		}
		if e.DelayUs < 0 || e.DelayUs > MaxDurationUs {
			add("OUT_OF_RANGE", p+".delayUs", "edge delay is out of range")
		}
	}
	return issues
}

func ValidateWorkload(w Workload) []ValidationIssue {
	issues := []ValidationIssue{}
	add := func(code, path, msg string) { issues = append(issues, ValidationIssue{code, path, msg}) }
	if w.SchemaVersion != WorkloadSchemaVersion {
		add("UNSUPPORTED_SCHEMA", "workload.schemaVersion", "expected version 1")
	}
	if _, err := strconv.ParseUint(w.Seed, 10, 64); err != nil {
		add("INVALID_SEED", "workload.seed", "seed must be a uint64 decimal string")
	}
	if !idPattern.MatchString(w.EntryNodeID) {
		add("INVALID_ID", "workload.entryNodeId", "invalid entry node ID")
	}
	if w.DurationUs < 1 || w.DurationUs > MaxDurationUs {
		add("OUT_OF_RANGE", "workload.durationUs", "duration exceeds initial limit")
	}
	if w.DrainTimeoutUs < 0 || w.DrainTimeoutUs > MaxDrainUs {
		add("OUT_OF_RANGE", "workload.drainTimeoutUs", "drain exceeds initial limit")
	}
	if w.ArrivalRateRps < 1 || w.ArrivalRateRps > 100000 {
		add("OUT_OF_RANGE", "workload.arrivalRateRps", "rate must be 1–100000")
	}
	if w.ReadFraction != nil && (math.IsNaN(*w.ReadFraction) || math.IsInf(*w.ReadFraction, 0) || *w.ReadFraction < 0 || *w.ReadFraction > 1) {
		add("OUT_OF_RANGE", "workload.readFraction", "read fraction must be between zero and one")
	}
	if w.ArrivalModel != "" && w.ArrivalModel != "fixed" && w.ArrivalModel != "poisson" {
		add("UNSUPPORTED_MODEL", "workload.arrivalModel", "arrival model must be fixed or poisson")
	}
	if len(w.Segments) > 100 {
		add("LIMIT_EXCEEDED", "workload.segments", "too many traffic segments")
	}
	if len(w.Segments) > 0 {
		previousEnd := int64(0)
		for i, segment := range w.Segments {
			path := fmt.Sprintf("workload.segments[%d]", i)
			if segment.StartUs != previousEnd || segment.EndUs <= segment.StartUs || segment.EndUs > w.DurationUs {
				add("INVALID_SEGMENT", path, "segments must cover the duration without gaps or overlaps")
			}
			if segment.RateRps < 1 || segment.RateRps > 100_000 {
				add("OUT_OF_RANGE", path+".rateRps", "rate must be 1–100000")
			}
			previousEnd = segment.EndUs
		}
		if previousEnd != w.DurationUs {
			add("INVALID_SEGMENT", "workload.segments", "segments must end at workload duration")
		}
	}
	return issues
}

func ValidateSimulation(c SimulationConfig) []ValidationIssue {
	issues := append(ValidateArchitecture(c.Architecture), ValidateWorkload(c.Workload)...)
	if c.SnapshotIntervalUs < 1 || c.SnapshotIntervalUs > MaxDurationUs {
		issues = append(issues, ValidationIssue{"OUT_OF_RANGE", "snapshotIntervalUs", "snapshot interval is invalid"})
	}
	nodes := map[string]Node{}
	out := map[string][]Edge{}
	for _, n := range c.Architecture.Nodes {
		nodes[n.ID] = n
	}
	for _, e := range c.Architecture.Edges {
		out[e.Source] = append(out[e.Source], e)
	}
	entry, ok := nodes[c.Workload.EntryNodeID]
	if !ok || entry.Kind != "source" {
		issues = append(issues, ValidationIssue{"INVALID_ENTRY", "workload.entryNodeId", "entry must be a source node"})
	}
	if len(c.Architecture.Nodes) == 0 {
		issues = append(issues, ValidationIssue{"EMPTY_GRAPH", "architecture.nodes", "graph has no nodes"})
	}
	for _, n := range c.Architecture.Nodes {
		edges := out[n.ID]
		switch n.Kind {
		case "source":
			if len(edges) != 1 || edges[0].Route != "always" {
				issues = append(issues, ValidationIssue{"MISSING_ROUTE", "architecture.edges", "source requires exactly one always edge"})
			}
		case "cache", "cdn":
			hits, misses := 0, 0
			for _, edge := range edges {
				if edge.Route == "cache_hit" {
					hits++
				}
				if edge.Route == "cache_miss" {
					misses++
				}
				if edge.Route != "cache_hit" && edge.Route != "cache_miss" {
					issues = append(issues, ValidationIssue{"INVALID_ROUTE", "architecture.edges", "cache edges must be hit or miss"})
				}
			}
			if hits > 1 || misses != 1 {
				issues = append(issues, ValidationIssue{"MISSING_ROUTE", "architecture.edges", "cache requires one miss edge and at most one hit edge"})
			}
		case "load_balancer":
			if len(edges) == 0 {
				issues = append(issues, ValidationIssue{"MISSING_ROUTE", "architecture.edges", "load balancer requires targets"})
			}
			weighted := n.Config.RoutingPolicy == "weighted"
			sum := 0.0
			for _, edge := range edges {
				if weighted && edge.Route != "weighted" || !weighted && edge.Route != "always" {
					issues = append(issues, ValidationIssue{"INVALID_ROUTE", "architecture.edges", "route does not match load balancer policy"})
				}
				sum += edge.Weight
			}
			if weighted && math.Abs(sum-1) > 1e-6 {
				issues = append(issues, ValidationIssue{"INVALID_WEIGHT", "architecture.edges", "weighted targets must sum to one"})
			}
		case "message_queue":
			if len(edges) == 0 {
				issues = append(issues, ValidationIssue{"MISSING_CONSUMER", "architecture.edges", "message queue requires at least one consumer"})
			}
			for _, edge := range edges {
				if edge.Route != "always" {
					issues = append(issues, ValidationIssue{"INVALID_ROUTE", "architecture.edges", "message queue consumer routes must be always"})
				}
			}
		default:
			if len(edges) > 1 || len(edges) == 1 && edges[0].Route != "always" {
				issues = append(issues, ValidationIssue{"AMBIGUOUS_ROUTE", "architecture.edges", "service nodes allow at most one always edge"})
			}
		}
	}
	if len(issues) > 0 {
		return issues
	}
	seen := map[string]int{}
	var visit func(string)
	visit = func(id string) {
		if seen[id] == 1 {
			issues = append(issues, ValidationIssue{"CYCLE", "architecture.edges", "cycles are not supported"})
			return
		}
		if seen[id] == 2 {
			return
		}
		seen[id] = 1
		for _, edge := range out[id] {
			visit(edge.Target)
		}
		seen[id] = 2
	}
	visit(c.Workload.EntryNodeID)
	if len(seen) != len(nodes) {
		issues = append(issues, ValidationIssue{"UNREACHABLE_NODE", "architecture.nodes", "all nodes must be reachable from entry"})
	}
	return issues
}

func NormalizeConfig(c SimulationConfig) (SimulationConfig, error) {
	if issues := ValidateSimulation(c); len(issues) > 0 {
		return c, fmt.Errorf("%s at %s: %s", issues[0].Code, issues[0].Path, issues[0].Message)
	}
	a := c.Architecture
	a.Nodes = append([]Node(nil), a.Nodes...)
	a.Edges = append([]Edge(nil), a.Edges...)
	for i := range a.Nodes {
		if a.Nodes[i].Kind != "source" && a.Nodes[i].Config.MaxInFlight == 0 {
			a.Nodes[i].Config.MaxInFlight = min(100000, a.Nodes[i].Config.ProcessingSlots+a.Nodes[i].Config.QueueLimit)
		}
	}
	sort.Slice(a.Nodes, func(i, j int) bool { return a.Nodes[i].ID < a.Nodes[j].ID })
	sort.Slice(a.Edges, func(i, j int) bool { return a.Edges[i].ID < a.Edges[j].ID })
	c.Architecture = a
	return c, nil
}
