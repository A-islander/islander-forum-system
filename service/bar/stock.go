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

// Ingredients returns public type metadata and aggregate usable stock without
// exposing individual batches or maintenance-only stock details.
func (s *Service) Ingredients(ctx context.Context) ([]IngredientCatalogItem, error) {
	var types []model.BarIngredientType
	if err := s.db.WithContext(ctx).Where("status = 0").Order("id ASC").Find(&types).Error; err != nil {
		return nil, err
	}
	type stockAggregate struct {
		TypeId uint64
		Qty    float64
	}
	var aggregates []stockAggregate
	now := s.now().Unix()
	if err := s.db.WithContext(ctx).Model(&model.BarIngredientInstance{}).
		Select("type_id, COALESCE(SUM(qty_remain),0) AS qty").
		Where("status = 0 AND qty_remain > 0 AND expire_at > ?", now).
		Group("type_id").Scan(&aggregates).Error; err != nil {
		return nil, err
	}
	stock := make(map[uint64]float64, len(aggregates))
	for _, aggregate := range aggregates {
		stock[aggregate.TypeId] = aggregate.Qty
	}
	result := make([]IngredientCatalogItem, 0, len(types))
	for _, ingredientType := range types {
		availableQty := round(stock[ingredientType.Id], 2)
		enabled := ingredientType.Mixable == 1 && ingredientType.ExtraEnabled == 1 && ingredientType.ExtraMaxQty > 0
		result = append(result, IngredientCatalogItem{
			TypeId: ingredientType.Id, Code: ingredientType.Code, Name: ingredientType.Name,
			Category: ingredientType.Category, Unit: ingredientType.Unit, Codex: ingredientType.Codex,
			ExtraEnabled: enabled, SuggestedQty: ingredientType.ExtraDefaultQty,
			MaxQtyPerDrink: ingredientType.ExtraMaxQty, AvailableQty: availableQty,
			Available: enabled && availableQty > 0,
		})
	}
	return result, nil
}

// Backpack returns the caller's currently usable ingredient batches grouped
// by type. Ownership is always supplied by authenticated server context.
func (s *Service) Backpack(ctx context.Context, userId uint64) ([]BackpackItem, error) {
	if userId == 0 {
		return nil, errors.New("user_id is required")
	}
	result := make([]BackpackItem, 0)
	now := s.now().Unix()
	err := s.db.WithContext(ctx).Table("bar_user_ingredient_instance AS i").
		Select(`t.id AS type_id, t.code, t.name, t.category, t.unit, t.codex,
		        SUM(i.qty_remain) AS quantity, COUNT(*) AS batch_count,
		        MIN(i.expire_at) AS earliest_expire_at`).
		Joins("JOIN bar_ingredient_type AS t ON t.id = i.type_id AND t.status = 0").
		Where("i.user_id = ? AND i.status = 0 AND i.qty_remain > 0 AND i.expire_at > ?", userId, now).
		Group("t.id, t.code, t.name, t.category, t.unit, t.codex").
		Order("t.id ASC").Scan(&result).Error
	if err != nil {
		return nil, err
	}
	for index := range result {
		result[index].Quantity = round(result[index].Quantity, 2)
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
		Where("status = 0 AND expire_at <= ?", now).Updates(map[string]interface{}{"status": 2, "updated_at": now})
	return result.RowsAffected, result.Error
}
