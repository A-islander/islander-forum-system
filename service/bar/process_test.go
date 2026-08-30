package bar

import (
	"context"
	"testing"
	"time"

	"github.com/forum_server/model"
)

func TestProcessConsumesInputsAndCreatesTraceableBatch(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()

	service := NewService(tx, nil)
	now := service.now().Unix()
	if err := tx.Model(&model.BarIngredientInstance{}).Where("type_id = ?", 17).Updates(map[string]interface{}{"status": 1, "qty_remain": 0}).Error; err != nil {
		t.Fatal(err)
	}
	var before float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = ? AND status = 0 AND expire_at > ?", 11, now).Scan(&before).Error; err != nil {
		t.Fatal(err)
	}
	output, err := service.Process(context.Background(), 1, 0)
	if err != nil {
		t.Fatal(err)
	}
	if output.TypeId != 17 || output.QtyRemain != 120 || output.Source != "process" || output.SourceId == 0 {
		t.Fatalf("unexpected process output: %+v", output)
	}
	var after float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = ? AND status = 0 AND expire_at > ?", 11, now).Scan(&after).Error; err != nil {
		t.Fatal(err)
	}
	if round(before-after, 2) != 300 {
		t.Fatalf("lemon consumption = %.2f, want 300", before-after)
	}
	var processLog model.BarProcessLog
	if err := tx.Where("id = ?", output.SourceId).Take(&processLog).Error; err != nil {
		t.Fatal(err)
	}
	if processLog.OutputInstanceId != output.Id || len(processLog.InputsSnapshot) == 0 {
		t.Fatalf("unexpected process log: %+v", processLog)
	}
}

func TestEffectiveFreshnessUsesElapsedTime(t *testing.T) {
	instance := model.BarIngredientInstance{ProducedAt: 1_000, Attrs: []byte(`{"freshness":90}`)}
	ingredientType := model.BarIngredientType{FreshnessDecayPerDay: 10}
	attrs := effectiveAttributes(instance, ingredientType, 1_000+2*86400)
	if attrs["freshness"] != 70 {
		t.Fatalf("freshness = %v, want 70", attrs["freshness"])
	}
}

func TestDeductStockKeepsPartialBatchAvailable(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	now := time.Now().Unix()
	batch := model.BarIngredientInstance{Code: "RUM-DEDUCT-BOUNDARY", TypeId: 1, QtyTotal: 500, QtyRemain: 500, ProducedAt: now, ExpireAt: now + 86400, Source: "restock", SourceId: 0, Status: 0, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	if err := deductStock(tx, batch.Id, 400, now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Where("id = ?", batch.Id).Take(&batch).Error; err != nil {
		t.Fatal(err)
	}
	if batch.QtyRemain != 100 || batch.Status != 0 {
		t.Fatalf("partial batch incorrectly exhausted: %+v", batch)
	}
	if err := deductStock(tx, batch.Id, 100, now); err != nil {
		t.Fatal(err)
	}
	if err := tx.Where("id = ?", batch.Id).Take(&batch).Error; err != nil {
		t.Fatal(err)
	}
	if batch.QtyRemain != 0 || batch.Status != 1 {
		t.Fatalf("empty batch not exhausted: %+v", batch)
	}
}
