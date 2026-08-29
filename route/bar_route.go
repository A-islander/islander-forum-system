package route

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/forum_server/controller"
	barservice "github.com/forum_server/service/bar"
	"github.com/gorilla/websocket"
)

func registerBarRoutes(mux *http.ServeMux) {
	mux.Handle("/api/bar/menu", methodMiddleware(http.HandlerFunc(barMenuHandler)))
	mux.Handle("/api/bar/order", methodMiddleware(http.HandlerFunc(barOrderHandler)))
	mux.Handle("/api/bar/drink/", methodMiddleware(http.HandlerFunc(barDrinkHandler)))
	mux.Handle("/api/bar/trace/", methodMiddleware(http.HandlerFunc(barTraceHandler)))
	mux.Handle("/api/bar/stock", methodMiddleware(http.HandlerFunc(barStockHandler)))
	mux.HandleFunc("/ws/bar/order", barWebSocketHandler)
}

func requireMethod(w http.ResponseWriter, r *http.Request, method string) bool {
	if r.Method == method {
		return true
	}
	writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	return false
}

func decodeJSON(r *http.Request, target interface{}) error {
	decoder := json.NewDecoder(io.LimitReader(r.Body, 1<<20))
	decoder.DisallowUnknownFields()
	return decoder.Decode(target)
}

func pathId(path, prefix string) (uint64, error) {
	value := strings.TrimPrefix(path, prefix)
	if value == "" || strings.Contains(value, "/") {
		return 0, errors.New("invalid id")
	}
	return strconv.ParseUint(value, 10, 64)
}

func barMenuHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	menu, err := controller.GetBarMenu(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	write(w, map[string]interface{}{"recipes": menu})
}

func barOrderHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	var request barservice.OrderRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := controller.MakeBarDrink(r.Context(), request)
	if err != nil {
		writeBarError(w, err)
		return
	}
	write(w, result)
}

func barDrinkHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	id, err := pathId(r.URL.Path, "/api/bar/drink/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	detail, err := controller.GetBarDrink(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	write(w, detail)
}

func barTraceHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	id, err := pathId(r.URL.Path, "/api/bar/trace/")
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	trace, err := controller.GetBarTrace(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	write(w, trace)
}

func barStockHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	stock, err := controller.GetBarStock(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	write(w, stock)
}

func writeBarError(w http.ResponseWriter, err error) {
	var missing *barservice.MissingError
	if errors.As(err, &missing) {
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": http.StatusConflict, "error": "missing", "details": missing.Details})
		return
	}
	writeError(w, http.StatusBadRequest, err.Error())
}

var barUpgrader = websocket.Upgrader{
	ReadBufferSize: 1024, WriteBufferSize: 4096,
	CheckOrigin: func(_ *http.Request) bool { return true },
}

type wsOrderRequest struct {
	Type    string                  `json:"type"`
	Payload barservice.OrderRequest `json:"payload"`
}

type wsEvent struct {
	Type    string      `json:"type"`
	Ts      int64       `json:"ts"`
	Payload interface{} `json:"payload"`
}

func barWebSocketHandler(w http.ResponseWriter, r *http.Request) {
	connection, err := barUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer connection.Close()
	connection.SetReadLimit(1 << 20)
	_ = connection.SetReadDeadline(time.Now().Add(30 * time.Second))
	var request wsOrderRequest
	if err := connection.ReadJSON(&request); err != nil {
		sendWSEvent(connection, "order.failed", map[string]interface{}{"reason": err.Error()})
		return
	}
	if request.Type != "order.create" {
		sendWSEvent(connection, "order.failed", map[string]interface{}{"reason": "first event must be order.create"})
		return
	}
	_ = connection.SetReadDeadline(time.Time{})
	if !sendWSEvent(connection, "order.accepted", map[string]interface{}{"recipe_id": request.Payload.RecipeId}) {
		return
	}
	if !sendWSEvent(connection, "bartender.say", map[string]interface{}{"text": "哼，单子收到了。等着，别催。"}) {
		return
	}
	result, err := controller.MakeBarDrink(r.Context(), request.Payload)
	if err != nil {
		sendWSEvent(connection, "order.failed", map[string]interface{}{"reason": err.Error()})
		return
	}
	for _, step := range result.Steps {
		if !sendWSEvent(connection, "action.start", step) {
			return
		}
		time.Sleep(800 * time.Millisecond)
	}
	duration := techniqueDuration(result.Technique)
	if !sendWSEvent(connection, "action.technique", map[string]interface{}{"technique": result.Technique, "duration_ms": duration}) {
		return
	}
	time.Sleep(time.Duration(duration) * time.Millisecond)
	if !sendWSEvent(connection, "bartender.say", map[string]interface{}{"text": fmt.Sprintf("好了，你的%s。别洒了。", result.RecipeName)}) {
		return
	}
	sendWSEvent(connection, "order.completed", result)
}

func techniqueDuration(technique string) int {
	durations := map[string]int{"摇和": 3000, "搅和": 2000, "捣压": 1500, "分层": 2000, "兑和": 1500}
	if value := durations[technique]; value != 0 {
		return value
	}
	return 1000
}

func sendWSEvent(connection *websocket.Conn, eventType string, payload interface{}) bool {
	_ = connection.SetWriteDeadline(time.Now().Add(5 * time.Second))
	return connection.WriteJSON(wsEvent{Type: eventType, Ts: time.Now().Unix(), Payload: payload}) == nil
}
