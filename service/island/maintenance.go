package island

import (
	"context"
	"log"
	"time"
)

func (s *Service) RunMaintenanceLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	run := func() {
		report, err := s.MaintainTimeline(ctx)
		if err != nil {
			log.Printf("island weather maintenance failed: %v", err)
			return
		}
		log.Printf("island weather maintenance completed: generated=%d deleted=%d", report.Generated, report.Deleted)
	}
	run()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			run()
		}
	}
}
