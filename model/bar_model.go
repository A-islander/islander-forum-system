package model

import (
	"encoding/json"

	"gorm.io/gorm"
)

type BarFlavorNode struct {
	Id                 uint64  `gorm:"column:id;primaryKey" json:"id"`
	ParentId           *uint64 `gorm:"column:parent_id" json:"parent_id"`
	Name               string  `gorm:"column:name" json:"name"`
	Level              uint8   `gorm:"column:level" json:"level"`
	SensitivityDefault float64 `gorm:"column:sensitivity_default" json:"sensitivity_default"`
	Status             uint8   `gorm:"column:status" json:"status"`
}

func (BarFlavorNode) TableName() string { return "bar_flavor_node" }

type BarIngredientType struct {
	Id                   uint64          `gorm:"column:id;primaryKey" json:"id"`
	Code                 string          `gorm:"column:code" json:"code"`
	Name                 string          `gorm:"column:name" json:"name"`
	Category             string          `gorm:"column:category" json:"category"`
	Mixable              uint8           `gorm:"column:mixable" json:"mixable"`
	Unit                 string          `gorm:"column:unit" json:"unit"`
	DefaultBatchQty      float64         `gorm:"column:default_batch_qty" json:"default_batch_qty"`
	ShelfLifeDays        uint            `gorm:"column:shelf_life_days" json:"shelf_life_days"`
	FreshnessDecayPerDay float64         `gorm:"column:freshness_decay_per_day" json:"freshness_decay_per_day"`
	DefaultAttrs         json.RawMessage `gorm:"column:default_attrs;type:json" json:"default_attrs"`
	Appearance           json.RawMessage `gorm:"column:appearance;type:json" json:"appearance"`
	Mouthfeel            json.RawMessage `gorm:"column:mouthfeel;type:json" json:"mouthfeel"`
	Codex                string          `gorm:"column:codex" json:"codex"`
	Status               uint8           `gorm:"column:status" json:"status"`
	CreatedAt            int64           `gorm:"column:created_at" json:"created_at"`
	UpdatedAt            int64           `gorm:"column:updated_at" json:"updated_at"`
}

func (BarIngredientType) TableName() string { return "bar_ingredient_type" }

type BarIngredientFlavor struct {
	Id            uint64   `gorm:"column:id;primaryKey"`
	TypeId        uint64   `gorm:"column:type_id"`
	NodeId        uint64   `gorm:"column:node_id"`
	BaseIntensity float64  `gorm:"column:base_intensity"`
	Sensitivity   *float64 `gorm:"column:sensitivity"`
}

func (BarIngredientFlavor) TableName() string { return "bar_ingredient_flavor" }

type BarIngredientInstance struct {
	Id         uint64          `gorm:"column:id;primaryKey" json:"id"`
	Code       string          `gorm:"column:code" json:"code"`
	TypeId     uint64          `gorm:"column:type_id" json:"type_id"`
	QtyTotal   float64         `gorm:"column:qty_total" json:"qty_total"`
	QtyRemain  float64         `gorm:"column:qty_remain" json:"qty_remain"`
	ProducedAt int64           `gorm:"column:produced_at" json:"produced_at"`
	ExpireAt   int64           `gorm:"column:expire_at" json:"expire_at"`
	Attrs      json.RawMessage `gorm:"column:attrs;type:json" json:"attrs"`
	Source     string          `gorm:"column:source" json:"source"`
	SourceId   uint64          `gorm:"column:source_id" json:"source_id"`
	Status     uint8           `gorm:"column:status" json:"status"`
	CreatedAt  int64           `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  int64           `gorm:"column:updated_at" json:"updated_at"`
}

func (BarIngredientInstance) TableName() string { return "bar_ingredient_instance" }

type BarRestockLog struct {
	Id         uint64          `gorm:"column:id;primaryKey" json:"id"`
	TypeId     uint64          `gorm:"column:type_id" json:"type_id"`
	Quantity   float64         `gorm:"column:quantity" json:"quantity"`
	InstanceId uint64          `gorm:"column:instance_id" json:"instance_id"`
	SourceType uint8           `gorm:"column:source_type" json:"source_type"`
	SourceUid  uint64          `gorm:"column:source_uid" json:"source_uid"`
	Attrs      json.RawMessage `gorm:"column:attrs;type:json" json:"attrs"`
	ExpireAt   int64           `gorm:"column:expire_at" json:"expire_at"`
	Note       string          `gorm:"column:note" json:"note"`
	CreatedAt  int64           `gorm:"column:created_at" json:"created_at"`
}

func (BarRestockLog) TableName() string { return "bar_restock_log" }

type BarRecipe struct {
	Id         uint64 `gorm:"column:id;primaryKey" json:"id"`
	Name       string `gorm:"column:name" json:"name"`
	Story      string `gorm:"column:story" json:"story"`
	CreatorId  uint64 `gorm:"column:creator_id" json:"creator_id"`
	Technique  string `gorm:"column:technique" json:"technique"`
	Status     uint8  `gorm:"column:status" json:"status"`
	OrderCount uint   `gorm:"column:order_count" json:"order_count"`
	CreatedAt  int64  `gorm:"column:created_at" json:"created_at"`
	UpdatedAt  int64  `gorm:"column:updated_at" json:"updated_at"`
}

func (BarRecipe) TableName() string { return "bar_recipe" }

type BarRecipeItem struct {
	Id          uint64          `gorm:"column:id;primaryKey" json:"id"`
	RecipeId    uint64          `gorm:"column:recipe_id" json:"recipe_id"`
	TypeId      uint64          `gorm:"column:type_id" json:"type_id"`
	Qty         float64         `gorm:"column:qty" json:"qty"`
	Requirement json.RawMessage `gorm:"column:requirement;type:json" json:"requirement"`
	Step        uint8           `gorm:"column:step" json:"step"`
}

func (BarRecipeItem) TableName() string { return "bar_recipe_item" }

type BarDrink struct {
	Id                 uint64          `gorm:"column:id;primaryKey" json:"id"`
	RecipeId           uint64          `gorm:"column:recipe_id" json:"recipe_id"`
	OrderedBy          uint64          `gorm:"column:ordered_by" json:"ordered_by"`
	MadeBy             uint64          `gorm:"column:made_by" json:"made_by"`
	Message            string          `gorm:"column:message" json:"message"`
	InputsSnapshot     json.RawMessage `gorm:"column:inputs_snapshot;type:json" json:"inputs_snapshot"`
	FlavorSnapshot     json.RawMessage `gorm:"column:flavor_snapshot;type:json" json:"flavor_snapshot"`
	AppearanceSnapshot json.RawMessage `gorm:"column:appearance_snapshot;type:json" json:"appearance_snapshot"`
	MouthfeelSnapshot  json.RawMessage `gorm:"column:mouthfeel_snapshot;type:json" json:"mouthfeel_snapshot"`
	Description        string          `gorm:"column:description" json:"description"`
	ConfigVersion      uint            `gorm:"column:config_version" json:"config_version"`
	CreatedAt          int64           `gorm:"column:created_at" json:"created_at"`
}

func (BarDrink) TableName() string { return "bar_drink" }

// BarDatabase exposes the shared forum database to the isolated bar service.
// Keeping the constructor here ensures the service uses the same pool as the forum.
func BarDatabase() *gorm.DB { return db }
