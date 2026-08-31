package bar

import (
	"context"
	"errors"
	"fmt"
	"math"

	"github.com/forum_server/model"
	"gorm.io/gorm"
)

func (s *Service) SubmitBackpack(ctx context.Context, userId uint64, request SubmitBackpackRequest) (SubmitBackpackResult, error) {
	if userId == 0 || request.TypeId == 0 || request.Quantity <= 0 || math.IsNaN(request.Quantity) || math.IsInf(request.Quantity, 0) {
		return SubmitBackpackResult{}, errors.New("type_id and positive quantity are required")
	}
	if math.Abs(round(request.Quantity, 2)-request.Quantity) > .000001 {
		return SubmitBackpackResult{}, errors.New("quantity supports at most two decimal places")
	}
	var result SubmitBackpackResult
	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		nowTime := s.now()
		now := nowTime.Unix()
		item := model.BarRecipeItem{TypeId: request.TypeId, Qty: request.Quantity}
		input, missing, err := s.allocateUserItem(tx, item, userId, now)
		if err != nil {
			return err
		}
		if missing != nil {
			return &MissingError{Details: []MissingDetail{*missing}}
		}
		if err := deductInput(tx, input, now); err != nil {
			return err
		}
		weighted := make(map[string]float64)
		for _, portion := range input.portions {
			for _, candidate := range input.userInstances {
				if candidate.Id != portion.InstanceId {
					continue
				}
				for key, value := range effectiveAttributes(userAsBarInstance(candidate), input.typeInfo, now) {
					weighted[key] += value * portion.Qty / request.Quantity
				}
			}
		}
		for key := range weighted {
			weighted[key] = round(weighted[key], 2)
		}
		attrsJSON, err := rawJSON(weighted)
		if err != nil {
			return err
		}
		expireAt := now + int64(input.typeInfo.ShelfLifeDays)*86400
		for _, candidate := range input.userInstances {
			if candidate.ExpireAt < expireAt {
				expireAt = candidate.ExpireAt
			}
		}
		log := model.BarRestockLog{TypeId: request.TypeId, Quantity: request.Quantity, SourceType: 1,
			SourceUid: userId, Attrs: attrsJSON, ExpireAt: expireAt, Note: "岛民上交物料", CreatedAt: now}
		if err := tx.Create(&log).Error; err != nil {
			return err
		}
		instance := model.BarIngredientInstance{Code: fmt.Sprintf("%s-USER-%06d", input.typeInfo.Code, log.Id),
			TypeId: request.TypeId, QtyTotal: request.Quantity, QtyRemain: request.Quantity, ProducedAt: now,
			ExpireAt: expireAt, Attrs: attrsJSON, Source: "restock", SourceId: log.Id, Status: 0, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&instance).Error; err != nil {
			return err
		}
		if err := tx.Model(&log).Update("instance_id", instance.Id).Error; err != nil {
			return err
		}
		var remaining float64
		if err := tx.Model(&model.BarUserIngredientInstance{}).Select("COALESCE(SUM(qty_remain),0)").
			Where("user_id=? AND type_id=? AND status=0 AND qty_remain>0 AND expire_at>?", userId, request.TypeId, now).Scan(&remaining).Error; err != nil {
			return err
		}
		result = SubmitBackpackResult{TypeId: request.TypeId, Name: input.typeInfo.Name, Quantity: request.Quantity,
			Unit: input.typeInfo.Unit, PublicInstanceId: instance.Id, BackpackQtyRemaining: round(remaining, 2)}
		return nil
	})
	return result, err
}
