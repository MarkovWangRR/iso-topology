package isotopo

import "testing"

func lbl(id string) *CompositePart {
	sz := 11.0
	return &CompositePart{
		Shape: "iso_text", groupLabel: true, Label: id,
		Offset: &WorldPoint{WX: 10, WY: 100, WZ: 0},
		Style:  &Style{Text: &Text{Size: &sz}},
	}
}

func tallNodeAt(wx, wy float64) *CompositePart {
	return &CompositePart{
		Shape: "rectangle", Label: "N",
		Geom:   &Geom{W: 60, D: 30, H: 120},
		Offset: &WorldPoint{WX: wx, WY: wy, WZ: 0},
	}
}

func TestOcclusion_LiftsCoveredLabel(t *testing.T) {
	label := lbl("Lane")
	cover := tallNodeAt(0, 90) // overlaps the label's projected box, painted later
	out := resolveGroupLabelOcclusion([]*CompositePart{label, cover})
	if out[len(out)-1] != label {
		t.Fatalf("an occluded group label must be repainted last; order = %v",
			[]string{out[0].Label, out[1].Label})
	}
}

func TestOcclusion_LeavesClearLabelInPlace(t *testing.T) {
	label := lbl("Lane")
	far := tallNodeAt(900, 900) // nowhere near the label, no occlusion
	out := resolveGroupLabelOcclusion([]*CompositePart{label, far})
	if out[0] != label {
		t.Fatal("a label that nothing covers must keep its original position")
	}
}

func TestOcclusion_SubstrateNeverOccludes(t *testing.T) {
	label := lbl("Lane")
	slab := tallNodeAt(0, 90)
	slab.isSubstrate = true // the slab a label sits on is not an occluder
	slab.Shape = "group"
	out := resolveGroupLabelOcclusion([]*CompositePart{label, slab})
	if out[0] != label {
		t.Fatal("a substrate/slab under the label must not count as occlusion")
	}
}
