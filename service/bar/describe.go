package bar

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type DescribeInput struct {
	RecipeName         string
	Flavor             FlavorSnapshot
	Appearance         AppearanceSnapshot
	Mouthfeel          MouthfeelSnapshot
	FlavorNames        map[uint64]string
	HasStaleIngredient bool
}

type Describer interface {
	Describe(ctx context.Context, input DescribeInput) (string, error)
}

type RuleDescriber struct{}

type flavorScore struct {
	id    uint64
	value float64
}

func (RuleDescriber) Describe(_ context.Context, input DescribeInput) (string, error) {
	scores := make([]flavorScore, 0, len(input.Flavor.Leaves))
	for key, value := range input.Flavor.Leaves {
		var id uint64
		fmt.Sscanf(key, "%d", &id)
		scores = append(scores, flavorScore{id: id, value: value})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].value > scores[j].value })
	tags := make([]string, 0, 3)
	for i := 0; i < len(scores) && i < 3; i++ {
		if name := input.FlavorNames[scores[i].id]; name != "" {
			tags = append(tags, name)
		}
	}
	if len(tags) == 0 {
		tags = append(tags, "海风般的香气")
	}

	textureWords := map[string]string{
		"cloudy": "微微浑浊", "clear": "清澈", "muddy": "带着草本碎末感",
		"bubbly": "气泡跳跃", "layered": "层次分明",
	}
	mouthWords := map[string]string{
		"body": "酒体厚实", "crisp": "口感清爽", "creamy": "质地绵密",
		"effervescent": "气泡充足", "heat": "烈感明显", "astringent": "尾韵微涩",
	}
	mouth := make([]string, 0, len(input.Mouthfeel.Dominant))
	for _, key := range input.Mouthfeel.Dominant {
		if word := mouthWords[key]; word != "" {
			mouth = append(mouth, word)
		}
	}
	if len(mouth) == 0 {
		mouth = append(mouth, "入口平衡")
	}
	ending := "用的是状态正好的原料。"
	if input.HasStaleIngredient {
		ending = "其中有原料过了最佳新鲜期，尾韵多了一点陈味。"
	}
	return fmt.Sprintf("今天这杯泛着%s的颜色，入口%s。%s依次浮现，%s。%s",
		input.Appearance.Color, textureWords[input.Appearance.Texture], strings.Join(tags, "、"), strings.Join(mouth, "、"), ending), nil
}
