package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"

	"github.com/MarkovWangRR/iso-topology/internal/llm"
	"github.com/MarkovWangRR/iso-topology/internal/playbook"
)

// pbDistill — Flow B: reverse-distil a design manual from a source image via the
// inverse-render loop (extract → synthesize → render → judge → refine).
func pbDistill(root string, args []string) {
	style, source := "", ""
	iters, target := 4, 75.0
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--source":
			if i+1 < len(args) {
				source = args[i+1]
				i++
			}
		case "--iters":
			if i+1 < len(args) {
				iters, _ = strconv.Atoi(args[i+1])
				i++
			}
		case "--target":
			if i+1 < len(args) {
				target, _ = strconv.ParseFloat(args[i+1], 64)
				i++
			}
		default:
			if style == "" {
				style = args[i]
			}
		}
	}
	if style == "" || source == "" {
		fmt.Fprintln(os.Stderr, "usage: isotopo playbook distill <style> --source <img> [--iters N] [--target 75]")
		os.Exit(2)
	}
	c := llm.New()
	if !c.Available() {
		fmt.Fprintln(os.Stderr, "distill: no OPENAI_API_KEY configured")
		os.Exit(2)
	}
	fmt.Printf("distilling %q from %s (iters=%d target=%.0f) …\n", style, filepath.Base(source), iters, target)

	res, err := playbook.RunDistill(c,
		playbook.DistillOpts{Root: root, Style: style, SourceImg: source, Iters: iters, Target: target},
		func(yml []byte) (string, error) { return renderYAML(yml) },
		rasterizeSVG,
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "distill:", err)
		if res == nil {
			os.Exit(1)
		}
	}

	// final preview from the in-memory manual
	previewSVG := ""
	if exemplar, e := os.ReadFile(filepath.Join(root, "_exemplar.yaml")); e == nil {
		if yml, e2 := playbook.Apply(exemplar, res.Manual); e2 == nil {
			if s, e3 := renderYAML(yml); e3 == nil {
				previewSVG = s
			}
		}
	}
	if err := playbook.SaveDistilled(root, res, source, previewSVG); err != nil {
		fmt.Fprintln(os.Stderr, "save:", err)
		os.Exit(1)
	}
	_, _ = playbook.WriteIndex(root)

	for _, it := range res.Iterations {
		fmt.Printf("  iter %d: score %.0f — %s\n", it.Iter, it.Score, it.Critique)
	}
	fmt.Printf("ok: %s → trust=auto, score=%.0f/100 · %s/%s/{manual.yaml,preview/exemplar.svg}\n",
		style, res.FinalScore, root, style)
}

// pbCritique vision-audits a rendered diagram against its reference source image.
func pbCritique(args []string) {
	if len(args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: isotopo playbook critique <render.png> <source.png>")
		os.Exit(2)
	}
	c := llm.New()
	if !c.Available() {
		fmt.Fprintln(os.Stderr, "critique: no OPENAI_API_KEY")
		os.Exit(2)
	}
	prompt := `Image 1 is a rendered isometric architecture diagram. Image 2 is the reference brand landing page whose visual DESIGN LANGUAGE the diagram should echo (colour, accent, mood, energy) — ignore that the content differs. Return ONLY JSON, no prose:
{"style_match": <0-100 how well image 1 echoes image 2's colour/mood/feel>,"vibrancy": <0-100 how lively/dynamic vs flat/rigid/boring image 1 looks>,"techniques":["visible techniques among: gradient-faces, glow, gradient-edges, dotted-edges, soft-shadow, rounded, transparency, outline, lighting"],"gaps":["up to 3 things missing vs the brand's look"]}`
	out, err := c.Vision(prompt, args[0], args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "critique:", err)
		os.Exit(1)
	}
	fmt.Println(llm.ExtractJSON(out))
}

var viewBoxRe = regexp.MustCompile(`viewBox="[\d.\-]+ [\d.\-]+ ([\d.]+) ([\d.]+)"`)

func svgSize(svg string) (int, int) {
	if m := viewBoxRe.FindStringSubmatch(svg); m != nil {
		w, _ := strconv.ParseFloat(m[1], 64)
		h, _ := strconv.ParseFloat(m[2], 64)
		if w > 0 && h > 0 {
			return int(w), int(h)
		}
	}
	return 1200, 800
}

// rasterizeSVG turns an SVG string into a PNG via headless Chrome (so the vision
// judge can see the rendered diagram). Override the binary with $CHROME.
func rasterizeSVG(svg, outPNG string) error {
	tmp := filepath.Join(os.TempDir(), "pb_distill.svg")
	if err := os.WriteFile(tmp, []byte(svg), 0o644); err != nil {
		return err
	}
	w, h := svgSize(svg)
	chrome := os.Getenv("CHROME")
	if chrome == "" {
		chrome = "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome"
	}
	cmd := exec.Command(chrome, "--headless", "--disable-gpu", "--hide-scrollbars",
		"--screenshot="+outPNG, fmt.Sprintf("--window-size=%d,%d", w, h), tmp)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("chrome rasterize: %w (%s)", err, string(out))
	}
	if _, err := os.Stat(outPNG); err != nil {
		return fmt.Errorf("chrome produced no PNG")
	}
	return nil
}
