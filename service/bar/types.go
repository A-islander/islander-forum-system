package bar

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"

	"github.com/forum_server/model"
)

type OrderRequest struct {
	RecipeId  uint64            `json:"recipe_id"`
	OrderedBy uint64            `json:"ordered_by"`
	Overrides map[uint64]uint64 `json:"overrides"`
	Message   string            `json:"message"`
}

type PortionSnapshot struct {
	InstanceId uint64   `json:"instance_id"`
	Code       string   `json:"code"`
	Qty        float64  `json:"qty"`
	Freshness  *float64 `json:"freshness,omitempty"`
}

type InputSnapshot struct {
	Kind     string            `json:"kind"`
	TypeId   uint64            `json:"type_id,omitempty"`
	Qty      float64           `json:"qty,omitempty"`
	Portions []PortionSnapshot `json:"portions,omitempty"`
	Name     string            `json:"name,omitempty"`
}

type FlavorSnapshot struct {
	Leaves map[string]float64 `json:"leaves"`
	Rolled map[string]float64 `json:"rolled"`
}

type FlavorValue struct {
	Id    uint64  `json:"id"`
	Name  string  `json:"name"`
	Value float64 `json:"value"`
}

type FlavorView struct {
	Leaves []FlavorValue `json:"leaves"`
	Rolled []FlavorValue `json:"rolled"`
}

type AppearanceSnapshot struct {
	Color   string  `json:"color"`
	Opacity float64 `json:"opacity"`
	Gloss   float64 `json:"gloss"`
	Texture string  `json:"texture"`
	Layers  int     `json:"layers"`
	Garnish *string `json:"garnish"`
}

type MouthfeelSnapshot struct {
	Base     map[string]float64 `json:"base"`
	Texture  string             `json:"texture"`
	Dominant []string           `json:"dominant"`
}

type TracePortion struct {
	TypeId     uint64  `json:"type_id"`
	TypeName   string  `json:"type"`
	Unit       string  `json:"unit"`
	InstanceId uint64  `json:"instance_id"`
	Code       string  `json:"code"`
	Qty        float64 `json:"qty"`
	Source     string  `json:"source"`
	SourceId   uint64  `json:"source_id"`
	SourceNote string  `json:"source_note"`
}

type PerformanceStep struct {
	Step     uint8   `json:"step"`
	Action   string  `json:"action"`
	TypeId   uint64  `json:"type_id"`
	TypeName string  `json:"type_name"`
	Qty      float64 `json:"qty"`
	Unit     string  `json:"unit"`
}

type DrinkView struct {
	Id                 uint64             `json:"id"`
	RecipeId           uint64             `json:"recipe_id"`
	RecipeName         string             `json:"recipe_name"`
	Technique          string             `json:"technique"`
	OrderedBy          uint64             `json:"ordered_by"`
	MadeBy             uint64             `json:"made_by"`
	Message            string             `json:"message"`
	InputsSnapshot     []InputSnapshot    `json:"inputs_snapshot"`
	FlavorSnapshot     FlavorView         `json:"flavor_snapshot"`
	AppearanceSnapshot AppearanceSnapshot `json:"appearance_snapshot"`
	MouthfeelSnapshot  MouthfeelSnapshot  `json:"mouthfeel_snapshot"`
	Description        string             `json:"description"`
	ConfigVersion      uint               `json:"config_version"`
	CreatedAt          int64              `json:"created_at"`
}

type OrderResult struct {
	OrderId    string             `json:"order_id"`
	Drink      DrinkView          `json:"drink"`
	Trace      []TracePortion     `json:"trace"`
	Steps      []PerformanceStep  `json:"steps,omitempty"`
	RecipeName string             `json:"-"`
	Technique  string             `json:"-"`
	Flavor     FlavorSnapshot     `json:"-"`
	Appearance AppearanceSnapshot `json:"-"`
	Mouthfeel  MouthfeelSnapshot  `json:"-"`
}

type MissingDetail struct {
	TypeId   uint64  `json:"type_id"`
	Name     string  `json:"name"`
	Need     float64 `json:"need"`
	Shortage float64 `json:"shortage"`
}

type MissingError struct {
	Details []MissingDetail `json:"details"`
}

func (e *MissingError) Error() string {
	if len(e.Details) == 0 {
		return "missing ingredients"
	}
	return fmt.Sprintf("missing ingredient: %s", e.Details[0].Name)
}

type MenuRecipe struct {
	Id            uint64        `json:"id"`
	Name          string        `json:"name"`
	Story         string        `json:"story"`
	Technique     string        `json:"technique"`
	Status        string        `json:"status"`
	Missing       []string      `json:"missing"`
	FlavorPreview []FlavorValue `json:"flavor_preview"`
	OrderCount    uint          `json:"order_count"`
}

type StockItem struct {
	TypeId           uint64  `json:"type_id"`
	Code             string  `json:"code"`
	Name             string  `json:"name"`
	Unit             string  `json:"unit"`
	QtyRemain        float64 `json:"qty_remain"`
	EarliestExpireAt int64   `json:"earliest_expire_at"`
}

type RestockRequest struct {
	TypeId     uint64                 `json:"type_id"`
	Quantity   float64                `json:"quantity"`
	SourceType uint8                  `json:"source_type"`
	SourceUid  uint64                 `json:"source_uid"`
	Attrs      map[string]interface{} `json:"attrs"`
	ExpireAt   int64                  `json:"expire_at"`
	Note       string                 `json:"note"`
}

type DrinkDetail struct {
	Drink  DrinkView       `json:"drink"`
	Recipe model.BarRecipe `json:"recipe"`
}

type TraceNode struct {
	Instance model.BarIngredientInstance `json:"instance"`
	TypeName string                      `json:"type_name"`
	Restock  *model.BarRestockLog        `json:"restock,omitempty"`
	Inputs   []TraceNode                 `json:"inputs,omitempty"`
}

type DrinkTrace struct {
	DrinkId uint64      `json:"drink_id"`
	Inputs  []TraceNode `json:"inputs"`
}

func rawJSON(value interface{}) (json.RawMessage, error) {
	data, err := json.Marshal(value)
	return json.RawMessage(data), err
}

func flavorView(snapshot FlavorSnapshot, names map[uint64]string) FlavorView {
	convert := func(values map[string]float64) []FlavorValue {
		result := make([]FlavorValue, 0, len(values))
		for key, value := range values {
			id, err := strconv.ParseUint(key, 10, 64)
			if err != nil {
				continue
			}
			result = append(result, FlavorValue{Id: id, Name: names[id], Value: value})
		}
		sort.Slice(result, func(i, j int) bool { return result[i].Id < result[j].Id })
		return result
	}
	return FlavorView{Leaves: convert(snapshot.Leaves), Rolled: convert(snapshot.Rolled)}
}

func drinkView(drink model.BarDrink, recipe model.BarRecipe, inputs []InputSnapshot, flavor FlavorSnapshot,
	appearance AppearanceSnapshot, mouthfeel MouthfeelSnapshot, names map[uint64]string) DrinkView {
	return DrinkView{Id: drink.Id, RecipeId: drink.RecipeId, RecipeName: recipe.Name, Technique: recipe.Technique,
		OrderedBy: drink.OrderedBy, MadeBy: drink.MadeBy, Message: drink.Message, InputsSnapshot: inputs,
		FlavorSnapshot: flavorView(flavor, names), AppearanceSnapshot: appearance, MouthfeelSnapshot: mouthfeel,
		Description: drink.Description, ConfigVersion: drink.ConfigVersion, CreatedAt: drink.CreatedAt}
}
