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
	previousDialogueScale, hadDialogueScale := os.LookupEnv("BAR_WS_DIALOGUE_TIME_SCALE")
	if err := os.Setenv("BAR_WS_DIALOGUE_TIME_SCALE", "0.25"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if hadScale {
			_ = os.Setenv("BAR_WS_TIME_SCALE", previousScale)
		} else {
			_ = os.Unsetenv("BAR_WS_TIME_SCALE")
		}
		if hadDialogueScale {
			_ = os.Setenv("BAR_WS_DIALOGUE_TIME_SCALE", previousDialogueScale)
		} else {
			_ = os.Unsetenv("BAR_WS_DIALOGUE_TIME_SCALE")
		}
	}()
	originalCueBuilder := buildBarPerformanceCue
	defer func() { buildBarPerformanceCue = originalCueBuilder }()
	originalWait := waitBarPerformance
	waitBarPerformance = func(time.Duration) {}
	defer func() { waitBarPerformance = originalWait }()
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
		if startedCues == 6 {
			close(allCuesStarted)
		}
		cueLock.Unlock()
		<-allCuesStarted
		switch stage {
		case "opening":
			return result.Drink.DrinkName + "马上开调，你刚说的话我听见啦。", nil
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
	actionLines := 0
	llmLines := 0
	dialogueLinesWithDuration := 0
	techniqueLineInAction := false
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
			if action.Text != "" && action.Source == "llm" {
				actionLines++
			}
		}
		if event.Type == "action.technique" {
			var action struct {
				Text       string `json:"text"`
				Source     string `json:"source"`
				DurationMs int    `json:"duration_ms"`
			}
			if err := json.Unmarshal(event.Payload, &action); err != nil {
				t.Fatal(err)
			}
			techniqueLineInAction = action.Text != "" && action.Source == "llm" && action.DurationMs > 0
		}
		if event.Type == "bartender.say" {
			var line struct {
				Source     string `json:"source"`
				DurationMs int    `json:"duration_ms"`
			}
			if err := json.Unmarshal(event.Payload, &line); err != nil {
				t.Fatal(err)
			}
			if line.Source == "llm" {
				llmLines++
			}
			if line.DurationMs > 0 {
				dialogueLinesWithDuration++
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
	if actionDuration != 2500 {
		t.Fatalf("action duration = %d, want 2500", actionDuration)
	}
	if actionLines != 3 {
		t.Fatalf("action lines = %d, want 3", actionLines)
	}
	if llmLines != 2 {
		t.Fatalf("LLM bartender lines = %d, want 2", llmLines)
	}
	if dialogueLinesWithDuration != 2 {
		t.Fatalf("dialogue lines with duration = %d, want 2", dialogueLinesWithDuration)
	}
	if !techniqueLineInAction {
		t.Fatal("technique action did not carry its LLM line and duration")
	}
	if completed.Drink.OrderedBy != 4321 {
		t.Fatalf("ordered_by = %d, want token user 4321", completed.Drink.OrderedBy)
	}
	if seen["bartender.say"] != 2 {
		t.Fatalf("expected only opening and serving bartender lines; got %d", seen["bartender.say"])
	}
	wantSequence := []string{
		"order.accepted", "bartender.say",
		"action.start",
		"action.start",
		"action.start",
		"action.technique",
		"bartender.say", "order.completed",
	}
	if strings.Join(sequence, ",") != strings.Join(wantSequence, ",") {
		t.Fatalf("unexpected websocket sequence: %v", sequence)
	}
}

func TestDialogueDurationUsesShortDisplayAndLongProcessing(t *testing.T) {
	short := performanceCueDuration("ingredient", "海浪之歌。", 1)
	long := performanceCueDuration("ingredient", strings.Repeat("海风", 30), 1)
	if short != 10000 {
		t.Fatalf("short ingredient duration=%d, want 10000", short)
	}
	if long != 15000 {
		t.Fatalf("long ingredient duration=%d, want 15000", long)
	}
	if performanceCueDuration("ingredient", "海浪之歌。", 2) != 20000 {
		t.Fatal("dialogue scale was not applied")
	}
	technique := performanceCueDuration("technique", strings.Repeat("海风", 20), 1)
	if technique != 30000 {
		t.Fatalf("technique duration=%d, want 30000", technique)
	}
	opening := performanceCueDuration("opening", strings.Repeat("海风", 20), 1)
	serving := performanceCueDuration("serving", strings.Repeat("海风", 40), 1)
	if opening != 6500 {
		t.Fatalf("opening duration=%d, want 6500", opening)
	}
	if serving != 19400 {
		t.Fatalf("serving duration=%d, want 19400", serving)
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

func TestBarBackpackRequiresAuthorization(t *testing.T) {
	request := httptest.NewRequest(http.MethodGet, "/api/bar/backpack", nil)
	response := httptest.NewRecorder()
	barBackpackHandler(response, request)
	var payload struct {
		Code int `json:"code"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != http.StatusUnauthorized {
		t.Fatalf("code=%d, want 401", payload.Code)
	}
}

func TestBarCollectionAndSubmitRequireAuthorization(t *testing.T) {
	tests := []struct {
		method string
		path   string
		body   string
	}{
		{method: http.MethodGet, path: "/api/bar/collect/status"},
		{method: http.MethodPost, path: "/api/bar/collect"},
		{method: http.MethodPost, path: "/api/bar/backpack/submit", body: `{"type_id":14,"quantity":1}`},
	}
	for _, test := range tests {
		t.Run(test.path, func(t *testing.T) {
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()
			mux := http.NewServeMux()
			registerBarRoutes(mux)
			mux.ServeHTTP(response, request)
			var payload struct {
				Code int `json:"code"`
			}
			if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
				t.Fatal(err)
			}
			if payload.Code != http.StatusUnauthorized {
				t.Fatalf("code=%d body=%s", payload.Code, response.Body.String())
			}
		})
	}
}

func TestBarBackpackUsesAuthenticatedUser(t *testing.T) {
	originalResolver := resolveBarUserId
	resolveBarUserId = func(token string) (uint64, error) {
		if token != "backpack-token" {
			return 0, errors.New("invalid token")
		}
		return 8848, nil
	}
	defer func() { resolveBarUserId = originalResolver }()
	request := httptest.NewRequest(http.MethodGet, "/api/bar/backpack", nil)
	request.Header.Set("Authorization", "backpack-token")
	response := httptest.NewRecorder()
	barBackpackHandler(response, request)
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Items []barservice.BackpackItem `json:"items"`
		} `json:"data"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Code != http.StatusOK || payload.Data.Items == nil {
		t.Fatalf("unexpected response: code=%d body=%s", payload.Code, response.Body.String())
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
