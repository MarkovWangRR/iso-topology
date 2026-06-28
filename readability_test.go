package isotopo

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestReadabilityRanksCorpus is the Phase-0 gate (docs/design/layout-engine-
// master-plan.md): the readability objective R must rank every human-labeled
// GOOD scene above every BAD scene in the benchmark corpus. This is the
// foundation the whole engine is driven by — if R can't tell good from bad,
// nothing downstream can optimize toward it.
func TestReadabilityRanksCorpus(t *testing.T) {
	const dir = "samples/bench"
	f, err := os.Open(filepath.Join(dir, "labels.txt"))
	if err != nil {
		t.Fatalf("open labels: %v", err)
	}
	defer f.Close()

	minGood, maxBad := 2.0, -1.0
	worstGood, bestBad := "", ""
	n := 0
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		label, file := fields[0], fields[1]
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil {
			t.Fatalf("read %s: %v", file, err)
		}
		doc, err := Parse(data)
		if err != nil {
			t.Fatalf("parse %s: %v", file, err)
		}
		r := Readability(doc).Score
		n++
		switch label {
		case "good":
			if r < minGood {
				minGood, worstGood = r, file
			}
		case "bad":
			if r > maxBad {
				maxBad, bestBad = r, file
			}
		default:
			t.Fatalf("%s: unknown label %q (want good|bad)", file, label)
		}
	}
	if n < 2 {
		t.Fatalf("corpus too small: %d labeled scenes", n)
	}
	if minGood <= maxBad {
		t.Fatalf("readability gate FAILED: worst good %q R=%.3f <= best bad %q R=%.3f",
			worstGood, minGood, bestBad, maxBad)
	}
	t.Logf("gate OK: worst good %q R=%.3f > best bad %q R=%.3f", worstGood, minGood, bestBad, maxBad)
}
