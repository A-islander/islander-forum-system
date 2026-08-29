package controller

import (
	"context"

	barservice "github.com/forum_server/service/bar"
)

var barService = barservice.NewDefaultService()

func GetBarMenu(ctx context.Context) ([]barservice.MenuRecipe, error) {
	return barService.Menu(ctx)
}

func MakeBarDrink(ctx context.Context, request barservice.OrderRequest) (barservice.OrderResult, error) {
	return barService.MakeDrink(ctx, request)
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
