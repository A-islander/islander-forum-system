package controller

import (
	"context"
	"log"
	"os"
	"strings"
	"time"

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

func GetBarIngredients(ctx context.Context) ([]barservice.IngredientCatalogItem, error) {
	return barService.Ingredients(ctx)
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

func BuildBarPerformanceCue(ctx context.Context, result *barservice.OrderResult, stage string, stepIndex int) (string, error) {
	return barService.BuildPerformanceCue(ctx, result, stage, stepIndex)
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

func StartBarStockMaintenance(ctx context.Context) {
	enabled := strings.ToLower(strings.TrimSpace(os.Getenv("BAR_STOCK_MAINTENANCE_ENABLED")))
	if enabled == "0" || enabled == "false" || enabled == "off" {
		log.Printf("bar stock maintenance disabled")
		return
	}
	interval := time.Hour
	if raw := strings.TrimSpace(os.Getenv("BAR_STOCK_MAINTENANCE_INTERVAL")); raw != "" {
		parsed, err := time.ParseDuration(raw)
		if err != nil || parsed <= 0 {
			log.Printf("invalid BAR_STOCK_MAINTENANCE_INTERVAL %q; using %s", raw, interval)
		} else {
			interval = parsed
		}
	}
	go barService.RunMaintenanceLoop(ctx, interval)
}
