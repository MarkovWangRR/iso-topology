package isotopo

import (
	"os"
	"strings"
	"testing"
)

// TestLICENSEIsCanonicalApache guards against LICENSE drifting away from the
// verbatim Apache-2.0 text. Issue #4: two clauses had been reworded, which
// dropped pkg.go.dev's license-classifier confidence below its threshold, so
// the license was reported as UNKNOWN and ALL package documentation was hidden
// module-wide ("Documentation not displayed due to license restrictions").
//
// The license body must be applied verbatim; only the appendix copyright line
// may be filled in. These assertions fail if anyone re-edits the license body
// into a near-Apache paraphrase again.
func TestLICENSEIsCanonicalApache(t *testing.T) {
	b, err := os.ReadFile("LICENSE")
	if err != nil {
		t.Fatalf("read LICENSE: %v", err)
	}
	text := string(b)

	// Canonical phrases that the previous corruption removed. Their presence is
	// what keeps licensecheck's Apache-2.0 match above pkg.go.dev's threshold.
	mustContain := []string{
		"Apache License",
		"Version 2.0, January 2004",
		// §4(d) — the dropped "reasonable and customary use in".
		"except as required for reasonable and customary use in describing the",
		// §9 — the canonical heading + body (was "Accepting Warranty or Support").
		"9. Accepting Warranty or Additional Liability. While redistributing",
		"accepting any such warranty or additional liability.",
		"APPENDIX: How to apply the Apache License to your work.",
	}
	for _, s := range mustContain {
		if !strings.Contains(text, s) {
			t.Errorf("LICENSE is missing canonical Apache-2.0 text %q — the license body must be verbatim (see issue #4)", s)
		}
	}

	// Paraphrases introduced by the corruption must never reappear.
	mustNotContain := []string{
		"Accepting Warranty or Support",
		"warranty or support.",
		"except as required for describing the origin of the Work",
	}
	for _, s := range mustNotContain {
		if strings.Contains(text, s) {
			t.Errorf("LICENSE contains a non-canonical Apache-2.0 paraphrase %q — this is what tripped pkg.go.dev (issue #4)", s)
		}
	}
}
