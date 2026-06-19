package isotopo

import (
	"math/rand"
	"testing"
)

// FuzzEditSession is the coverage-guided discovery layer. It drives the SAME
// generator + invariant library as the deterministic sequence test, but lets Go
// mutate the (seed, steps) inputs to explore op-chains we didn't enumerate. A
// crasher is reproducible from the printed seed and should be minimized into a
// deterministic regression test in bugfix_regression_test.go.
//
//	go test -run x -fuzz FuzzEditSession -fuzztime 60s
func FuzzEditSession(f *testing.F) {
	f.Add(int64(1), 12)
	f.Add(int64(7), 25)
	f.Add(int64(42), 40)
	f.Fuzz(func(t *testing.T, seed int64, steps int) {
		if steps < 1 || steps > 80 {
			t.Skip()
		}
		rng := rand.New(rand.NewSource(seed))
		src := genScene(rng)
		if _, iss, _ := RenderSource("yaml", []byte(src)); hasErr(iss) {
			t.Skip() // non-clean base — not under test
		}
		for i := 0; i < steps; i++ {
			op := genOp(rng, src)
			out, err := ApplyOpText("yaml", []byte(src), op) // a panic auto-fails the fuzz case
			if v := checkFloorInvariants(src, out, err); v != "" {
				t.Fatalf("seed=%d step=%d op=%+v: %s\nsrc=\n%s\nout=\n%s", seed, i, op, v, src, out)
			}
			if err == nil {
				src = string(out)
			}
		}
	})
}
