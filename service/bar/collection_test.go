package bar

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/forum_server/model"
)

func TestCollectTwiceAddsConfiguredLootToBackpack(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	service := NewService(tx, nil)
	service.now = func() time.Time { return time.Date(2026, 8, 31, 10, 0, 0, 0, time.FixedZone("HKT", 8*3600)) }
	service.randomIntn = func(int) (int, error) { return 0, nil }

	for attempt := 1; attempt <= 2; attempt++ {
		result, err := service.Collect(context.Background(), 70001)
		if err != nil {
			t.Fatal(err)
		}
		if result.Location.Name != "东边悬崖" || result.Item.Name != "柠檬" || result.Item.Quantity != 50 {
			t.Fatalf("unexpected deterministic loot: %+v", result)
		}
		if result.Item.Attrs["freshness"] != float64(90) || result.UsedToday != attempt || result.RemainingToday != 2-attempt {
			t.Fatalf("unexpected attrs/status: %+v", result)
		}
	}
	_, err := service.Collect(context.Background(), 70001)
	var limit *DailyCollectLimitError
	if !errors.As(err, &limit) || limit.Status.RemainingToday != 0 {
		t.Fatalf("third collect should hit daily limit, got %v", err)
	}
	items, err := service.Backpack(context.Background(), 70001)
	if err != nil || len(items) != 1 || items[0].TypeId != 11 || items[0].Quantity != 100 || items[0].BatchCount != 2 {
		t.Fatalf("collected backpack mismatch: items=%+v err=%v", items, err)
	}
}

func TestConcurrentCollectNeverExceedsDailyLimit(t *testing.T) {
	db := model.BarDatabase()
	userId := uint64(70002)
	dayKey, _ := hongKongDay(time.Now())
	defer db.Where("user_id=?", userId).Delete(&model.BarUserIngredientInstance{})
	defer db.Where("user_id=?", userId).Delete(&model.BarCollectLog{})
	defer db.Where("user_id=? AND day_key=?", userId, dayKey).Delete(&model.BarCollectDaily{})
	service := NewService(db, nil)
	service.randomIntn = func(int) (int, error) { return 0, nil }

	var wait sync.WaitGroup
	var lock sync.Mutex
	succeeded, limited, failed := 0, 0, 0
	for index := 0; index < 6; index++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			_, err := service.Collect(context.Background(), userId)
			lock.Lock()
			defer lock.Unlock()
			var limit *DailyCollectLimitError
			switch {
			case err == nil:
				succeeded++
			case errors.As(err, &limit):
				limited++
			default:
				failed++
			}
		}()
	}
	wait.Wait()
	if succeeded != 2 || limited != 4 || failed != 0 {
		t.Fatalf("concurrent collects: success=%d limited=%d failed=%d", succeeded, limited, failed)
	}
}

func TestBackpackExtraMakesNamedDrinkAndDeductsOwnerOnly(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	now := time.Now()
	instance := model.BarUserIngredientInstance{UserId: 70003, TypeId: 15, QtyTotal: 2, QtyRemain: 2,
		ProducedAt: now.Unix(), ExpireAt: now.Add(24 * time.Hour).Unix(), Source: "reward", Status: 0,
		CreatedAt: now.Unix(), UpdatedAt: now.Unix()}
	if err := tx.Create(&instance).Error; err != nil {
		t.Fatal(err)
	}
	var publicBefore float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id=15 AND status=0").Scan(&publicBefore).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(tx, nil)
	result, err := service.MakeDrink(context.Background(), OrderRequest{RecipeId: 2, OrderedBy: 70003, OrderedByName: "R5RCGeZ",
		Extras: []ExtraIngredient{{TypeId: 15, Quantity: 1, Source: "backpack"}}})
	if err != nil {
		t.Fatal(err)
	}
	if result.Drink.DrinkName != "海浪之歌-海盐-R5RCGeZ" {
		t.Fatalf("drink_name=%q", result.Drink.DrinkName)
	}
	if result.Trace[len(result.Trace)-1].Inventory != "backpack" || result.Drink.InputsSnapshot[len(result.Drink.InputsSnapshot)-2].Portions[0].Inventory != "backpack" {
		t.Fatalf("backpack origin missing: trace=%+v inputs=%+v", result.Trace, result.Drink.InputsSnapshot)
	}
	var userAfter, publicAfter float64
	_ = tx.Model(&model.BarUserIngredientInstance{}).Select("qty_remain").Where("id=?", instance.Id).Scan(&userAfter).Error
	_ = tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id=15 AND status=0").Scan(&publicAfter).Error
	if userAfter != 1 || publicAfter != publicBefore {
		t.Fatalf("wrong inventory deducted: user=%v public before=%v after=%v", userAfter, publicBefore, publicAfter)
	}
	plain, err := service.MakeDrink(context.Background(), OrderRequest{RecipeId: 2, OrderedBy: 70003, OrderedByName: "R5RCGeZ"})
	if err != nil || plain.Drink.DrinkName != "海浪之歌" {
		t.Fatalf("default drink name changed: name=%q err=%v", plain.Drink.DrinkName, err)
	}
}

func TestSubmitBackpackMovesQuantityIntoPublicStock(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	now := time.Now()
	attrs := []byte(`{"freshness":95,"aroma":88}`)
	instance := model.BarUserIngredientInstance{UserId: 70004, TypeId: 13, QtyTotal: 15, QtyRemain: 15,
		ProducedAt: now.Unix(), ExpireAt: now.Add(4 * 24 * time.Hour).Unix(), Attrs: attrs, Source: "collect", SourceId: 42,
		Status: 0, CreatedAt: now.Unix(), UpdatedAt: now.Unix()}
	if err := tx.Create(&instance).Error; err != nil {
		t.Fatal(err)
	}
	result, err := NewService(tx, nil).SubmitBackpack(context.Background(), 70004, SubmitBackpackRequest{TypeId: 13, Quantity: 10})
	if err != nil {
		t.Fatal(err)
	}
	if result.BackpackQtyRemaining != 5 || result.Quantity != 10 || result.PublicInstanceId == 0 {
		t.Fatalf("unexpected submit result: %+v", result)
	}
	var public model.BarIngredientInstance
	if err := tx.Where("id=?", result.PublicInstanceId).Take(&public).Error; err != nil {
		t.Fatal(err)
	}
	var log model.BarRestockLog
	if err := tx.Where("id=?", public.SourceId).Take(&log).Error; err != nil {
		t.Fatal(err)
	}
	if public.QtyRemain != 10 || log.SourceUid != 70004 || log.SourceType != 1 {
		t.Fatalf("submitted stock is not traceable: public=%+v log=%+v", public, log)
	}
}
