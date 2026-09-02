package island

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/forum_server/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const generatorVersion = 1

var islandLocation = loadIslandLocation()

type Service struct {
	db          *gorm.DB
	now         func() time.Time
	barSchedule barScheduleConfig
}

type BarSchedulePhase struct {
	Code             string `json:"code"`
	StartTime        string `json:"start_time"`
	EndTime          string `json:"end_time"`
	BartenderPresent bool   `json:"bartender_present"`
	CanOrder         bool   `json:"can_order"`
}

type BarSchedule struct {
	VenueCode        string             `json:"venue_code"`
	Timezone         string             `json:"timezone"`
	CurrentPhase     string             `json:"current_phase"`
	BartenderPresent bool               `json:"bartender_present"`
	CanOrder         bool               `json:"can_order"`
	NextTransitionAt int64              `json:"next_transition_at"`
	BackendEnforced  bool               `json:"backend_enforced"`
	Phases           []BarSchedulePhase `json:"phases"`
}

type barScheduleConfig struct {
	CloseTime    string
	StockingTime string
	OpenTime     string
}

type Season struct {
	Code string `json:"code"`
	Name string `json:"name"`
}

type CalendarEvent struct {
	Code        string `json:"code"`
	Name        string `json:"name"`
	Description string `json:"description"`
	StartsAt    int64  `json:"starts_at"`
	EndsAt      int64  `json:"ends_at"`
	ThemeCode   string `json:"theme_code,omitempty"`
}

type Weather struct {
	SlotAt                   int64   `json:"slot_at"`
	ConditionCode            string  `json:"condition_code"`
	ConditionName            string  `json:"condition_name"`
	Cloudiness               float64 `json:"cloudiness"`
	Precipitation            float64 `json:"precipitation"`
	PrecipitationProbability float64 `json:"precipitation_probability"`
	PrecipitationMmPerHour   float64 `json:"precipitation_mm_per_hour"`
	TemperatureC             float64 `json:"temperature_c"`
	Humidity                 float64 `json:"humidity"`
	VisibilityKm             float64 `json:"visibility_km"`
	WindSpeedMps             float64 `json:"wind_speed_mps"`
	WindDirectionDeg         uint16  `json:"wind_direction_deg"`
	WindDirectionName        string  `json:"wind_direction_name"`
	WaveLevel                float64 `json:"wave_level"`
}

type WeatherSummary struct {
	ConditionCode               string  `json:"condition_code"`
	ConditionName               string  `json:"condition_name"`
	TemperatureMinC             float64 `json:"temperature_min_c"`
	TemperatureMaxC             float64 `json:"temperature_max_c"`
	PrecipitationProbabilityMax float64 `json:"precipitation_probability_max"`
}

type WeatherAlert struct {
	Type     string `json:"type"`
	Level    string `json:"level"`
	StartsAt int64  `json:"starts_at"`
	EndsAt   int64  `json:"ends_at"`
	Message  string `json:"message"`
}

type Environment struct {
	Time struct {
		ServerTime int64  `json:"server_time"`
		Timezone   string `json:"timezone"`
		IslandDate string `json:"island_date"`
	} `json:"time"`
	Calendar struct {
		Season Season          `json:"season"`
		Events []CalendarEvent `json:"events"`
	} `json:"calendar"`
	BarSchedule BarSchedule `json:"bar_schedule"`
	Weather     struct {
		Current            Weather        `json:"current"`
		Next               Weather        `json:"next"`
		TransitionProgress float64        `json:"transition_progress"`
		Today              WeatherSummary `json:"today"`
		Tomorrow           WeatherSummary `json:"tomorrow"`
		Hourly             []Weather      `json:"hourly"`
		Alerts             []WeatherAlert `json:"alerts"`
	} `json:"weather"`
}

type MaintenanceReport struct {
	Generated int   `json:"generated"`
	Deleted   int64 `json:"deleted"`
}

type weatherModifier struct {
	ConditionWeights    map[string]float64 `json:"condition_weights"`
	ForbiddenConditions []string           `json:"forbidden_conditions"`
	ForceCondition      string             `json:"force_condition"`
	TemperatureOffsetC  float64            `json:"temperature_offset_c"`
}

type generationContext struct {
	SeasonWeights map[string]float64 `json:"season_weights"`
	EventCodes    []string           `json:"event_codes,omitempty"`
}

func NewService(db *gorm.DB) *Service {
	return &Service{db: db, now: time.Now, barSchedule: defaultBarScheduleConfig()}
}

func NewDefaultService() *Service {
	service := NewService(model.IslandDatabase())
	service.barSchedule = barScheduleConfigFromEnv()
	return service
}

func defaultBarScheduleConfig() barScheduleConfig {
	return barScheduleConfig{CloseTime: "02:00", StockingTime: "16:00", OpenTime: "18:00"}
}

func barScheduleConfigFromEnv() barScheduleConfig {
	schedule := defaultBarScheduleConfig()
	if value, ok := validClock(os.Getenv("ISLAND_BAR_CLOSE_TIME")); ok {
		schedule.CloseTime = value
	}
	if value, ok := validClock(os.Getenv("ISLAND_BAR_STOCKING_TIME")); ok {
		schedule.StockingTime = value
	}
	if value, ok := validClock(os.Getenv("ISLAND_BAR_OPEN_TIME")); ok {
		schedule.OpenTime = value
	}
	if !(clockMinutes(schedule.CloseTime) < clockMinutes(schedule.StockingTime) && clockMinutes(schedule.StockingTime) < clockMinutes(schedule.OpenTime)) {
		return defaultBarScheduleConfig()
	}
	return schedule
}

func validClock(value string) (string, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", false
	}
	parsed, err := time.Parse("15:04", value)
	if err != nil || parsed.Format("15:04") != value {
		return "", false
	}
	return value, true
}

func clockMinutes(value string) int {
	parsed, _ := time.Parse("15:04", value)
	return parsed.Hour()*60 + parsed.Minute()
}

func (config barScheduleConfig) at(now time.Time) BarSchedule {
	local := now.In(islandLocation)
	minute := local.Hour()*60 + local.Minute()
	closeMinute := clockMinutes(config.CloseTime)
	stockingMinute := clockMinutes(config.StockingTime)
	openMinute := clockMinutes(config.OpenTime)
	phases := []BarSchedulePhase{
		{Code: "open", StartTime: config.OpenTime, EndTime: config.CloseTime, BartenderPresent: true, CanOrder: true},
		{Code: "stocking", StartTime: config.StockingTime, EndTime: config.OpenTime, BartenderPresent: true, CanOrder: false},
		{Code: "closed", StartTime: config.CloseTime, EndTime: config.StockingTime, BartenderPresent: false, CanOrder: false},
	}
	current := phases[0]
	nextMinute := closeMinute
	nextDay := minute >= openMinute
	if minute >= closeMinute && minute < stockingMinute {
		current = phases[2]
		nextMinute = stockingMinute
		nextDay = false
	} else if minute >= stockingMinute && minute < openMinute {
		current = phases[1]
		nextMinute = openMinute
		nextDay = false
	}
	next := time.Date(local.Year(), local.Month(), local.Day(), nextMinute/60, nextMinute%60, 0, 0, islandLocation)
	if nextDay {
		next = next.AddDate(0, 0, 1)
	}
	return BarSchedule{VenueCode: "wave_song_bar", Timezone: "Asia/Shanghai", CurrentPhase: current.Code, BartenderPresent: current.BartenderPresent, CanOrder: current.CanOrder, NextTransitionAt: next.Unix(), BackendEnforced: false, Phases: phases}
}

func loadIslandLocation() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	return loc
}

func floorHour(t time.Time) time.Time { return t.Truncate(time.Hour) }

func SeasonAt(t time.Time) (Season, map[string]float64) {
	local := t.In(islandLocation)
	code := "winter"
	switch local.Month() {
	case time.March, time.April, time.May:
		code = "spring"
	case time.June, time.July, time.August:
		code = "summer"
	case time.September, time.October, time.November:
		code = "autumn"
	}
	weights := map[string]float64{code: 1}
	boundaries := []struct {
		month    time.Month
		from, to string
	}{{time.March, "winter", "spring"}, {time.June, "spring", "summer"}, {time.September, "summer", "autumn"}, {time.December, "autumn", "winter"}}
	for _, boundary := range boundaries {
		b := time.Date(local.Year(), boundary.month, 1, 0, 0, 0, 0, islandLocation)
		days := local.Sub(b).Hours() / 24
		if days >= -7 && days < 7 {
			toWeight := (days + 7) / 14
			weights = map[string]float64{boundary.from: round(toWeight*-1+1, 4), boundary.to: round(toWeight, 4)}
			break
		}
	}
	return Season{Code: code, Name: seasonName(code)}, weights
}

func seasonName(code string) string {
	return map[string]string{"spring": "花潮季", "summer": "盛潮季", "autumn": "丰风季", "winter": "静海季"}[code]
}

func conditionName(code string) string {
	return map[string]string{"clear": "晴朗", "partly_cloudy": "少云", "cloudy": "多云", "fog": "海雾", "light_rain": "小雨", "heavy_rain": "大雨", "storm": "风暴"}[code]
}

var transitions = map[string][]string{
	"clear":         {"clear", "partly_cloudy"},
	"partly_cloudy": {"clear", "partly_cloudy", "cloudy", "light_rain"},
	"cloudy":        {"partly_cloudy", "cloudy", "fog", "light_rain"},
	"fog":           {"fog", "cloudy", "partly_cloudy"},
	"light_rain":    {"partly_cloudy", "cloudy", "light_rain", "heavy_rain"},
	"heavy_rain":    {"cloudy", "light_rain", "heavy_rain", "storm"},
	"storm":         {"light_rain", "heavy_rain", "storm"},
}

var minimumRun = map[string]int{"clear": 3, "partly_cloudy": 2, "cloudy": 2, "fog": 1, "light_rain": 1, "heavy_rain": 1, "storm": 1}
var maximumRun = map[string]int{"clear": 8, "partly_cloudy": 6, "cloudy": 6, "fog": 4, "light_rain": 4, "heavy_rain": 3, "storm": 2}

func (s *Service) GenerateSlot(ctx context.Context, at time.Time, previous *model.IslandWeatherSlot, runLength int, events []model.IslandCalendarEvent) model.IslandWeatherSlot {
	at = floorHour(at)
	season, seasonWeights := SeasonAt(at)
	rng := rand.New(rand.NewSource(slotSeed(at.Unix(), conditionOf(previous))))
	condition := chooseCondition(rng, seasonWeights, previous, runLength)
	tempOffset := 0.0
	forcedCondition := ""
	eventCodes := make([]string, 0, len(events))
	for _, event := range events {
		eventCodes = append(eventCodes, event.Code)
		var modifier weatherModifier
		if len(event.WeatherModifier) > 0 && json.Unmarshal(event.WeatherModifier, &modifier) == nil {
			tempOffset += modifier.TemperatureOffsetC
			if event.WeatherMode == "force" && modifier.ForceCondition != "" && forcedCondition == "" {
				forcedCondition = modifier.ForceCondition
			} else if forcedCondition == "" {
				condition = applyModifier(rng, condition, event.WeatherMode, modifier)
			}
		}
	}
	if forcedCondition != "" {
		condition = forcedCondition
	} else {
		condition = preserveValidTransition(conditionOf(previous), condition)
	}
	values := generateValues(rng, at, seasonWeights, condition, previous, tempOffset)
	contextJSON, _ := json.Marshal(generationContext{SeasonWeights: seasonWeights, EventCodes: eventCodes})
	now := s.now().Unix()
	values.SeasonCode = season.Code
	values.GenerationContext = contextJSON
	values.Source = "auto"
	values.GeneratorVersion = generatorVersion
	values.CreatedAt = now
	values.UpdatedAt = now
	return values
}

func chooseCondition(rng *rand.Rand, seasons map[string]float64, previous *model.IslandWeatherSlot, runLength int) string {
	if previous != nil && runLength < minimumRun[previous.ConditionCode] {
		return previous.ConditionCode
	}
	weightsBySeason := map[string]map[string]float64{
		"spring": {"clear": 26, "partly_cloudy": 28, "cloudy": 20, "fog": 9, "light_rain": 13, "heavy_rain": 3, "storm": 1},
		"summer": {"clear": 22, "partly_cloudy": 22, "cloudy": 17, "fog": 4, "light_rain": 18, "heavy_rain": 12, "storm": 5},
		"autumn": {"clear": 34, "partly_cloudy": 28, "cloudy": 17, "fog": 6, "light_rain": 10, "heavy_rain": 4, "storm": 1},
		"winter": {"clear": 42, "partly_cloudy": 29, "cloudy": 16, "fog": 7, "light_rain": 5, "heavy_rain": 1, "storm": 0.2},
	}
	candidates := transitions[conditionOf(previous)]
	if len(candidates) == 0 {
		candidates = []string{"clear", "partly_cloudy", "cloudy"}
	}
	if previous != nil && runLength >= maximumRun[previous.ConditionCode] {
		filtered := make([]string, 0, len(candidates)-1)
		for _, candidate := range candidates {
			if candidate != previous.ConditionCode {
				filtered = append(filtered, candidate)
			}
		}
		if len(filtered) > 0 {
			candidates = filtered
		}
	}
	weights := make(map[string]float64)
	for _, candidate := range candidates {
		for season, ratio := range seasons {
			weights[candidate] += weightsBySeason[season][candidate] * ratio
		}
	}
	return weightedPick(rng, candidates, weights)
}

func applyModifier(rng *rand.Rand, current, mode string, modifier weatherModifier) string {
	for _, forbidden := range modifier.ForbiddenConditions {
		if current == forbidden {
			current = "cloudy"
		}
	}
	if mode == "prefer" && len(modifier.ConditionWeights) > 0 && rng.Float64() < .55 {
		keys := make([]string, 0, len(modifier.ConditionWeights))
		for key := range modifier.ConditionWeights {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		return weightedPick(rng, keys, modifier.ConditionWeights)
	}
	return current
}

func preserveValidTransition(previous, next string) string {
	if previous == "" {
		return next
	}
	for _, allowed := range transitions[previous] {
		if allowed == next {
			return next
		}
	}
	return previous
}

func conditionOf(slot *model.IslandWeatherSlot) string {
	if slot == nil {
		return ""
	}
	return slot.ConditionCode
}

func weightedPick(rng *rand.Rand, candidates []string, weights map[string]float64) string {
	total := 0.0
	for _, candidate := range candidates {
		total += math.Max(0, weights[candidate])
	}
	if total <= 0 {
		return candidates[0]
	}
	pick := rng.Float64() * total
	for _, candidate := range candidates {
		pick -= math.Max(0, weights[candidate])
		if pick <= 0 {
			return candidate
		}
	}
	return candidates[len(candidates)-1]
}

func generateValues(rng *rand.Rand, at time.Time, seasons map[string]float64, condition string, previous *model.IslandWeatherSlot, tempOffset float64) model.IslandWeatherSlot {
	baseTemp := 0.0
	for season, weight := range seasons {
		baseTemp += map[string]float64{"spring": 21, "summer": 29, "autumn": 24, "winter": 16}[season] * weight
	}
	hour := float64(at.In(islandLocation).Hour())
	temperature := baseTemp + 3.2*math.Sin((hour-8)*math.Pi/12) + between(rng, -1.2, 1.2) + tempOffset
	cloudRanges := map[string][2]float64{"clear": {.02, .22}, "partly_cloudy": {.25, .55}, "cloudy": {.6, .9}, "fog": {.55, .9}, "light_rain": {.7, .96}, "heavy_rain": {.88, 1}, "storm": {.95, 1}}
	precipRanges := map[string][2]float64{"clear": {0, .02}, "partly_cloudy": {0, .12}, "cloudy": {.05, .3}, "fog": {.02, .18}, "light_rain": {.35, .7}, "heavy_rain": {.75, .95}, "storm": {.9, 1}}
	mmRanges := map[string][2]float64{"clear": {0, 0}, "partly_cloudy": {0, .1}, "cloudy": {0, .2}, "fog": {0, .1}, "light_rain": {.2, 3}, "heavy_rain": {3, 14}, "storm": {10, 30}}
	humidityRanges := map[string][2]float64{"clear": {.48, .7}, "partly_cloudy": {.55, .76}, "cloudy": {.65, .84}, "fog": {.88, .99}, "light_rain": {.78, .94}, "heavy_rain": {.88, .98}, "storm": {.9, 1}}
	visibilityRanges := map[string][2]float64{"clear": {18, 35}, "partly_cloudy": {14, 28}, "cloudy": {9, 20}, "fog": {.4, 3}, "light_rain": {5, 14}, "heavy_rain": {1.5, 7}, "storm": {.5, 4}}
	windRanges := map[string][2]float64{"clear": {.5, 4}, "partly_cloudy": {1, 6}, "cloudy": {2, 8}, "fog": {.2, 3}, "light_rain": {2, 8}, "heavy_rain": {5, 13}, "storm": {12, 24}}
	cloud := fromRange(rng, cloudRanges[condition])
	precip := fromRange(rng, precipRanges[condition])
	mm := fromRange(rng, mmRanges[condition])
	humidity := fromRange(rng, humidityRanges[condition])
	visibility := fromRange(rng, visibilityRanges[condition])
	wind := fromRange(rng, windRanges[condition])
	if previous != nil {
		temperature = blend(previous.TemperatureC, temperature, .45)
		cloud = blend(previous.Cloudiness, cloud, .55)
		humidity = blend(previous.Humidity, humidity, .55)
		wind = blend(previous.WindSpeedMps, wind, .55)
	}
	direction := uint16(rng.Intn(360))
	if previous != nil {
		direction = uint16((int(previous.WindDirectionDeg) + rng.Intn(61) - 30 + 360) % 360)
	}
	return model.IslandWeatherSlot{SlotAt: at.Unix(), ConditionCode: condition, Cloudiness: round(cloud, 4), Precipitation: round(precip, 4), PrecipitationProbability: round(math.Min(1, precip+.08), 4), PrecipitationMmPerHour: round(mm, 2), TemperatureC: round(temperature, 2), Humidity: round(humidity, 4), VisibilityKm: round(visibility, 2), WindSpeedMps: round(wind, 2), WindDirectionDeg: direction, WaveLevel: round(math.Max(0, math.Min(1, wind/22+between(rng, -.05, .08))), 4)}
}

func between(rng *rand.Rand, min, max float64) float64  { return min + rng.Float64()*(max-min) }
func fromRange(rng *rand.Rand, pair [2]float64) float64 { return between(rng, pair[0], pair[1]) }
func blend(a, b, ratio float64) float64                 { return a*(1-ratio) + b*ratio }
func round(value float64, places int) float64 {
	p := math.Pow10(places)
	return math.Round(value*p) / p
}
func slotSeed(at int64, previous string) int64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("island-weather-v%d:%d:%s", generatorVersion, at, previous)))
	return int64(h.Sum64())
}

func (s *Service) MaintainTimeline(ctx context.Context) (MaintenanceReport, error) {
	report := MaintenanceReport{}
	locked, release, err := s.acquireLock(ctx)
	if err != nil || !locked {
		return report, err
	}
	defer release()
	now := s.now()
	startLocal := time.Date(now.In(islandLocation).Year(), now.In(islandLocation).Month(), now.In(islandLocation).Day(), 0, 0, 0, 0, islandLocation)
	end := floorHour(now).Add(48 * time.Hour)
	var previous *model.IslandWeatherSlot
	var prior model.IslandWeatherSlot
	priorResult := s.db.WithContext(ctx).Where("slot_at < ?", startLocal.Unix()).Order("slot_at DESC").Limit(1).Find(&prior)
	if priorResult.Error != nil {
		return report, priorResult.Error
	}
	if priorResult.RowsAffected > 0 {
		previous = &prior
	}
	var existingSlots []model.IslandWeatherSlot
	if err := s.db.WithContext(ctx).Where("slot_at >= ? AND slot_at <= ?", startLocal.Unix(), end.Unix()).Order("slot_at ASC").Find(&existingSlots).Error; err != nil {
		return report, err
	}
	existingByAt := make(map[int64]model.IslandWeatherSlot, len(existingSlots))
	for _, slot := range existingSlots {
		existingByAt[slot.SlotAt] = slot
	}
	var timelineEvents []model.IslandCalendarEvent
	if err := s.db.WithContext(ctx).Where("status = 0 AND starts_at < ? AND ends_at > ?", end.Add(time.Hour).Unix(), startLocal.Unix()).Order("priority DESC, id ASC").Find(&timelineEvents).Error; err != nil {
		return report, err
	}
	runLength := 0
	lastStormAt := int64(0)
	if previous != nil && previous.ConditionCode == "storm" {
		lastStormAt = previous.SlotAt
	} else {
		if err := s.db.WithContext(ctx).Model(&model.IslandWeatherSlot{}).Select("COALESCE(MAX(slot_at), 0)").Where("condition_code = ? AND slot_at < ?", "storm", startLocal.Unix()).Scan(&lastStormAt).Error; err != nil {
			return report, err
		}
	}
	for at := startLocal; !at.After(end); at = at.Add(time.Hour) {
		if existing, ok := existingByAt[at.Unix()]; ok {
			if previous != nil && previous.ConditionCode == existing.ConditionCode {
				runLength++
			} else {
				runLength = 1
			}
			previous = &existing
			if existing.ConditionCode == "storm" {
				lastStormAt = existing.SlotAt
			}
			continue
		}
		events := eventsForAt(timelineEvents, at.Unix())
		slot := s.GenerateSlot(ctx, at, previous, runLength, events)
		if slot.ConditionCode == "storm" && !eventsForceCondition(events, "storm") && (!stormDay(at, seasonWeightsAt(at)) || lastStormAt > at.Add(-72*time.Hour).Unix()) {
			slot.ConditionCode = "heavy_rain"
			slot = regenerateValues(slot, at, previous, events)
		}
		result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&slot)
		if result.Error != nil {
			return report, result.Error
		}
		if result.RowsAffected > 0 {
			report.Generated++
		}
		if previous != nil && previous.ConditionCode == slot.ConditionCode {
			runLength++
		} else {
			runLength = 1
		}
		previous = &slot
		if slot.ConditionCode == "storm" {
			lastStormAt = slot.SlotAt
		}
	}
	deleted := s.db.WithContext(ctx).Where("source = ? AND slot_at < ?", "auto", now.Add(-7*24*time.Hour).Unix()).Delete(&model.IslandWeatherSlot{})
	if deleted.Error != nil {
		return report, deleted.Error
	}
	report.Deleted = deleted.RowsAffected
	return report, nil
}

func eventsForAt(events []model.IslandCalendarEvent, at int64) []model.IslandCalendarEvent {
	active := make([]model.IslandCalendarEvent, 0)
	for _, event := range events {
		if event.StartsAt <= at && event.EndsAt > at {
			active = append(active, event)
		}
	}
	return active
}

func seasonWeightsAt(at time.Time) map[string]float64 {
	_, weights := SeasonAt(at)
	return weights
}

func stormDay(at time.Time, seasons map[string]float64) bool {
	chance := 0.0
	seasonChance := map[string]float64{"spring": .05, "summer": .15, "autumn": .08, "winter": .03}
	for season, weight := range seasons {
		chance += seasonChance[season] * weight
	}
	localDate := at.In(islandLocation).Format("2006-01-02")
	rng := rand.New(rand.NewSource(slotSeed(0, "storm-day:"+localDate)))
	return rng.Float64() < chance
}

func eventsForceCondition(events []model.IslandCalendarEvent, condition string) bool {
	for _, event := range events {
		if event.WeatherMode != "force" {
			continue
		}
		var modifier weatherModifier
		if json.Unmarshal(event.WeatherModifier, &modifier) == nil && modifier.ForceCondition == condition {
			return true
		}
	}
	return false
}

func regenerateValues(slot model.IslandWeatherSlot, at time.Time, previous *model.IslandWeatherSlot, events []model.IslandCalendarEvent) model.IslandWeatherSlot {
	season, weights := SeasonAt(at)
	rng := rand.New(rand.NewSource(slotSeed(at.Unix(), conditionOf(previous))))
	tempOffset := 0.0
	for _, event := range events {
		var modifier weatherModifier
		if json.Unmarshal(event.WeatherModifier, &modifier) == nil {
			tempOffset += modifier.TemperatureOffsetC
		}
	}
	values := generateValues(rng, at, weights, slot.ConditionCode, previous, tempOffset)
	values.SeasonCode = season.Code
	values.GenerationContext = slot.GenerationContext
	values.Source = slot.Source
	values.GeneratorVersion = slot.GeneratorVersion
	values.CreatedAt = slot.CreatedAt
	values.UpdatedAt = slot.UpdatedAt
	return values
}

func (s *Service) eventsAt(ctx context.Context, at int64) ([]model.IslandCalendarEvent, error) {
	var events []model.IslandCalendarEvent
	err := s.db.WithContext(ctx).Where("status = 0 AND starts_at <= ? AND ends_at > ?", at, at).Order("priority DESC, id ASC").Find(&events).Error
	return events, err
}

func (s *Service) acquireLock(ctx context.Context) (bool, func(), error) {
	sqlDB, err := s.db.DB()
	if err != nil {
		return false, func() {}, err
	}
	conn, err := sqlDB.Conn(ctx)
	if err != nil {
		return false, func() {}, err
	}
	var locked int
	if err := conn.QueryRowContext(ctx, "SELECT GET_LOCK('island_weather_timeline', 0)").Scan(&locked); err != nil {
		conn.Close()
		return false, func() {}, err
	}
	release := func() {
		_, _ = conn.ExecContext(context.Background(), "SELECT RELEASE_LOCK('island_weather_timeline')")
		_ = conn.Close()
	}
	return locked == 1, release, nil
}

func (s *Service) Environment(ctx context.Context) (Environment, error) {
	var response Environment
	now := s.now()
	currentAt := floorHour(now)
	response.Time.ServerTime = now.Unix()
	response.Time.Timezone = "Asia/Shanghai"
	response.Time.IslandDate = now.In(islandLocation).Format("2006-01-02")
	response.BarSchedule = s.barSchedule.at(now)
	season, _ := SeasonAt(now)
	response.Calendar.Season = season
	events, err := s.eventsAt(ctx, now.Unix())
	if err != nil {
		return response, err
	}
	response.Calendar.Events = make([]CalendarEvent, 0, len(events))
	for _, event := range events {
		response.Calendar.Events = append(response.Calendar.Events, CalendarEvent{Code: event.Code, Name: event.Name, Description: event.Description, StartsAt: event.StartsAt, EndsAt: event.EndsAt, ThemeCode: event.ThemeCode})
	}
	var slots []model.IslandWeatherSlot
	dayStart := time.Date(now.In(islandLocation).Year(), now.In(islandLocation).Month(), now.In(islandLocation).Day(), 0, 0, 0, 0, islandLocation)
	queryStart := dayStart.Unix()
	queryEnd := dayStart.Add(48 * time.Hour).Unix()
	if err := s.db.WithContext(ctx).Where("slot_at >= ? AND slot_at <= ?", queryStart, queryEnd).Order("slot_at ASC").Find(&slots).Error; err != nil {
		return response, err
	}
	byAt := make(map[int64]model.IslandWeatherSlot, len(slots))
	for _, slot := range slots {
		byAt[slot.SlotAt] = slot
	}
	current, ok := byAt[currentAt.Unix()]
	if !ok {
		return response, errors.New("weather timeline is not ready")
	}
	next, ok := byAt[currentAt.Add(time.Hour).Unix()]
	if !ok {
		next = current
	}
	progress := now.Sub(currentAt).Seconds() / 3600
	response.Weather.TransitionProgress = round(progress, 4)
	response.Weather.Current = interpolateWeather(current, next, progress)
	response.Weather.Next = weatherView(next)
	response.Weather.Today = summarizeSlots(slots, dayStart.Unix(), dayStart.Add(24*time.Hour).Unix())
	response.Weather.Tomorrow = summarizeSlots(slots, dayStart.Add(24*time.Hour).Unix(), dayStart.Add(48*time.Hour).Unix())
	response.Weather.Hourly = make([]Weather, 0, 25)
	for i := 0; i <= 24; i++ {
		if slot, exists := byAt[currentAt.Add(time.Duration(i)*time.Hour).Unix()]; exists {
			response.Weather.Hourly = append(response.Weather.Hourly, weatherView(slot))
		}
	}
	response.Weather.Alerts = buildAlerts(response.Weather.Hourly, now.Unix())
	return response, nil
}

func weatherView(slot model.IslandWeatherSlot) Weather {
	return Weather{SlotAt: slot.SlotAt, ConditionCode: slot.ConditionCode, ConditionName: conditionName(slot.ConditionCode), Cloudiness: slot.Cloudiness, Precipitation: slot.Precipitation, PrecipitationProbability: slot.PrecipitationProbability, PrecipitationMmPerHour: slot.PrecipitationMmPerHour, TemperatureC: slot.TemperatureC, Humidity: slot.Humidity, VisibilityKm: slot.VisibilityKm, WindSpeedMps: slot.WindSpeedMps, WindDirectionDeg: slot.WindDirectionDeg, WindDirectionName: windDirectionName(slot.WindDirectionDeg), WaveLevel: slot.WaveLevel}
}
func interpolateWeather(a, b model.IslandWeatherSlot, p float64) Weather {
	w := weatherView(a)
	w.Cloudiness = round(blend(a.Cloudiness, b.Cloudiness, p), 4)
	w.Precipitation = round(blend(a.Precipitation, b.Precipitation, p), 4)
	w.PrecipitationProbability = round(blend(a.PrecipitationProbability, b.PrecipitationProbability, p), 4)
	w.PrecipitationMmPerHour = round(blend(a.PrecipitationMmPerHour, b.PrecipitationMmPerHour, p), 2)
	w.TemperatureC = round(blend(a.TemperatureC, b.TemperatureC, p), 2)
	w.Humidity = round(blend(a.Humidity, b.Humidity, p), 4)
	w.VisibilityKm = round(blend(a.VisibilityKm, b.VisibilityKm, p), 2)
	w.WindSpeedMps = round(blend(a.WindSpeedMps, b.WindSpeedMps, p), 2)
	w.WaveLevel = round(blend(a.WaveLevel, b.WaveLevel, p), 4)
	return w
}
func windDirectionName(deg uint16) string {
	names := []string{"北", "东北", "东", "东南", "南", "西南", "西", "西北"}
	return names[(int(deg)+22)%360/45]
}
func summarizeSlots(slots []model.IslandWeatherSlot, start, end int64) WeatherSummary {
	summary := WeatherSummary{TemperatureMinC: 999, TemperatureMaxC: -999}
	counts := map[string]int{}
	for _, slot := range slots {
		if slot.SlotAt < start || slot.SlotAt >= end {
			continue
		}
		counts[slot.ConditionCode]++
		summary.TemperatureMinC = math.Min(summary.TemperatureMinC, slot.TemperatureC)
		summary.TemperatureMaxC = math.Max(summary.TemperatureMaxC, slot.TemperatureC)
		summary.PrecipitationProbabilityMax = math.Max(summary.PrecipitationProbabilityMax, slot.PrecipitationProbability)
	}
	maxCount := 0
	for _, condition := range []string{"storm", "heavy_rain", "light_rain", "fog", "cloudy", "partly_cloudy", "clear"} {
		count := counts[condition]
		if count > maxCount {
			summary.ConditionCode = condition
			maxCount = count
		}
	}
	summary.ConditionName = conditionName(summary.ConditionCode)
	if maxCount == 0 {
		summary.TemperatureMinC = 0
		summary.TemperatureMaxC = 0
	}
	return summary
}
func buildAlerts(hourly []Weather, now int64) []WeatherAlert {
	alerts := make([]WeatherAlert, 0)
	for _, w := range hourly {
		if w.SlotAt > now+6*3600 {
			break
		}
		kind, message := "", ""
		if w.ConditionCode == "storm" {
			kind = "storm"
			message = "风暴临近，请留意海边与港口区域"
		} else if w.VisibilityKm <= 2 {
			kind = "fog"
			message = "海雾较浓，岛内能见度偏低"
		} else if w.WaveLevel >= .8 {
			kind = "high_wave"
			message = "近岸浪高，请留意海滩与港口"
		}
		if kind == "" {
			continue
		}
		level := "watch"
		if w.SlotAt <= now+2*3600 {
			level = "warning"
		}
		if len(alerts) > 0 && alerts[len(alerts)-1].Type == kind && alerts[len(alerts)-1].Level == level && alerts[len(alerts)-1].EndsAt == w.SlotAt {
			alerts[len(alerts)-1].EndsAt = w.SlotAt + 3600
		} else {
			alerts = append(alerts, WeatherAlert{Type: kind, Level: level, StartsAt: w.SlotAt, EndsAt: w.SlotAt + 3600, Message: message})
		}
	}
	return alerts
}
