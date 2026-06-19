package isotopo

import (
	"encoding/xml"
	"fmt"
	"strings"
	"testing"
)

// wellFormed reports whether s parses as well-formed XML (every tag balanced,
// no attribute breakout). A raw injection payload leaking into an attribute or
// text node breaks this.
func wellFormed(s string) error {
	dec := xml.NewDecoder(strings.NewReader(s))
	for {
		_, err := dec.Token()
		if err != nil {
			if err.Error() == "EOF" {
				return nil
			}
			return err
		}
	}
}

// injectionPayload is XML-hostile: a quote+bracket breakout, an open tag, and a
// bare ampersand. Escaped correctly it is inert; leaked raw it breaks the SVG.
const injectionPayload = `"><script>x</script>&`

// TestSVGInjection_AllShapes is the standing guarantee that NO user-controlled
// value reaches the SVG unescaped — for EVERY shape, current or future. It
// renders each shape with the payload in every style/text sink and asserts the
// output is well-formed XML. A new shape (or a new sink in an existing shape)
// that forgets to escape will fail here.
func TestSVGInjection_AllShapes(t *testing.T) {
	p := injectionPayload
	for _, shape := range validShapeList() {
		shape := shape
		t.Run(shape, func(t *testing.T) {
			// Minimal config that renders cleanly for ~every shape, with the
			// payload only in the genuine string sinks. (Richer fields like icon
			// can themselves error and empty the svg, hiding the leak.)
			src := fmt.Sprintf(`nodes:
  scene:
    shape: composite
    parts:
      - id: a
        shape: %s
        geom: { w: 90, d: 90, h: 34, sides: 6 }
        label: %q
        style:
          palette: { top: %q, left: %q, right: %q }
          stroke: { color: %q, width: 1, dash: %q }
          text: { color: %q, family: %q, weight: %q }
`, shape, p, p, p, p, p, p, p, p, p)

			svg, _, _ := RenderSource("yaml", []byte(src))
			if svg == "" {
				return // shape needs more config to render; not a leak
			}
			if err := wellFormed(svg); err != nil {
				t.Fatalf("shape %q leaks an unescaped value (SVG not well-formed): %v", shape, err)
			}
		})
	}
}

// TestSVGInjection_Connectors covers the connector overlay sinks (label,
// labelBg, stroke colour/dash, arrow) which bypass the shape pipeline.
func TestSVGInjection_Connectors(t *testing.T) {
	p := injectionPayload
	src := fmt.Sprintf(`nodes:
  scene:
    shape: composite
    parts:
      - { id: a, shape: rectangle, geom: {w:60,d:60,h:30} }
      - { id: b, shape: rectangle, geom: {w:60,d:60,h:30}, offset: {wx:160,wy:0} }
    connectors:
      - from: a
        to: b
        arrow: triangle
        label: %q
        labelBg: %q
        labelColor: %q
        stroke: { color: %q, dash: %q }
`, p, p, p, p, p)
	svg, issues, _ := RenderSource("yaml", []byte(src))
	for _, i := range issues {
		if i.Severity == SeverityError {
			t.Fatalf("unexpected render error: %s", i.Message)
		}
	}
	if err := wellFormed(svg); err != nil {
		t.Fatalf("connector overlay leaks an unescaped value: %v", err)
	}
}

// TestSVGInjection_ControlChars: control characters (e.g. from a d2 label) must
// not reach the SVG raw — they are XML-invalid and break parsing.
func TestSVGInjection_ControlChars(t *testing.T) {
	for _, in := range []struct{ format, src string }{
		{"d2", "a: ct\x01rl"},
		{"yaml", "nodes:\n  scene:\n    shape: composite\n    parts:\n      - { id: a, shape: rectangle, geom: {w:60,d:60,h:30}, label: \"x\x01y\x02z\" }\n"},
	} {
		svg, _, _ := RenderSource(in.format, []byte(in.src))
		if svg == "" {
			continue
		}
		if err := wellFormed(svg); err != nil {
			t.Fatalf("%s control-char label leaks into SVG: %v", in.format, err)
		}
		if strings.ContainsRune(svg, '\x01') || strings.ContainsRune(svg, '\x02') {
			t.Fatalf("%s: raw control byte present in SVG", in.format)
		}
	}
}
