package bar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/forum_server/model"
	"gorm.io/gorm"
)

type processInputRule struct {
	TypeId      uint64          `json:"type_id"`
	Qty         float64         `json:"qty"`
	Requirement json.RawMessage `json:"requirement"`
}

func (s *Service) Process(ctx context.Context, processId, operatorUid uint64) (model.BarIngredientInstance, error) {
	if processId == 0 {
		return model.BarIngredientInstance{}, errors.New("process_id is required")
	}
	var output model.BarIngredientInstance
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var process model.BarProcess
		if err := tx.Where("id = ? AND status = 0", processId).Take(&process).Error; err != nil {
			return err
		}
		var rules []processInputRule
		if err := json.Unmarshal(process.Inputs, &rules); err != nil || len(rules) == 0 {
			return errors.New("process has invalid inputs")
		}
		now := s.now().Unix()
		inputs := make([]allocatedInput, 0, len(rules))
		for index, rule := range rules {
			if rule.TypeId == 0 || rule.Qty <= 0 {
				return errors.New("process has invalid input quantity")
			}
			item := model.BarRecipeItem{TypeId: rule.TypeId, Qty: rule.Qty, Requirement: rule.Requirement, Step: uint8(index + 1)}
			input, missing, err := s.allocateItem(tx, item, 0, now)
			if err != nil {
				return err
			}
			if missing != nil {
				return &MissingError{Details: []MissingDetail{*missing}}
			}
			inputs = append(inputs, input)
		}
		for _, input := range inputs {
			for _, portion := range input.portions {
				if err := deductStock(tx, portion.InstanceId, portion.Qty, now); err != nil {
					return err
				}
			}
		}

		var outputType model.BarIngredientType
		if err := tx.Where("id = ? AND status = 0", process.OutputTypeId).Take(&outputType).Error; err != nil {
			return err
		}
		attrs := numberMap(outputType.DefaultAttrs)
		for key, value := range deriveProcessAttributes(process.AttributeRule, inputs, now) {
			attrs[key] = value
		}
		attrsJSON, err := rawJSON(attrs)
		if err != nil {
			return err
		}
		snapshots := make([]InputSnapshot, 0, len(inputs))
		for _, input := range inputs {
			snapshots = append(snapshots, InputSnapshot{Kind: "ingredient", TypeId: input.item.TypeId, Qty: input.item.Qty, Portions: input.portions})
		}
		inputsJSON, err := rawJSON(snapshots)
		if err != nil {
			return err
		}
		log := model.BarProcessLog{ProcessId: process.Id, OperatorUid: operatorUid, InputsSnapshot: inputsJSON, OutputInstanceId: 0, CreatedAt: now}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		shelfLifeDays := outputType.ShelfLifeDays
		if process.ShelfLifeDays != nil {
			shelfLifeDays = *process.ShelfLifeDays
		}
		output = model.BarIngredientInstance{
			Code: fmt.Sprintf("%s-P%s-%04d", outputType.Code, s.now().Format("060102"), log.Id), TypeId: outputType.Id,
			QtyTotal: process.OutputQty, QtyRemain: process.OutputQty, ProducedAt: now, ExpireAt: now + int64(shelfLifeDays)*86400,
			Attrs: attrsJSON, Source: "process", SourceId: log.Id, Status: 0, CreatedAt: now, UpdatedAt: now,
		}
		if err := tx.Create(&output).Error; err != nil {
			return err
		}
		return tx.Model(&log).Update("output_instance_id", output.Id).Error
	})
	return output, err
}

func deriveProcessAttributes(raw json.RawMessage, inputs []allocatedInput, now int64) map[string]float64 {
	var rules map[string]string
	if json.Unmarshal(raw, &rules) != nil {
		return map[string]float64{}
	}
	result := make(map[string]float64)
	for key, expression := range rules {
		values, weights := []float64{}, []float64{}
		for _, input := range inputs {
			for index, portion := range input.portions {
				attrs := effectiveAttributes(input.instances[index], input.typeInfo, now)
				if value, ok := attrs[key]; ok {
					values, weights = append(values, value), append(weights, portion.Qty)
				}
			}
		}
		if len(values) == 0 {
			continue
		}
		parts := strings.Fields(expression)
		mode, factor := parts[0], 1.0
		if len(parts) == 3 && parts[1] == "*" {
			if parsed, err := strconv.ParseFloat(parts[2], 64); err == nil {
				factor = parsed
			}
		}
		value := values[0]
		switch mode {
		case "min":
			for _, candidate := range values[1:] {
				value = math.Min(value, candidate)
			}
		case "max":
			for _, candidate := range values[1:] {
				value = math.Max(value, candidate)
			}
		case "avg":
			value = 0
			for _, candidate := range values {
				value += candidate
			}
			value /= float64(len(values))
		case "wavg":
			value, totalWeight := 0.0, 0.0
			for index, candidate := range values {
				value, totalWeight = value+candidate*weights[index], totalWeight+weights[index]
			}
			if totalWeight > 0 {
				value /= totalWeight
			}
		default:
			continue
		}
		result[key] = round(math.Max(0, math.Min(100, value*factor)), 2)
	}
	return result
}
