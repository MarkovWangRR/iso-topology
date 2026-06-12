// visreg — pixel-level visual regression over the golden SVGs.
//
// The golden suite asserts BYTES; this asserts PIXELS. The two catch
// different failure classes: a byte-identical golden can't regress
// visually, but the moment goldens are intentionally regenerated, byte
// comparison says nothing about whether the picture is still right.
// visreg rasterizes every samples/*/*/expected.svg at its exact
// integral size in headless Chrome and compares against committed
// baseline PNGs.
//
//	go run ./tools/visreg            # compare against baselines
//	go run ./tools/visreg -approve   # bless current renders as baseline
//	go run ./tools/visreg -only ai-  # filter by substring
//
// A sample fails when >0.5% of pixels differ by more than 12/255 in
// any channel (tolerant of antialiasing jitter, intolerant of layout
// or color drift). Not part of `go test` — needs Chrome; run it
// whenever goldens are regenerated, before committing them.
package main

import (
	"flag"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

var (
	approve = flag.Bool("approve", false, "write current renders as the new baselines")
	only    = flag.String("only", "", "substring filter on sample names")
	chrome  = flag.String("chrome", defaultChrome(), "chrome binary")
)

func defaultChrome() string {
	if c := os.Getenv("VISREG_CHROME"); c != "" {
		return c
	}
	return "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
}

var dimsRe = regexp.MustCompile(`<svg[^>]*\bwidth="(\d+)" height="(\d+)"`)

func main() {
	flag.Parse()
	repo, err := repoRoot()
	if err != nil {
		fatal("locate repo: %v", err)
	}
	baseDir := filepath.Join(repo, "tools", "visreg", "baselines")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		fatal("mkdir: %v", err)
	}

	type result struct {
		name string
		err  string
	}
	var failures []result
	total := 0

	for _, category := range []string{"node", "topology"} {
		root := filepath.Join(repo, "samples", category)
		entries, err := os.ReadDir(root)
		if err != nil {
			fatal("read %s: %v", root, err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := category + "__" + e.Name()
			if *only != "" && !strings.Contains(name, *only) {
				continue
			}
			svgPath := filepath.Join(root, e.Name(), "expected.svg")
			svg, err := os.ReadFile(svgPath)
			if err != nil {
				continue // no golden here
			}
			total++
			m := dimsRe.FindSubmatch(svg)
			if m == nil {
				failures = append(failures, result{name, "no integral width/height on root svg"})
				continue
			}
			w, h := string(m[1]), string(m[2])

			shot := filepath.Join(os.TempDir(), "visreg-"+name+".png")
			cmd := exec.Command(*chrome,
				"--headless", "--disable-gpu", "--hide-scrollbars",
				"--window-size="+w+","+h,
				"--screenshot="+shot,
				"file://"+svgPath,
			)
			cmd.Stdout, cmd.Stderr = nil, nil
			if err := cmd.Run(); err != nil {
				failures = append(failures, result{name, "chrome: " + err.Error()})
				continue
			}

			baseline := filepath.Join(baseDir, name+".png")
			if *approve {
				if err := os.Rename(shot, baseline); err != nil {
					fatal("approve %s: %v", name, err)
				}
				fmt.Printf("approved  %s\n", name)
				continue
			}
			if _, err := os.Stat(baseline); err != nil {
				failures = append(failures, result{name, "no baseline (run -approve)"})
				continue
			}
			ratio, err := diffPNGs(baseline, shot)
			if err != nil {
				failures = append(failures, result{name, err.Error()})
				continue
			}
			if ratio > 0.005 {
				kept := filepath.Join(baseDir, name+".actual.png")
				_ = os.Rename(shot, kept)
				failures = append(failures, result{name,
					fmt.Sprintf("%.2f%% pixels drifted (actual kept at %s)", ratio*100, kept)})
				continue
			}
			_ = os.Remove(shot)
			fmt.Printf("ok        %s (drift %.3f%%)\n", name, ratio*100)
		}
	}

	if *approve {
		fmt.Printf("\n%d baselines written to %s\n", total, baseDir)
		return
	}
	if len(failures) > 0 {
		fmt.Printf("\nFAIL %d/%d:\n", len(failures), total)
		for _, f := range failures {
			fmt.Printf("  %-28s %s\n", f.name, f.err)
		}
		os.Exit(1)
	}
	fmt.Printf("\nall %d samples within tolerance\n", total)
}

// diffPNGs returns the fraction of pixels whose any-channel delta
// exceeds 12/255. Dimension mismatch is a hard error.
func diffPNGs(aPath, bPath string) (float64, error) {
	a, err := loadPNG(aPath)
	if err != nil {
		return 0, err
	}
	b, err := loadPNG(bPath)
	if err != nil {
		return 0, err
	}
	ab, bb := a.Bounds(), b.Bounds()
	if ab.Dx() != bb.Dx() || ab.Dy() != bb.Dy() {
		return 0, fmt.Errorf("size changed: baseline %dx%d vs actual %dx%d",
			ab.Dx(), ab.Dy(), bb.Dx(), bb.Dy())
	}
	const tol = 12 * 257 // 8-bit channel tolerance in 16-bit space
	bad, n := 0, 0
	for y := ab.Min.Y; y < ab.Max.Y; y++ {
		for x := ab.Min.X; x < ab.Max.X; x++ {
			ar, ag, abl, _ := a.At(x, y).RGBA()
			br, bg, bbl, _ := b.At(x+bb.Min.X-ab.Min.X, y+bb.Min.Y-ab.Min.Y).RGBA()
			if delta(ar, br) > tol || delta(ag, bg) > tol || delta(abl, bbl) > tol {
				bad++
			}
			n++
		}
	}
	if n == 0 {
		return 0, fmt.Errorf("empty image")
	}
	return float64(bad) / float64(n), nil
}

func delta(x, y uint32) uint32 {
	if x > y {
		return x - y
	}
	return y - x
}

func loadPNG(p string) (image.Image, error) {
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", dir)
		}
		dir = parent
	}
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "visreg: "+format+"\n", args...)
	os.Exit(1)
}
