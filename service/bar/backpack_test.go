package bar

import (
	"context"
	"testing"
	"time"

	"github.com/forum_server/model"
)

func TestBackpackGroupsUsableBatchesForAuthenticatedUser(t *testing.T) {
	tx := model.BarDatabase().Begin()
	if tx.Error != nil {
		t.Fatal(tx.Error)
	}
	defer tx.Rollback()
	now := time.Unix(1800000000, 0)
	rows := []model.BarUserIngredientInstance{
		{UserId: 8848, TypeId: 14, QtyTotal: 2, QtyRemain: 2, ProducedAt: now.Unix(), ExpireAt: now.Add(24 * time.Hour).Unix(), Source: "collect", Status: 0, CreatedAt: now.Unix(), UpdatedAt: now.Unix()},
		{UserId: 8848, TypeId: 14, QtyTotal: 4, QtyRemain: 3, ProducedAt: now.Unix(), ExpireAt: now.Add(48 * time.Hour).Unix(), Source: "gift", SourceId: 9, Status: 0, CreatedAt: now.Unix(), UpdatedAt: now.Unix()},
		{UserId: 8848, TypeId: 14, QtyTotal: 10, QtyRemain: 10, ProducedAt: now.Add(-48 * time.Hour).Unix(), ExpireAt: now.Add(-time.Hour).Unix(), Source: "collect", Status: 0, CreatedAt: now.Unix(), UpdatedAt: now.Unix()},
		{UserId: 8848, TypeId: 15, QtyTotal: 2, QtyRemain: 2, ProducedAt: now.Unix(), ExpireAt: now.Add(24 * time.Hour).Unix(), Source: "reward", Status: 1, CreatedAt: now.Unix(), UpdatedAt: now.Unix()},
		{UserId: 9900, TypeId: 14, QtyTotal: 8, QtyRemain: 8, ProducedAt: now.Unix(), ExpireAt: now.Add(24 * time.Hour).Unix(), Source: "collect", Status: 0, CreatedAt: now.Unix(), UpdatedAt: now.Unix()},
	}
	if err := tx.Create(&rows).Error; err != nil {
		t.Fatal(err)
	}
	service := NewService(tx, nil)
	service.now = func() time.Time { return now }
	items, err := service.Backpack(context.Background(), 8848)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 {
		t.Fatalf("items=%+v, want one usable type", items)
	}
	item := items[0]
	if item.TypeId != 14 || item.Name != "薄荷" || item.Quantity != 5 || item.BatchCount != 2 {
		t.Fatalf("unexpected grouped backpack item: %+v", item)
	}
	if item.EarliestExpireAt != now.Add(24*time.Hour).Unix() {
		t.Fatalf("earliest_expire_at=%d", item.EarliestExpireAt)
	}
}

func TestBackpackRejectsMissingUser(t *testing.T) {
	if _, err := NewService(model.BarDatabase(), nil).Backpack(context.Background(), 0); err == nil {
		t.Fatal("Backpack accepted user_id=0")
	}
}
