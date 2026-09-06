package bar

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/forum_server/model"
	"gorm.io/gorm"
)

type CreateRecipeRequest struct {
	Name      string                    `json:"name"`
	Story     string                    `json:"story"`
	Technique string                    `json:"technique"`
	Items     []CreateRecipeItemRequest `json:"items"`
}

type CreateRecipeItemRequest struct {
	TypeId      uint64               `json:"type_id"`
	Quantity    float64              `json:"quantity"`
	Requirement map[string][]float64 `json:"requirement,omitempty"`
}

type CreatedRecipeItem struct {
	TypeId      uint64               `json:"type_id"`
	Code        string               `json:"code"`
	Name        string               `json:"name"`
	Quantity    float64              `json:"quantity"`
	Unit        string               `json:"unit"`
	Requirement map[string][]float64 `json:"requirement,omitempty"`
	Step        uint8                `json:"step"`
}

type CreateRecipeResult struct {
	Id         uint64              `json:"id"`
	Name       string              `json:"name"`
	Story      string              `json:"story"`
	Technique  string              `json:"technique"`
	CreatorId  uint64              `json:"creator_id"`
	Status     uint8               `json:"status"`
	OrderCount uint                `json:"order_count"`
	Items      []CreatedRecipeItem `json:"items"`
	CreatedAt  int64               `json:"created_at"`
}

type DuplicateRecipeError struct{ Name string }

func (e *DuplicateRecipeError) Error() string { return fmt.Sprintf("recipe %q already exists", e.Name) }

func (s *Service) CreateOfficialRecipe(ctx context.Context, request CreateRecipeRequest) (CreateRecipeResult, error) {
	request.Name = strings.TrimSpace(request.Name)
	request.Story = strings.TrimSpace(request.Story)
	request.Technique = strings.TrimSpace(request.Technique)
	if err := validateCreateRecipeRequest(request); err != nil {
		return CreateRecipeResult{}, err
	}

	typeIds := make([]uint64, 0, len(request.Items))
	for _, item := range request.Items {
		typeIds = append(typeIds, item.TypeId)
	}

	var result CreateRecipeResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing int64
		if err := tx.Model(&model.BarRecipe{}).Where("name = ?", request.Name).Count(&existing).Error; err != nil {
			return err
		}
		if existing > 0 {
			return &DuplicateRecipeError{Name: request.Name}
		}

		var ingredientTypes []model.BarIngredientType
		if err := tx.Where("id IN ? AND status = 0 AND mixable = 1", typeIds).Find(&ingredientTypes).Error; err != nil {
			return err
		}
		if len(ingredientTypes) != len(typeIds) {
			return errors.New("one or more ingredient types are unavailable or not mixable")
		}
		typeById := make(map[uint64]model.BarIngredientType, len(ingredientTypes))
		for _, ingredientType := range ingredientTypes {
			typeById[ingredientType.Id] = ingredientType
		}

		now := s.now().Unix()
		recipe := model.BarRecipe{Name: request.Name, Story: request.Story, CreatorId: 0, Technique: request.Technique, Status: 0, OrderCount: 0, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&recipe).Error; err != nil {
			return err
		}
		// BarRecipe defaults zero-valued status to pending (1) for future user
		// submissions. This admin-only path explicitly publishes official recipes.
		if err := tx.Model(&recipe).UpdateColumn("status", 0).Error; err != nil {
			return err
		}
		recipe.Status = 0
		createdItems := make([]CreatedRecipeItem, 0, len(request.Items))
		for index, item := range request.Items {
			var requirement json.RawMessage
			if len(item.Requirement) > 0 {
				raw, err := json.Marshal(item.Requirement)
				if err != nil {
					return err
				}
				requirement = raw
			}
			step := uint8(index + 1)
			recipeItem := model.BarRecipeItem{RecipeId: recipe.Id, TypeId: item.TypeId, Qty: item.Quantity, Requirement: requirement, Step: step}
			if err := tx.Create(&recipeItem).Error; err != nil {
				return err
			}
			ingredientType := typeById[item.TypeId]
			createdItems = append(createdItems, CreatedRecipeItem{TypeId: item.TypeId, Code: ingredientType.Code, Name: ingredientType.Name, Quantity: item.Quantity, Unit: ingredientType.Unit, Requirement: item.Requirement, Step: step})
		}
		result = CreateRecipeResult{Id: recipe.Id, Name: recipe.Name, Story: recipe.Story, Technique: recipe.Technique, CreatorId: recipe.CreatorId, Status: recipe.Status, OrderCount: recipe.OrderCount, Items: createdItems, CreatedAt: recipe.CreatedAt}
		return nil
	})
	return result, err
}

func validateCreateRecipeRequest(request CreateRecipeRequest) error {
	if request.Name == "" || utf8.RuneCountInString(request.Name) > 64 {
		return errors.New("name is required and must not exceed 64 characters")
	}
	if request.Technique == "" || utf8.RuneCountInString(request.Technique) > 32 {
		return errors.New("technique is required and must not exceed 32 characters")
	}
	if utf8.RuneCountInString(request.Story) > 10000 {
		return errors.New("story must not exceed 10000 characters")
	}
	if len(request.Items) == 0 || len(request.Items) > 16 {
		return errors.New("items must contain between 1 and 16 ingredients")
	}
	seen := make(map[uint64]bool, len(request.Items))
	for _, item := range request.Items {
		if item.TypeId == 0 || seen[item.TypeId] {
			return errors.New("ingredient type_id must be non-zero and unique")
		}
		seen[item.TypeId] = true
		if item.Quantity <= 0 || item.Quantity > 5000 || math.Abs(item.Quantity*100-math.Round(item.Quantity*100)) > .000001 {
			return errors.New("ingredient quantity must be between 0.01 and 5000 with at most two decimal places")
		}
		keys := make([]string, 0, len(item.Requirement))
		for key := range item.Requirement {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			bounds := item.Requirement[key]
			if strings.TrimSpace(key) == "" || len(bounds) != 2 || math.IsNaN(bounds[0]) || math.IsNaN(bounds[1]) || math.IsInf(bounds[0], 0) || math.IsInf(bounds[1], 0) || bounds[0] > bounds[1] {
				return errors.New("each requirement must contain a named [min,max] range")
			}
		}
	}
	return nil
}
