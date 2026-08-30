package controller

import (
	"context"
	"log"

	barservice "github.com/forum_server/service/bar"
)

var barService = newBarService()

func newBarService() *barservice.Service {
	describer, err := barservice.NewMiniMaxDescriberFromEnv()
	if err != nil {
		log.Printf("bar LLM disabled: %v", err)
	}
	return barservice.NewDefaultService(describer)
}

func GetBarMenu(ctx context.Context) ([]barservice.MenuRecipe, error) {
	return barService.Menu(ctx)
}

func MakeBarDrink(ctx context.Context, request barservice.OrderRequest) (barservice.OrderResult, error) {
	return barService.MakeDrinkAsync(ctx, request)
}

func MakeBarDrinkForPerformance(ctx context.Context, request barservice.OrderRequest) (barservice.OrderResult, error) {
	return barService.MakeDrinkForPerformance(ctx, request)
}

func EnhanceBarDrinkDescription(ctx context.Context, result *barservice.OrderResult) {
	barService.EnhanceDescription(ctx, result)
}

func GetBarDrink(ctx context.Context, id uint64) (barservice.DrinkDetail, error) {
	return barService.Drink(ctx, id)
}

func GetBarTrace(ctx context.Context, id uint64) (barservice.DrinkTrace, error) {
	return barService.Trace(ctx, id)
}

func GetBarStock(ctx context.Context) ([]barservice.StockItem, error) {
	return barService.Stock(ctx)
}

func RestockBar(ctx context.Context, request barservice.RestockRequest) (interface{}, error) {
	return barService.Restock(ctx, request)
}
