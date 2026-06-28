package isotopo

import (
	"strings"
	"testing"
)

func nodeWithID(id string, wx, wy float64) *CompositePart {
	n := tallNodeAt(wx, wy)
	n.ID = id
	return n
}

// A later-painted node covering a group label must produce a warning naming it.
func TestLabelOcclusion_Warns(t *testing.T) {
	label := lbl("Lane")
	cover := nodeWithID("api", 0, 90) // overlaps the label's projected box
	issues := labelOcclusionInFlat([]*CompositePart{label, cover}, nil, "scene")
	if len(issues) == 0 {
		t.Fatal("a node covering a group label must be flagged")
	}
	m := issues[0].Message
	if !strings.Contains(m, "Lane") || !strings.Contains(m, `"api"`) || !strings.Contains(m, "covered") {
		t.Fatalf("warning must name the label and occluder, got %q", m)
	}
}

// A label nothing covers stays silent.
func TestLabelOcclusion_ClearIsSilent(t *testing.T) {
	label := lbl("Lane")
	far := nodeWithID("api", 900, 900)
	if issues := labelOcclusionInFlat([]*CompositePart{label, far}, nil, "scene"); len(issues) != 0 {
		t.Fatalf("a clear label must not warn, got %v", issues)
	}
}

// A substrate/slab under the label is not an occluder.
func TestLabelOcclusion_SubstrateIgnored(t *testing.T) {
	label := lbl("Lane")
	slab := nodeWithID("tray", 0, 90)
	slab.isSubstrate = true
	slab.Shape = "group"
	if issues := labelOcclusionInFlat([]*CompositePart{label, slab}, nil, "scene"); len(issues) != 0 {
		t.Fatalf("a substrate must not be flagged as occluding, got %v", issues)
	}
}
