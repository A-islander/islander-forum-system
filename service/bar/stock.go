package bar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/forum_server/model"
	"gorm.io/gorm"
)

func (s *Service) Stock(ctx context.Context) ([]StockItem, error) {
	var types []model.BarIngredientType
	if err := s.db.WithContext(ctx).Where("status = 0").Order("id ASC").Find(&types).Error; err != nil {
		return nil, err
	}
	now := s.now().Unix()
	result := make([]StockItem, 0, len(types))
	for _, ingredientType := range types {
		var aggregate struct {
			Qty      float64
			Earliest *int64
		}
		err := s.db.WithContext(ctx).Model(&model.BarIngredientInstance{}).
			Select("COALESCE(SUM(qty_remain),0) AS qty, MIN(expire_at) AS earliest").
			Where("type_id = ? AND status = 0 AND qty_remain > 0 AND expire_at > ?", ingredientType.Id, now).
			Scan(&aggregate).Error
		if err != nil {
			return nil, err
		}
		earliest := int64(0)
		if aggregate.Earliest != nil {
			earliest = *aggregate.Earliest
		}
		result = append(result, StockItem{TypeId: ingredientType.Id, Code: ingredientType.Code, Name: ingredientType.Name,
			Unit: ingredientType.Unit, QtyRemain: round(aggregate.Qty, 2), EarliestExpireAt: earliest})
	}
	return result, nil
}

func (s *Service) Restock(ctx context.Context, request RestockRequest) (model.BarIngredientInstance, error) {
	if request.TypeId == 0 || request.Quantity <= 0 {
		return model.BarIngredientInstance{}, errors.New("type_id and positive quantity are required")
	}
	var result model.BarIngredientInstance
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var ingredientType model.BarIngredientType
		if err := tx.Where("id = ? AND status = 0", request.TypeId).Take(&ingredientType).Error; err != nil {
			return err
		}
		now := s.now().Unix()
		expireAt := request.ExpireAt
		if expireAt == 0 {
			expireAt = now + int64(ingredientType.ShelfLifeDays)*86400
		}
		attrs := request.Attrs
		if attrs == nil && len(ingredientType.DefaultAttrs) > 0 && string(ingredientType.DefaultAttrs) != "null" {
			_ = json.Unmarshal(ingredientType.DefaultAttrs, &attrs)
		}
		attrsJSON, err := rawJSON(attrs)
		if err != nil {
			return err
		}
		log := model.BarRestockLog{TypeId: request.TypeId, Quantity: request.Quantity, InstanceId: 0,
			SourceType: request.SourceType, SourceUid: request.SourceUid, Attrs: attrsJSON, ExpireAt: expireAt,
			Note: request.Note, CreatedAt: now}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		code := fmt.Sprintf("%s-%s-%04d", ingredientType.Code, s.now().Format("060102"), log.Id)
		instance := model.BarIngredientInstance{Code: code, TypeId: request.TypeId, QtyTotal: request.Quantity,
			QtyRemain: request.Quantity, ProducedAt: now, ExpireAt: expireAt, Attrs: attrsJSON,
			Source: "restock", SourceId: log.Id, Status: 0, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&instance).Error; err != nil {
			return err
		}
		if err := tx.Model(&log).Update("instance_id", instance.Id).Error; err != nil {
			return err
		}
		result = instance
		return nil
	})
	return result, err
}

func (s *Service) Trace(ctx context.Context, drinkId uint64) (DrinkTrace, error) {
	detail, err := s.Drink(ctx, drinkId)
	if err != nil {
		return DrinkTrace{}, err
	}
	result := DrinkTrace{DrinkId: drinkId, Inputs: []TraceNode{}}
	for _, input := range detail.Drink.InputsSnapshot {
		if input.Kind != "ingredient" {
			continue
		}
		for _, portion := range input.Portions {
			node, err := s.traceInstance(ctx, portion.InstanceId, map[uint64]bool{})
			if err != nil {
				return DrinkTrace{}, err
			}
			result.Inputs = append(result.Inputs, node)
		}
	}
	return result, nil
}

func (s *Service) traceInstance(ctx context.Context, instanceId uint64, seen map[uint64]bool) (TraceNode, error) {
	if seen[instanceId] {
		return TraceNode{}, errors.New("cycle detected in ingredient trace")
	}
	seen[instanceId] = true
	var instance model.BarIngredientInstance
	if err := s.db.WithContext(ctx).Where("id = ?", instanceId).Take(&instance).Error; err != nil {
		return TraceNode{}, err
	}
	var ingredientType model.BarIngredientType
	if err := s.db.WithContext(ctx).Where("id = ?", instance.TypeId).Take(&ingredientType).Error; err != nil {
		return TraceNode{}, err
	}
	node := TraceNode{Instance: instance, TypeName: ingredientType.Name, Inputs: []TraceNode{}}
	if instance.Source == "restock" {
		var log model.BarRestockLog
		if err := s.db.WithContext(ctx).Where("id = ?", instance.SourceId).Take(&log).Error; err != nil {
			return TraceNode{}, err
		}
		node.Restock = &log
		return node, nil
	}
	if instance.Source == "process" {
		var row struct {
			InputsSnapshot json.RawMessage `gorm:"column:inputs_snapshot"`
		}
		if err := s.db.WithContext(ctx).Table("bar_process_log").Select("inputs_snapshot").Where("id = ?", instance.SourceId).Take(&row).Error; err != nil {
			return TraceNode{}, err
		}
		var inputs []InputSnapshot
		if err := json.Unmarshal(row.InputsSnapshot, &inputs); err != nil {
			return TraceNode{}, err
		}
		for _, input := range inputs {
			for _, portion := range input.Portions {
				child, err := s.traceInstance(ctx, portion.InstanceId, seen)
				if err != nil {
					return TraceNode{}, err
				}
				node.Inputs = append(node.Inputs, child)
			}
		}
		return node, nil
	}
	return TraceNode{}, fmt.Errorf("unknown ingredient source %q", instance.Source)
}

func (s *Service) ExpireStock(ctx context.Context) (int64, error) {
	now := s.now().Unix()
	result := s.db.WithContext(ctx).Model(&model.BarIngredientInstance{}).
		Where("status = 0 AND expire_at < ?", now).Updates(map[string]interface{}{"status": 2, "updated_at": now})
	return result.RowsAffected, result.Error
}
