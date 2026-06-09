package isotopo

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"oss.terrastruct.com/d2/d2graph"
	"oss.terrastruct.com/d2/d2layouts/d2dagrelayout"
	"oss.terrastruct.com/d2/d2layouts/d2elklayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2target"
	d2log "oss.terrastruct.com/d2/lib/log"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

// LayoutEngine selects which d2 layout engine to use.
//
//	"dagre" — fast, natural-bend polyline edges (default; what most d2 docs use)
//	"elk"   — Eclipse Layout Kernel, defaults to ORTHOGONAL right-angle edge
//	          routing with obstacle avoidance
type LayoutEngine string

const (
	LayoutDagre LayoutEngine = "dagre"
	LayoutELK   LayoutEngine = "elk"
)

// CompileD2 runs d2's parse + compile + chosen layout on the input text
// and returns the laid-out d2target.Diagram. d2lib's internal layers
// expect a slog.Logger on the context — we attach a silent one so we
// don't dump warning stacks during normal renders.
func CompileD2(ctx context.Context, d2text string, engine LayoutEngine) (*d2target.Diagram, error) {
	ctx = d2log.With(ctx, slog.New(slog.NewTextHandler(io.Discard, nil)))
	ruler, err := textmeasure.NewRuler()
	if err != nil {
		return nil, fmt.Errorf("isotopo: textmeasure ruler: %w", err)
	}

	name := string(engine)
	if name == "" {
		name = "dagre"
	}
	layoutResolver := func(name string) (d2graph.LayoutGraph, error) {
		switch name {
		case "elk":
			return d2elklayout.DefaultLayout, nil
		case "dagre", "":
			return d2dagrelayout.DefaultLayout, nil
		default:
			return d2dagrelayout.DefaultLayout, nil
		}
	}

	opts := &d2lib.CompileOptions{
		Layout:         &name,
		Ruler:          ruler,
		LayoutResolver: layoutResolver,
	}

	diagram, _, err := d2lib.Compile(ctx, d2text, opts, nil)
	if err != nil {
		return nil, fmt.Errorf("isotopo: d2lib compile: %w", err)
	}
	return diagram, nil
}
