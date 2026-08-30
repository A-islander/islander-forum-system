package route

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	barservice "github.com/forum_server/service/bar"
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
	var sequence []string
	var accepted struct {
		OrderId    string `json:"order_id"`
		RecipeName string `json:"recipe_name"`
		Technique  string `json:"technique"`
	}
	for {
		var event struct {
			Type    string          `json:"type"`
			Payload json.RawMessage `json:"payload"`
		}
		if err := connection.ReadJSON(&event); err != nil {
			t.Fatal(err)
		}
		seen[event.Type]++
		sequence = append(sequence, event.Type)
		if event.Type == "order.accepted" {
			if err := json.Unmarshal(event.Payload, &accepted); err != nil {
				t.Fatal(err)
			}
		}
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
	if accepted.OrderId == "" || accepted.RecipeName != "海角黄昏" || accepted.Technique != "摇和" {
		t.Fatalf("unexpected accepted payload: %+v", accepted)
	}
	if seen["bartender.say"] < 6 {
		t.Fatalf("expected opening, ingredient, technique and serving lines; got %d", seen["bartender.say"])
	}
	wantSequence := []string{
		"order.accepted", "bartender.say",
		"bartender.say", "action.start",
		"bartender.say", "action.start",
		"bartender.say", "action.start",
		"bartender.say", "action.technique",
		"bartender.say", "order.completed",
	}
	if strings.Join(sequence, ",") != strings.Join(wantSequence, ",") {
		t.Fatalf("unexpected websocket sequence: %v", sequence)
	}
}

func TestIngredientPerformanceLineUsesIslandGirlSourceVoice(t *testing.T) {
	line := ingredientPerformanceLine(
		barservice.PerformanceStep{TypeId: 11, TypeName: "柠檬", Qty: 15, Unit: "g"},
		[]barservice.TracePortion{{TypeId: 11, SourceNote: "岛民娘从码头市场挑的，约10颗"}},
	)
	for _, expected := range []string{"柠檬15g", "我从码头市场挑的"} {
		if !strings.Contains(line, expected) {
			t.Fatalf("performance line %q does not contain %q", line, expected)
		}
	}
}

func TestWSFailurePayloadIncludesMissingDetails(t *testing.T) {
	details := []barservice.MissingDetail{{TypeId: 11, Name: "柠檬", Need: 15, Shortage: 10}}
	payload := wsFailurePayload(&barservice.MissingError{Details: details})
	if payload["reason"] != "missing" {
		t.Fatalf("reason = %v, want missing", payload["reason"])
	}
	got, ok := payload["details"].([]barservice.MissingDetail)
	if !ok || len(got) != 1 || got[0].TypeId != 11 {
		t.Fatalf("unexpected missing details: %#v", payload["details"])
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
