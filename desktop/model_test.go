package desktop

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// This is a transport contract test with a scripted endpoint, not a model experiment.
func TestModelTrialTransportAndIsolation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Messages []modelMessage `json:"messages"`
		}
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			t.Error(err)
			return
		}
		message := modelMessage{Role: "assistant", Content: "done"}
		var name, args string
		if len(q.Messages) == 2 {
			name = "search"
			args = `{"scope":{}}`
		} else if len(q.Messages) == 4 {
			last := q.Messages[3].Content.(string)
			var data struct {
				Receipt struct {
					ID string `json:"id"`
				} `json:"receipt"`
			}
			_ = json.Unmarshal([]byte(last), &data)
			name = "create"
			b, _ := json.Marshal(map[string]any{"receipts": []string{data.Receipt.ID}})
			args = string(b)
		}
		if name != "" {
			tool := modelToolCall{ID: name, Type: "function"}
			tool.Function.Name = name
			tool.Function.Arguments = args
			message.ToolCalls = []modelToolCall{tool}
		}
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": message}}})
	}))
	defer server.Close()
	a, err := New(t.TempDir(), "token")
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	a.model = ModelConfig{Endpoint: server.URL, Model: "scripted-contract-only"}
	for _, q := range []struct {
		mode, path string
		published  bool
	}{{"E", "fresh.txt", true}, {"E", "database.yaml", false}, {"A", "database.yaml", true}} {
		r, err := a.modelTrial(context.Background(), q.mode, q.path)
		if err != nil {
			t.Fatal(err)
		}
		if r.Error != "" || r.Published != q.published || r.Calls != 3 {
			t.Fatalf("trial %+v", r)
		}
	}
	if a.service != nil || a.source != "" {
		t.Fatal("trial changed user's workspace")
	}
	if (ModelConfig{Endpoint: "http://example.com/v1", Model: "m"}).validate() == nil {
		t.Fatal("remote plaintext accepted")
	}
}
