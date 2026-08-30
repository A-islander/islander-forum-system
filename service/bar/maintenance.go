package bar

import (
	"context"
	"errors"
	"log"
	"sort"
	"time"

	"github.com/forum_server/model"
)

type MaintenanceReport struct {
	Expired   int64 `json:"expired"`
	Retired   int64 `json:"retired"`
	Restocks  int   `json:"restocks"`
	Processes int   `json:"processes"`
}

func (s *Service) MaintainStock(ctx context.Context) (MaintenanceReport, error) {
	report := MaintenanceReport{}
	locked, release, err := s.acquireMaintenanceLock(ctx)
	if err != nil {
		return report, err
	}
	if !locked {
		log.Printf("bar stock maintenance skipped: another instance holds the lock")
		return report, nil
	}
	defer release()

	now := s.now().Unix()
	result := s.db.WithContext(ctx).Model(&model.BarIngredientInstance{}).
		Where("status = 0 AND expire_at <= ?", now).
		Updates(map[string]interface{}{"status": 2, "updated_at": now})
	if result.Error != nil {
		return report, result.Error
	}
	report.Expired = result.RowsAffected

	var policies []model.BarStockPolicy
	if err := s.db.WithContext(ctx).Where("enabled = 1").Find(&policies).Error; err != nil {
		return report, err
	}
	if err := validateStockPolicies(policies); err != nil {
		return report, err
	}
	retired, err := s.retireStaleStock(ctx, policies, now)
	if err != nil {
		return report, err
	}
	report.Retired = retired
	sort.SliceStable(policies, func(i, j int) bool {
		return policies[i].ReplenishMode == "restock" && policies[j].ReplenishMode != "restock"
	})
	for _, policy := range policies {
		current, err := s.availableQuantity(ctx, policy.TypeId, now)
		if err != nil {
			return report, err
		}
		if current >= policy.MinQty || policy.ReplenishMode == "none" {
			continue
		}
		switch policy.ReplenishMode {
		case "restock":
			quantity := round(policy.MaxQty-current, 2)
			if quantity <= 0 {
				continue
			}
			instance, err := s.Restock(ctx, RestockRequest{TypeId: policy.TypeId, Quantity: quantity, SourceType: 0, Note: "岛民娘日常补货"})
			if err != nil {
				return report, err
			}
			log.Printf("bar maintenance restocked type=%d quantity=%.2f instance=%d", policy.TypeId, quantity, instance.Id)
			report.Restocks++
		case "process":
			var process model.BarProcess
			if err := s.db.WithContext(ctx).Where("id = ? AND status = 0", *policy.ProcessId).Take(&process).Error; err != nil {
				return report, err
			}
			if process.OutputTypeId != policy.TypeId || process.OutputQty <= 0 {
				return report, errors.New("process stock policy does not match process output")
			}
			for current+process.OutputQty <= policy.MaxQty+.000001 {
				instance, err := s.Process(ctx, process.Id, 0)
				if err != nil {
					var missing *MissingError
					if errors.As(err, &missing) {
						log.Printf("bar maintenance skipped process %d: %v", process.Id, err)
						break
					}
					return report, err
				}
				log.Printf("bar maintenance processed process=%d output_type=%d quantity=%.2f instance=%d", process.Id, policy.TypeId, process.OutputQty, instance.Id)
				current += process.OutputQty
				report.Processes++
			}
		}
	}
	return report, nil
}

func validateStockPolicies(policies []model.BarStockPolicy) error {
	for _, policy := range policies {
		if policy.MinQty < 0 || policy.MaxQty <= 0 || policy.MaxQty < policy.MinQty {
			return errors.New("invalid bar stock quantity policy")
		}
		if policy.RetireFreshnessBelow != nil && (*policy.RetireFreshnessBelow < 0 || *policy.RetireFreshnessBelow > 100) {
			return errors.New("invalid bar stock freshness policy")
		}
		if policy.ReplenishMode != "restock" && policy.ReplenishMode != "process" && policy.ReplenishMode != "none" {
			return errors.New("invalid bar stock replenish mode")
		}
		if policy.ReplenishMode == "process" && policy.ProcessId == nil {
			return errors.New("process stock policy requires process_id")
		}
	}
	return nil
}

func (s *Service) retireStaleStock(ctx context.Context, policies []model.BarStockPolicy, now int64) (int64, error) {
	var retired int64
	for _, policy := range policies {
		if policy.RetireFreshnessBelow == nil {
			continue
		}
		var ingredientType model.BarIngredientType
		if err := s.db.WithContext(ctx).Where("id = ? AND status = 0", policy.TypeId).Take(&ingredientType).Error; err != nil {
			return retired, err
		}
		var instances []model.BarIngredientInstance
		if err := s.db.WithContext(ctx).Where("type_id = ? AND status = 0 AND expire_at > ?", policy.TypeId, now).Find(&instances).Error; err != nil {
			return retired, err
		}
		for _, instance := range instances {
			freshness, ok := effectiveAttributes(instance, ingredientType, now)["freshness"]
			if !ok || freshness >= *policy.RetireFreshnessBelow {
				continue
			}
			result := s.db.WithContext(ctx).Model(&model.BarIngredientInstance{}).
				Where("id = ? AND status = 0", instance.Id).
				Updates(map[string]interface{}{"status": 2, "updated_at": now})
			if result.Error != nil {
				return retired, result.Error
			}
			retired += result.RowsAffected
		}
	}
	return retired, nil
}

func (s *Service) availableQuantity(ctx context.Context, typeId uint64, now int64) (float64, error) {
	var quantity float64
	err := s.db.WithContext(ctx).Model(&model.BarIngredientInstance{}).
		Select("COALESCE(SUM(qty_remain),0)").
		Where("type_id = ? AND status = 0 AND qty_remain > 0 AND expire_at > ?", typeId, now).
		Scan(&quantity).Error
	return round(quantity, 2), err
}

func (s *Service) acquireMaintenanceLock(ctx context.Context) (bool, func(), error) {
	sqlDB, err := s.db.DB()
	if err != nil {
		return false, func() {}, err
	}
	connection, err := sqlDB.Conn(ctx)
	if err != nil {
		return false, func() {}, err
	}
	var locked int
	if err := connection.QueryRowContext(ctx, "SELECT GET_LOCK('bar_stock_maintenance', 0)").Scan(&locked); err != nil {
		connection.Close()
		return false, func() {}, err
	}
	release := func() {
		_, _ = connection.ExecContext(context.Background(), "SELECT RELEASE_LOCK('bar_stock_maintenance')")
		_ = connection.Close()
	}
	return locked == 1, release, nil
}

func (s *Service) RunMaintenanceLoop(ctx context.Context, interval time.Duration) {
	if interval <= 0 {
		interval = time.Hour
	}
	run := func() {
		report, err := s.MaintainStock(ctx)
		if err != nil {
			log.Printf("bar stock maintenance failed: %v", err)
			return
		}
		log.Printf("bar stock maintenance completed: expired=%d retired=%d restocks=%d processes=%d", report.Expired, report.Retired, report.Restocks, report.Processes)
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
