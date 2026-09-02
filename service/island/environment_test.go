package island

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/forum_server/model"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestSeasonAtUsesIslandDateAndSmoothBoundary(t *testing.T) {
	before := time.Date(2026, time.February, 25, 12, 0, 0, 0, islandLocation)
	season, weights := SeasonAt(before)
	if season.Code != "winter" || weights["spring"] <= 0 || weights["winter"] <= 0 {
		t.Fatalf("unexpected pre-spring season: %+v %+v", season, weights)
	}
	onBoundary := time.Date(2026, time.March, 1, 0, 0, 0, 0, islandLocation)
	season, weights = SeasonAt(onBoundary)
	if season.Code != "spring" || weights["spring"] != .5 || weights["winter"] != .5 {
		t.Fatalf("unexpected spring boundary: %+v %+v", season, weights)
	}
}

func TestGenerateSlotRespectsDurationsAndTransitions(t *testing.T) {
	service := &Service{now: func() time.Time { return time.Unix(1, 0) }}
	previous := &model.IslandWeatherSlot{SlotAt: 1, ConditionCode: "clear", TemperatureC: 20, Cloudiness: .1, Humidity: .6, WindSpeedMps: 2}
	at := time.Date(2026, time.July, 8, 12, 0, 0, 0, islandLocation)
	for run := 1; run < minimumRun["clear"]; run++ {
		slot := service.GenerateSlot(context.Background(), at.Add(time.Duration(run)*time.Hour), previous, run, nil)
		if slot.ConditionCode != "clear" {
			t.Fatalf("clear changed before minimum duration at run %d", run)
		}
	}
	slot := service.GenerateSlot(context.Background(), at, previous, maximumRun["clear"], nil)
	if slot.ConditionCode == "clear" {
		t.Fatal("clear did not change after maximum duration")
	}
	if slot.ConditionCode != "partly_cloudy" {
		t.Fatalf("invalid clear transition to %q", slot.ConditionCode)
	}
}

func TestForceEventUsesHighestPriorityAndKeepsNameableWeather(t *testing.T) {
	service := &Service{now: func() time.Time { return time.Unix(1, 0) }}
	high, _ := json.Marshal(weatherModifier{ForceCondition: "storm"})
	low, _ := json.Marshal(weatherModifier{ForceCondition: "clear"})
	events := []model.IslandCalendarEvent{{Priority: 100, WeatherMode: "force", WeatherModifier: high}, {Priority: 10, WeatherMode: "force", WeatherModifier: low}}
	previous := &model.IslandWeatherSlot{ConditionCode: "clear", TemperatureC: 26, Cloudiness: .1, Humidity: .6, WindSpeedMps: 2}
	slot := service.GenerateSlot(context.Background(), time.Date(2026, 7, 1, 12, 0, 0, 0, islandLocation), previous, 3, events)
	if slot.ConditionCode != "storm" || conditionName(slot.ConditionCode) == "" {
		t.Fatalf("highest-priority force event was not applied: %+v", slot)
	}
}

func TestBuildAlertsSplitsWatchAndWarning(t *testing.T) {
	now := int64(10000)
	hourly := []Weather{{SlotAt: now + 3600, ConditionCode: "storm"}, {SlotAt: now + 2*3600, ConditionCode: "storm"}, {SlotAt: now + 3*3600, ConditionCode: "storm"}}
	alerts := buildAlerts(hourly, now)
	if len(alerts) != 2 || alerts[0].Level != "warning" || alerts[1].Level != "watch" {
		t.Fatalf("unexpected alerts: %+v", alerts)
	}
}

func TestBusinessHoursUseViewerLocalTime(t *testing.T) {
	hours := defaultBusinessHours()
	if hours.TimezoneMode != "viewer_local" || hours.OpenTime != "18:00" || hours.CloseTime != "02:00" || !hours.CrossesMidnight {
		t.Fatalf("unexpected default business hours: %+v", hours)
	}
	if hours.BackendEnforced || hours.OffHoursMode != "stocking" {
		t.Fatalf("unexpected business policy: %+v", hours)
	}
	if _, ok := validClock("8:00"); ok {
		t.Fatal("non-padded clock should be rejected")
	}
	if value, ok := validClock("08:30"); !ok || value != "08:30" {
		t.Fatalf("valid clock rejected: value=%q ok=%v", value, ok)
	}
}

func TestMaintainTimelineIsIdempotentAndPreservesManualSlot(t *testing.T) {
	user, password, address := os.Getenv("ISLANDER_DB_USER"), os.Getenv("ISLANDER_DB_PASSWORD"), os.Getenv("ISLANDER_DB_ADDR")
	if user == "" || address == "" {
		t.Skip("database environment is not configured")
	}
	database := fmt.Sprintf("island_service_test_%d", time.Now().UnixNano())
	rootDSN := fmt.Sprintf("%s:%s@tcp(%s)/mysql?charset=utf8mb4&multiStatements=true", user, password, address)
	root, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.Exec("CREATE DATABASE `" + database + "` CHARACTER SET utf8mb4").Error; err != nil {
		t.Fatal(err)
	}
	defer root.Exec("DROP DATABASE IF EXISTS `" + database + "`")
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&multiStatements=true", user, password, address, database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	ddl, err := os.ReadFile("../../migrations/007_island_environment.sql")
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Exec(string(ddl)).Error; err != nil {
		t.Fatal(err)
	}
	fixedNow := time.Date(2026, time.September, 2, 16, 25, 0, 0, islandLocation)
	manual := model.IslandWeatherSlot{SlotAt: floorHour(fixedNow).Unix(), ConditionCode: "fog", SeasonCode: "autumn", Cloudiness: .9, TemperatureC: 23, Humidity: .95, VisibilityKm: 1, WindSpeedMps: 1, Source: "manual", GeneratorVersion: 1, CreatedAt: fixedNow.Unix(), UpdatedAt: fixedNow.Unix()}
	if err := db.Create(&manual).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(db)
	service.now = func() time.Time { return fixedNow }
	first, err := service.MaintainTimeline(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if first.Generated < 48 {
		t.Fatalf("generated only %d slots", first.Generated)
	}
	second, err := service.MaintainTimeline(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Generated != 0 {
		t.Fatalf("second maintenance generated %d slots", second.Generated)
	}
	var stored model.IslandWeatherSlot
	if err := db.Where("slot_at = ?", manual.SlotAt).Take(&stored).Error; err != nil {
		t.Fatal(err)
	}
	if stored.Source != "manual" || stored.ConditionCode != "fog" {
		t.Fatalf("manual slot was overwritten: %+v", stored)
	}
	environment, err := service.Environment(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if environment.Time.Timezone != "Asia/Hong_Kong" || environment.Time.ServerTime != fixedNow.Unix() || len(environment.Weather.Hourly) != 25 {
		t.Fatalf("unexpected environment response: time=%+v hourly=%d", environment.Time, len(environment.Weather.Hourly))
	}
	if environment.BarBusinessHours.TimezoneMode != "viewer_local" || environment.BarBusinessHours.BackendEnforced {
		t.Fatalf("unexpected business hours response: %+v", environment.BarBusinessHours)
	}
}
