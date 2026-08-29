package bar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/forum_server/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db        *gorm.DB
	describer Describer
	now       func() time.Time
}

func NewService(db *gorm.DB, describer Describer) *Service {
	return &Service{db: db, describer: describer, now: time.Now}
}

func NewDefaultService() *Service { return NewService(model.BarDatabase(), nil) }

func parseRequirement(raw json.RawMessage) map[string][]float64 {
	result := make(map[string][]float64)
	if len(raw) == 0 || string(raw) == "null" {
		return result
	}
	_ = json.Unmarshal(raw, &result)
	return result
}

func matchesRequirement(instance model.BarIngredientInstance, requirement map[string][]float64) bool {
	attrs := numberMap(instance.Attrs)
	for key, bounds := range requirement {
		if len(bounds) != 2 {
			return false
		}
		value, ok := attrs[key]
		if !ok || value < bounds[0] || value > bounds[1] {
			return false
		}
	}
	return true
}

func (s *Service) loadRecipe(tx *gorm.DB, recipeId uint64) (model.BarRecipe, []model.BarRecipeItem, error) {
	var recipe model.BarRecipe
	if err := tx.Where("id = ? AND status = 0", recipeId).Take(&recipe).Error; err != nil {
		return recipe, nil, err
	}
	var items []model.BarRecipeItem
	if err := tx.Where("recipe_id = ?", recipeId).Order("step ASC, id ASC").Find(&items).Error; err != nil {
		return recipe, nil, err
	}
	if len(items) == 0 {
		return recipe, nil, errors.New("recipe has no ingredients")
	}
	return recipe, items, nil
}

func (s *Service) allocateItem(tx *gorm.DB, item model.BarRecipeItem, overrideId uint64, now int64) (allocatedInput, *MissingDetail, error) {
	var ingredientType model.BarIngredientType
	if err := tx.Where("id = ? AND status = 0 AND mixable = 1", item.TypeId).Take(&ingredientType).Error; err != nil {
		return allocatedInput{}, nil, err
	}
	var candidates []model.BarIngredientInstance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("type_id = ? AND status = 0 AND qty_remain > 0 AND expire_at > ?", item.TypeId, now).
		Order("expire_at ASC, id ASC").Find(&candidates).Error; err != nil {
		return allocatedInput{}, nil, err
	}

	if overrideId != 0 {
		index := -1
		for i := range candidates {
			if candidates[i].Id == overrideId {
				index = i
				break
			}
		}
		if index < 0 {
			return allocatedInput{}, nil, fmt.Errorf("override instance %d is unavailable for type %d", overrideId, item.TypeId)
		}
		if !matchesRequirement(candidates[index], parseRequirement(item.Requirement)) {
			return allocatedInput{}, nil, fmt.Errorf("override instance %d does not satisfy quality requirements", overrideId)
		}
		chosen := candidates[index]
		candidates = append([]model.BarIngredientInstance{chosen}, append(candidates[:index], candidates[index+1:]...)...)
	}

	need := item.Qty
	input := allocatedInput{item: item, typeInfo: ingredientType}
	requirement := parseRequirement(item.Requirement)
	for _, candidate := range candidates {
		if !matchesRequirement(candidate, requirement) {
			continue
		}
		take := math.Min(candidate.QtyRemain, need)
		if take <= 0 {
			continue
		}
		attrs := numberMap(candidate.Attrs)
		portion := PortionSnapshot{InstanceId: candidate.Id, Code: candidate.Code, Qty: round(take, 2)}
		if freshness, ok := attrs["freshness"]; ok {
			value := freshness
			portion.Freshness = &value
		}
		input.instances = append(input.instances, candidate)
		input.portions = append(input.portions, portion)
		need -= take
		if need <= .000001 {
			break
		}
	}
	if need > .000001 {
		detail := &MissingDetail{TypeId: item.TypeId, Name: ingredientType.Name, Need: item.Qty, Shortage: round(need, 2)}
		return allocatedInput{}, detail, nil
	}
	return input, nil, nil
}

func (s *Service) MakeDrink(ctx context.Context, request OrderRequest) (OrderResult, error) {
	if request.RecipeId == 0 {
		return OrderResult{}, errors.New("recipe_id is required")
	}
	if request.Message == "" {
		request.Message = ""
	}
	var result OrderResult
	var describeInput DescribeInput
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := s.now().Unix()
		recipe, items, err := s.loadRecipe(tx, request.RecipeId)
		if err != nil {
			return err
		}
		inputs := make([]allocatedInput, 0, len(items))
		missing := make([]MissingDetail, 0)
		for _, item := range items {
			input, detail, err := s.allocateItem(tx, item, request.Overrides[item.TypeId], now)
			if err != nil {
				return err
			}
			if detail != nil {
				missing = append(missing, *detail)
				continue
			}
			inputs = append(inputs, input)
		}
		if len(missing) > 0 {
			return &MissingError{Details: missing}
		}

		for _, input := range inputs {
			for _, portion := range input.portions {
				updates := map[string]interface{}{
					"qty_remain": gorm.Expr("qty_remain - ?", portion.Qty),
					"status":     gorm.Expr("IF(qty_remain - ? <= 0, 1, 0)", portion.Qty),
					"updated_at": now,
				}
				updated := tx.Model(&model.BarIngredientInstance{}).
					Where("id = ? AND status = 0 AND qty_remain >= ?", portion.InstanceId, portion.Qty).
					Updates(updates)
				if updated.Error != nil {
					return updated.Error
				}
				if updated.RowsAffected != 1 {
					return errors.New("stock changed concurrently; please retry")
				}
			}
		}

		var mappings []model.BarIngredientFlavor
		if err := tx.Find(&mappings).Error; err != nil {
			return err
		}
		var nodes []model.BarFlavorNode
		if err := tx.Where("status = 0").Find(&nodes).Error; err != nil {
			return err
		}
		flavor, names, stale := computeFlavor(inputs, mappings, nodes)
		appearance := computeAppearance(inputs, recipe.Technique)
		mouthfeel := computeMouthfeel(inputs, recipe.Technique)
		describeInput = DescribeInput{
			RecipeName: recipe.Name, Flavor: flavor, Appearance: appearance, Mouthfeel: mouthfeel,
			FlavorNames: names, HasStaleIngredient: stale,
		}
		// Always persist a fast deterministic description inside the stock
		// transaction. A future remote describer runs only after commit.
		description, err := (RuleDescriber{}).Describe(ctx, describeInput)
		if err != nil {
			return err
		}

		inputSnapshots := make([]InputSnapshot, 0, len(inputs)+1)
		trace := make([]TracePortion, 0)
		steps := make([]PerformanceStep, 0, len(inputs))
		for _, input := range inputs {
			inputSnapshots = append(inputSnapshots, InputSnapshot{Kind: "ingredient", TypeId: input.item.TypeId, Qty: input.item.Qty, Portions: input.portions})
			steps = append(steps, PerformanceStep{Step: input.item.Step, Action: "取料", TypeId: input.item.TypeId, TypeName: input.typeInfo.Name, Qty: input.item.Qty, Unit: input.typeInfo.Unit})
			for _, portion := range input.portions {
				var instance model.BarIngredientInstance
				for _, candidate := range input.instances {
					if candidate.Id == portion.InstanceId {
						instance = candidate
						break
					}
				}
				traceItem := TracePortion{TypeId: input.item.TypeId, TypeName: input.typeInfo.Name, Unit: input.typeInfo.Unit, InstanceId: portion.InstanceId, Code: portion.Code, Qty: portion.Qty, Source: instance.Source, SourceId: instance.SourceId}
				if instance.Source == "restock" {
					var log model.BarRestockLog
					if err := tx.Where("id = ?", instance.SourceId).Take(&log).Error; err == nil {
						traceItem.SourceNote = log.Note
					}
				}
				trace = append(trace, traceItem)
			}
		}
		inputSnapshots = append(inputSnapshots, InputSnapshot{Kind: "technique", Name: recipe.Technique})
		inputsJSON, err := rawJSON(inputSnapshots)
		if err != nil {
			return err
		}
		flavorJSON, err := rawJSON(flavor)
		if err != nil {
			return err
		}
		appearanceJSON, err := rawJSON(appearance)
		if err != nil {
			return err
		}
		mouthfeelJSON, err := rawJSON(mouthfeel)
		if err != nil {
			return err
		}
		drink := model.BarDrink{RecipeId: recipe.Id, OrderedBy: request.OrderedBy, MadeBy: 0, Message: request.Message,
			InputsSnapshot: inputsJSON, FlavorSnapshot: flavorJSON, AppearanceSnapshot: appearanceJSON,
			MouthfeelSnapshot: mouthfeelJSON, Description: description, ConfigVersion: 1, CreatedAt: now}
		if err := tx.Create(&drink).Error; err != nil {
			return err
		}
		if err := tx.Model(&model.BarRecipe{}).Where("id = ?", recipe.Id).
			Updates(map[string]interface{}{"order_count": gorm.Expr("order_count + 1"), "updated_at": now}).Error; err != nil {
			return err
		}
		result = OrderResult{OrderId: fmt.Sprintf("ORD-%d", drink.Id), Drink: drinkView(drink, recipe, inputSnapshots, flavor, appearance, mouthfeel, names), RecipeName: recipe.Name,
			Technique: recipe.Technique, Flavor: flavor, Appearance: appearance, Mouthfeel: mouthfeel, Trace: trace, Steps: steps}
		return nil
	})
	if err == nil && s.describer != nil {
		if description, describeErr := s.describer.Describe(ctx, describeInput); describeErr == nil && description != "" {
			if updateErr := s.db.WithContext(ctx).Model(&model.BarDrink{}).Where("id = ?", result.Drink.Id).
				Update("description", description).Error; updateErr == nil {
				result.Drink.Description = description
			}
		}
	}
	return result, err
}

func (s *Service) Menu(ctx context.Context) ([]MenuRecipe, error) {
	var recipes []model.BarRecipe
	if err := s.db.WithContext(ctx).Where("status = 0").Order("order_count DESC, id ASC").Find(&recipes).Error; err != nil {
		return nil, err
	}
	menu := make([]MenuRecipe, 0, len(recipes))
	now := s.now().Unix()
	for _, recipe := range recipes {
		var items []model.BarRecipeItem
		if err := s.db.WithContext(ctx).Where("recipe_id = ?", recipe.Id).Order("step ASC").Find(&items).Error; err != nil {
			return nil, err
		}
		entry := MenuRecipe{Id: recipe.Id, Name: recipe.Name, Story: recipe.Story, Technique: recipe.Technique,
			Status: "available", Missing: []string{}, FlavorPreview: []FlavorValue{}, OrderCount: recipe.OrderCount}
		hasAnyStock := true
		for _, item := range items {
			var ingredientType model.BarIngredientType
			if err := s.db.WithContext(ctx).Where("id = ?", item.TypeId).Take(&ingredientType).Error; err != nil {
				return nil, err
			}
			var candidates []model.BarIngredientInstance
			if err := s.db.WithContext(ctx).Where("type_id = ? AND status = 0 AND qty_remain > 0 AND expire_at > ?", item.TypeId, now).Find(&candidates).Error; err != nil {
				return nil, err
			}
			available := 0.0
			for _, candidate := range candidates {
				if matchesRequirement(candidate, parseRequirement(item.Requirement)) {
					available += candidate.QtyRemain
				}
			}
			if len(candidates) == 0 {
				hasAnyStock = false
			}
			if available+.000001 < item.Qty {
				entry.Missing = append(entry.Missing, ingredientType.Name)
			}
		}
		if len(entry.Missing) > 0 {
			entry.Status = "missing"
			if !hasAnyStock {
				entry.Status = "sold_out"
			}
		}
		menu = append(menu, entry)
	}
	return menu, nil
}

func (s *Service) Drink(ctx context.Context, id uint64) (DrinkDetail, error) {
	var drink model.BarDrink
	if err := s.db.WithContext(ctx).Where("id = ?", id).Take(&drink).Error; err != nil {
		return DrinkDetail{}, err
	}
	var recipe model.BarRecipe
	if err := s.db.WithContext(ctx).Where("id = ?", drink.RecipeId).Take(&recipe).Error; err != nil {
		return DrinkDetail{}, err
	}
	var inputs []InputSnapshot
	var flavor FlavorSnapshot
	var appearance AppearanceSnapshot
	var mouthfeel MouthfeelSnapshot
	if err := json.Unmarshal(drink.InputsSnapshot, &inputs); err != nil {
		return DrinkDetail{}, err
	}
	if err := json.Unmarshal(drink.FlavorSnapshot, &flavor); err != nil {
		return DrinkDetail{}, err
	}
	if err := json.Unmarshal(drink.AppearanceSnapshot, &appearance); err != nil {
		return DrinkDetail{}, err
	}
	if err := json.Unmarshal(drink.MouthfeelSnapshot, &mouthfeel); err != nil {
		return DrinkDetail{}, err
	}
	var nodes []model.BarFlavorNode
	if err := s.db.WithContext(ctx).Where("status = 0").Find(&nodes).Error; err != nil {
		return DrinkDetail{}, err
	}
	names := make(map[uint64]string, len(nodes))
	for _, node := range nodes {
		names[node.Id] = node.Name
	}
	return DrinkDetail{Drink: drinkView(drink, recipe, inputs, flavor, appearance, mouthfeel, names), Recipe: recipe}, nil
}
