package bar

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/forum_server/model"
)

func TestValidateCreateRecipeRequest(t *testing.T) {
	valid := CreateRecipeRequest{Name: "椰林飘香", Technique: "摇和", Items: []CreateRecipeItemRequest{{TypeId: 1, Quantity: 45}, {TypeId: 9, Quantity: 90}, {TypeId: 8, Quantity: 30}}}
	if err := validateCreateRecipeRequest(valid); err != nil {
		t.Fatalf("valid request failed: %v", err)
	}
	invalid := []CreateRecipeRequest{
		{Name: "", Technique: "摇和", Items: valid.Items},
		{Name: "椰林飘香", Technique: "", Items: valid.Items},
		{Name: "椰林飘香", Technique: "摇和"},
		{Name: "椰林飘香", Technique: "摇和", Items: []CreateRecipeItemRequest{{TypeId: 1, Quantity: 1}, {TypeId: 1, Quantity: 2}}},
		{Name: "椰林飘香", Technique: "摇和", Items: []CreateRecipeItemRequest{{TypeId: 1, Quantity: .001}}},
		{Name: "椰林飘香", Technique: "摇和", Items: []CreateRecipeItemRequest{{TypeId: 1, Quantity: 1, Requirement: map[string][]float64{"freshness": {100, 20}}}}},
	}
	for index, request := range invalid {
		if err := validateCreateRecipeRequest(request); err == nil {
			t.Fatalf("invalid request %d succeeded", index)
		}
	}
}

func TestCreateOfficialRecipeTransactionAndDuplicate(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	service := NewService(tx, nil)
	name := fmt.Sprintf("椰林飘香-测试-%d", time.Now().UnixNano())
	request := CreateRecipeRequest{Name: name, Story: "菠萝、椰浆和甘蔗烧吹来一阵热带海风。", Technique: "摇和", Items: []CreateRecipeItemRequest{{TypeId: 1, Quantity: 45}, {TypeId: 9, Quantity: 90, Requirement: map[string][]float64{"freshness": {30, 100}}}, {TypeId: 8, Quantity: 30}}}
	result, err := service.CreateOfficialRecipe(context.Background(), 70003, request)
	if err != nil {
		t.Fatal(err)
	}
	if result.Id == 0 || result.CreatorId != 70003 || result.Status != 0 || len(result.Items) != 3 {
		t.Fatalf("unexpected result: %+v", result)
	}
	for index, item := range result.Items {
		if item.Step != uint8(index+1) || item.Name == "" || item.Unit == "" {
			t.Fatalf("unexpected item %d: %+v", index, item)
		}
	}
	menu, err := service.Menu(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, recipe := range menu {
		if recipe.Id == result.Id && recipe.Name == name {
			found = true
		}
	}
	if !found {
		t.Fatal("created recipe was not visible in menu")
	}
	_, err = service.CreateOfficialRecipe(context.Background(), 70003, request)
	var duplicate *DuplicateRecipeError
	if !errors.As(err, &duplicate) {
		t.Fatalf("duplicate create error=%v", err)
	}
}

func TestCreateOfficialRecipeRejectsUnavailableIngredientWithoutWriting(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	service := NewService(tx, nil)
	name := fmt.Sprintf("坏配方-%d", time.Now().UnixNano())
	_, err := service.CreateOfficialRecipe(context.Background(), 70003, CreateRecipeRequest{Name: name, Technique: "摇和", Items: []CreateRecipeItemRequest{{TypeId: 999999, Quantity: 1}}})
	if err == nil {
		t.Fatal("unavailable ingredient was accepted")
	}
	var count int64
	if err := tx.Model(&model.BarRecipe{}).Where("name = ?", name).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("invalid recipe left %d rows", count)
	}
}
