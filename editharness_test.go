package isotopo

import (
	"context"
	"fmt"
	"math/rand"
	"strings"
)

// ─────────────────────────────────────────────────────────────────────────────
// SYSTEMATIC EDIT-ENGINE TEST HARNESS
//
// Bugs in the stateless edit engine lived at the INTERSECTION of independent
// axes — operation × YAML form × container kind × value class × reference
// topology — not in any single "feature". This harness makes those axes
// first-class: a generator spans them, a shared invariant library is the oracle,
// and the matrix / sequence / fuzz layers all reuse both. Discover with fuzz,
// lock with the deterministic layers; every layer asserts the SAME invariants.
// ─────────────────────────────────────────────────────────────────────────────

// ── Axes ─────────────────────────────────────────────────────────────────────

// containerKinds: every container flavour an edit must stay correct inside.
var containerKinds = []string{"plain", "offset", "autosize", "layout-row", "layout-col", "layout-grid", "boundary", "nested"}

// leafShapes: a spread across renderers (box/path/ellipse families).
var leafShapes = []string{"rectangle", "cylinder", "cloud", "person", "hexprism", "diamond", "sphere"}

// hostileValues: value classes that historically broke escaping / parsing.
var hostileValues = []string{
	`R&D <x>`, `"><b>`, "ctrl\x01here", "null", "true", "123", "a:b", "x,y", "emoji✓ünïcode", "",
}

// opKinds: the user operations a session draws from.
var opKinds = []string{"move", "reparent", "set-field-label", "set-field-style", "set-field-shape", "set-field-id", "add", "add-edge", "delete", "duplicate"}

// ── Generator ────────────────────────────────────────────────────────────────

// genScene builds a random but VALID composite document spanning the container
// axis and both YAML forms (flow vs block). The base always validates clean, so
// any invariant violation downstream is attributable to the edit, not the seed.
func genScene(rng *rand.Rand) string {
	var b strings.Builder
	b.WriteString("nodes:\n  scene:\n    shape: composite\n    parts:\n")
	leafIDs := []string{}
	emitLeaf := func(indent, id string, flow bool, off [2]int) {
		shape := leafShapes[rng.Intn(len(leafShapes))]
		if flow {
			fmt.Fprintf(&b, "%s- { id: %s, shape: %s, geom: { w: %d, d: %d, h: %d }, offset: { wx: %d, wy: %d } }\n",
				indent, id, shape, 60+rng.Intn(60), 50+rng.Intn(50), 24+rng.Intn(24), off[0], off[1])
		} else {
			fmt.Fprintf(&b, "%s- id: %s\n%s  shape: %s\n%s  geom: { w: %d, d: %d, h: %d }\n%s  offset: { wx: %d, wy: %d }\n",
				indent, id, indent, shape, indent, 60+rng.Intn(60), 50+rng.Intn(50), 24+rng.Intn(24), indent, off[0], off[1])
		}
		leafIDs = append(leafIDs, id)
	}
	ng := 2 + rng.Intn(2)
	for g := 0; g < ng; g++ {
		gid := fmt.Sprintf("g%d", g)
		kind := containerKinds[rng.Intn(len(containerKinds))]
		gx, gy := rng.Intn(400), rng.Intn(300)
		fmt.Fprintf(&b, "      - id: %s\n        shape: %s\n", gid, map[bool]string{true: "boundary", false: "group"}[kind == "boundary"])
		switch kind {
		case "autosize":
			b.WriteString("        geom: { h: 6 }\n")
		default:
			fmt.Fprintf(&b, "        geom: { w: %d, d: %d, h: 6 }\n", 240+rng.Intn(160), 160+rng.Intn(120))
		}
		fmt.Fprintf(&b, "        offset: { wx: %d, wy: %d }\n", gx, gy)
		switch kind {
		case "layout-row":
			b.WriteString("        layout: { mode: row, gap: 24 }\n")
		case "layout-col":
			b.WriteString("        layout: { mode: column, gap: 24 }\n")
		case "layout-grid":
			b.WriteString("        layout: { mode: grid, cols: 2, gap: 24 }\n")
		}
		b.WriteString("        parts:\n")
		nc := 1 + rng.Intn(3)
		for c := 0; c < nc; c++ {
			cid := fmt.Sprintf("%s_c%d", gid, c)
			if kind == "nested" && c == 0 {
				// a nested sub-group with its own child
				fmt.Fprintf(&b, "          - id: %s_sub\n            shape: group\n            geom: { w: 160, d: 120, h: 6 }\n            offset: { wx: 20, wy: 20 }\n            parts:\n", gid)
				emitLeaf("              ", cid, rng.Intn(2) == 0, [2]int{20, 20})
				continue
			}
			emitLeaf("          ", cid, rng.Intn(2) == 0, [2]int{20 + c*100, 20})
		}
	}
	// a couple of root-level leaves
	for r := 0; r < 1+rng.Intn(2); r++ {
		emitLeaf("      ", fmt.Sprintf("r%d", r), rng.Intn(2) == 0, [2]int{rng.Intn(300), 360 + rng.Intn(80)})
	}
	// connectors among random leaf pairs
	if len(leafIDs) >= 2 {
		b.WriteString("    connectors:\n")
		for e := 0; e < 1+rng.Intn(3); e++ {
			a := leafIDs[rng.Intn(len(leafIDs))]
			z := leafIDs[rng.Intn(len(leafIDs))]
			fmt.Fprintf(&b, "      - { from: %s, to: %s }\n", a, z)
		}
	}
	return b.String()
}

// allPartIDs parses src and returns every part id (nil on parse failure).
func allPartIDs(src string) []string {
	doc, err := Parse([]byte(src))
	if err != nil || doc.Scene() == nil {
		return nil
	}
	var ids []string
	var walk func(ps []*CompositePart)
	walk = func(ps []*CompositePart) {
		for _, p := range ps {
			if p != nil {
				if p.ID != "" {
					ids = append(ids, p.ID)
				}
				walk(p.Parts)
			}
		}
	}
	walk(doc.Scene().Parts)
	return ids
}

// groupIDs returns the ids that are containers (valid reparent targets).
func groupIDs(src string) []string {
	doc, err := Parse([]byte(src))
	if err != nil || doc.Scene() == nil {
		return nil
	}
	var ids []string
	var walk func(ps []*CompositePart)
	walk = func(ps []*CompositePart) {
		for _, p := range ps {
			if p != nil {
				if p.ID != "" && isContainerShape(p.Shape) {
					ids = append(ids, p.ID)
				}
				walk(p.Parts)
			}
		}
	}
	walk(doc.Scene().Parts)
	return ids
}

// connectorCount returns how many connectors the scene has.
func connectorCount(src string) int {
	doc, err := Parse([]byte(src))
	if err != nil || doc.Scene() == nil {
		return 0
	}
	return len(doc.Scene().Connectors)
}

// genOp produces a random EditOp targeting the current document, drawing across
// the operation, value, and reference axes. occasionally targets a hostile value
// or a nonexistent id to exercise the error paths too.
func genOp(rng *rand.Rand, src string) EditOp {
	ids := allPartIDs(src)
	gids := groupIDs(src)
	pick := func(xs []string) string {
		if len(xs) == 0 {
			return "ghost"
		}
		return xs[rng.Intn(len(xs))]
	}
	hostile := func() string { return hostileValues[rng.Intn(len(hostileValues))] }
	switch opKinds[rng.Intn(len(opKinds))] {
	case "move":
		return EditOp{Kind: "move", Target: "node", ID: pick(ids), DWX: float64(rng.Intn(200) - 100), DWY: float64(rng.Intn(200) - 100)}
	case "reparent":
		tgt := ""
		if len(gids) > 0 && rng.Intn(2) == 0 {
			tgt = gids[rng.Intn(len(gids))]
		}
		return EditOp{Kind: "reparent", ID: pick(ids), Target: tgt}
	case "set-field-label":
		return EditOp{Kind: "set-field", Target: "node", ID: pick(ids), Fields: map[string]string{"label": hostile()}}
	case "set-field-style":
		return EditOp{Kind: "set-field", Target: "node", ID: pick(ids), Fields: map[string]string{"style.palette.top": hostile()}}
	case "set-field-shape":
		return EditOp{Kind: "set-field", Target: "node", ID: pick(ids), Fields: map[string]string{"shape": leafShapes[rng.Intn(len(leafShapes))]}}
	case "set-field-id":
		return EditOp{Kind: "set-field", Target: "node", ID: pick(ids), Fields: map[string]string{"id": fmt.Sprintf("rn%d", rng.Intn(1000))}}
	case "add":
		return EditOp{Kind: "add"}
	case "add-edge":
		return EditOp{Kind: "add-edge", Fields: map[string]string{"from": pick(ids), "to": pick(ids)}}
	case "delete":
		if rng.Intn(3) == 0 && connectorCount(src) > 0 {
			return EditOp{Kind: "delete", Target: "edge", CI: rng.Intn(connectorCount(src))}
		}
		return EditOp{Kind: "delete", Target: "node", ID: pick(ids)}
	default: // duplicate
		return EditOp{Kind: "duplicate", Target: "node", ID: pick(ids)}
	}
}

// ── Invariant library (shared oracle) ────────────────────────────────────────

// invViolation is returned (non-empty) when a floor invariant is broken. The
// floor holds for ANY op on ANY doc and never false-fails, so it is safe to run
// over random sequences and fuzz input. Richer semantic invariants (position,
// idempotency, injection) live in the deterministic matrix/property tests where
// the input is controlled.
//
//	F1 no panic                — recovered by the caller
//	F2 out always re-parses    — success ⇒ safety net; error ⇒ original (parseable)
//	F3 err ⇒ output unchanged  — a refused op must not mutate the source
//	F4 no NEW duplicate id     — only `duplicate` may add an id, never a collision
func checkFloorInvariants(before string, out []byte, err error) string {
	o := string(out)
	if err != nil {
		if o != before {
			return "F3: op returned an error but still mutated the source"
		}
		// the unchanged source parsed before, so nothing more to check
		return ""
	}
	if _, perr := LoadInput(context.Background(), "yaml", out, LayoutDagre); perr != nil {
		return fmt.Sprintf("F2: successful op produced an unparseable document: %v", perr)
	}
	if dup := firstDuplicateID(o); dup != "" {
		return fmt.Sprintf("F4: op introduced a duplicate part id %q", dup)
	}
	return ""
}

// firstDuplicateID returns a part id that appears more than once (rendered would
// be ambiguous), or "" if all are unique.
func firstDuplicateID(src string) string {
	seen := map[string]bool{}
	for _, id := range allPartIDs(src) {
		if seen[id] {
			return id
		}
		seen[id] = true
	}
	return ""
}
