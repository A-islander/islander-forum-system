package route

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func TestBarWebSocketPerformance(t *testing.T) {
	mux := http.NewServeMux()
	registerBarRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/bar/order"
	connection, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	_ = connection.SetReadDeadline(time.Now().Add(12 * time.Second))
	if err := connection.WriteJSON(map[string]interface{}{
		"type":    "order.create",
		"payload": map[string]interface{}{"recipe_id": 1, "ordered_by": 8848, "message": "ws integration test"},
	}); err != nil {
		t.Fatal(err)
	}

	seen := make(map[string]int)
	for {
		var event struct {
			Type string `json:"type"`
		}
		if err := connection.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		seen[event.Type]++
		if event.Type == "order.failed" {
			t.Fatal("websocket order failed")
		}
		if event.Type == "order.completed" {
			break
		}
	}
	for _, eventType := range []string{"order.accepted", "bartender.say", "action.start", "action.technique", "order.completed"} {
		if seen[eventType] == 0 {
			t.Fatalf("event %s was not received; seen=%v", eventType, seen)
		}
	}
	if seen["action.start"] != 3 {
		t.Fatalf("expected 3 ingredient actions, got %d", seen["action.start"])
	}
}

func TestBarRestockIsNotPublic(t *testing.T) {
	mux := http.NewServeMux()
	registerBarRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/bar/restock", strings.NewReader(`{"type_id":11,"quantity":100}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("restock endpoint status = %d, want 404", response.Code)
	}
}
