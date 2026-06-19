package isotopo

import (
	"math/rand"
	"testing"
)

// runEditSession applies a deterministic, seeded chain of random ops (each fed
// the previous result, like Studio's accumulation) and enforces the floor
// invariants after EVERY step. This is the workhorse layer: it spans the edit
// axes via genScene/genOp and catches corruption / panics / error-contract
// breaks that only emerge under accumulation.
func runEditSession(t *testing.T, seed int64, steps int) {
	t.Helper()
	rng := rand.New(rand.NewSource(seed))
	src := genScene(rng)
	if _, iss, _ := RenderSource("yaml", []byte(src)); hasErr(iss) {
		return // generator produced a non-clean base — skip (rare); not under test
	}
	trace := []EditOp{}
	for i := 0; i < steps; i++ {
		op := genOp(rng, src)
		trace = append(trace, op)
		var out []byte
		var err error
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("seed %d step %d op %+v PANIC: %v\ntrace=%+v\nsrc=\n%s", seed, i, op, r, trace, src)
				}
			}()
			out, err = ApplyOpText("yaml", []byte(src), op)
		}()
		if v := checkFloorInvariants(src, out, err); v != "" {
			t.Fatalf("seed %d step %d op %+v\n%s\ntrace=%+v\nsrc=\n%s\nout=\n%s", seed, i, op, v, trace, src, out)
		}
		if err == nil {
			src = string(out)
		}
	}
}

// TestEditSession_FloorInvariants runs many seeded cumulative sessions. A
// failure prints the exact seed + op-trace for a deterministic minimal repro.
func TestEditSession_FloorInvariants(t *testing.T) {
	steps := 25
	seeds := 120
	if testing.Short() {
		seeds = 20
	}
	for seed := int64(1); seed <= int64(seeds); seed++ {
		runEditSession(t, seed, steps)
	}
}
