package controller

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

	islandservice "github.com/forum_server/service/island"
)

var islandService = islandservice.NewDefaultService()

func GetIslandEnvironment(ctx context.Context) (islandservice.Environment, error) {
	return islandService.Environment(ctx)
}

func StartIslandEnvironmentMaintenance(ctx context.Context) {
	enabled := strings.ToLower(strings.TrimSpace(os.Getenv("ISLAND_WEATHER_MAINTENANCE_ENABLED")))
	if enabled == "0" || enabled == "false" || enabled == "off" {
		log.Printf("island weather maintenance disabled")
		return
	}
	interval := 10 * time.Minute
	if raw := strings.TrimSpace(os.Getenv("ISLAND_WEATHER_MAINTENANCE_INTERVAL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			log.Printf("invalid ISLAND_WEATHER_MAINTENANCE_INTERVAL %q; using %s", raw, interval)
		} else {
			interval = parsed
		}
	}
	if report, err := islandService.MaintainTimeline(ctx); err != nil {
		log.Printf("initial island weather maintenance failed: %v", err)
	} else {
		log.Printf("initial island weather maintenance completed: generated=%d deleted=%d", report.Generated, report.Deleted)
	}
	go islandService.RunMaintenanceLoop(ctx, interval)
}
