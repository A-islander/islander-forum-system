package bar

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/forum_server/model"
)

func TestRuleDescriber(t *testing.T) {
	description, err := (RuleDescriber{}).Describe(context.Background(), DescribeInput{
		Flavor:      FlavorSnapshot{Leaves: map[string]float64{"601": 3, "1001": 2.4}},
		Appearance:  AppearanceSnapshot{Color: "#d0c460", Texture: "cloudy"},
		Mouthfeel:   MouthfeelSnapshot{Dominant: []string{"body", "heat"}},
		FlavorNames: map[uint64]string{601: "菠萝香", 1001: "甘蔗醇香"},
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, expected := range []string{"菠萝香", "甘蔗醇香", "酒体厚实", "烈感明显"} {
		if !strings.Contains(description, expected) {
			t.Fatalf("description %q does not contain %q", description, expected)
		}
	}
}

func TestValidateExtras(t *testing.T) {
	tests := []struct {
		name   string
		extras []ExtraIngredient
	}{
		{name: "more than two", extras: []ExtraIngredient{{TypeId: 6, Quantity: 1}, {TypeId: 7, Quantity: 1}, {TypeId: 8, Quantity: 1}}},
		{name: "missing type", extras: []ExtraIngredient{{Quantity: 1}}},
		{name: "non-positive", extras: []ExtraIngredient{{TypeId: 6, Quantity: 0}}},
		{name: "too precise", extras: []ExtraIngredient{{TypeId: 6, Quantity: 1.001}}},
		{name: "duplicate", extras: []ExtraIngredient{{TypeId: 6, Quantity: 1}, {TypeId: 6, Quantity: 2}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := validateExtras(test.extras); err == nil {
				t.Fatalf("validateExtras(%+v) succeeded", test.extras)
			}
		})
	}
	if err := validateExtras([]ExtraIngredient{{TypeId: 6, Quantity: 1.25}, {TypeId: 7, Quantity: 2}}); err != nil {
		t.Fatalf("valid extras failed: %v", err)
	}
}

func TestIngredientCatalogAndExtraDrink(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	service := NewService(tx, nil)

	catalog, err := service.Ingredients(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(catalog) != 20 {
		t.Fatalf("catalog length=%d, want 20", len(catalog))
	}
	var salt IngredientCatalogItem
	for _, item := range catalog {
		if item.TypeId == 15 {
			salt = item
		}
	}
	if !salt.ExtraEnabled || salt.SuggestedQty != 1 || salt.MaxQtyPerDrink != 2 || salt.Unit != "g" {
		t.Fatalf("unexpected salt catalog item: %+v", salt)
	}

	var before float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = 15 AND status = 0").Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	result, err := service.MakeDrink(context.Background(), OrderRequest{
		RecipeId: 2, OrderedBy: 8848, Extras: []ExtraIngredient{{TypeId: 15, Quantity: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var after float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = 15 AND status = 0").Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if before-after != 1 {
		t.Fatalf("salt deducted=%v, want 1", before-after)
	}
	last := result.Drink.InputsSnapshot[len(result.Drink.InputsSnapshot)-2]
	if last.Role != "extra" || last.TypeId != 15 || last.Qty != 1 {
		t.Fatalf("extra snapshot missing: %+v", result.Drink.InputsSnapshot)
	}
	if result.Steps[len(result.Steps)-1].Action != "加料" || result.Trace[len(result.Trace)-1].Role != "extra" {
		t.Fatalf("extra performance/trace missing: steps=%+v trace=%+v", result.Steps, result.Trace)
	}
	if result.DescribeInput.Ingredients[len(result.DescribeInput.Ingredients)-1].Role != "extra" {
		t.Fatalf("LLM input does not identify the extra: %+v", result.DescribeInput.Ingredients)
	}
}

func TestExtraIngredientPolicyAndRollback(t *testing.T) {
	tests := []struct {
		name  string
		extra ExtraIngredient
	}{
		{name: "base spirit is disabled", extra: ExtraIngredient{TypeId: 1, Quantity: 5}},
		{name: "quantity over limit", extra: ExtraIngredient{TypeId: 15, Quantity: 3}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			tx := model.BarDatabase().Begin()
			if tx.Error != nil {
				t.Fatal(tx.Error)
			}
			defer tx.Rollback()
			var before float64
			if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = 1 AND status = 0").Scan(&before).Error; err != nil {
				t.Fatal(err)
			}
			_, err := NewService(tx, nil).MakeDrink(context.Background(), OrderRequest{RecipeId: 2, OrderedBy: 8848, Extras: []ExtraIngredient{test.extra}})
			if err == nil {
				t.Fatal("order unexpectedly succeeded")
			}
			var after float64
			if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = 1 AND status = 0").Scan(&after).Error; err != nil {
				t.Fatal(err)
			}
			if after != before {
				t.Fatalf("base stock changed on invalid extra: before=%v after=%v", before, after)
			}
		})
	}
}

func TestMissingExtraIngredientRollsBackRecipeStock(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Model(&model.BarIngredientInstance{}).Where("type_id = 14").Updates(map[string]interface{}{"qty_remain": 0, "status": 1}).Error; err != nil {
		t.Fatal(err)
	}
	var before float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = 1 AND status = 0").Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	_, err := NewService(tx, nil).MakeDrink(context.Background(), OrderRequest{
		RecipeId: 2, OrderedBy: 8848, Extras: []ExtraIngredient{{TypeId: 14, Quantity: 1}},
	})
	var missing *MissingError
	if !errors.As(err, &missing) || len(missing.Details) != 1 || missing.Details[0].TypeId != 14 {
		t.Fatalf("expected missing mint detail, got %v", err)
	}
	var after float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = 1 AND status = 0").Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after != before {
		t.Fatalf("recipe stock changed when extra was missing: before=%v after=%v", before, after)
	}
}

func TestExtraOfRecipeTypeUsesRemainingStock(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Model(&model.BarIngredientInstance{}).Where("type_id = 6").Updates(map[string]interface{}{"qty_remain": 0, "status": 1}).Error; err != nil {
		t.Fatal(err)
	}
	var batch model.BarIngredientInstance
	if err := tx.Unscoped().Where("type_id = 6").First(&batch).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&batch).Updates(map[string]interface{}{"qty_remain": 20, "status": 0}).Error; err != nil {
		t.Fatal(err)
	}
	_, err := NewService(tx, nil).MakeDrink(context.Background(), OrderRequest{
		RecipeId: 2, OrderedBy: 8848, Extras: []ExtraIngredient{{TypeId: 6, Quantity: 10}},
	})
	var missing *MissingError
	if !errors.As(err, &missing) || len(missing.Details) != 1 || missing.Details[0].TypeId != 6 || missing.Details[0].Shortage != 5 {
		t.Fatalf("expected five units of blue syrup missing, got %v", err)
	}
	var after float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = 6 AND status = 0").Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if after != 20 {
		t.Fatalf("same-type missing order did not roll back: qty=%v", after)
	}
}

type blockingDescriber struct {
	started chan struct{}
	release chan struct{}
}

func (describer *blockingDescriber) Describe(ctx context.Context, _ DescribeInput) (string, error) {
	close(describer.started)
	select {
	case <-describer.release:
		return "异步生成的岛民娘上酒文案。", nil
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func TestMakeDrinkAsyncReturnsBeforeDescription(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()

	describer := &blockingDescriber{started: make(chan struct{}), release: make(chan struct{})}
	result, err := NewService(tx, describer).MakeDrinkAsync(context.Background(), OrderRequest{RecipeId: 2, OrderedBy: 8848})
	if err != nil {
		t.Fatal(err)
	}
	if !result.DescriptionPending || result.Drink.Description == "异步生成的岛民娘上酒文案。" {
		t.Fatalf("HTTP result did not return the pending rule description: %+v", result)
	}
	select {
	case <-describer.started:
	case <-time.After(time.Second):
		t.Fatal("background describer did not start")
	}
	close(describer.release)
	// The service writes on the same transaction used by this integration test.
	// Give that write ownership of the connection before polling it; database/sql
	// transactions do not support concurrent use of one driver connection.
	time.Sleep(100 * time.Millisecond)

	deadline := time.Now().Add(time.Second)
	for {
		var description string
		if err := tx.Model(&model.BarDrink{}).Select("description").Where("id = ?", result.Drink.Id).Scan(&description).Error; err != nil {
			t.Fatal(err)
		}
		if description == "异步生成的岛民娘上酒文案。" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("asynchronous description was not persisted, got %q", description)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestMissingIngredientRollsBackAllDeductions(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()

	var rumBefore float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("qty_remain").Where("id = 1").Scan(&rumBefore).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&model.BarIngredientInstance{}).Where("type_id = 6").Updates(map[string]interface{}{"qty_remain": 0, "status": 1}).Error; err != nil {
		t.Fatal(err)
	}

	service := NewService(tx, nil)
	_, err := service.MakeDrink(context.Background(), OrderRequest{RecipeId: 2, OrderedBy: 8848})
	var missing *MissingError
	if !errors.As(err, &missing) {
		t.Fatalf("expected MissingError, got %v", err)
	}
	var rumAfter float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("qty_remain").Where("id = 1").Scan(&rumAfter).Error; err != nil {
		t.Fatal(err)
	}
	if rumAfter != rumBefore {
		t.Fatalf("rum was deducted on failed order: before=%v after=%v", rumBefore, rumAfter)
	}
}

func TestMakeDrinkAgainstLocalDatabase(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()

	service := NewService(tx, nil)
	result, err := service.MakeDrink(context.Background(), OrderRequest{RecipeId: 2, OrderedBy: 8848, Message: "integration test"})
	if err != nil {
		t.Fatal(err)
	}
	if result.Drink.Id == 0 || result.RecipeName != "海浪之歌" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(result.Trace) != 4 || len(result.Steps) != 4 {
		t.Fatalf("expected four ingredients, trace=%d steps=%d", len(result.Trace), len(result.Steps))
	}
	pineappleFlavor := result.Flavor.Leaves["601"]
	if pineappleFlavor <= 0 || pineappleFlavor > 3 {
		t.Fatalf("pineapple flavor = %v, want a decayed value in (0,3]", pineappleFlavor)
	}
	if len(result.Drink.FlavorSnapshot.Leaves) == 0 || len(result.Drink.FlavorSnapshot.Rolled) == 0 {
		t.Fatalf("public flavor arrays are empty: %+v", result.Drink.FlavorSnapshot)
	}
	foundPineapple := false
	for _, flavor := range result.Drink.FlavorSnapshot.Leaves {
		if flavor.Id == 601 && flavor.Name == "菠萝香" && flavor.Value == pineappleFlavor {
			foundPineapple = true
		}
	}
	if !foundPineapple {
		t.Fatalf("named pineapple flavor not found: %+v", result.Drink.FlavorSnapshot.Leaves)
	}
	if result.Appearance.Color != "#d0c460" {
		t.Fatalf("appearance color = %s, want #d0c460", result.Appearance.Color)
	}
	if result.Mouthfeel.Dominant[0] != "body" {
		t.Fatalf("dominant mouthfeel = %v", result.Mouthfeel.Dominant)
	}
	inputs := result.Drink.InputsSnapshot
	if len(inputs) != 5 || inputs[4].Kind != "technique" {
		t.Fatalf("unexpected inputs snapshot: %+v", inputs)
	}

	detail, err := service.Drink(context.Background(), result.Drink.Id)
	if err != nil || detail.Recipe.Name != "海浪之歌" {
		t.Fatalf("drink lookup failed: detail=%+v err=%v", detail, err)
	}
	trace, err := service.Trace(context.Background(), result.Drink.Id)
	if err != nil || len(trace.Inputs) != 4 {
		t.Fatalf("trace failed: trace=%+v err=%v", trace, err)
	}
}

func TestFEFOCanSplitAcrossBatches(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	if err := tx.Model(&model.BarIngredientInstance{}).Where("type_id = 6").Updates(map[string]interface{}{"qty_remain": 0, "status": 1}).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().Unix()
	batches := []model.BarIngredientInstance{
		{Code: "BSYR-TEST-EARLY", TypeId: 6, QtyTotal: 10, QtyRemain: 10, ProducedAt: now, ExpireAt: now + 3600, Source: "restock", SourceId: 6, Status: 0, CreatedAt: now, UpdatedAt: now},
		{Code: "BSYR-TEST-LATE", TypeId: 6, QtyTotal: 10, QtyRemain: 10, ProducedAt: now, ExpireAt: now + 7200, Source: "restock", SourceId: 6, Status: 0, CreatedAt: now, UpdatedAt: now},
	}
	if err := tx.Create(&batches).Error; err != nil {
		t.Fatal(err)
	}
	result, err := NewService(tx, nil).MakeDrink(context.Background(), OrderRequest{RecipeId: 2, OrderedBy: 8848})
	if err != nil {
		t.Fatal(err)
	}
	inputs := result.Drink.InputsSnapshot
	for _, input := range inputs {
		if input.TypeId != 6 {
			continue
		}
		if len(input.Portions) != 2 {
			t.Fatalf("expected two syrup batches, got %+v", input.Portions)
		}
		if input.Portions[0].Code != "BSYR-TEST-EARLY" || input.Portions[0].Qty != 10 || input.Portions[1].Qty != 5 {
			t.Fatalf("FEFO allocation is wrong: %+v", input.Portions)
		}
		return
	}
	t.Fatal("syrup input not found")
}

func TestRestockUsesDefaults(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	instance, err := NewService(tx, nil).Restock(context.Background(), RestockRequest{TypeId: 11, Quantity: 25, Note: "test restock"})
	if err != nil {
		t.Fatal(err)
	}
	if instance.Id == 0 || instance.QtyRemain != 25 || !strings.Contains(string(instance.Attrs), "freshness") {
		t.Fatalf("unexpected restock instance: %+v", instance)
	}
}
