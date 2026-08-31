package route

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/forum_server/controller"
	barservice "github.com/forum_server/service/bar"
	"github.com/gorilla/websocket"
)

func registerBarRoutes(mux *http.ServeMux) {
	mux.Handle("/api/bar/menu", methodMiddleware(http.HandlerFunc(barMenuHandler)))
	mux.Handle("/api/bar/ingredients", methodMiddleware(http.HandlerFunc(barIngredientsHandler)))
	mux.Handle("/api/bar/backpack", methodMiddleware(http.HandlerFunc(barBackpackHandler)))
	mux.Handle("/api/bar/backpack/submit", methodMiddleware(http.HandlerFunc(barBackpackSubmitHandler)))
	mux.Handle("/api/bar/collect/status", methodMiddleware(http.HandlerFunc(barCollectStatusHandler)))
	mux.Handle("/api/bar/collect", methodMiddleware(http.HandlerFunc(barCollectHandler)))
	mux.Handle("/api/bar/order", methodMiddleware(http.HandlerFunc(barOrderHandler)))
	mux.Handle("/api/bar/drink/", methodMiddleware(http.HandlerFunc(barDrinkHandler)))
	mux.Handle("/api/bar/trace/", methodMiddleware(http.HandlerFunc(barTraceHandler)))
	mux.HandleFunc("/ws/bar/order", barWebSocketHandler)
}

func authenticatedBarUser(w http.ResponseWriter, r *http.Request) (uint64, bool) {
	userId, ok := authenticateBarToken(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid or missing Authorization token")
	}
	return userId, ok
}

func barCollectStatusHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	userId, ok := authenticatedBarUser(w, r)
	if !ok {
		return
	}
	status, err := controller.GetBarCollectStatus(r.Context(), userId)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	write(w, status)
}

func barCollectHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	userId, ok := authenticatedBarUser(w, r)
	if !ok {
		return
	}
	result, err := controller.CollectBarItem(r.Context(), userId)
	if err != nil {
		var limit *barservice.DailyCollectLimitError
		if errors.As(err, &limit) {
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"code": http.StatusTooManyRequests,
				"error": "daily_collect_limit_reached", "daily_limit": limit.Status.DailyLimit,
				"used_today": limit.Status.UsedToday, "remaining_today": 0, "resets_at": limit.Status.ResetsAt})
			return
		}
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	write(w, result)
}

func barBackpackSubmitHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodPost) {
		return
	}
	userId, ok := authenticatedBarUser(w, r)
	if !ok {
		return
	}
	var request barservice.SubmitBackpackRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	result, err := controller.SubmitBarBackpackItem(r.Context(), userId, request)
	if err != nil {
		writeBarError(w, err)
		return
	}
	write(w, result)
}

func barBackpackHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	userId, ok := authenticatedBarUser(w, r)
	if !ok {
		return
	}
	items, err := controller.GetBarBackpack(r.Context(), userId)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	write(w, map[string]interface{}{"items": items})
}

func barIngredientsHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	items, err := controller.GetBarIngredients(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	write(w, map[string]interface{}{"ingredients": items, "max_extras_per_drink": 2})
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
	userId, ok := authenticateBarToken(r.Header.Get("Authorization"))
	if !ok {
		writeError(w, http.StatusUnauthorized, "invalid or missing Authorization token")
		return
	}
	var request barservice.OrderRequest
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	request.OrderedBy = userId
	request.OrderedByName = controller.GetUserById(int(userId)).Name
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
	Token   string                  `json:"token"`
	Payload barservice.OrderRequest `json:"payload"`
}

type wsEvent struct {
	Type    string      `json:"type"`
	Ts      int64       `json:"ts"`
	Payload interface{} `json:"payload"`
}

type performanceCueResult struct {
	text   string
	result barservice.OrderResult
	err    error
}

var buildBarPerformanceCue = controller.BuildBarPerformanceCue
var resolveBarUserId = func(token string) (uint64, error) {
	user, err := controller.GetUserByToken(token)
	if err != nil {
		return 0, err
	}
	if user.Id <= 0 {
		return 0, errors.New("invalid user")
	}
	return uint64(user.Id), nil
}

func authenticateBarToken(token string) (uint64, bool) {
	token = strings.TrimSpace(token)
	if token == "" {
		return 0, false
	}
	userId, err := resolveBarUserId(token)
	return userId, err == nil && userId > 0
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
	userId, ok := authenticateBarToken(request.Token)
	if !ok {
		sendWSEvent(connection, "order.failed", map[string]interface{}{"reason": "unauthorized"})
		return
	}
	request.Payload.OrderedBy = userId
	request.Payload.OrderedByName = controller.GetUserById(int(userId)).Name
	_ = connection.SetReadDeadline(time.Time{})
	result, err := controller.MakeBarDrinkForPerformance(r.Context(), request.Payload)
	if err != nil {
		sendWSEvent(connection, "order.failed", wsFailurePayload(err))
		return
	}
	timeScale := wsTimeScale()
	if !sendWSEvent(connection, "order.accepted", map[string]interface{}{
		"order_id": result.OrderId, "recipe_name": result.RecipeName, "technique": result.Technique, "time_scale": timeScale,
	}) {
		return
	}
	buildCue := func(stage string, stepIndex int) <-chan performanceCueResult {
		ready := make(chan performanceCueResult, 1)
		go func(current barservice.OrderResult) {
			line, cueErr := buildBarPerformanceCue(r.Context(), &current, stage, stepIndex)
			ready <- performanceCueResult{text: line, result: current, err: cueErr}
		}(result)
		return ready
	}
	ingredientCues := make([]<-chan performanceCueResult, len(result.Steps))
	for index := range result.Steps {
		ingredientCues[index] = buildCue("ingredient", index)
	}
	techniqueCue := buildCue("technique", -1)
	servingCue := buildCue("serving", -1)
	if !sendWSEvent(connection, "bartender.say", map[string]interface{}{
		"text": fmt.Sprintf("哼，%s？算你会挑。等着，别催。", result.RecipeName), "stage": "opening", "source": "rule",
	}) {
		return
	}
	time.Sleep(time.Duration(scaledPerformanceDuration(2000, timeScale)) * time.Millisecond)
	for index, step := range result.Steps {
		line := ingredientPerformanceLine(step, result.Trace)
		lineSource := "rule"
		if cue, ok := waitPerformanceCue(r.Context(), ingredientCues[index]); ok && cue.err == nil && cue.text != "" {
			line = cue.text
			lineSource = "llm"
		}
		if !sendWSEvent(connection, "bartender.say", map[string]interface{}{
			"text": line, "stage": "ingredient", "source": lineSource, "type_id": step.TypeId,
		}) {
			return
		}
		step.DurationMs = scaledPerformanceDuration(1200, timeScale)
		if !sendWSEvent(connection, "action.start", step) {
			return
		}
		time.Sleep(time.Duration(step.DurationMs) * time.Millisecond)
	}
	techniqueLine := techniquePerformanceLine(result.Technique)
	techniqueSource := "rule"
	if cue, ok := waitPerformanceCue(r.Context(), techniqueCue); ok && cue.err == nil && cue.text != "" {
		techniqueLine = cue.text
		techniqueSource = "llm"
	}
	duration := scaledPerformanceDuration(techniqueDuration(result.Technique), timeScale)
	if !sendWSEvent(connection, "bartender.say", map[string]interface{}{
		"text": techniqueLine, "stage": "technique", "source": techniqueSource,
	}) {
		return
	}
	if !sendWSEvent(connection, "action.technique", map[string]interface{}{"technique": result.Technique, "duration_ms": duration}) {
		return
	}
	time.Sleep(time.Duration(duration) * time.Millisecond)
	serveText := result.Drink.Description
	serveSource := "rule"
	if cue, ok := waitPerformanceCue(r.Context(), servingCue); ok && cue.err == nil && cue.text != "" {
		result = cue.result
		serveText = cue.text
		serveSource = "llm"
	}
	if serveText == "" {
		serveText = fmt.Sprintf("好了，你的%s。别洒了。", result.RecipeName)
	}
	if !sendWSEvent(connection, "bartender.say", map[string]interface{}{
		"text": serveText, "stage": "serving", "source": serveSource,
	}) {
		return
	}
	sendWSEvent(connection, "order.completed", result)
}

func waitPerformanceCue(ctx context.Context, ready <-chan performanceCueResult) (performanceCueResult, bool) {
	select {
	case result := <-ready:
		return result, true
	case <-ctx.Done():
		return performanceCueResult{}, false
	}
}

func wsTimeScale() float64 {
	value, err := strconv.ParseFloat(strings.TrimSpace(os.Getenv("BAR_WS_TIME_SCALE")), 64)
	if err != nil || value <= 0 {
		return 1.5
	}
	if value < 0.25 {
		return 0.25
	}
	if value > 4 {
		return 4
	}
	return value
}

func scaledPerformanceDuration(baseMs int, scale float64) int {
	return int(float64(baseMs)*scale + 0.5)
}

func wsFailurePayload(err error) map[string]interface{} {
	payload := map[string]interface{}{"reason": err.Error()}
	var missing *barservice.MissingError
	if errors.As(err, &missing) {
		payload["reason"] = "missing"
		payload["details"] = missing.Details
	}
	return payload
}

func ingredientPerformanceLine(step barservice.PerformanceStep, trace []barservice.TracePortion) string {
	verb := "先取"
	if step.Action == "加料" {
		verb = "再加入"
	}
	line := fmt.Sprintf("%s%s%g%s。", verb, step.TypeName, step.Qty, step.Unit)
	for _, portion := range trace {
		if portion.TypeId != step.TypeId || portion.SourceNote == "" || portion.SourceNote == "海浪之歌进货" {
			continue
		}
		note := portion.SourceNote
		if strings.HasPrefix(note, "岛民娘从") {
			note = "我从" + strings.TrimPrefix(note, "岛民娘从")
		} else if strings.HasPrefix(note, "岛民娘在") {
			note = "我在" + strings.TrimPrefix(note, "岛民娘在")
		}
		return line + "这批是" + strings.TrimSuffix(note, "。") + "，算你赶上了。"
	}
	return line + "嗯，状态正好。"
}

func techniquePerformanceLine(technique string) string {
	lines := map[string]string{
		"摇和": "抓稳吧台，我要摇了——洒出来可不算我的。",
		"搅和": "别急，慢慢搅开才不会惊了香气。",
		"捣压": "让开一点，草本得现捣才肯出味。",
		"分层": "看好了，手别晃，这层次可不是随便倒出来的。",
		"兑和": "最后兑在一起，气泡马上就醒了。",
	}
	if line := lines[technique]; line != "" {
		return line
	}
	return "最后这一下看着简单，分寸可都在手上。"
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
