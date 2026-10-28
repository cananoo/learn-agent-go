// Package fakeapi is a scripted stand-in for the Messages API, so every
// lesson's agent loop can be tested offline without an API key.
package fakeapi

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Server replays canned responses in order and records every request body.
type Server struct {
	t         *testing.T
	mu        sync.Mutex
	responses []string
	Requests  []map[string]any
}

// New starts a server that returns responses in order, one per request.
func New(t *testing.T, responses ...string) (*Server, anthropic.Client) {
	t.Helper()
	s := &Server{t: t, responses: responses}
	srv := httptest.NewServer(http.HandlerFunc(s.handle))
	t.Cleanup(srv.Close)
	client := anthropic.NewClient(
		option.WithBaseURL(srv.URL),
		option.WithAPIKey("test-key"),
		option.WithMaxRetries(0),
	)
	return s, client
}

func (s *Server) handle(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	body, _ := io.ReadAll(r.Body)
	var req map[string]any
	if err := json.Unmarshal(body, &req); err != nil {
		s.t.Errorf("fakeapi: bad request body: %v", err)
	}
	s.Requests = append(s.Requests, req)
	if len(s.responses) == 0 {
		s.t.Errorf("fakeapi: unexpected request #%d", len(s.Requests))
		http.Error(w, `{"type":"error","error":{"type":"api_error","message":"no more scripted responses"}}`, 500)
		return
	}
	resp := s.responses[0]
	s.responses = s.responses[1:]
	w.Header().Set("Content-Type", "application/json")
	io.WriteString(w, resp)
}

// Calls returns how many requests the server has seen.
func (s *Server) Calls() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.Requests)
}

// Text builds an end_turn response with one text block.
func Text(text string) string {
	return message("end_turn", []any{map[string]any{"type": "text", "text": text}})
}

// ToolCall describes one tool_use block.
type ToolCall struct {
	ID    string
	Name  string
	Input any
}

// Tools builds a tool_use response with the given calls (parallel when more than one).
func Tools(calls ...ToolCall) string {
	var content []any
	for _, c := range calls {
		content = append(content, map[string]any{"type": "tool_use", "id": c.ID, "name": c.Name, "input": c.Input})
	}
	return message("tool_use", content)
}

func message(stop string, content []any) string {
	b, _ := json.Marshal(map[string]any{
		"id": "msg_test", "type": "message", "role": "assistant", "model": "test-model",
		"content": content, "stop_reason": stop, "stop_sequence": nil,
		"usage": map[string]any{"input_tokens": 100, "output_tokens": 10},
	})
	return string(b)
}

// ToolResults extracts the tool_result blocks from the last user message of
// request i, keyed by tool_use_id, with their text content.
func (s *Server) ToolResults(i int) map[string]string {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs, _ := s.Requests[i]["messages"].([]any)
	if len(msgs) == 0 {
		return nil
	}
	last, _ := msgs[len(msgs)-1].(map[string]any)
	out := map[string]string{}
	blocks, _ := last["content"].([]any)
	for _, b := range blocks {
		block, _ := b.(map[string]any)
		if block["type"] != "tool_result" {
			continue
		}
		out[fmt.Sprint(block["tool_use_id"])] = contentText(block["content"])
	}
	return out
}

// LastUserText returns the concatenated text blocks of the last message in request i.
func (s *Server) LastUserText(i int) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	msgs, _ := s.Requests[i]["messages"].([]any)
	last, _ := msgs[len(msgs)-1].(map[string]any)
	return contentText(last["content"])
}

func contentText(c any) string {
	switch v := c.(type) {
	case string:
		return v
	case []any:
		text := ""
		for _, b := range v {
			if m, ok := b.(map[string]any); ok && m["type"] == "text" {
				text += fmt.Sprint(m["text"])
			}
		}
		return text
	}
	return ""
}
