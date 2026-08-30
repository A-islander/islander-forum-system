package route

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	barservice "github.com/forum_server/service/bar"
	"github.com/gorilla/websocket"
)

func TestBarWebSocketPerformance(t *testing.T) {
	previousScale, hadScale := os.LookupEnv("BAR_WS_TIME_SCALE")
	if err := os.Setenv("BAR_WS_TIME_SCALE", "0.25"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if hadScale {
			_ = os.Setenv("BAR_WS_TIME_SCALE", previousScale)
		} else {
			_ = os.Unsetenv("BAR_WS_TIME_SCALE")
		}
	}()
	originalCueBuilder := buildBarPerformanceCue
	defer func() { buildBarPerformanceCue = originalCueBuilder }()
	originalResolver := resolveBarUserId
	resolveBarUserId = func(token string) (uint64, error) {
		if token != "test-token" {
			return 0, errors.New("invalid token")
		}
		return 4321, nil
	}
	defer func() { resolveBarUserId = originalResolver }()
	var cueLock sync.Mutex
	startedCues := 0
	allCuesStarted := make(chan struct{})
	buildBarPerformanceCue = func(_ context.Context, result *barservice.OrderResult, stage string, stepIndex int) (string, error) {
		cueLock.Lock()
		startedCues++
		if startedCues == 5 {
			close(allCuesStarted)
		}
		cueLock.Unlock()
		<-allCuesStarted
		switch stage {
		case "ingredient":
			return result.Steps[stepIndex].TypeName + "正在取，别催。", nil
		case "technique":
			return "要开始摇和了，抓稳吧台。", nil
		case "serving":
			result.Drink.Description = "这杯是并发生成的上酒文案，拿稳了。"
			return result.Drink.Description, nil
		default:
			return "", nil
		}
	}
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
		"token":   "test-token",
		"payload": map[string]interface{}{"recipe_id": 1, "ordered_by": 9999, "message": "ws integration test"},
	}); err != nil {
		t.Fatal(err)
	}

	seen := make(map[string]int)
	var sequence []string
	var accepted struct {
		OrderId    string  `json:"order_id"`
		RecipeName string  `json:"recipe_name"`
		Technique  string  `json:"technique"`
		TimeScale  float64 `json:"time_scale"`
	}
	var actionDuration int
	llmLines := 0
	var completed barservice.OrderResult
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
		if event.Type == "action.start" {
			var action barservice.PerformanceStep
			if err := json.Unmarshal(event.Payload, &action); err != nil {
				t.Fatal(err)
			}
			actionDuration = action.DurationMs
		}
		if event.Type == "bartender.say" {
			var line struct {
				Source string `json:"source"`
			}
			if err := json.Unmarshal(event.Payload, &line); err != nil {
				t.Fatal(err)
			}
			if line.Source == "llm" {
				llmLines++
			}
		}
		if event.Type == "order.completed" {
			if err := json.Unmarshal(event.Payload, &completed); err != nil {
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
	if accepted.OrderId == "" || accepted.RecipeName != "海角黄昏" || accepted.Technique != "摇和" || accepted.TimeScale != 0.25 {
		t.Fatalf("unexpected accepted payload: %+v", accepted)
	}
	if actionDuration != 300 {
		t.Fatalf("action duration = %d, want 300", actionDuration)
	}
	if llmLines != 5 {
		t.Fatalf("LLM performance lines = %d, want 5", llmLines)
	}
	if completed.Drink.OrderedBy != 4321 {
		t.Fatalf("ordered_by = %d, want token user 4321", completed.Drink.OrderedBy)
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

func TestBarHTTPOrderRequiresAuthorization(t *testing.T) {
	mux := http.NewServeMux()
	registerBarRoutes(mux)
	request := httptest.NewRequest(http.MethodPost, "/api/bar/order", strings.NewReader(`{"recipe_id":1,"ordered_by":9999}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	var payload struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != http.StatusUnauthorized {
		t.Fatalf("code = %d, want 401", payload.Code)
	}
}

func TestBarIngredientsArePublic(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/bar/ingredients", nil)
	response := httptest.NewRecorder()
	barIngredientsHandler(response, request)
	if response.Code != http.StatusOK {
		t.Fatalf("status=%d body=%s", response.Code, response.Body.String())
	}
	var payload struct {
		Data struct {
			MaxExtras   int                                `json:"max_extras_per_drink"`
			Ingredients []barservice.IngredientCatalogItem `json:"ingredients"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.MaxExtras != 2 || len(payload.Data.Ingredients) == 0 {
		t.Fatalf("unexpected catalog response: %+v", payload)
	}
}

func TestBarWebSocketRejectsInvalidToken(t *testing.T) {
	originalResolver := resolveBarUserId
	resolveBarUserId = func(string) (uint64, error) { return 0, errors.New("invalid token") }
	defer func() { resolveBarUserId = originalResolver }()

	mux := http.NewServeMux()
	registerBarRoutes(mux)
	server := httptest.NewServer(mux)
	defer server.Close()
	connection, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http")+"/ws/bar/order", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer connection.Close()
	if err := connection.WriteJSON(map[string]interface{}{
		"type": "order.create", "token": "bad-token", "payload": map[string]interface{}{"recipe_id": 1},
	}); err != nil {
		t.Fatal(err)
	}
	var event struct {
		Type    string `json:"type"`
		Payload struct {
			Reason string `json:"reason"`
		} `json:"payload"`
	}
	if err := connection.ReadJSON(&event); err != nil {
		t.Fatal(err)
	}
	if event.Type != "order.failed" || event.Payload.Reason != "unauthorized" {
		t.Fatalf("unexpected event: %+v", event)
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

func TestBarStockIsNotPublic(t *testing.T) {
	mux := http.NewServeMux()
	registerBarRoutes(mux)
	request := httptest.NewRequest(http.MethodGet, "/api/bar/stock", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusNotFound {
		t.Fatalf("stock endpoint status = %d, want 404", response.Code)
	}
}
