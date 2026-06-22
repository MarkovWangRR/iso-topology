// Package llm is a thin OpenAI-compatible chat+vision client for the playbook
// distillation pipeline. It targets any OpenAI-compatible endpoint (OPENAI_BASE_URL),
// using a text model for synthesis/refinement and a vision model for extraction
// and the inverse-render judge.
package llm

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"
)

type Client struct {
	base        string
	key         string
	chatModel   string
	visionModel string
	http        *http.Client
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// New builds a client from the environment:
//
//	OPENAI_API_KEY / OPENAI_BASE_URL / OPENAI_MODEL   (chat)
//	PLAYBOOK_LLM_MODEL                                (override chat model)
//	PLAYBOOK_VISION_MODEL                             (vision model; default Qwen3-VL)
func New() *Client {
	return &Client{
		base:        strings.TrimRight(env("OPENAI_BASE_URL", "https://api.openai.com/v1"), "/"),
		key:         os.Getenv("OPENAI_API_KEY"),
		chatModel:   env("PLAYBOOK_LLM_MODEL", env("OPENAI_MODEL", "gpt-4o-mini")),
		visionModel: env("PLAYBOOK_VISION_MODEL", "Qwen/Qwen3-VL-32B-Instruct"),
		http:        &http.Client{Timeout: 180 * time.Second},
	}
}

// Available reports whether an API key is configured (callers should Skip tests
// otherwise).
func (c *Client) Available() bool { return c.key != "" }

type msg struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

func (c *Client) post(model string, messages []msg, maxTokens int) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":       model,
		"messages":    messages,
		"max_tokens":  maxTokens,
		"temperature": 0.2,
	})
	req, _ := http.NewRequest("POST", c.base+"/chat/completions", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer "+c.key)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
		Error any `json:"error"`
	}
	dec := json.NewDecoder(resp.Body)
	if err := dec.Decode(&out); err != nil {
		return "", err
	}
	if resp.StatusCode >= 300 || len(out.Choices) == 0 {
		return "", fmt.Errorf("llm %s: status %d: %v", model, resp.StatusCode, out.Error)
	}
	return out.Choices[0].Message.Content, nil
}

// Chat runs a text completion.
func (c *Client) Chat(system, user string) (string, error) {
	var ms []msg
	if system != "" {
		ms = append(ms, msg{Role: "system", Content: system})
	}
	ms = append(ms, msg{Role: "user", Content: user})
	return c.post(c.chatModel, ms, 1200)
}

// Vision runs a multimodal completion over a prompt + image files (paths).
func (c *Client) Vision(prompt string, imagePaths ...string) (string, error) {
	content := []any{map[string]any{"type": "text", "text": prompt}}
	for _, p := range imagePaths {
		uri, err := dataURI(p)
		if err != nil {
			return "", err
		}
		content = append(content, map[string]any{
			"type":      "image_url",
			"image_url": map[string]any{"url": uri},
		})
	}
	return c.post(c.visionModel, []msg{{Role: "user", Content: content}}, 800)
}

func dataURI(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	mime := "image/png"
	if strings.HasSuffix(strings.ToLower(path), ".jpg") || strings.HasSuffix(strings.ToLower(path), ".jpeg") {
		mime = "image/jpeg"
	}
	return "data:" + mime + ";base64," + base64.StdEncoding.EncodeToString(b), nil
}

// ExtractJSON pulls the first balanced {...} object out of a model reply
// (models often wrap JSON in prose or ```json fences).
func ExtractJSON(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}
