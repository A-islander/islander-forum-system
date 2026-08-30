package bar

import (
	"context"
	"testing"

	"github.com/forum_server/model"
)

func TestMaintainStockRestocksWithNewBatchAndIsRepeatable(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()

	service := NewService(tx, nil)
	now := service.now().Unix()
	if err := tx.Model(&model.BarStockPolicy{}).Where("type_id <> ?", 6).Update("enabled", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&model.BarIngredientInstance{}).Where("type_id = ?", 6).Updates(map[string]interface{}{"status": 1, "qty_remain": 0}).Error; err != nil {
		t.Fatal(err)
	}
	old := model.BarIngredientInstance{Code: "BSYR-MAINT-OLD", TypeId: 6, QtyTotal: 10, QtyRemain: 10, ProducedAt: now, ExpireAt: now + 86400, Source: "restock", SourceId: 0, Status: 0, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&old).Error; err != nil {
		t.Fatal(err)
	}

	report, err := service.MaintainStock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Restocks != 1 {
		t.Fatalf("restocks = %d, want 1", report.Restocks)
	}
	var unchanged model.BarIngredientInstance
	if err := tx.Where("id = ?", old.Id).Take(&unchanged).Error; err != nil {
		t.Fatal(err)
	}
	if unchanged.QtyRemain != 10 {
		t.Fatalf("old batch was refilled in place: %+v", unchanged)
	}
	quantity, err := service.availableQuantity(context.Background(), 6, now)
	if err != nil {
		t.Fatal(err)
	}
	if quantity != 500 {
		t.Fatalf("available quantity = %.2f, want 500", quantity)
	}

	second, err := service.MaintainStock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if second.Restocks != 0 || second.Processes != 0 {
		t.Fatalf("repeat maintenance changed stock: %+v", second)
	}
}

func TestMaintainStockRetiresExpiredBatch(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	service := NewService(tx, nil)
	now := service.now().Unix()
	if err := tx.Model(&model.BarStockPolicy{}).Where("1 = 1").Update("enabled", 0).Error; err != nil {
		t.Fatal(err)
	}
	batch := model.BarIngredientInstance{Code: "SALT-MAINT-EXPIRED", TypeId: 15, QtyTotal: 20, QtyRemain: 20, ProducedAt: now - 100, ExpireAt: now, Source: "restock", SourceId: 0, Status: 0, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	report, err := service.MaintainStock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Expired < 1 {
		t.Fatalf("expired = %d, want at least 1", report.Expired)
	}
	if err := tx.Where("id = ?", batch.Id).Take(&batch).Error; err != nil {
		t.Fatal(err)
	}
	if batch.Status != 2 || batch.QtyRemain != 20 {
		t.Fatalf("expired batch not soft-retired: %+v", batch)
	}
}

func TestMaintainStockBuildsIntermediateFromBaseStock(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	service := NewService(tx, nil)
	if err := tx.Model(&model.BarStockPolicy{}).Where("type_id <> ?", 17).Update("enabled", 0).Error; err != nil {
		t.Fatal(err)
	}
	if err := tx.Model(&model.BarIngredientInstance{}).Where("type_id = ?", 17).Updates(map[string]interface{}{"status": 1, "qty_remain": 0}).Error; err != nil {
		t.Fatal(err)
	}
	report, err := service.MaintainStock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Processes != 2 || report.Restocks != 0 {
		t.Fatalf("unexpected maintenance report: %+v", report)
	}
	var output model.BarIngredientInstance
	if err := tx.Where("type_id = ? AND status = 0", 17).Order("id DESC").Take(&output).Error; err != nil {
		t.Fatal(err)
	}
	var quantity float64
	if err := tx.Model(&model.BarIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").Where("type_id = ? AND status = 0", 17).Scan(&quantity).Error; err != nil {
		t.Fatal(err)
	}
	if output.Source != "process" || quantity != 240 {
		t.Fatalf("intermediate was not produced through process: %+v", output)
	}
}

func TestMaintainStockRetiresLowFreshnessWithoutErasingQuantity(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	service := NewService(tx, nil)
	now := service.now().Unix()
	if err := tx.Model(&model.BarStockPolicy{}).Where("type_id <> ?", 14).Update("enabled", 0).Error; err != nil {
		t.Fatal(err)
	}
	batch := model.BarIngredientInstance{Code: "MINT-MAINT-STALE", TypeId: 14, QtyTotal: 5, QtyRemain: 5, ProducedAt: now - 5*86400, ExpireAt: now + 86400, Attrs: []byte(`{"freshness":100}`), Source: "restock", SourceId: 0, Status: 0, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&batch).Error; err != nil {
		t.Fatal(err)
	}
	report, err := service.MaintainStock(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Retired < 1 {
		t.Fatalf("retired = %d, want at least 1", report.Retired)
	}
	if err := tx.Where("id = ?", batch.Id).Take(&batch).Error; err != nil {
		t.Fatal(err)
	}
	if batch.Status != 2 || batch.QtyRemain != 5 {
		t.Fatalf("stale batch not soft-retired: %+v", batch)
	}
}
