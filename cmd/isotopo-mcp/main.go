// isotopo-mcp is a Model Context Protocol (MCP) server exposing the
// iso-topology agent loop as three tools — iso_capabilities,
// iso_validate, iso_render — over the stdio transport, so MCP clients
// (Claude Code, Claude Desktop, Cursor, …) can draw isometric
// diagrams without shelling out to the CLI.
//
// The implementation is a deliberately minimal, dependency-free
// JSON-RPC 2.0 loop (newline-delimited messages per the MCP stdio
// transport). It handles: initialize, ping, tools/list, tools/call,
// and ignores notifications. Anything fancier (resources, prompts)
// belongs in a future revision.
//
// Setup: docs/agent/MCP.md.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	isotopo "github.com/MarkovWangRR/iso-topology"
)

const serverVersion = "0.3.1"

// ── JSON-RPC plumbing ────────────────────────────────────────────────

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func main() {
	in := bufio.NewScanner(os.Stdin)
	in.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024) // DSL payloads can be large
	out := json.NewEncoder(os.Stdout)

	for in.Scan() {
		line := strings.TrimSpace(in.Text())
		if line == "" {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			continue // not JSON-RPC; ignore
		}
		isNotification := len(req.ID) == 0 || string(req.ID) == "null"

		result, rpcErr := dispatch(&req)
		if isNotification {
			continue
		}
		resp := rpcResponse{JSONRPC: "2.0", ID: req.ID}
		if rpcErr != nil {
			resp.Error = rpcErr
		} else {
			resp.Result = result
		}
		if err := out.Encode(resp); err != nil {
			fmt.Fprintln(os.Stderr, "isotopo-mcp: write:", err)
			os.Exit(1)
		}
	}
}

func dispatch(req *rpcRequest) (any, *rpcError) {
	switch req.Method {
	case "initialize":
		var p struct {
			ProtocolVersion string `json:"protocolVersion"`
		}
		_ = json.Unmarshal(req.Params, &p)
		if p.ProtocolVersion == "" {
			p.ProtocolVersion = "2024-11-05"
		}
		return map[string]any{
			"protocolVersion": p.ProtocolVersion,
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo": map[string]any{
				"name":    "isotopo",
				"version": serverVersion,
			},
		}, nil

	case "ping":
		return map[string]any{}, nil

	case "tools/list":
		return map[string]any{"tools": toolList()}, nil

	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return nil, &rpcError{Code: -32602, Message: "bad tools/call params: " + err.Error()}
		}
		text, isErr := callTool(p.Name, p.Arguments)
		return map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"isError": isErr,
		}, nil

	case "notifications/initialized", "notifications/cancelled":
		return nil, nil

	default:
		return nil, &rpcError{Code: -32601, Message: "method not found: " + req.Method}
	}
}

// ── Tools ────────────────────────────────────────────────────────────

func toolList() []map[string]any {
	dslProps := map[string]any{
		"dsl": map[string]any{
			"type":        "string",
			"description": "The diagram source text (YAML composite, d2 graph, or JSON).",
		},
		"format": map[string]any{
			"type":        "string",
			"enum":        []string{"yaml", "d2", "json"},
			"description": "Source dialect. Default yaml.",
		},
	}
	renderProps := map[string]any{
		"dsl":    dslProps["dsl"],
		"format": dslProps["format"],
		"output_dir": map[string]any{
			"type":        "string",
			"description": "Directory to write topology.svg / topology.html / nodes/* into. Defaults to a fresh temp directory.",
		},
	}
	return []map[string]any{
		{
			"name":        "iso_capabilities",
			"description": "Machine-readable inventory of everything the iso-topology DSL can express: shapes, composition primitives (layout, place, group, stack, connector, …), style keys, icons. Read once per session before emitting DSL.",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "iso_validate",
			"description": "Validate iso-topology DSL without rendering. Returns JSONPath-located issues with 'did you mean' fix suggestions: unknown shapes, dangling place/connector references, place cycles, post-solve overlaps (warnings). Empty issues = clean.",
			"inputSchema": map[string]any{
				"type":       "object",
				"required":   []string{"dsl"},
				"properties": dslProps,
			},
		},
		{
			"name":        "iso_render",
			"description": "Render iso-topology DSL to a 2.5D isometric SVG scene. Validates first (refuses on errors), then writes topology.svg, topology.html (SVG beside editable source), and per-element nodes/<id>.svg fragments. Returns the output paths.",
			"inputSchema": map[string]any{
				"type":       "object",
				"required":   []string{"dsl"},
				"properties": renderProps,
			},
		},
	}
}

func callTool(name string, args json.RawMessage) (text string, isError bool) {
	switch name {
	case "iso_capabilities":
		enc, err := json.MarshalIndent(isotopo.CapabilityReport(), "", "  ")
		if err != nil {
			return "capabilities: " + err.Error(), true
		}
		return string(enc), false

	case "iso_validate":
		doc, a, errText := loadFromArgs(args)
		if errText != "" {
			return errText, true
		}
		issues := isotopo.Validate(doc)
		if a != nil && a.Format != "d2" {
			issues = append(issues, isotopo.UnknownKeyIssues([]byte(a.DSL))...)
		}
		enc, _ := json.MarshalIndent(map[string]any{"issues": issues}, "", "  ")
		return string(enc), hasErrorIssue(issues)

	case "iso_render":
		doc, parsed, errText := loadFromArgs(args)
		if errText != "" {
			return errText, true
		}
		if issues := isotopo.Validate(doc); hasErrorIssue(issues) {
			enc, _ := json.MarshalIndent(map[string]any{
				"error":  "document has validation errors; fix them and retry",
				"issues": issues,
			}, "", "  ")
			return string(enc), true
		}
		outDir := parsed.OutputDir
		if outDir == "" {
			d, err := os.MkdirTemp("", "isotopo-")
			if err != nil {
				return "mkdtemp: " + err.Error(), true
			}
			outDir = d
		}
		paths, err := renderAll(doc, parsed, outDir)
		if err != nil {
			return "render: " + err.Error(), true
		}
		enc, _ := json.MarshalIndent(map[string]any{
			"output_dir": outDir,
			"files":      paths,
		}, "", "  ")
		return string(enc), false

	default:
		return "unknown tool: " + name, true
	}
}

// ── Document loading / rendering (mirrors the CLI pipeline) ─────────

type toolArgs struct {
	DSL       string `json:"dsl"`
	Format    string `json:"format"`
	OutputDir string `json:"output_dir"`
}

func loadFromArgs(raw json.RawMessage) (*isotopo.Document, *toolArgs, string) {
	var a toolArgs
	if err := json.Unmarshal(raw, &a); err != nil {
		return nil, nil, "bad arguments: " + err.Error()
	}
	if strings.TrimSpace(a.DSL) == "" {
		return nil, nil, "argument 'dsl' is required"
	}
	if a.Format == "" {
		a.Format = "yaml"
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	doc, err := isotopo.LoadInput(ctx, a.Format, []byte(a.DSL), isotopo.LayoutDagre)
	if err != nil {
		return nil, nil, "load: " + err.Error()
	}
	return doc, &a, ""
}

// renderAll mirrors `isotopo render`: topology.svg + topology.html +
// source copy + per-element nodes/<id>.{svg,yaml,html}.
func renderAll(doc *isotopo.Document, a *toolArgs, outDir string) ([]string, error) {
	nodesDir := filepath.Join(outDir, "nodes")
	if err := os.MkdirAll(nodesDir, 0o755); err != nil {
		return nil, err
	}
	var paths []string
	write := func(rel string, data []byte) error {
		p := filepath.Join(outDir, rel)
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return err
		}
		paths = append(paths, p)
		return nil
	}

	svg := isotopo.RenderWithCanvas(doc.Scene(), doc.Theme, doc.Canvas, doc.Annotations)
	if err := write("topology.svg", []byte(svg)); err != nil {
		return nil, err
	}
	srcName := "topology." + a.Format
	if err := write(srcName, []byte(a.DSL)); err != nil {
		return nil, err
	}
	if err := write("topology.html", []byte(isotopo.TopologyHTML(svg, a.DSL, a.Format, srcName))); err != nil {
		return nil, err
	}

	parts := isotopo.RenderParts(doc)
	frags := isotopo.PartFragments(doc)
	for _, id := range isotopo.PartIDs(doc) {
		if s := parts[id]; s != "" {
			if err := write(filepath.Join("nodes", id+".svg"), []byte(s)); err != nil {
				return nil, err
			}
		}
		if f := frags[id]; f != nil {
			if y, err := isotopo.MarshalFragmentYAML(f); err == nil {
				if err := write(filepath.Join("nodes", id+".yaml"), y); err != nil {
					return nil, err
				}
			}
		}
	}
	return paths, nil
}

func hasErrorIssue(issues []isotopo.Issue) bool {
	for _, i := range issues {
		if i.Severity == isotopo.SeverityError {
			return true
		}
	}
	return false
}
