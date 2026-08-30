package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMiniMaxClientDisablesThinking(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/anthropic/v1/messages" || r.Header.Get("Authorization") != "Bearer test-key" {
			t.Fatalf("unexpected request: path=%s authorization=%s", r.URL.Path, r.Header.Get("Authorization"))
		}
		var request miniMaxRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatal(err)
		}
		if request.Model != "MiniMax-M3" || request.Thinking.Type != "disabled" || request.Stream {
			t.Fatalf("unexpected body: %+v", request)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"content":[{"type":"text","text":"哼，给你的酒。"}]}`))
	}))
	defer server.Close()

	client, err := NewMiniMaxClient(server.URL+"/anthropic", "test-key", "MiniMax-M3", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	content, err := client.Complete(context.Background(), Request{System: "岛民娘", Prompt: "上酒", MaxTokens: 200})
	if err != nil || content != "哼，给你的酒。" {
		t.Fatalf("content=%q err=%v", content, err)
	}
}

func TestMiniMaxClientRejectsErrorResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":{"message":"bad key"}}`, http.StatusUnauthorized)
	}))
	defer server.Close()
	client, err := NewMiniMaxClient(server.URL, "test-key", "MiniMax-M3", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Complete(context.Background(), Request{}); err == nil {
		t.Fatal("expected HTTP error")
	}
}
