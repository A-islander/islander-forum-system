package bar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/forum_server/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Service struct {
	db         *gorm.DB
	describer  Describer
	now        func() time.Time
	randomIntn func(int) (int, error)
}

func NewService(db *gorm.DB, describer Describer) *Service {
	return &Service{db: db, describer: describer, now: time.Now, randomIntn: defaultRandomIntn}
}

func NewDefaultService(describer ...Describer) *Service {
	var selected Describer
	if len(describer) > 0 {
		selected = describer[0]
	}
	return NewService(model.BarDatabase(), selected)
}

func parseRequirement(raw json.RawMessage) map[string][]float64 {
	result := make(map[string][]float64)
	if len(raw) == 0 || string(raw) == "null" {
		return result
	}
	_ = json.Unmarshal(raw, &result)
	return result
}

func matchesRequirement(instance model.BarIngredientInstance, ingredientType model.BarIngredientType, requirement map[string][]float64, now int64) bool {
	attrs := effectiveAttributes(instance, ingredientType, now)
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
		if !matchesRequirement(candidates[index], ingredientType, parseRequirement(item.Requirement), now) {
			return allocatedInput{}, nil, fmt.Errorf("override instance %d does not satisfy quality requirements", overrideId)
		}
		chosen := candidates[index]
		candidates = append([]model.BarIngredientInstance{chosen}, append(candidates[:index], candidates[index+1:]...)...)
	}

	need := item.Qty
	input := allocatedInput{inventory: "bar", item: item, typeInfo: ingredientType}
	requirement := parseRequirement(item.Requirement)
	for _, candidate := range candidates {
		if !matchesRequirement(candidate, ingredientType, requirement, now) {
			continue
		}
		take := math.Min(candidate.QtyRemain, need)
		if take <= 0 {
			continue
		}
		attrs := effectiveAttributes(candidate, ingredientType, now)
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

func userAsBarInstance(instance model.BarUserIngredientInstance) model.BarIngredientInstance {
	return model.BarIngredientInstance{Id: instance.Id, TypeId: instance.TypeId, QtyTotal: instance.QtyTotal,
		QtyRemain: instance.QtyRemain, ProducedAt: instance.ProducedAt, ExpireAt: instance.ExpireAt,
		Attrs: instance.Attrs, Source: instance.Source, SourceId: instance.SourceId, Status: instance.Status,
		CreatedAt: instance.CreatedAt, UpdatedAt: instance.UpdatedAt}
}

func (s *Service) allocateUserItem(tx *gorm.DB, item model.BarRecipeItem, userId uint64, now int64) (allocatedInput, *MissingDetail, error) {
	var ingredientType model.BarIngredientType
	if err := tx.Where("id = ? AND status = 0 AND mixable = 1", item.TypeId).Take(&ingredientType).Error; err != nil {
		return allocatedInput{}, nil, err
	}
	var candidates []model.BarUserIngredientInstance
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).
		Where("user_id = ? AND type_id = ? AND status = 0 AND qty_remain > 0 AND expire_at > ?", userId, item.TypeId, now).
		Order("expire_at ASC, id ASC").Find(&candidates).Error; err != nil {
		return allocatedInput{}, nil, err
	}
	need := item.Qty
	input := allocatedInput{inventory: "backpack", item: item, typeInfo: ingredientType}
	for _, candidate := range candidates {
		take := math.Min(candidate.QtyRemain, need)
		if take <= 0 {
			continue
		}
		attrs := effectiveAttributes(userAsBarInstance(candidate), ingredientType, now)
		portion := PortionSnapshot{Inventory: "backpack", InstanceId: candidate.Id,
			Code: fmt.Sprintf("%s-U-%d", ingredientType.Code, candidate.Id), Qty: round(take, 2)}
		if freshness, ok := attrs["freshness"]; ok {
			value := freshness
			portion.Freshness = &value
		}
		input.userInstances = append(input.userInstances, candidate)
		input.portions = append(input.portions, portion)
		need -= take
		if need <= .000001 {
			break
		}
	}
	if need > .000001 {
		return allocatedInput{}, &MissingDetail{TypeId: item.TypeId, Name: ingredientType.Name, Need: item.Qty, Shortage: round(need, 2)}, nil
	}
	return input, nil, nil
}

func deductStock(tx *gorm.DB, instanceId uint64, quantity float64, now int64) error {
	result := tx.Exec(`
		UPDATE bar_ingredient_instance
		SET status = IF(qty_remain - ? <= 0, 1, 0),
		    qty_remain = qty_remain - ?,
		    updated_at = ?
		WHERE id = ? AND status = 0 AND qty_remain >= ?`,
		quantity, quantity, now, instanceId, quantity)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected != 1 {
		return errors.New("stock changed concurrently; please retry")
	}
	return nil
}

func deductInput(tx *gorm.DB, input allocatedInput, now int64) error {
	for _, portion := range input.portions {
		var err error
		if input.inventory == "backpack" {
			result := tx.Exec(`UPDATE bar_user_ingredient_instance
				SET status=IF(qty_remain-?<=0,1,0), qty_remain=qty_remain-?, updated_at=?
				WHERE id=? AND status=0 AND qty_remain>=?`, portion.Qty, portion.Qty, now, portion.InstanceId, portion.Qty)
			err = result.Error
			if err == nil && result.RowsAffected != 1 {
				err = errors.New("backpack stock changed concurrently; please retry")
			}
		} else {
			err = deductStock(tx, portion.InstanceId, portion.Qty, now)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (s *Service) MakeDrink(ctx context.Context, request OrderRequest) (OrderResult, error) {
	return s.makeDrink(ctx, request, true)
}

func validateExtras(extras []ExtraIngredient) error {
	if len(extras) > 2 {
		return errors.New("at most two extra ingredients are allowed")
	}
	seen := make(map[uint64]bool, len(extras))
	for _, extra := range extras {
		if extra.Source != "" && extra.Source != "bar" && extra.Source != "backpack" {
			return fmt.Errorf("unsupported extra ingredient source %q", extra.Source)
		}
		if extra.TypeId == 0 || extra.Quantity <= 0 || math.IsNaN(extra.Quantity) || math.IsInf(extra.Quantity, 0) {
			return errors.New("each extra ingredient requires type_id and positive quantity")
		}
		if math.Abs(round(extra.Quantity, 2)-extra.Quantity) > .000001 {
			return errors.New("extra ingredient quantity supports at most two decimal places")
		}
		if seen[extra.TypeId] {
			return fmt.Errorf("duplicate extra ingredient type %d", extra.TypeId)
		}
		seen[extra.TypeId] = true
	}
	return nil
}

func (s *Service) loadExtraItem(tx *gorm.DB, extra ExtraIngredient, step uint8) (model.BarRecipeItem, error) {
	var ingredientType model.BarIngredientType
	if err := tx.Where("id = ? AND status = 0 AND mixable = 1 AND extra_enabled = 1", extra.TypeId).Take(&ingredientType).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.BarRecipeItem{}, fmt.Errorf("ingredient type %d cannot be added to a drink", extra.TypeId)
		}
		return model.BarRecipeItem{}, err
	}
	if ingredientType.ExtraMaxQty <= 0 || extra.Quantity > ingredientType.ExtraMaxQty {
		return model.BarRecipeItem{}, fmt.Errorf("extra %s exceeds the per-drink maximum of %g%s", ingredientType.Name, ingredientType.ExtraMaxQty, ingredientType.Unit)
	}
	return model.BarRecipeItem{TypeId: extra.TypeId, Qty: extra.Quantity, Step: step}, nil
}

func (s *Service) makeDrink(ctx context.Context, request OrderRequest, enhance bool) (OrderResult, error) {
	if request.RecipeId == 0 {
		return OrderResult{}, errors.New("recipe_id is required")
	}
	if err := validateExtras(request.Extras); err != nil {
		return OrderResult{}, err
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
		inputs := make([]allocatedInput, 0, len(items)+len(request.Extras))
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
			if err := deductInput(tx, input, now); err != nil {
				return err
			}
			input.role = "recipe"
			inputs = append(inputs, input)
		}
		for index, extra := range request.Extras {
			item, err := s.loadExtraItem(tx, extra, uint8(len(items)+index+1))
			if err != nil {
				return err
			}
			var input allocatedInput
			var detail *MissingDetail
			if extra.Source == "backpack" {
				input, detail, err = s.allocateUserItem(tx, item, request.OrderedBy, now)
			} else {
				input, detail, err = s.allocateItem(tx, item, 0, now)
			}
			if err != nil {
				return err
			}
			if detail != nil {
				missing = append(missing, *detail)
				continue
			}
			if err := deductInput(tx, input, now); err != nil {
				return err
			}
			input.role = "extra"
			inputs = append(inputs, input)
		}
		if len(missing) > 0 {
			return &MissingError{Details: missing}
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
		drinkName := recipe.Name
		if len(request.Extras) > 0 {
			parts := []string{recipe.Name}
			for _, input := range inputs {
				if input.role == "extra" {
					parts = append(parts, input.typeInfo.Name)
				}
			}
			ownerName := strings.TrimSpace(request.OrderedByName)
			if ownerName == "" {
				ownerName = fmt.Sprintf("岛民%d", request.OrderedBy)
			}
			parts = append(parts, ownerName)
			drinkName = strings.Join(parts, "-")
			if runes := []rune(drinkName); len(runes) > 128 {
				drinkName = string(runes[:128])
			}
		}
		describeIngredients := make([]DescribeIngredient, 0, len(inputs))
		for _, input := range inputs {
			ingredient := DescribeIngredient{Name: input.typeInfo.Name, Qty: input.item.Qty, Unit: input.typeInfo.Unit, Role: input.role}
			for _, portion := range input.portions {
				if portion.Freshness != nil && (ingredient.Freshness == nil || *portion.Freshness < *ingredient.Freshness) {
					freshness := *portion.Freshness
					ingredient.Freshness = &freshness
				}
			}
			if ingredient.Freshness != nil && *ingredient.Freshness < 30 {
				ingredient.Condition = "已过最佳状态，会带来陈味"
			}
			describeIngredients = append(describeIngredients, ingredient)
		}
		describeInput = DescribeInput{
			RecipeName: drinkName, RecipeStory: recipe.Story, Technique: recipe.Technique,
			CustomerMessage: request.Message, Flavor: flavor, Appearance: appearance, Mouthfeel: mouthfeel,
			FlavorNames: names, Ingredients: describeIngredients, HasStaleIngredient: stale,
		}

		inputSnapshots := make([]InputSnapshot, 0, len(inputs)+1)
		trace := make([]TracePortion, 0)
		steps := make([]PerformanceStep, 0, len(inputs))
		for _, input := range inputs {
			inputSnapshots = append(inputSnapshots, InputSnapshot{Kind: "ingredient", Role: input.role, TypeId: input.item.TypeId, Qty: input.item.Qty, Portions: input.portions})
			action := "取料"
			if input.role == "extra" {
				action = "加料"
			}
			steps = append(steps, PerformanceStep{Step: input.item.Step, Action: action, TypeId: input.item.TypeId, TypeName: input.typeInfo.Name, Qty: input.item.Qty, Unit: input.typeInfo.Unit})
			for _, portion := range input.portions {
				var instance model.BarIngredientInstance
				for _, candidate := range input.instances {
					if candidate.Id == portion.InstanceId {
						instance = candidate
						break
					}
				}
				traceItem := TracePortion{Role: input.role, Inventory: input.inventory, TypeId: input.item.TypeId, TypeName: input.typeInfo.Name, Unit: input.typeInfo.Unit, InstanceId: portion.InstanceId, Code: portion.Code, Qty: portion.Qty}
				if input.inventory == "backpack" {
					for _, candidate := range input.userInstances {
						if candidate.Id != portion.InstanceId {
							continue
						}
						traceItem.Source, traceItem.SourceId = candidate.Source, candidate.SourceId
						if candidate.Source == "collect" {
							var row struct{ LocationName string }
							_ = tx.Table("bar_collect_log AS l").Select("p.name AS location_name").
								Joins("JOIN bar_collect_location AS p ON p.id=l.location_id").Where("l.id=?", candidate.SourceId).Scan(&row).Error
							traceItem.SourceNote = row.LocationName + "搜集"
						}
						break
					}
				} else {
					traceItem.Source, traceItem.SourceId = instance.Source, instance.SourceId
				}
				if input.inventory != "backpack" && instance.Source == "restock" {
					var log model.BarRestockLog
					if err := tx.Where("id = ?", instance.SourceId).Take(&log).Error; err == nil {
						traceItem.SourceNote = log.Note
					}
				}
				trace = append(trace, traceItem)
			}
		}
		describeInput.Trace = trace
		// Persist a deterministic description inside the stock transaction. A
		// remote description may replace it only after the drink is committed.
		description, err := (RuleDescriber{}).Describe(ctx, describeInput)
		if err != nil {
			return err
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
		drink := model.BarDrink{RecipeId: recipe.Id, OrderedBy: request.OrderedBy, MadeBy: 0, Name: drinkName, Message: request.Message,
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
			Technique: recipe.Technique, Flavor: flavor, Appearance: appearance, Mouthfeel: mouthfeel, Trace: trace, Steps: steps, DescribeInput: describeInput}
		return nil
	})
	if err == nil && enhance {
		s.EnhanceDescription(ctx, &result)
	}
	return result, err
}

// MakeDrinkForPerformance commits the drink with its rule description and
// leaves the remote description to the caller so it can overlap with the show.
func (s *Service) MakeDrinkForPerformance(ctx context.Context, request OrderRequest) (OrderResult, error) {
	return s.makeDrink(ctx, request, false)
}

// MakeDrinkAsync returns as soon as the deterministic drink is committed. A
// remote description is generated outside the request context and replaces the
// rule description in the same bar_drink row when it succeeds.
func (s *Service) MakeDrinkAsync(ctx context.Context, request OrderRequest) (OrderResult, error) {
	result, err := s.makeDrink(ctx, request, false)
	if err != nil || s.describer == nil {
		return result, err
	}
	result.DescriptionPending = true
	go func(current OrderResult) {
		background, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		s.EnhanceDescription(background, &current)
	}(result)
	return result, nil
}

func (s *Service) EnhanceDescription(ctx context.Context, result *OrderResult) {
	if s.describer == nil || result == nil || result.Drink.Id == 0 {
		return
	}
	description, err := s.describer.Describe(ctx, result.DescribeInput)
	if err != nil || strings.TrimSpace(description) == "" {
		return
	}
	description = strings.TrimSpace(description)
	if updateErr := s.db.WithContext(ctx).Model(&model.BarDrink{}).Where("id = ?", result.Drink.Id).
		Update("description", description).Error; updateErr == nil {
		result.Drink.Description = description
	}
}

// BuildPerformanceCue generates one WS line. Callers may launch cues in
// parallel and consume them in recipe order. Deterministic lines remain the
// fallback for any individual failed cue.
func (s *Service) BuildPerformanceCue(ctx context.Context, result *OrderResult, stage string, stepIndex int) (string, error) {
	if s.describer == nil || result == nil || result.Drink.Id == 0 {
		return "", errors.New("performance describer is unavailable")
	}
	if stage == "serving" {
		description, err := s.describer.Describe(ctx, result.DescribeInput)
		if err != nil {
			return "", err
		}
		description = strings.TrimSpace(description)
		if description == "" {
			return "", errors.New("empty serving description")
		}
		if updateErr := s.db.WithContext(ctx).Model(&model.BarDrink{}).Where("id = ?", result.Drink.Id).
			Update("description", description).Error; updateErr != nil {
			return "", updateErr
		}
		result.Drink.Description = description
		return description, nil
	}
	describer, ok := s.describer.(PerformanceDescriber)
	if !ok {
		return "", errors.New("describer does not support performance cues")
	}
	var step *PerformanceStep
	if stage == "ingredient" {
		if stepIndex < 0 || stepIndex >= len(result.Steps) {
			return "", errors.New("ingredient performance cue index is out of range")
		}
		step = &result.Steps[stepIndex]
	}
	return describer.DescribePerformanceCue(ctx, result.DescribeInput, stage, step)
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
				if matchesRequirement(candidate, ingredientType, parseRequirement(item.Requirement), now) {
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
