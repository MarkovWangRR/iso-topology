package isotopo

import (
	"fmt"
	"github.com/MarkovWangRR/iso-topology/iso25d"
	"sort"
	"strings"
)

// Severity is how loud an Issue is. Errors mean the document won't
// render correctly; warnings mean it'll render but probably not how
// the author intended.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
)

// Issue is one structural problem found by Validate. Path is a
// JSONPath-style locator (e.g. "nodes.scene.parts[3].shape"); Message
// is one line; Suggest is the closest valid value when relevant.
//
// Designed for agent self-correction: an agent reads JSON-serialized
// Issues, finds the path, applies the suggestion, and re-validates —
// no human in the loop.
type Issue struct {
	Severity Severity `json:"severity"`
	Path     string   `json:"path"`
	Message  string   `json:"message"`
	Suggest  string   `json:"suggest,omitempty"`
}

// Validate runs structural checks over a parsed Document. It catches:
//   - unknown shape names (with nearest-neighbor suggestion)
//   - annotation anchors that don't resolve to any part id
//   - connector from/to references that don't resolve
//   - unknown canvas.grid values
//   - empty groups (no nested parts) and orphan stack declarations
//
// Validate does NOT check YAML syntax — Parse already does that. It
// runs on a successfully-decoded Document and answers "is this
// document logically coherent?".
func Validate(doc *Document) []Issue {
	if doc == nil {
		return []Issue{{Severity: SeverityError, Path: "$", Message: "document is nil"}}
	}
	var issues []Issue

	// Build the universe of valid ids (every CompositePart in every
	// scene, including nested-in-groups). Connector and annotation
	// references must point into this set.
	//
	// Stack expansion is accounted for: a part with stack.count = 3
	// contributes ids "pods_a", "pods_a~1", "pods_a~2" — same ids the
	// lowering pass will emit at render time. Without this, every
	// existing showcase using stacked replicas would false-fail.
	allIDs := map[string]struct{}{}
	registerID := func(p *CompositePart) {
		if p.ID == "" {
			return
		}
		allIDs[p.ID] = struct{}{}
		if p.Stack != nil && p.Stack.Count > 1 {
			for k := 1; k < p.Stack.Count; k++ {
				allIDs[fmt.Sprintf("%s~%d", p.ID, k)] = struct{}{}
			}
		}
	}
	for _, n := range doc.Nodes {
		walkAtomicParts(n.Parts, registerID)
		// group nodes themselves are addressable too
		var collectGroupIDs func(parts []*CompositePart)
		collectGroupIDs = func(parts []*CompositePart) {
			for _, p := range parts {
				if isContainerShape(p.Shape) {
					registerID(p)
					collectGroupIDs(p.Parts)
				}
			}
		}
		collectGroupIDs(n.Parts)
	}

	// Validate canvas.grid
	if doc.Canvas != nil && doc.Canvas.Grid != "" {
		valid := []string{"iso", "dots", "hatch", "solid", "none"}
		if !contains(valid, strings.ToLower(doc.Canvas.Grid)) {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     "canvas.grid",
				Message:  fmt.Sprintf("unknown grid mode %q", doc.Canvas.Grid),
				Suggest:  nearest(doc.Canvas.Grid, valid),
			})
		}
	}

	validShapes := validShapeList()

	// Walk every Node and its Parts. We collect node and part issues
	// with full JSONPath-style locators.
	for nodeID, n := range doc.Nodes {
		nodePath := fmt.Sprintf("nodes.%s", nodeID)
		if !contains(validShapes, n.Shape) {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     nodePath + ".shape",
				Message:  fmt.Sprintf("unknown shape %q", n.Shape),
				Suggest:  nearest(n.Shape, validShapes),
			})
		}
		for i, p := range n.Parts {
			validatePart(p, fmt.Sprintf("%s.parts[%d]", nodePath, i), validShapes, &issues)
		}
		for i, c := range n.Connectors {
			cPath := fmt.Sprintf("%s.connectors[%d]", nodePath, i)
			if _, ok := allIDs[connectorTarget(c.From)]; !ok && c.From != "" {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Path:     cPath + ".from",
					Message:  fmt.Sprintf("connector references unknown part id %q", c.From),
					Suggest:  nearestID(c.From, allIDs),
				})
			}
			if _, ok := allIDs[connectorTarget(c.To)]; !ok && c.To != "" {
				issues = append(issues, Issue{
					Severity: SeverityError,
					Path:     cPath + ".to",
					Message:  fmt.Sprintf("connector references unknown part id %q", c.To),
					Suggest:  nearestID(c.To, allIDs),
				})
			}
		}
	}

	// v2.5 — preset references must exist in theme.presets.
	presetNames := map[string]struct{}{}
	if doc.Theme != nil {
		for name := range doc.Theme.Presets {
			presetNames[name] = struct{}{}
		}
	}
	checkPreset := func(preset, path string) {
		if preset == "" {
			return
		}
		if _, ok := presetNames[preset]; !ok {
			cand := make([]string, 0, len(presetNames))
			for n := range presetNames {
				cand = append(cand, n)
			}
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     path + ".preset",
				Message:  fmt.Sprintf("preset %q is not defined in theme.presets", preset),
				Suggest:  nearest(preset, cand),
			})
		}
	}
	for nodeID, n := range doc.Nodes {
		checkPreset(n.Preset, "nodes."+nodeID)
		var walkPresets func(parts []*CompositePart, prefix string)
		walkPresets = func(parts []*CompositePart, prefix string) {
			for i, p := range parts {
				if p == nil {
					continue
				}
				pp := fmt.Sprintf("%s.parts[%d]", prefix, i)
				checkPreset(p.Preset, pp)
				walkPresets(p.Parts, pp)
			}
		}
		walkPresets(n.Parts, "nodes."+nodeID)
	}

	// v2.9 — icon URIs must resolve: an unknown iso://… icon renders
	// as a silently-missing image, which is worse than an error.
	checkIcon := func(icon, path string) {
		if icon == "" {
			return
		}
		// Only built-in iso://… refs are validated against the catalog. Any
		// other value (a data: URI, an http(s) URL, or a local file path the
		// renderer inlines) is an external resource — pass it through.
		if !strings.HasPrefix(icon, "iso://") {
			return
		}
		if iso25d.KnownIsoIconURI(icon) {
			return
		}
		issues = append(issues, Issue{
			Severity: SeverityError,
			Path:     path + ".icon",
			Message:  fmt.Sprintf("unknown built-in icon %q", icon),
			Suggest:  nearest(icon, iso25d.IconNames()),
		})
	}
	for nodeID, n := range doc.Nodes {
		checkIcon(n.Icon, "nodes."+nodeID)
		var walkIcons func(parts []*CompositePart, prefix string)
		walkIcons = func(parts []*CompositePart, prefix string) {
			for i, p := range parts {
				if p == nil {
					continue
				}
				pp := fmt.Sprintf("%s.parts[%d]", prefix, i)
				checkIcon(p.Icon, pp)
				walkIcons(p.Parts, pp)
			}
		}
		walkIcons(n.Parts, "nodes."+nodeID)
	}

	// v3.3 — style.faces structural checks.
	checkFaces := func(st *Style, path string) {
		if st == nil || st.Faces == nil {
			return
		}
		kinds := []string{"solid", "linearGradient", "radialGradient", "pattern"}
		for name, face := range st.Faces {
			if face == nil || face.Fill == nil {
				continue
			}
			fp := fmt.Sprintf("%s.faces.%s.fill", path, name)
			f := face.Fill
			if f.Kind != "" && !contains(kinds, f.Kind) {
				issues = append(issues, Issue{
					Severity: SeverityError, Path: fp + ".kind",
					Message: fmt.Sprintf("unknown fill kind %q", f.Kind),
					Suggest: nearest(f.Kind, kinds),
				})
			}
			if strings.Contains(f.Kind, "Gradient") {
				if len(f.Stops) < 2 {
					issues = append(issues, Issue{
						Severity: SeverityError, Path: fp + ".stops",
						Message: "gradient needs at least 2 stops",
					})
				}
				for i, s := range f.Stops {
					if s.Offset < 0 || s.Offset > 1 {
						issues = append(issues, Issue{
							Severity: SeverityError,
							Path:     fmt.Sprintf("%s.stops[%d].offset", fp, i),
							Message:  "offset must be within 0..1",
						})
					}
				}
			}
			if f.Kind == "pattern" && f.Pattern == nil {
				issues = append(issues, Issue{
					Severity: SeverityError, Path: fp + ".pattern",
					Message: "kind: pattern requires a pattern block",
				})
			}
		}
	}
	for nodeID, n := range doc.Nodes {
		checkFaces(n.Style, "nodes."+nodeID+".style")
		var walkFaceStyles func(parts []*CompositePart, prefix string)
		walkFaceStyles = func(parts []*CompositePart, prefix string) {
			for i, p := range parts {
				if p == nil {
					continue
				}
				pp := fmt.Sprintf("%s.parts[%d]", prefix, i)
				checkFaces(p.Style, pp+".style")
				walkFaceStyles(p.Parts, pp)
			}
		}
		walkFaceStyles(n.Parts, "nodes."+nodeID)
	}
	if doc.Theme != nil {
		for name, ps := range doc.Theme.Presets {
			checkFaces(ps, "theme.presets."+name)
		}
	}

	// v2.2 — layout/place dry run (dangling refs, cycles, conflicting
	// constraints → errors; post-solve sibling overlaps → warnings).
	// Runs against a clone so Validate never mutates the document.
	for nodeID, n := range doc.Nodes {
		if n.Shape != "composite" {
			continue
		}
		for _, iss := range layoutIssues(n, doc.Canvas) {
			iss.Path = strings.Replace(iss.Path, "nodes.scene", "nodes."+nodeID, 1)
			issues = append(issues, iss)
		}
	}

	// Annotations — document-level and node-level (v3.0) share checks.
	type annAt struct {
		a    *Annotation
		path string
	}
	var annList []annAt
	for i, a := range doc.Annotations {
		annList = append(annList, annAt{a, fmt.Sprintf("annotations[%d]", i)})
	}
	for nodeID, n := range doc.Nodes {
		for i, a := range n.Annotations {
			annList = append(annList, annAt{a, fmt.Sprintf("nodes.%s.annotations[%d]", nodeID, i)})
		}
	}
	for _, item := range annList {
		a, aPath := item.a, item.path
		if a.Anchor == "" {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     aPath + ".anchor",
				Message:  "annotation requires an anchor (part id)",
			})
			continue
		}
		if _, ok := allIDs[a.Anchor]; !ok {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     aPath + ".anchor",
				Message:  fmt.Sprintf("annotation anchor %q does not match any part id", a.Anchor),
				Suggest:  nearestID(a.Anchor, allIDs),
			})
		}
		if a.Side != "" && !contains([]string{"top", "right", "bottom", "left"}, a.Side) {
			issues = append(issues, Issue{
				Severity: SeverityError,
				Path:     aPath + ".side",
				Message:  fmt.Sprintf("unknown side %q", a.Side),
				Suggest:  nearest(a.Side, []string{"top", "right", "bottom", "left"}),
			})
		}
	}

	return issues
}

// validatePart recurses into one CompositePart's structural checks.
func validatePart(p *CompositePart, path string, validShapes []string, issues *[]Issue) {
	if p == nil {
		return
	}
	if !contains(validShapes, p.Shape) {
		*issues = append(*issues, Issue{
			Severity: SeverityError,
			Path:     path + ".shape",
			Message:  fmt.Sprintf("unknown shape %q", p.Shape),
			Suggest:  nearest(p.Shape, validShapes),
		})
	}
	if isContainerShape(p.Shape) && len(p.Parts) == 0 {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Path:     path + ".parts",
			Message:  "group has no nested parts — renders as an empty substrate",
		})
	}
	if p.Stack != nil && p.Stack.Count <= 0 {
		*issues = append(*issues, Issue{
			Severity: SeverityWarning,
			Path:     path + ".stack.count",
			Message:  "stack.count must be > 0; ignored",
		})
	}
	for i, child := range p.Parts {
		validatePart(child, fmt.Sprintf("%s.parts[%d]", path, i), validShapes, issues)
	}
}

func connectorTarget(ref string) string {
	if dot := strings.Index(ref, "."); dot >= 0 {
		return ref[:dot]
	}
	return ref
}

func validShapeList() []string {
	cap := CapabilityReport()
	set := map[string]struct{}{}
	for _, s := range cap.Shapes {
		set[s.IsoName] = struct{}{}
		for _, alias := range s.AcceptedAs {
			set[alias] = struct{}{}
		}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func contains(haystack []string, needle string) bool {
	for _, h := range haystack {
		if h == needle {
			return true
		}
	}
	return false
}

// nearest returns the candidate with the smallest Levenshtein distance
// to `bad`, but only if that distance is reasonable (≤ 3) — otherwise
// the suggestion would be misleading.
func nearest(bad string, candidates []string) string {
	bad = strings.ToLower(strings.TrimSpace(bad))
	bestScore := 999
	best := ""
	for _, c := range candidates {
		d := levenshtein(bad, strings.ToLower(c))
		if d < bestScore {
			bestScore = d
			best = c
		}
	}
	if bestScore > 3 {
		return ""
	}
	return best
}

func nearestID(bad string, ids map[string]struct{}) string {
	cand := make([]string, 0, len(ids))
	for id := range ids {
		cand = append(cand, id)
	}
	return nearest(bad, cand)
}

// levenshtein is the classical edit-distance. Iterative two-row form.
func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	curr := make([]int, len(b)+1)
	for i := 1; i <= len(a); i++ {
		curr[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			curr[j] = min3(curr[j-1]+1, prev[j]+1, prev[j-1]+cost)
		}
		prev, curr = curr, prev
	}
	return prev[len(b)]
}

func min3(a, b, c int) int {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	return m
}
