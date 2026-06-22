package playbook

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// color.go — minimal hex/HSL utilities for the shading kernel. The isotopo
// package has parseHex/lumOf but they're unexported; this is the playbook layer's
// own small, self-contained colour math. Alpha (#RRGGBBAA) is preserved so a
// translucent token like #FFFFFFE6 shades without losing its alpha.

func hx(s string) int { v, _ := strconv.ParseInt(s, 16, 64); return int(v) }

// parseHex accepts #RGB, #RRGGBB, #RRGGBBAA. Returns 0..255 channels plus the
// raw 2-char alpha suffix ("" when none).
func parseHex(s string) (r, g, b int, alpha string, ok bool) {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "#") {
		return 0, 0, 0, "", false
	}
	h := s[1:]
	switch len(h) {
	case 3:
		return hx(h[0:1] + h[0:1]), hx(h[1:2] + h[1:2]), hx(h[2:3] + h[2:3]), "", true
	case 6:
		return hx(h[0:2]), hx(h[2:4]), hx(h[4:6]), "", true
	case 8:
		return hx(h[0:2]), hx(h[2:4]), hx(h[4:6]), strings.ToUpper(h[6:8]), true
	}
	return 0, 0, 0, "", false
}

func toHex(r, g, b int, alpha string) string {
	cl := func(v int) int {
		if v < 0 {
			return 0
		}
		if v > 255 {
			return 255
		}
		return v
	}
	return fmt.Sprintf("#%02X%02X%02X%s", cl(r), cl(g), cl(b), alpha)
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func rgbToHSL(r, g, b int) (h, s, l float64) {
	rf, gf, bf := float64(r)/255, float64(g)/255, float64(b)/255
	max := math.Max(rf, math.Max(gf, bf))
	min := math.Min(rf, math.Min(gf, bf))
	l = (max + min) / 2
	if max == min {
		return 0, 0, l
	}
	d := max - min
	if l > 0.5 {
		s = d / (2 - max - min)
	} else {
		s = d / (max + min)
	}
	switch max {
	case rf:
		h = (gf - bf) / d
		if gf < bf {
			h += 6
		}
	case gf:
		h = (bf-rf)/d + 2
	default:
		h = (rf-gf)/d + 4
	}
	return h / 6, s, l
}

func hue2rgb(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	switch {
	case t < 1.0/6:
		return p + (q-p)*6*t
	case t < 1.0/2:
		return q
	case t < 2.0/3:
		return p + (q-p)*(2.0/3-t)*6
	}
	return p
}

func hslToRGB(h, s, l float64) (int, int, int) {
	if s == 0 {
		v := int(math.Round(l * 255))
		return v, v, v
	}
	var q float64
	if l < 0.5 {
		q = l * (1 + s)
	} else {
		q = l + s - l*s
	}
	p := 2*l - q
	r := hue2rgb(p, q, h+1.0/3)
	g := hue2rgb(p, q, h)
	b := hue2rgb(p, q, h-1.0/3)
	return int(math.Round(r * 255)), int(math.Round(g * 255)), int(math.Round(b * 255))
}

// shade shifts a colour's lightness by dL (−1..1); negative darkens. Alpha kept.
// Returns the input unchanged if it isn't a hex colour (e.g. "none").
func shade(hex string, dL float64) string {
	r, g, b, a, ok := parseHex(hex)
	if !ok {
		return hex
	}
	h, s, l := rgbToHSL(r, g, b)
	nr, ng, nb := hslToRGB(h, s, clamp01(l+dL))
	return toHex(nr, ng, nb, a)
}

// lightnessOf returns the HSL lightness (0..1) of a hex colour.
func lightnessOf(hex string) float64 {
	r, g, b, _, ok := parseHex(hex)
	if !ok {
		return 1
	}
	_, _, l := rgbToHSL(r, g, b)
	return l
}

// mix blends two hex colours in RGB by t (0=a, 1=b). Alpha follows a.
func mix(a, b string, t float64) string {
	ar, ag, ab, aa, ok := parseHex(a)
	br, bg, bb, _, ok2 := parseHex(b)
	if !ok || !ok2 {
		return a
	}
	t = clamp01(t)
	lerp := func(x, y int) int { return int(float64(x) + (float64(y)-float64(x))*t + 0.5) }
	return toHex(lerp(ar, br), lerp(ag, bg), lerp(ab, bb), aa)
}

// satOf returns the HSL saturation (0..1) of a hex colour.
func satOf(hex string) float64 {
	r, g, b, _, ok := parseHex(hex)
	if !ok {
		return 0
	}
	_, s, _ := rgbToHSL(r, g, b)
	return s
}

// saturate multiplies a colour's HSL saturation by k (keeps hue/lightness/alpha).
func saturate(hex string, k float64) string {
	r, g, b, a, ok := parseHex(hex)
	if !ok {
		return hex
	}
	h, s, l := rgbToHSL(r, g, b)
	nr, ng, nb := hslToRGB(h, clamp01(s*k), l)
	return toHex(nr, ng, nb, a)
}

// isDark reports whether a colour reads as dark (for picking ink/icon variants).
func isDark(hex string) bool {
	r, g, b, _, ok := parseHex(hex)
	if !ok {
		return false
	}
	// perceptual luma
	return (0.299*float64(r)+0.587*float64(g)+0.114*float64(b))/255 < 0.5
}
