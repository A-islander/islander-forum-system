package bar

import (
	"context"
	"strings"
	"testing"

	llmservice "github.com/forum_server/service/llm"
)

type recordingLLM struct {
	request llmservice.Request
	content string
}

func (client *recordingLLM) Complete(_ context.Context, request llmservice.Request) (string, error) {
	client.request = request
	if client.content != "" {
		return client.content, nil
	}
	return "哼，浪花似的蓝色可不是随便摇出来的。椰香柔柔托着菠萝的明亮酸甜，入口清爽又圆润——拿稳了，别让海风抢走。", nil
}

func TestIslandGirlDescriberKeepsPersonaAndDrinkData(t *testing.T) {
	client := &recordingLLM{}
	describer := NewIslandGirlDescriber(client)
	description, err := describer.Describe(context.Background(), DescribeInput{
		RecipeName: "海浪之歌", RecipeStory: "本店同名招牌", Technique: "摇和",
		Flavor: FlavorSnapshot{Leaves: map[string]float64{"601": 3}}, FlavorNames: map[uint64]string{601: "菠萝香"},
		Appearance:  AppearanceSnapshot{Color: "#d0c460", Texture: "cloudy"},
		Mouthfeel:   MouthfeelSnapshot{Dominant: []string{"crisp"}},
		Ingredients: []DescribeIngredient{{Name: "菠萝汁", Qty: 60, Unit: "ml"}},
	})
	if err != nil || description == "" {
		t.Fatalf("description=%q err=%v", description, err)
	}
	for _, expected := range []string{"岛民娘", "俏皮灵动", "傲娇和嘴硬", "不自称AI", "绝不使用「您」", "不要连续嘴硬"} {
		if !strings.Contains(client.request.System, expected) {
			t.Fatalf("system prompt does not contain %q: %s", expected, client.request.System)
		}
	}
	for _, expected := range []string{"drink.serve", "海浪之歌", "菠萝香", "只输出岛民娘"} {
		if !strings.Contains(client.request.Prompt, expected) {
			t.Fatalf("drink prompt does not contain %q: %s", expected, client.request.Prompt)
		}
	}
	if client.request.MaxTokens != 180 {
		t.Fatalf("max tokens=%d, want 180", client.request.MaxTokens)
	}
}

type fixedLLM struct {
	content string
}

func (client fixedLLM) Complete(_ context.Context, _ llmservice.Request) (string, error) {
	return client.content, nil
}

func TestIslandGirlDescriberEnforcesStaleIngredientWarning(t *testing.T) {
	describer := NewIslandGirlDescriber(fixedLLM{content: "这杯菠萝香很亮，拿稳了。"})
	freshness := 20.0
	description, err := describer.Describe(context.Background(), DescribeInput{
		HasStaleIngredient: true,
		Ingredients:        []DescribeIngredient{{Name: "菠萝汁", Freshness: &freshness, Condition: "已过最佳状态，会带来陈味"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(description, "过了最佳状态") || !strings.Contains(description, "陈味") {
		t.Fatalf("critical stale fact was not appended: %s", description)
	}
}

func TestIslandGirlDescriberDoesNotDuplicateStaleWarning(t *testing.T) {
	const generated = "这批菠萝已经不新鲜了，尾韵会有一点陈味。"
	describer := NewIslandGirlDescriber(fixedLLM{content: generated})
	description, err := describer.Describe(context.Background(), DescribeInput{HasStaleIngredient: true})
	if err != nil {
		t.Fatal(err)
	}
	if description != generated {
		t.Fatalf("stale warning was duplicated: %s", description)
	}
}

func TestIslandGirlDescriberBuildsIngredientPerformanceCue(t *testing.T) {
	client := &recordingLLM{content: "柠檬要现切，酸香跑掉我可不负责。"}
	describer := NewIslandGirlDescriber(client)
	line, err := describer.DescribePerformanceCue(context.Background(), DescribeInput{
		RecipeName: "海角黄昏", Technique: "摇和",
		Ingredients: []DescribeIngredient{{Name: "柠檬", Qty: 15, Unit: "g"}},
		Trace:       []TracePortion{{TypeId: 11, TypeName: "柠檬", SourceNote: "岛民娘从码头市场挑的"}},
	}, "ingredient", &PerformanceStep{Step: 1, TypeId: 11, TypeName: "柠檬", Qty: 15, Unit: "g"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(line, "柠檬") {
		t.Fatalf("unexpected performance cue: %q", line)
	}
	for _, expected := range []string{"俏皮的现场互动", "不要使用哼、别催、拿稳"} {
		if !strings.Contains(client.request.Prompt, expected) {
			t.Fatalf("performance prompt does not contain %q: %s", expected, client.request.Prompt)
		}
	}
}

func TestIslandGirlDescriberRejectsCompletedTechniqueCue(t *testing.T) {
	describer := NewIslandGirlDescriber(fixedLLM{content: "摇和好了，拿稳。"})
	_, err := describer.DescribePerformanceCue(context.Background(), DescribeInput{Technique: "摇和"}, "technique", nil)
	if err == nil {
		t.Fatal("expected a completed-technique cue to be rejected")
	}
}
