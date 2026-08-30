package bar

import (
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"

	"github.com/forum_server/model"
)

var mouthfeelKeys = []string{"body", "crisp", "creamy", "effervescent", "heat", "astringent"}

type allocatedInput struct {
	item      model.BarRecipeItem
	typeInfo  model.BarIngredientType
	instances []model.BarIngredientInstance
	portions  []PortionSnapshot
}

func numberMap(raw json.RawMessage) map[string]float64 {
	result := make(map[string]float64)
	if len(raw) == 0 || string(raw) == "null" {
		return result
	}
	_ = json.Unmarshal(raw, &result)
	return result
}

func effectiveAttributes(instance model.BarIngredientInstance, ingredientType model.BarIngredientType, now int64) map[string]float64 {
	attrs := numberMap(instance.Attrs)
	freshness, ok := attrs["freshness"]
	if !ok {
		return attrs
	}
	elapsed := float64(now-instance.ProducedAt) / 86400
	if elapsed < 0 {
		elapsed = 0
	}
	freshness -= elapsed * ingredientType.FreshnessDecayPerDay
	attrs["freshness"] = math.Max(0, math.Min(100, freshness))
	return attrs
}

func round(value float64, digits int) float64 {
	pow := math.Pow10(digits)
	return math.Round(value*pow) / pow
}

func computeFlavor(inputs []allocatedInput, mappings []model.BarIngredientFlavor, nodes []model.BarFlavorNode) (FlavorSnapshot, map[uint64]string, bool) {
	total := 0.0
	for _, input := range inputs {
		total += input.item.Qty
	}
	parents := make(map[uint64]*uint64, len(nodes))
	names := make(map[uint64]string, len(nodes))
	defaultSensitivity := make(map[uint64]float64, len(nodes))
	for _, node := range nodes {
		parents[node.Id], names[node.Id], defaultSensitivity[node.Id] = node.ParentId, node.Name, node.SensitivityDefault
	}
	byType := make(map[uint64][]model.BarIngredientFlavor)
	for _, mapping := range mappings {
		byType[mapping.TypeId] = append(byType[mapping.TypeId], mapping)
	}
	leaves := make(map[uint64]float64)
	stale := false
	for _, input := range inputs {
		for _, mapping := range byType[input.item.TypeId] {
			sensitivity := defaultSensitivity[mapping.NodeId]
			if mapping.Sensitivity != nil {
				sensitivity = *mapping.Sensitivity
			}
			for _, portion := range input.portions {
				freshness := 100.0
				if portion.Freshness != nil {
					freshness = *portion.Freshness
				}
				coefficient := 1 - sensitivity + sensitivity*freshness/100
				if sensitivity < 0 {
					coefficient = 1 + math.Abs(sensitivity)*(1-freshness/100)
				}
				leaves[mapping.NodeId] += mapping.BaseIntensity * coefficient * portion.Qty / total
				if freshness < 30 {
					stale = true
				}
			}
		}
	}
	if stale {
		for _, input := range inputs {
			for _, portion := range input.portions {
				if portion.Freshness != nil && *portion.Freshness < 30 {
					leaves[1201] += .5 * (30 - *portion.Freshness)
				}
			}
		}
	}
	rolled := make(map[uint64]float64, len(leaves))
	for id, value := range leaves {
		rolled[id] += value
		seen := map[uint64]bool{id: true}
		parent := parents[id]
		for parent != nil && !seen[*parent] {
			rolled[*parent] += value
			seen[*parent] = true
			parent = parents[*parent]
		}
	}
	leafJSON, rolledJSON := make(map[string]float64), make(map[string]float64)
	for id, value := range leaves {
		leafJSON[strconv.FormatUint(id, 10)] = round(value, 2)
	}
	for id, value := range rolled {
		rolledJSON[strconv.FormatUint(id, 10)] = round(value, 2)
	}
	return FlavorSnapshot{Leaves: leafJSON, Rolled: rolledJSON}, names, stale
}

func parseColor(value string) (float64, float64, float64, bool) {
	if len(value) != 7 || value[0] != '#' {
		return 0, 0, 0, false
	}
	r, e1 := strconv.ParseUint(value[1:3], 16, 8)
	g, e2 := strconv.ParseUint(value[3:5], 16, 8)
	b, e3 := strconv.ParseUint(value[5:7], 16, 8)
	return float64(r), float64(g), float64(b), e1 == nil && e2 == nil && e3 == nil
}

func techniqueTexture(technique string) string {
	texture := map[string]string{"摇和": "cloudy", "搅和": "clear", "捣压": "muddy", "兑和": "bubbly", "分层": "layered"}[technique]
	if texture == "" {
		return "clear"
	}
	return texture
}

func computeAppearance(inputs []allocatedInput, technique string) AppearanceSnapshot {
	total := 0.0
	for _, input := range inputs {
		total += input.item.Qty
	}
	r, g, b, opacity, gloss := 0.0, 0.0, 0.0, 0.0, 0.0
	for _, input := range inputs {
		appearance := numberMap(input.typeInfo.Appearance)
		var raw map[string]interface{}
		_ = json.Unmarshal(input.typeInfo.Appearance, &raw)
		color, _ := raw["color"].(string)
		rr, gg, bb, ok := parseColor(color)
		if !ok {
			continue
		}
		weight := input.item.Qty / total
		r, g, b = r+rr*weight, g+gg*weight, b+bb*weight
		opacity += appearance["opacity"] * weight
		gloss += appearance["gloss"] * weight
	}
	texture := techniqueTexture(technique)
	layers := 1
	if texture == "layered" {
		layers = 2
	}
	return AppearanceSnapshot{
		Color:   fmt.Sprintf("#%02x%02x%02x", int(math.Max(0, math.Min(255, r))), int(math.Max(0, math.Min(255, g))), int(math.Max(0, math.Min(255, b)))),
		Opacity: round(math.Max(0, math.Min(1, opacity)), 2), Gloss: round(math.Max(0, math.Min(1, gloss)), 2),
		Texture: texture, Layers: layers,
	}
}

func computeMouthfeel(inputs []allocatedInput, technique string) MouthfeelSnapshot {
	total := 0.0
	for _, input := range inputs {
		total += input.item.Qty
	}
	base := make(map[string]float64)
	for _, key := range mouthfeelKeys {
		base[key] = 0
	}
	for _, input := range inputs {
		values := numberMap(input.typeInfo.Mouthfeel)
		weight := input.item.Qty / total
		for _, key := range mouthfeelKeys {
			base[key] += values[key] * weight
		}
	}
	mods := map[string]map[string]float64{
		"摇和": {"body": .05, "effervescent": .15}, "搅和": {"body": .10},
		"捣压": {"astringent": .10}, "兑和": {"effervescent": .10}, "分层": {},
	}
	for key, value := range mods[technique] {
		base[key] = math.Min(1, base[key]+value)
	}
	type pair struct {
		key   string
		value float64
	}
	pairs := make([]pair, 0, len(base))
	for key, value := range base {
		base[key] = round(value, 2)
		pairs = append(pairs, pair{key, value})
	}
	sort.SliceStable(pairs, func(i, j int) bool {
		if pairs[i].value == pairs[j].value {
			return strings.Index(strings.Join(mouthfeelKeys, ","), pairs[i].key) < strings.Index(strings.Join(mouthfeelKeys, ","), pairs[j].key)
		}
		return pairs[i].value > pairs[j].value
	})
	dominant := make([]string, 0, 2)
	for i := 0; i < len(pairs) && i < 2; i++ {
		dominant = append(dominant, pairs[i].key)
	}
	return MouthfeelSnapshot{Base: base, Texture: techniqueTexture(technique), Dominant: dominant}
}
