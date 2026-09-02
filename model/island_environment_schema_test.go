package model

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestIslandEnvironmentModelSchemaMatchesDDL(t *testing.T) {
	user, password, address := os.Getenv("ISLANDER_DB_USER"), os.Getenv("ISLANDER_DB_PASSWORD"), os.Getenv("ISLANDER_DB_ADDR")
	if user == "" || address == "" {
		t.Skip("database environment is not configured")
	}
	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	autoDatabase, ddlDatabase := "island_model_auto_"+suffix, "island_model_ddl_"+suffix
	rootDSN := fmt.Sprintf("%s:%s@tcp(%s)/mysql?charset=utf8mb4&multiStatements=true", user, password, address)
	root, err := gorm.Open(mysql.Open(rootDSN), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	for _, database := range []string{autoDatabase, ddlDatabase} {
		if err := root.Exec("CREATE DATABASE `" + database + "` CHARACTER SET utf8mb4").Error; err != nil {
			t.Fatal(err)
		}
		defer root.Exec("DROP DATABASE IF EXISTS `" + database + "`")
	}
	autoDB := openBarSchemaDatabase(t, user, password, address, autoDatabase)
	if err := autoDB.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").AutoMigrate(IslandModels()...); err != nil {
		t.Fatal(err)
	}
	ddl, err := os.ReadFile("../migrations/007_island_environment.sql")
	if err != nil {
		t.Fatal(err)
	}
	ddlDB := openBarSchemaDatabase(t, user, password, address, ddlDatabase)
	if err := ddlDB.Exec(string(ddl)).Error; err != nil {
		t.Fatal(err)
	}
	autoColumns, autoIndexes := readIslandSchema(t, root, autoDatabase)
	ddlColumns, ddlIndexes := readIslandSchema(t, root, ddlDatabase)
	if len(islandTableNames(autoColumns)) != 2 {
		t.Fatalf("AutoMigrate created %d island tables, want 2", len(islandTableNames(autoColumns)))
	}
	if !reflect.DeepEqual(autoColumns, ddlColumns) {
		t.Fatalf("AutoMigrate columns differ from DDL: %s", firstSchemaDifference(autoColumns, ddlColumns))
	}
	if !reflect.DeepEqual(autoIndexes, ddlIndexes) {
		t.Fatalf("AutoMigrate indexes differ from DDL: %s", firstSchemaDifference(autoIndexes, ddlIndexes))
	}
}

func islandTableNames(columns []barColumnSpec) map[string]bool {
	names := make(map[string]bool)
	for _, column := range columns {
		names[column.TableName] = true
	}
	return names
}

func readIslandSchema(t *testing.T, db *gorm.DB, database string) ([]barColumnSpec, []barIndexSpec) {
	t.Helper()
	var columns []barColumnSpec
	if err := db.Raw(`
		SELECT table_name, column_name, ordinal_position AS ordinal, column_type,
		       is_nullable, COALESCE(column_default, '<NULL>') AS column_default, extra
		FROM information_schema.columns
		WHERE table_schema = ? AND table_name IN ('island_weather_slot', 'island_calendar_event')
		ORDER BY table_name, ordinal_position`, database).Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	var indexes []barIndexSpec
	if err := db.Raw(`
		SELECT table_name, index_name, non_unique, seq_in_index AS sequence, column_name
		FROM information_schema.statistics
		WHERE table_schema = ? AND table_name IN ('island_weather_slot', 'island_calendar_event')
		ORDER BY table_name, index_name, seq_in_index`, database).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	return columns, indexes
}
