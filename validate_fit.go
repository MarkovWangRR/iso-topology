package isotopo

import "fmt"

// containerFitLimit is the fraction of a FIXED-size container's footprint a
// single child may occupy before it reads as bursting the slab. Auto-sized
// containers grow to wrap their children, so this only applies to containers the
// author pinned with an explicit geom.w/d.
const containerFitLimit = 0.90

// ContainerFitIssues warns when a child's EFFECTIVE footprint exceeds 90% of a
// fixed-size parent container on either axis — the "child bursting its tray /
// child nearly as big as its parent" defect. It reads the authored tree (auto
// containers have no fixed size yet) and uses partFootprint, so a clamped shape
// like a cloud (rendered floor 200×140) is measured at its true drawn size, not
// its smaller authored geom.
func ContainerFitIssues(doc *Document) []Issue {
	if doc == nil {
		return nil
	}
	var issues []Issue
	for nodeID, n := range doc.Nodes {
		if n == nil || n.Shape != "composite" {
			continue
		}
		walkContainerFit(n.Parts, fmt.Sprintf("nodes.%s", nodeID), &issues)
	}
	return issues
}

func walkContainerFit(parts []*CompositePart, path string, issues *[]Issue) {
	for i, p := range parts {
		if p == nil {
			continue
		}
		ppath := fmt.Sprintf("%s.parts[%d]", path, i)
		if p.ID != "" {
			ppath = fmt.Sprintf("%s.parts[%s]", path, p.ID)
		}
		// A fixed-size container is one the author pinned with geom.w and/or d.
		if isContainerShape(p.Shape) && p.Geom != nil && (p.Geom.W > 0 || p.Geom.D > 0) {
			pid := p.ID
			if pid == "" {
				pid = p.Shape
			}
			for _, ch := range p.Parts {
				if ch == nil || ch.ID == "" {
					continue
				}
				cw, cd := partFootprint(ch) // effective (clamped) footprint
				if p.Geom.W > 0 && cw > containerFitLimit*p.Geom.W {
					*issues = append(*issues, Issue{
						Severity: SeverityWarning,
						Path:     ppath,
						Message: fmt.Sprintf("child %q is %.0f wide, over %.0f%% of fixed container %q (%.0f wide) — it bursts the slab; enlarge the container or shrink the child",
							ch.ID, cw, containerFitLimit*100, pid, p.Geom.W),
						Suggest: "give the container a larger geom.w/d, or shrink the child / drop its explicit size so the container auto-sizes",
					})
				}
				if p.Geom.D > 0 && cd > containerFitLimit*p.Geom.D {
					*issues = append(*issues, Issue{
						Severity: SeverityWarning,
						Path:     ppath,
						Message: fmt.Sprintf("child %q is %.0f deep, over %.0f%% of fixed container %q (%.0f deep) — it bursts the slab; enlarge the container or shrink the child",
							ch.ID, cd, containerFitLimit*100, pid, p.Geom.D),
						Suggest: "give the container a larger geom.w/d, or shrink the child / drop its explicit size so the container auto-sizes",
					})
				}
			}
		}
		if len(p.Parts) > 0 {
			walkContainerFit(p.Parts, ppath, issues)
		}
	}
}
