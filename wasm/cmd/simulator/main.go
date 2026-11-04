//go:build js && wasm

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"syscall/js"

	"playhld/simulation/engine"
	"playhld/simulation/goals"
	"playhld/simulation/model"
)

var handles = map[string]*engine.Engine{}
var nextHandle uint64
var callbacks []js.Func

type response struct {
	OK      bool                 `json:"ok"`
	Handle  string               `json:"handle,omitempty"`
	Outcome *engine.BatchOutcome `json:"outcome,omitempty"`
	Goals   json.RawMessage      `json:"goals,omitempty"`
	State   string               `json:"state,omitempty"`
	Error   string               `json:"error,omitempty"`
}

func encode(v response) string     { b, _ := json.Marshal(v); return string(b) }
func errResponse(err error) string { return encode(response{Error: err.Error()}) }

func register(name string, fn func([]js.Value) string) {
	cb := js.FuncOf(func(_ js.Value, args []js.Value) (result any) {
		defer func() {
			if recovered := recover(); recovered != nil {
				result = errResponse(fmt.Errorf("invalid JS input"))
			}
		}()
		return fn(args)
	})
	callbacks = append(callbacks, cb)
	js.Global().Set(name, cb)
}

func main() {
	register("playHLDCreate", func(args []js.Value) string {
		if len(args) != 1 || args[0].Type() != js.TypeString {
			return errResponse(fmt.Errorf("expected config JSON string"))
		}
		data := []byte(args[0].String())
		if len(data) > model.MaxPayloadBytes {
			return errResponse(fmt.Errorf("config exceeds payload limit"))
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.DisallowUnknownFields()
		var cfg model.SimulationConfig
		if err := decoder.Decode(&cfg); err != nil {
			return errResponse(err)
		}
		if decoder.Decode(new(any)) != io.EOF {
			return errResponse(fmt.Errorf("multiple JSON values"))
		}
		e, err := engine.NewEngine(cfg)
		if err != nil {
			return errResponse(err)
		}
		nextHandle++
		id := strconv.FormatUint(nextHandle, 10)
		handles[id] = e
		return encode(response{OK: true, Handle: id})
	})
	register("playHLDStep", func(args []js.Value) string {
		if len(args) != 2 {
			return errResponse(fmt.Errorf("expected handle and batch size"))
		}
		handle := args[0].String()
		e, ok := handles[handle]
		if !ok {
			return errResponse(fmt.Errorf("unknown simulation handle"))
		}
		out, err := e.RunBatch(args[1].Int())
		if err != nil {
			return errResponse(err)
		}
		return encode(response{OK: true, Outcome: &out})
	})
	register("playHLDStepUntilSnapshot", func(args []js.Value) string {
		if len(args) != 2 {
			return errResponse(fmt.Errorf("expected handle and batch size"))
		}
		e, ok := handles[args[0].String()]
		if !ok {
			return errResponse(fmt.Errorf("unknown simulation handle"))
		}
		out, err := e.RunBatchUntilSnapshot(args[1].Int())
		if err != nil {
			return errResponse(err)
		}
		return encode(response{OK: true, Outcome: &out})
	})
	register("playHLDUpdateArchitecture", func(args []js.Value) string {
		if len(args) != 2 || args[1].Type() != js.TypeString {
			return errResponse(fmt.Errorf("expected handle and architecture JSON"))
		}
		e, ok := handles[args[0].String()]
		if !ok {
			return errResponse(fmt.Errorf("unknown simulation handle"))
		}
		data := []byte(args[1].String())
		architecture, err := model.ParseArchitecture(data)
		if err != nil {
			return errResponse(err)
		}
		if err := e.ApplyArchitecture(architecture); err != nil {
			return errResponse(err)
		}
		return encode(response{OK: true})
	})
	register("playHLDExportState", func(args []js.Value) string {
		if len(args) != 1 {
			return errResponse(fmt.Errorf("expected handle"))
		}
		e, ok := handles[args[0].String()]
		if !ok {
			return errResponse(fmt.Errorf("unknown simulation handle"))
		}
		state, err := e.ExportFunctionalState()
		if err != nil {
			return errResponse(err)
		}
		return encode(response{OK: true, State: string(state)})
	})
	register("playHLDImportState", func(args []js.Value) string {
		if len(args) != 2 || args[1].Type() != js.TypeString {
			return errResponse(fmt.Errorf("expected handle and state JSON"))
		}
		e, ok := handles[args[0].String()]
		if !ok {
			return errResponse(fmt.Errorf("unknown simulation handle"))
		}
		if err := e.ImportFunctionalState([]byte(args[1].String())); err != nil {
			return errResponse(err)
		}
		return encode(response{OK: true})
	})
	register("playHLDReset", func(args []js.Value) string {
		if len(args) != 1 {
			return errResponse(fmt.Errorf("expected handle"))
		}
		e, ok := handles[args[0].String()]
		if !ok {
			return errResponse(fmt.Errorf("unknown simulation handle"))
		}
		if err := e.Reset(); err != nil {
			return errResponse(err)
		}
		return encode(response{OK: true})
	})
	register("playHLDDispose", func(args []js.Value) string {
		if len(args) != 1 {
			return errResponse(fmt.Errorf("expected handle"))
		}
		delete(handles, args[0].String())
		return encode(response{OK: true})
	})
	register("playHLDEvaluateGoals", func(args []js.Value) string {
		if len(args) != 3 {
			return errResponse(fmt.Errorf("expected result, workload and goals JSON"))
		}
		var result engine.Result
		var workload model.Workload
		var rules []goals.Rule
		if err := json.Unmarshal([]byte(args[0].String()), &result); err != nil {
			return errResponse(err)
		}
		if err := json.Unmarshal([]byte(args[1].String()), &workload); err != nil {
			return errResponse(err)
		}
		if err := json.Unmarshal([]byte(args[2].String()), &rules); err != nil {
			return errResponse(err)
		}
		outcomes, err := goals.Evaluate(result, workload, rules)
		if err != nil {
			return errResponse(err)
		}
		data, err := json.Marshal(outcomes)
		if err != nil {
			return errResponse(err)
		}
		return encode(response{OK: true, Goals: data})
	})
	js.Global().Set("playHLDReady", true)
	select {}
}
