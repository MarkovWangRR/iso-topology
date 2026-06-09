# 01 — Install

This is step 1 of 5. By the end of the series you'll have a real
iso architecture diagram in your repo.

iso-topology is a single Go binary plus an importable Go library. No
runtime dependencies beyond what the Go toolchain provides — all
external libraries (yaml, d2, freetype, …) are pure Go and statically
linked into the binary.

## Requirements

- **Go 1.25 or later** (older versions may work but aren't tested)
- macOS, Linux, or Windows
- No CGO, no C compiler, no system fonts required

## Three install paths

### A. `go install` (recommended for CLI users)

```bash
go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@latest
```

This builds and installs the `isotopo` binary into `$GOBIN`
(`$HOME/go/bin` by default). Make sure that's on your `PATH`:

```bash
echo 'export PATH="$HOME/go/bin:$PATH"' >> ~/.zshrc   # or ~/.bashrc
```

Verify:

```bash
isotopo capabilities | head
```

To pin to a specific release:

```bash
go install github.com/MarkovWangRR/iso-topology/cmd/isotopo@v0.2.1
```

### B. Clone & build (for hacking on the renderer)

```bash
git clone https://github.com/MarkovWangRR/iso-topology
cd iso-topology
go build -o isotopo ./cmd/isotopo
./isotopo render testdata/microservice/input.d2 ./out
```

Run the test suite:

```bash
go test ./...
```

All six fixtures in `testdata/` should pass in under a second.

### C. Library import (use as a Go module)

In another Go project:

```bash
go get github.com/MarkovWangRR/iso-topology
```

```go
import isotopo "github.com/MarkovWangRR/iso-topology"

func renderScene(yamlBytes []byte) (string, error) {
    doc, err := isotopo.Parse(yamlBytes)
    if err != nil {
        return "", err
    }
    return isotopo.RenderWithCanvas(
        doc.Scene(),
        doc.Theme,
        doc.Canvas,
        doc.Annotations,
    ), nil
}
```

See [USAGE.md](../reference/cli.md) for the full library API surface.

## Dependency note: d2

`.d2` input support pulls in `oss.terrastruct.com/d2 v0.7.1` (pinned).
The d2 module is pure Go too — `go install` or `go build` will fetch
it transparently from the public module proxy. Nothing else to do.

If you need to track d2 HEAD for local development:

```go.mod
// uncomment in go.mod
replace oss.terrastruct.com/d2 => ../path/to/d2/repo
```

## Verifying the install

```bash
# Quick "everything is wired" check
isotopo capabilities | jq '.version, .shapes | length'
# → "0.2.0"
# → 8     (or however many shape clusters the catalog reports)

# Round-trip render
echo 'a -> b -> c' > /tmp/test.d2
isotopo render /tmp/test.d2 /tmp/test-out
ls /tmp/test-out
# topology.svg, topology.html, topology.d2, nodes/
```

## Uninstall

```bash
rm "$(go env GOBIN)/isotopo"   # or: rm $HOME/go/bin/isotopo
```

The Go module cache for d2 / dependencies lives under
`$GOPATH/pkg/mod/`; cleaning that is optional (`go clean -modcache`).

## Next step

→ [`02-first-scene.md`](02-first-scene.md) — render your first
multi-element architecture diagram.
