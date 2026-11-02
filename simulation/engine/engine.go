package engine

import (
	"container/heap"
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"strconv"

	"playhld/simulation/model"
)

const EngineVersion = "0.6.0"
const MaxEvents = 2_000_000
const MaxRequests = 100_000
const MaxPendingEvents = 200_000
const MaxSnapshots = 3_000

type eventKind uint8

const (
	arrival eventKind = iota
	complete
	sample
)

type event struct {
	atUs      int64
	sequence  uint64
	kind      eventKind
	nodeID    string
	requestID uint64
}
type eventHeap []event

func (h eventHeap) Len() int { return len(h) }
func (h eventHeap) Less(i, j int) bool {
	a, b := h[i], h[j]
	if a.atUs != b.atUs {
		return a.atUs < b.atUs
	}
	if a.kind == sample && b.kind != sample {
		return true
	}
	if b.kind == sample && a.kind != sample {
		return false
	}
	return a.sequence < b.sequence
}
func (h eventHeap) Swap(i, j int) { h[i], h[j] = h[j], h[i] }
func (h *eventHeap) Push(x any)   { *h = append(*h, x.(event)) }
func (h *eventHeap) Pop() any     { old := *h; n := len(old); x := old[n-1]; *h = old[:n-1]; return x }

type request struct {
	startedAtUs int64
	isRead      bool
}
type nodeState struct {
	active             int
	queue              []uint64
	lastAtUs           int64
	busySlotUs         int64
	previousBusySlotUs int64
	maxQueue           int
	completions        int64
	rejected           int64
	cacheHits          int64
	cacheLookups       int64
	nextTarget         int
}

type NodeMetrics struct {
	Completions           int64    `json:"completions"`
	Rejected              int64    `json:"rejected"`
	QueueDepth            int      `json:"queueDepth"`
	MaxQueueDepth         int      `json:"maxQueueDepth"`
	ProcessingUtilization *float64 `json:"processingUtilization"`
	CacheHits             int64    `json:"cacheHits"`
	CacheLookups          int64    `json:"cacheLookups"`
}
type Snapshot struct {
	AtUs             int64                  `json:"atUs"`
	WindowDurationUs int64                  `json:"windowDurationUs"`
	Offered          int64                  `json:"offered"`
	Succeeded        int64                  `json:"succeeded"`
	Failed           int64                  `json:"failed"`
	OfferedRps       float64                `json:"offeredRps"`
	ThroughputRps    float64                `json:"throughputRps"`
	NodeMetrics      map[string]NodeMetrics `json:"nodeMetrics"`
	EdgeTraversals   map[string]int64       `json:"edgeTraversals"`
}
type Summary struct {
	Generated       int64  `json:"generated"`
	Succeeded       int64  `json:"succeeded"`
	Failed          int64  `json:"failed"`
	Unfinished      int64  `json:"unfinished"`
	ProcessedEvents int    `json:"processedEvents"`
	P95LatencyUs    *int64 `json:"p95LatencyUs"`
	P99LatencyUs    *int64 `json:"p99LatencyUs"`
}
type Result struct {
	ResultSchemaVersion int        `json:"resultSchemaVersion"`
	EngineVersion       string     `json:"engineVersion"`
	Status              string     `json:"status"`
	Snapshots           []Snapshot `json:"snapshots"`
	Summary             Summary    `json:"summary"`
	Warnings            []string   `json:"warnings"`
}

type BatchOutcome struct {
	Snapshots       []Snapshot `json:"snapshots"`
	ProcessedEvents int        `json:"processedEvents"`
	Done            bool       `json:"done"`
	Result          *Result    `json:"result,omitempty"`
}

type Engine struct {
	cfg               model.SimulationConfig
	nodes             map[string]model.Node
	out               map[string][]model.Edge
	states            map[string]*nodeState
	edgeTraversals    map[string]int64
	events            eventHeap
	requests          map[uint64]request
	nextSequence      uint64
	nextRequest       uint64
	processed         int
	nowUs             int64
	lastSampleAtUs    int64
	generated         int64
	succeeded         int64
	failed            int64
	previousGenerated int64
	previousSucceeded int64
	previousFailed    int64
	latencies         []int64
	snapshots         []Snapshot
	done              bool
	arrivalRNG        rngStream
	classRNG          rngStream
	routingRNG        rngStream
	nodeRNG           map[string]*rngStream
	arrivalSegments   []model.ArrivalSegment
	segmentIndex      int
	segmentGenerated  int64
}

func NewEngine(c model.SimulationConfig) (*Engine, error) {
	normal, err := model.NormalizeConfig(c)
	if err != nil {
		return nil, err
	}
	segments := normal.Workload.Segments
	if len(segments) == 0 {
		segments = []model.ArrivalSegment{{StartUs: 0, EndUs: normal.Workload.DurationUs, RateRps: normal.Workload.ArrivalRateRps}}
	}
	count := int64(0)
	for _, segment := range segments {
		count += ((segment.EndUs-segment.StartUs)*segment.RateRps + 999_999) / 1_000_000
	}
	if count > MaxRequests {
		return nil, fmt.Errorf("request limit exceeded: %d", count)
	}
	if (normal.Workload.DurationUs+normal.Workload.DrainTimeoutUs)/normal.SnapshotIntervalUs > MaxSnapshots {
		return nil, errors.New("snapshot limit exceeded")
	}
	e := &Engine{cfg: normal, nodes: map[string]model.Node{}, out: map[string][]model.Edge{}, states: map[string]*nodeState{}, edgeTraversals: map[string]int64{}, requests: map[uint64]request{}, arrivalSegments: segments}
	for _, n := range normal.Architecture.Nodes {
		e.nodes[n.ID] = n
		if n.Kind != "source" {
			e.states[n.ID] = &nodeState{}
		}
	}
	for _, edge := range normal.Architecture.Edges {
		e.out[edge.Source] = append(e.out[edge.Source], edge)
	}
	if err := e.Initialize(); err != nil {
		return nil, err
	}
	return e, nil
}

func (e *Engine) Initialize() error {
	e.events = nil
	heap.Init(&e.events)
	e.requests = map[uint64]request{}
	e.edgeTraversals = make(map[string]int64, len(e.cfg.Architecture.Edges))
	e.nextSequence = 0
	e.nextRequest = 0
	e.processed = 0
	e.nowUs = 0
	e.lastSampleAtUs = 0
	e.generated = 0
	e.succeeded = 0
	e.failed = 0
	e.previousGenerated = 0
	e.previousSucceeded = 0
	e.previousFailed = 0
	e.latencies = nil
	e.snapshots = nil
	e.done = false
	e.segmentIndex = 0
	e.segmentGenerated = 0
	seed, _ := strconv.ParseUint(e.cfg.Workload.Seed, 10, 64)
	e.arrivalRNG = newNamedStream(seed, "arrivals")
	e.classRNG = newNamedStream(seed, "request-classes")
	e.routingRNG = newNamedStream(seed, "routing")
	e.nodeRNG = make(map[string]*rngStream, len(e.cfg.Architecture.Nodes))
	for _, node := range e.cfg.Architecture.Nodes {
		stream := newNamedStream(seed, "node:"+node.ID)
		e.nodeRNG[node.ID] = &stream
	}
	for id := range e.states {
		e.states[id] = &nodeState{}
	}
	if at, ok := e.firstArrivalAt(); ok {
		if err := e.schedule(event{atUs: at, kind: arrival, nodeID: e.cfg.Workload.EntryNodeID}); err != nil {
			return err
		}
	}
	if e.cfg.SnapshotIntervalUs < e.cfg.Workload.DurationUs {
		return e.schedule(event{atUs: e.cfg.SnapshotIntervalUs, kind: sample})
	}
	return nil
}

func (e *Engine) Reset() error { return e.Initialize() }

// ApplyArchitecture updates a reachable topology without discarding requests
// already queued or in flight. Existing node IDs and kinds must remain so
// pending events continue to reference valid components.
func (e *Engine) ApplyArchitecture(architecture model.Architecture) error {
	config := e.cfg
	config.Architecture = architecture
	normal, err := model.NormalizeConfig(config)
	if err != nil {
		return err
	}
	nodes := make(map[string]model.Node, len(normal.Architecture.Nodes))
	for _, node := range normal.Architecture.Nodes {
		nodes[node.ID] = node
	}
	for id, previous := range e.nodes {
		next, ok := nodes[id]
		if !ok || next.Kind != previous.Kind {
			return fmt.Errorf("removing or changing component %s requires a fresh simulation window", id)
		}
	}
	out := make(map[string][]model.Edge)
	for _, edge := range normal.Architecture.Edges {
		out[edge.Source] = append(out[edge.Source], edge)
	}
	for id, node := range nodes {
		if node.Kind != "source" && e.states[id] == nil {
			e.states[id] = &nodeState{lastAtUs: e.nowUs}
		}
	}
	e.cfg.Architecture = normal.Architecture
	e.nodes = nodes
	e.out = out
	return nil
}

func (e *Engine) schedule(v event) error {
	maxTime := e.cfg.Workload.DurationUs + e.cfg.Workload.DrainTimeoutUs
	if v.atUs < e.nowUs || v.atUs < 0 || v.atUs > maxTime {
		return errors.New("event timestamp outside simulation window")
	}
	if len(e.events) >= MaxPendingEvents {
		return errors.New("pending event limit exceeded")
	}
	e.nextSequence++
	v.sequence = e.nextSequence
	heap.Push(&e.events, v)
	return nil
}

func (e *Engine) Step() (*Snapshot, bool, error) {
	if e.done {
		return nil, true, nil
	}
	if len(e.events) == 0 {
		e.done = true
		return nil, true, nil
	}
	if e.processed >= MaxEvents {
		return nil, false, errors.New("processed event limit exceeded")
	}
	v := heap.Pop(&e.events).(event)
	e.nowUs = v.atUs
	e.processed++
	switch v.kind {
	case arrival:
		if v.requestID == 0 {
			if e.generated >= MaxRequests {
				return nil, false, errors.New("request limit exceeded")
			}
			e.nextRequest++
			v.requestID = e.nextRequest
			e.generated++
			isRead := true
			if e.cfg.Workload.ReadFraction != nil {
				isRead = e.classRNG.nextFloat() < *e.cfg.Workload.ReadFraction
			}
			e.requests[v.requestID] = request{startedAtUs: v.atUs, isRead: isRead}
			e.segmentGenerated++
			if nextAt, ok := e.nextArrivalAt(); ok {
				if err := e.schedule(event{atUs: nextAt, kind: arrival, nodeID: e.cfg.Workload.EntryNodeID}); err != nil {
					return nil, false, err
				}
			}
		}
		if err := e.handleArrival(v.nodeID, v.requestID); err != nil {
			return nil, false, err
		}
	case complete:
		if err := e.handleCompletion(v.nodeID, v.requestID); err != nil {
			return nil, false, err
		}
	case sample:
		s := e.sampleAt(v.atUs)
		e.snapshots = append(e.snapshots, s)
		if len(e.snapshots) > MaxSnapshots {
			return nil, false, errors.New("snapshot limit exceeded")
		}
		nextAt := v.atUs + e.cfg.SnapshotIntervalUs
		if nextAt < e.cfg.Workload.DurationUs {
			if err := e.schedule(event{atUs: nextAt, kind: sample}); err != nil {
				return nil, false, err
			}
		}
		return &s, false, nil
	}
	return nil, false, nil
}

func (e *Engine) nextArrivalAt() (int64, bool) {
	for e.segmentIndex < len(e.arrivalSegments) {
		segment := e.arrivalSegments[e.segmentIndex]
		if e.cfg.Workload.ArrivalModel == "poisson" {
			nextAt := e.nowUs + e.exponentialDuration(&e.arrivalRNG, float64(1_000_000)/float64(segment.RateRps))
			if nextAt < segment.EndUs {
				return nextAt, true
			}
			e.segmentIndex++
			return e.firstArrivalAt()
		}
		nextAt := segment.StartUs + e.segmentGenerated*1_000_000/segment.RateRps
		if nextAt < segment.EndUs {
			return nextAt, true
		}
		e.segmentIndex++
		e.segmentGenerated = 0
		if e.segmentIndex < len(e.arrivalSegments) {
			return e.arrivalSegments[e.segmentIndex].StartUs, true
		}
	}
	return 0, false
}

func (e *Engine) firstArrivalAt() (int64, bool) {
	for e.segmentIndex < len(e.arrivalSegments) {
		segment := e.arrivalSegments[e.segmentIndex]
		at := segment.StartUs
		if e.cfg.Workload.ArrivalModel == "poisson" {
			at += e.exponentialDuration(&e.arrivalRNG, float64(1_000_000)/float64(segment.RateRps))
		}
		if at < segment.EndUs {
			return at, true
		}
		e.segmentIndex++
		e.segmentGenerated = 0
	}
	return 0, false
}

func (e *Engine) exponentialDuration(stream *rngStream, meanUs float64) int64 {
	u := stream.nextOpenFloat()
	value := int64(math.Ceil(-math.Log1p(-u) * meanUs))
	if value < 1 {
		return 1
	}
	return value
}

func (e *Engine) handleArrival(nodeID string, id uint64) error {
	n, ok := e.nodes[nodeID]
	if !ok {
		return fmt.Errorf("unknown node %q", nodeID)
	}
	if n.Kind == "source" {
		return e.route(nodeID, id)
	}
	s := e.states[nodeID]
	if s == nil {
		return fmt.Errorf("node %q not implemented", nodeID)
	}
	e.updateBusy(s)
	if s.active+len(s.queue) >= n.Config.MaxInFlight {
		s.rejected++
		e.failed++
		delete(e.requests, id)
		return nil
	}
	if s.active < n.Config.ProcessingSlots {
		return e.startService(nodeID, id)
	}
	if len(s.queue) < n.Config.QueueLimit {
		s.queue = append(s.queue, id)
		if len(s.queue) > s.maxQueue {
			s.maxQueue = len(s.queue)
		}
		return nil
	}
	s.rejected++
	e.failed++
	delete(e.requests, id)
	return nil
}

func (e *Engine) startService(nodeID string, id uint64) error {
	n := e.nodes[nodeID]
	s := e.states[nodeID]
	s.active++
	serviceTime := n.Config.MeanServiceTimeUs
	if n.Kind == "database" || n.Kind == "object_storage" {
		if e.requests[id].isRead && n.Config.ReadServiceTimeUs > 0 {
			serviceTime = n.Config.ReadServiceTimeUs
		}
		if !e.requests[id].isRead && n.Config.WriteServiceTimeUs > 0 {
			serviceTime = n.Config.WriteServiceTimeUs
		}
	}
	if n.Config.ServiceTimeModel == "exponential" {
		serviceTime = e.exponentialDuration(e.nodeRNG[nodeID], float64(serviceTime))
	}
	at := e.nowUs + serviceTime
	if at > e.cfg.Workload.DurationUs+e.cfg.Workload.DrainTimeoutUs {
		return nil
	}
	return e.schedule(event{atUs: at, kind: complete, nodeID: nodeID, requestID: id})
}

func (e *Engine) handleCompletion(nodeID string, id uint64) error {
	s := e.states[nodeID]
	if s == nil || s.active == 0 {
		return errors.New("completion without active service")
	}
	e.updateBusy(s)
	s.active--
	s.completions++
	if len(s.queue) > 0 {
		next := s.queue[0]
		s.queue = s.queue[1:]
		if err := e.startService(nodeID, next); err != nil {
			return err
		}
	}
	return e.route(nodeID, id)
}

func (e *Engine) route(nodeID string, id uint64) error {
	n := e.nodes[nodeID]
	edges := e.out[nodeID]
	var selected *model.Edge
	switch n.Kind {
	case "cache", "cdn":
		state := e.states[nodeID]
		state.cacheLookups++
		hit := e.requests[id].isRead && e.nodeRNG[nodeID].nextFloat() < n.Config.HitRate
		if hit {
			state.cacheHits++
		}
		for i := range edges {
			if hit && edges[i].Route == "cache_hit" || !hit && edges[i].Route == "cache_miss" {
				selected = &edges[i]
				break
			}
		}
	case "load_balancer", "message_queue":
		if len(edges) > 0 {
			if n.Kind == "load_balancer" && n.Config.RoutingPolicy == "weighted" {
				draw := e.routingRNG.nextFloat()
				sum := 0.0
				selected = &edges[len(edges)-1]
				for i := range edges {
					sum += edges[i].Weight
					if draw < sum {
						selected = &edges[i]
						break
					}
				}
			} else {
				state := e.states[nodeID]
				selected = &edges[state.nextTarget%len(edges)]
				state.nextTarget++
			}
		}
	default:
		if len(edges) > 0 {
			selected = &edges[0]
		}
	}
	if selected != nil {
		at := e.nowUs + selected.DelayUs
		if at > e.cfg.Workload.DurationUs+e.cfg.Workload.DrainTimeoutUs {
			return nil
		}
		if err := e.schedule(event{atUs: at, kind: arrival, nodeID: selected.Target, requestID: id}); err != nil {
			return err
		}
		e.edgeTraversals[selected.ID]++
		return nil
	}
	req, ok := e.requests[id]
	if !ok {
		return errors.New("completion for missing request")
	}
	e.succeeded++
	e.latencies = append(e.latencies, e.nowUs-req.startedAtUs)
	delete(e.requests, id)
	return nil
}

func (e *Engine) updateBusy(s *nodeState) {
	s.busySlotUs += int64(s.active) * (e.nowUs - s.lastAtUs)
	s.lastAtUs = e.nowUs
}

func (e *Engine) sampleAt(atUs int64) Snapshot {
	snap := Snapshot{AtUs: atUs, Offered: e.generated, Succeeded: e.succeeded, Failed: e.failed, NodeMetrics: map[string]NodeMetrics{}, EdgeTraversals: make(map[string]int64, len(e.edgeTraversals))}
	for id, count := range e.edgeTraversals {
		snap.EdgeTraversals[id] = count
	}
	window := atUs - e.lastSampleAtUs
	snap.WindowDurationUs = window
	if window > 0 {
		snap.OfferedRps = float64(e.generated-e.previousGenerated) * 1_000_000 / float64(window)
		snap.ThroughputRps = float64(e.succeeded-e.previousSucceeded) * 1_000_000 / float64(window)
	}
	ids := make([]string, 0, len(e.states))
	for id := range e.states {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	for _, id := range ids {
		s := e.states[id]
		e.updateBusy(s)
		capacity := e.nodes[id].Config.ProcessingSlots
		var utilization *float64
		if window > 0 && capacity > 0 {
			u := float64(s.busySlotUs-s.previousBusySlotUs) / float64(int64(capacity)*window)
			utilization = &u
		}
		snap.NodeMetrics[id] = NodeMetrics{Completions: s.completions, Rejected: s.rejected, QueueDepth: len(s.queue), MaxQueueDepth: s.maxQueue, ProcessingUtilization: utilization, CacheHits: s.cacheHits, CacheLookups: s.cacheLookups}
		s.previousBusySlotUs = s.busySlotUs
	}
	e.lastSampleAtUs = atUs
	e.previousGenerated = e.generated
	e.previousSucceeded = e.succeeded
	e.previousFailed = e.failed
	return snap
}

func (e *Engine) Finalize() Result {
	if !e.done {
		e.done = true
	}
	if e.nowUs < e.cfg.Workload.DurationUs {
		e.nowUs = e.cfg.Workload.DurationUs
	}
	if e.generated-e.succeeded-e.failed > 0 {
		e.nowUs = e.cfg.Workload.DurationUs + e.cfg.Workload.DrainTimeoutUs
	}
	if len(e.snapshots) == 0 || e.snapshots[len(e.snapshots)-1].AtUs != e.nowUs {
		e.snapshots = append(e.snapshots, e.sampleAt(e.nowUs))
	}
	var p95, p99 *int64
	if len(e.latencies) > 0 {
		values := append([]int64(nil), e.latencies...)
		sort.Slice(values, func(i, j int) bool { return values[i] < values[j] })
		rank := (95*len(values)+99)/100 - 1
		v := values[rank]
		p95 = &v
		p99Value := values[(99*len(values)+99)/100-1]
		p99 = &p99Value
	}
	unfinished := e.generated - e.succeeded - e.failed
	result := Result{ResultSchemaVersion: 1, EngineVersion: EngineVersion, Status: "completed", Snapshots: append([]Snapshot(nil), e.snapshots...), Summary: Summary{Generated: e.generated, Succeeded: e.succeeded, Failed: e.failed, Unfinished: unfinished, ProcessedEvents: e.processed, P95LatencyUs: p95, P99LatencyUs: p99}}
	if unfinished > 0 {
		result.Warnings = append(result.Warnings, "requests remained in flight at the drain deadline")
	}
	return result
}

func (e *Engine) Run(ctx context.Context) (Result, error) {
	for {
		select {
		case <-ctx.Done():
			return Result{}, ctx.Err()
		default:
		}
		_, done, err := e.Step()
		if err != nil {
			return Result{}, err
		}
		if done {
			return e.Finalize(), nil
		}
	}
}

func (e *Engine) RunUntil(targetUs int64) ([]Snapshot, error) {
	if targetUs < e.nowUs {
		return nil, errors.New("target precedes current time")
	}
	before := len(e.snapshots)
	for len(e.events) > 0 && e.events[0].atUs <= targetUs {
		_, done, err := e.Step()
		if err != nil {
			return nil, err
		}
		if done {
			break
		}
	}
	return append([]Snapshot(nil), e.snapshots[before:]...), nil
}

// RunBatch advances at most maxEvents events so a browser worker can service
// cancellation messages between calls. It never uses wall-clock time.
func (e *Engine) RunBatch(maxEvents int) (BatchOutcome, error) {
	if maxEvents < 1 || maxEvents > 10_000 {
		return BatchOutcome{}, errors.New("batch size must be 1–10000")
	}
	if e.done {
		result := e.Finalize()
		return BatchOutcome{Done: true, Result: &result}, nil
	}
	start := e.processed
	frames := make([]Snapshot, 0)
	for e.processed-start < maxEvents {
		frame, done, err := e.Step()
		if err != nil {
			return BatchOutcome{}, err
		}
		if frame != nil {
			frames = append(frames, *frame)
		}
		if done {
			result := e.Finalize()
			return BatchOutcome{Snapshots: frames, ProcessedEvents: e.processed - start, Done: true, Result: &result}, nil
		}
	}
	return BatchOutcome{Snapshots: frames, ProcessedEvents: e.processed - start}, nil
}

// RunBatchUntilSnapshot keeps the browser worker responsive while advancing a
// live simulation. It stops as soon as a new snapshot is available so the
// worker can pace snapshots against wall-clock time.
func (e *Engine) RunBatchUntilSnapshot(maxEvents int) (BatchOutcome, error) {
	if maxEvents < 1 || maxEvents > 10_000 {
		return BatchOutcome{}, errors.New("batch size must be 1–10000")
	}
	if e.done {
		result := e.Finalize()
		return BatchOutcome{Done: true, Result: &result}, nil
	}
	start := e.processed
	for e.processed-start < maxEvents {
		frame, done, err := e.Step()
		if err != nil {
			return BatchOutcome{}, err
		}
		if frame != nil {
			return BatchOutcome{Snapshots: []Snapshot{*frame}, ProcessedEvents: e.processed - start}, nil
		}
		if done {
			result := e.Finalize()
			return BatchOutcome{ProcessedEvents: e.processed - start, Done: true, Result: &result}, nil
		}
	}
	return BatchOutcome{ProcessedEvents: e.processed - start}, nil
}
