package bar

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"time"

	"github.com/forum_server/model"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func defaultRandomIntn(max int) (int, error) {
	if max <= 0 {
		return 0, errors.New("random upper bound must be positive")
	}
	value, err := cryptorand.Int(cryptorand.Reader, big.NewInt(int64(max)))
	if err != nil {
		return 0, err
	}
	return int(value.Int64()), nil
}

func collectDailyLimit() int {
	if value, err := strconv.Atoi(os.Getenv("BAR_COLLECT_DAILY_LIMIT")); err == nil && value > 0 && value <= 20 {
		return value
	}
	return 2
}

func hongKongDay(now time.Time) (uint, int64) {
	location, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		location = time.FixedZone("Asia/Shanghai", 8*60*60)
	}
	local := now.In(location)
	key, _ := strconv.Atoi(local.Format("20060102"))
	next := time.Date(local.Year(), local.Month(), local.Day()+1, 0, 0, 0, 0, location)
	return uint(key), next.Unix()
}

func (s *Service) CollectStatus(ctx context.Context, userId uint64) (CollectStatus, error) {
	if userId == 0 {
		return CollectStatus{}, errors.New("user_id is required")
	}
	dayKey, resetsAt := hongKongDay(s.now())
	var daily model.BarCollectDaily
	err := s.db.WithContext(ctx).Where("user_id = ? AND day_key = ?", userId, dayKey).Take(&daily).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return CollectStatus{}, err
	}
	limit := collectDailyLimit()
	used := int(daily.UsedCount)
	remaining := limit - used
	if remaining < 0 {
		remaining = 0
	}
	return CollectStatus{DailyLimit: limit, UsedToday: used, RemainingToday: remaining, ResetsAt: resetsAt}, nil
}

func weightedIndex(weights []uint, randomIntn func(int) (int, error)) (int, error) {
	total := 0
	for _, weight := range weights {
		total += int(weight)
	}
	if total <= 0 {
		return 0, errors.New("loot weights are not configured")
	}
	value, err := randomIntn(total)
	if err != nil {
		return 0, err
	}
	for index, weight := range weights {
		value -= int(weight)
		if value < 0 {
			return index, nil
		}
	}
	return len(weights) - 1, nil
}

func (s *Service) randomRange(minimum, maximum float64) (float64, error) {
	if minimum < 0 || maximum < minimum {
		return 0, errors.New("invalid collection range")
	}
	if maximum == minimum {
		return minimum, nil
	}
	value, err := s.randomIntn(10001)
	if err != nil {
		return 0, err
	}
	return round(minimum+(maximum-minimum)*float64(value)/10000, 2), nil
}

func (s *Service) Collect(ctx context.Context, userId uint64) (CollectResult, error) {
	if userId == 0 {
		return CollectResult{}, errors.New("user_id is required")
	}
	var result CollectResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nowTime := s.now()
		now := nowTime.Unix()
		dayKey, resetsAt := hongKongDay(nowTime)
		if err := tx.Exec(`INSERT INTO bar_collect_daily (user_id,day_key,used_count,created_at,updated_at)
			VALUES (?,?,0,?,?) ON DUPLICATE KEY UPDATE updated_at=updated_at`, userId, dayKey, now, now).Error; err != nil {
			return err
		}
		var daily model.BarCollectDaily
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("user_id = ? AND day_key = ?", userId, dayKey).Take(&daily).Error; err != nil {
			return err
		}
		limit := collectDailyLimit()
		if int(daily.UsedCount) >= limit {
			return &DailyCollectLimitError{Status: CollectStatus{DailyLimit: limit, UsedToday: int(daily.UsedCount), RemainingToday: 0, ResetsAt: resetsAt}}
		}
		var locations []model.BarCollectLocation
		if err := tx.Where("status = 0 AND weight > 0").Order("id ASC").Find(&locations).Error; err != nil {
			return err
		}
		if len(locations) == 0 {
			return errors.New("no collection locations are configured")
		}
		locationWeights := make([]uint, len(locations))
		for index := range locations {
			locationWeights[index] = locations[index].Weight
		}
		locationIndex, err := weightedIndex(locationWeights, s.randomIntn)
		if err != nil {
			return err
		}
		location := locations[locationIndex]
		var loot []model.BarCollectLoot
		if err := tx.Where("location_id = ? AND status = 0 AND weight > 0", location.Id).Order("id ASC").Find(&loot).Error; err != nil {
			return err
		}
		if len(loot) == 0 {
			return errors.New("selected collection location has no loot")
		}
		lootWeights := make([]uint, len(loot))
		for index := range loot {
			lootWeights[index] = loot[index].Weight
		}
		lootIndex, err := weightedIndex(lootWeights, s.randomIntn)
		if err != nil {
			return err
		}
		selected := loot[lootIndex]
		var ingredientType model.BarIngredientType
		if err := tx.Where("id = ? AND status = 0", selected.TypeId).Take(&ingredientType).Error; err != nil {
			return err
		}
		quantity, err := s.randomRange(selected.MinQty, selected.MaxQty)
		if err != nil {
			return err
		}
		attrs := make(map[string]interface{})
		if len(ingredientType.DefaultAttrs) > 0 && string(ingredientType.DefaultAttrs) != "null" {
			_ = json.Unmarshal(ingredientType.DefaultAttrs, &attrs)
		}
		var rules map[string][]float64
		if len(selected.AttrsRule) > 0 && string(selected.AttrsRule) != "null" {
			if err := json.Unmarshal(selected.AttrsRule, &rules); err != nil {
				return fmt.Errorf("invalid attrs_rule for loot %d: %w", selected.Id, err)
			}
		}
		for key, bounds := range rules {
			if len(bounds) != 2 {
				return fmt.Errorf("invalid %s range for loot %d", key, selected.Id)
			}
			value, err := s.randomRange(bounds[0], bounds[1])
			if err != nil {
				return err
			}
			attrs[key] = value
		}
		attrsJSON, err := rawJSON(attrs)
		if err != nil {
			return err
		}
		shelfLife := ingredientType.ShelfLifeDays
		if selected.ShelfLifeDays != nil {
			shelfLife = *selected.ShelfLifeDays
		}
		expireAt := now + int64(shelfLife)*86400
		sequence := daily.UsedCount + 1
		log := model.BarCollectLog{UserId: userId, DayKey: dayKey, DailySeq: sequence, LocationId: location.Id,
			TypeId: selected.TypeId, Quantity: quantity, Attrs: attrsJSON, CreatedAt: now}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		instance := model.BarUserIngredientInstance{UserId: userId, TypeId: selected.TypeId, QtyTotal: quantity,
			QtyRemain: quantity, ProducedAt: now, ExpireAt: expireAt, Attrs: attrsJSON, Source: "collect",
			SourceId: log.Id, Status: 0, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&instance).Error; err != nil {
			return err
		}
		if err := tx.Model(&log).Update("instance_id", instance.Id).Error; err != nil {
			return err
		}
		if err := tx.Model(&daily).Updates(map[string]interface{}{"used_count": sequence, "updated_at": now}).Error; err != nil {
			return err
		}
		result = CollectResult{
			Location: CollectLocationView{Id: location.Id, Code: location.Code, Name: location.Name, Description: location.Description},
			Item: CollectedItemView{InstanceId: instance.Id, TypeId: ingredientType.Id, Code: ingredientType.Code,
				Name: ingredientType.Name, Quantity: quantity, Unit: ingredientType.Unit, Attrs: attrs, ExpireAt: expireAt},
			CollectStatus: CollectStatus{DailyLimit: limit, UsedToday: int(sequence), RemainingToday: limit - int(sequence), ResetsAt: resetsAt},
		}
		return nil
	})
	return result, err
}
