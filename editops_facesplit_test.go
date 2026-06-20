package isotopo

import (
	"strings"
	"testing"
)

// The detail editor's bool fields (effects.faceSplit) must round-trip as a YAML
// boolean, NOT the quoted string "true" (which fails to parse into *bool). And
// toggling off (empty value) must remove the key.
func TestSetFieldFaceSplitBareAndRemove(t *testing.T) {
	src := []byte("nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: card, shape: rectangle, geom: { w: 120, d: 120, h: 60 }, style: { effects: { cornerRadius: 14 } } }\n")

	on, err := ApplyOpText("yaml", src, EditOp{Kind: "set-field", Target: "node", ID: "card",
		Fields: map[string]string{"style.effects.faceSplit": "true"}})
	if err != nil {
		t.Fatalf("set faceSplit: %v", err)
	}
	if !strings.Contains(string(on), "faceSplit: true") {
		t.Fatalf("expected bare `faceSplit: true`, got:\n%s", on)
	}
	if strings.Contains(string(on), `faceSplit: "true"`) {
		t.Fatalf("faceSplit written as a quoted string — would not parse into *bool")
	}
	doc, perr := Parse(on)
	if perr != nil {
		t.Fatalf("doc with faceSplit must parse: %v", perr)
	}
	if eff := ResolvePartStyle(doc, "card"); eff == nil || eff.Effects == nil || eff.Effects.FaceSplit == nil || !*eff.Effects.FaceSplit {
		t.Fatalf("faceSplit did not resolve to bool true")
	}

	off, err := ApplyOpText("yaml", on, EditOp{Kind: "set-field", Target: "node", ID: "card",
		Fields: map[string]string{"style.effects.faceSplit": ""}})
	if err != nil {
		t.Fatalf("clear faceSplit: %v", err)
	}
	if strings.Contains(string(off), "faceSplit") {
		t.Fatalf("toggling off should remove the key, got:\n%s", off)
	}
}
