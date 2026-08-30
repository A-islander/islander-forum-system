package model

import (
	"encoding/json"

	"gorm.io/gorm"
)

type BarFlavorNode struct {
	Id                 uint64  `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	ParentId           *uint64 `gorm:"column:parent_id;type:bigint unsigned;default:null;index:idx_parent;uniqueIndex:uk_parent_name,priority:1" json:"parent_id"`
	Name               string  `gorm:"column:name;type:varchar(64);not null;uniqueIndex:uk_parent_name,priority:2" json:"name"`
	Level              uint8   `gorm:"column:level;type:tinyint unsigned;not null;default:1" json:"level"`
	Description        string  `gorm:"column:description;type:varchar(512);not null;default:''" json:"description"`
	SensitivityDefault float64 `gorm:"column:sensitivity_default;type:decimal(3,2);not null;default:0.00" json:"sensitivity_default"`
	IsHidden           uint8   `gorm:"column:is_hidden;type:tinyint unsigned;not null;default:0" json:"is_hidden"`
	Sort               uint    `gorm:"column:sort;type:int unsigned;not null;default:0" json:"sort"`
	Status             uint8   `gorm:"column:status;type:tinyint unsigned;not null;default:0" json:"status"`
	CreatedAt          int64   `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt          int64   `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (BarFlavorNode) TableName() string { return "bar_flavor_node" }

type BarIngredientType struct {
	Id                   uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	Code                 string          `gorm:"column:code;type:varchar(16);not null;uniqueIndex:uk_code" json:"code"`
	Name                 string          `gorm:"column:name;type:varchar(64);not null;uniqueIndex:uk_name" json:"name"`
	Category             string          `gorm:"column:category;type:varchar(32);not null;default:''" json:"category"`
	Mixable              uint8           `gorm:"column:mixable;type:tinyint unsigned;not null;default:1" json:"mixable"`
	Unit                 string          `gorm:"column:unit;type:varchar(8);not null;default:ml" json:"unit"`
	DefaultBatchQty      float64         `gorm:"column:default_batch_qty;type:decimal(10,2);not null;default:1.00" json:"default_batch_qty"`
	ShelfLifeDays        uint            `gorm:"column:shelf_life_days;type:int unsigned;not null;default:30" json:"shelf_life_days"`
	FreshnessDecayPerDay float64         `gorm:"column:freshness_decay_per_day;type:decimal(5,2);not null;default:0.00" json:"freshness_decay_per_day"`
	DefaultAttrs         json.RawMessage `gorm:"column:default_attrs;type:json;default:null" json:"default_attrs"`
	Appearance           json.RawMessage `gorm:"column:appearance;type:json;default:null" json:"appearance"`
	Mouthfeel            json.RawMessage `gorm:"column:mouthfeel;type:json;default:null" json:"mouthfeel"`
	Codex                string          `gorm:"column:codex;type:text;default:null" json:"codex"`
	Status               uint8           `gorm:"column:status;type:tinyint unsigned;not null;default:0" json:"status"`
	CreatedAt            int64           `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt            int64           `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (BarIngredientType) TableName() string { return "bar_ingredient_type" }

type BarIngredientFlavor struct {
	Id            uint64   `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	TypeId        uint64   `gorm:"column:type_id;type:bigint unsigned;not null;uniqueIndex:uk_type_node,priority:1" json:"type_id"`
	NodeId        uint64   `gorm:"column:node_id;type:bigint unsigned;not null;uniqueIndex:uk_type_node,priority:2;index:idx_node" json:"node_id"`
	BaseIntensity float64  `gorm:"column:base_intensity;type:decimal(4,1);not null" json:"base_intensity"`
	Sensitivity   *float64 `gorm:"column:sensitivity;type:decimal(3,2);default:null" json:"sensitivity"`
	CreatedAt     int64    `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt     int64    `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (BarIngredientFlavor) TableName() string { return "bar_ingredient_flavor" }

type BarIngredientInstance struct {
	Id         uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	Code       string          `gorm:"column:code;type:varchar(32);not null;uniqueIndex:uk_code" json:"code"`
	TypeId     uint64          `gorm:"column:type_id;type:bigint unsigned;not null;index:idx_type_status_expire,priority:1" json:"type_id"`
	QtyTotal   float64         `gorm:"column:qty_total;type:decimal(10,2);not null" json:"qty_total"`
	QtyRemain  float64         `gorm:"column:qty_remain;type:decimal(10,2);not null" json:"qty_remain"`
	ProducedAt int64           `gorm:"column:produced_at;type:bigint;not null" json:"produced_at"`
	ExpireAt   int64           `gorm:"column:expire_at;type:bigint;not null;index:idx_type_status_expire,priority:3;index:idx_expire_status,priority:1" json:"expire_at"`
	Attrs      json.RawMessage `gorm:"column:attrs;type:json;default:null" json:"attrs"`
	Source     string          `gorm:"column:source;type:varchar(16);not null;default:restock" json:"source"`
	SourceId   uint64          `gorm:"column:source_id;type:bigint unsigned;not null;default:0" json:"source_id"`
	Status     uint8           `gorm:"column:status;type:tinyint unsigned;not null;default:0;index:idx_type_status_expire,priority:2;index:idx_expire_status,priority:2" json:"status"`
	CreatedAt  int64           `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt  int64           `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (BarIngredientInstance) TableName() string { return "bar_ingredient_instance" }

type BarRestockLog struct {
	Id         uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	TypeId     uint64          `gorm:"column:type_id;type:bigint unsigned;not null;index:idx_type_time,priority:1" json:"type_id"`
	Quantity   float64         `gorm:"column:quantity;type:decimal(10,2);not null" json:"quantity"`
	InstanceId uint64          `gorm:"column:instance_id;type:bigint unsigned;not null" json:"instance_id"`
	SourceType uint8           `gorm:"column:source_type;type:tinyint unsigned;not null;default:0;index:idx_source_uid,priority:1" json:"source_type"`
	SourceUid  uint64          `gorm:"column:source_uid;type:bigint unsigned;not null;default:0;index:idx_source_uid,priority:2" json:"source_uid"`
	Attrs      json.RawMessage `gorm:"column:attrs;type:json;default:null" json:"attrs"`
	ExpireAt   int64           `gorm:"column:expire_at;type:bigint;not null" json:"expire_at"`
	Note       string          `gorm:"column:note;type:varchar(255);not null;default:''" json:"note"`
	CreatedAt  int64           `gorm:"column:created_at;type:bigint;not null;index:idx_type_time,priority:2" json:"created_at"`
}

func (BarRestockLog) TableName() string { return "bar_restock_log" }

type BarProcess struct {
	Id            uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	Name          string          `gorm:"column:name;type:varchar(64);not null" json:"name"`
	Story         string          `gorm:"column:story;type:text;default:null" json:"story"`
	CreatorId     uint64          `gorm:"column:creator_id;type:bigint unsigned;not null;default:0;index:idx_creator_status,priority:1" json:"creator_id"`
	Inputs        json.RawMessage `gorm:"column:inputs;type:json;not null" json:"inputs"`
	OutputTypeId  uint64          `gorm:"column:output_type_id;type:bigint unsigned;not null;index:idx_output_type" json:"output_type_id"`
	OutputQty     float64         `gorm:"column:output_qty;type:decimal(10,2);not null" json:"output_qty"`
	AttributeRule json.RawMessage `gorm:"column:attribute_rule;type:json;default:null" json:"attribute_rule"`
	ShelfLifeDays *uint           `gorm:"column:shelf_life_days;type:int unsigned;default:null" json:"shelf_life_days"`
	Status        uint8           `gorm:"column:status;type:tinyint unsigned;not null;default:1;index:idx_creator_status,priority:2" json:"status"`
	CreatedAt     int64           `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt     int64           `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (BarProcess) TableName() string { return "bar_process" }

type BarProcessLog struct {
	Id               uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	ProcessId        uint64          `gorm:"column:process_id;type:bigint unsigned;not null;index:idx_process_time,priority:1" json:"process_id"`
	OperatorUid      uint64          `gorm:"column:operator_uid;type:bigint unsigned;not null;index:idx_operator,priority:1" json:"operator_uid"`
	InputsSnapshot   json.RawMessage `gorm:"column:inputs_snapshot;type:json;not null" json:"inputs_snapshot"`
	OutputInstanceId uint64          `gorm:"column:output_instance_id;type:bigint unsigned;not null" json:"output_instance_id"`
	CreatedAt        int64           `gorm:"column:created_at;type:bigint;not null;index:idx_process_time,priority:2;index:idx_operator,priority:2" json:"created_at"`
}

func (BarProcessLog) TableName() string { return "bar_process_log" }

type BarStockPolicy struct {
	TypeId               uint64   `gorm:"column:type_id;type:bigint unsigned;primaryKey;not null" json:"type_id"`
	MinQty               float64  `gorm:"column:min_qty;type:decimal(10,2);not null" json:"min_qty"`
	MaxQty               float64  `gorm:"column:max_qty;type:decimal(10,2);not null" json:"max_qty"`
	ReplenishMode        string   `gorm:"column:replenish_mode;type:varchar(16);not null;default:restock" json:"replenish_mode"`
	ProcessId            *uint64  `gorm:"column:process_id;type:bigint unsigned;default:null;index:idx_process" json:"process_id"`
	RetireFreshnessBelow *float64 `gorm:"column:retire_freshness_below;type:decimal(5,2);default:null" json:"retire_freshness_below"`
	Enabled              uint8    `gorm:"column:enabled;type:tinyint unsigned;not null;default:1;index:idx_enabled" json:"enabled"`
	CreatedAt            int64    `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt            int64    `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (BarStockPolicy) TableName() string { return "bar_stock_policy" }

type BarRecipe struct {
	Id         uint64 `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	Name       string `gorm:"column:name;type:varchar(64);not null" json:"name"`
	Story      string `gorm:"column:story;type:text;default:null" json:"story"`
	CreatorId  uint64 `gorm:"column:creator_id;type:bigint unsigned;not null;default:0;index:idx_creator,priority:1" json:"creator_id"`
	Technique  string `gorm:"column:technique;type:varchar(32);not null;default:''" json:"technique"`
	Status     uint8  `gorm:"column:status;type:tinyint unsigned;not null;default:1;index:idx_status_order,priority:1;index:idx_creator,priority:2" json:"status"`
	OrderCount uint   `gorm:"column:order_count;type:int unsigned;not null;default:0;index:idx_status_order,priority:2" json:"order_count"`
	CreatedAt  int64  `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt  int64  `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (BarRecipe) TableName() string { return "bar_recipe" }

type BarRecipeItem struct {
	Id          uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	RecipeId    uint64          `gorm:"column:recipe_id;type:bigint unsigned;not null;index:idx_recipe" json:"recipe_id"`
	TypeId      uint64          `gorm:"column:type_id;type:bigint unsigned;not null;index:idx_type" json:"type_id"`
	Qty         float64         `gorm:"column:qty;type:decimal(10,2);not null" json:"qty"`
	Requirement json.RawMessage `gorm:"column:requirement;type:json;default:null" json:"requirement"`
	Step        uint8           `gorm:"column:step;type:tinyint unsigned;not null;default:0" json:"step"`
}

func (BarRecipeItem) TableName() string { return "bar_recipe_item" }

type BarDrink struct {
	Id                 uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	RecipeId           uint64          `gorm:"column:recipe_id;type:bigint unsigned;not null;index:idx_recipe_time,priority:1" json:"recipe_id"`
	OrderedBy          uint64          `gorm:"column:ordered_by;type:bigint unsigned;not null;index:idx_ordered_by,priority:1" json:"ordered_by"`
	MadeBy             uint64          `gorm:"column:made_by;type:bigint unsigned;not null;default:0" json:"made_by"`
	Message            string          `gorm:"column:message;type:varchar(255);not null;default:''" json:"message"`
	InputsSnapshot     json.RawMessage `gorm:"column:inputs_snapshot;type:json;not null" json:"inputs_snapshot"`
	FlavorSnapshot     json.RawMessage `gorm:"column:flavor_snapshot;type:json;default:null" json:"flavor_snapshot"`
	AppearanceSnapshot json.RawMessage `gorm:"column:appearance_snapshot;type:json;default:null" json:"appearance_snapshot"`
	MouthfeelSnapshot  json.RawMessage `gorm:"column:mouthfeel_snapshot;type:json;default:null" json:"mouthfeel_snapshot"`
	Description        string          `gorm:"column:description;type:text;default:null" json:"description"`
	ConfigVersion      uint            `gorm:"column:config_version;type:int unsigned;not null;default:1" json:"config_version"`
	CreatedAt          int64           `gorm:"column:created_at;type:bigint;not null;index:idx_recipe_time,priority:2;index:idx_ordered_by,priority:2" json:"created_at"`
}

func (BarDrink) TableName() string { return "bar_drink" }

// BarModels returns the complete V1 schema in dependency order. It is kept
// separate from application startup so adding model metadata does not enable
// AutoMigrate implicitly.
func BarModels() []interface{} {
	return []interface{}{
		&BarFlavorNode{},
		&BarIngredientType{},
		&BarIngredientFlavor{},
		&BarIngredientInstance{},
		&BarRestockLog{},
		&BarProcess{},
		&BarProcessLog{},
		&BarStockPolicy{},
		&BarRecipe{},
		&BarRecipeItem{},
		&BarDrink{},
	}
}

// BarDatabase exposes the shared forum database to the isolated bar service.
// Keeping the constructor here ensures the service uses the same pool as the forum.
func BarDatabase() *gorm.DB { return db }
