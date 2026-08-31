package model

import (
	"fmt"
	"os"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type barColumnSpec struct {
	TableName     string
	ColumnName    string
	Ordinal       int
	ColumnType    string
	IsNullable    string
	ColumnDefault string
	Extra         string
}

type barIndexSpec struct {
	TableName  string
	IndexName  string
	NonUnique  int
	Sequence   int
	ColumnName string
}

func TestBarModelSchemaMatchesDDL(t *testing.T) {
	user := os.Getenv("ISLANDER_DB_USER")
	password := os.Getenv("ISLANDER_DB_PASSWORD")
	address := os.Getenv("ISLANDER_DB_ADDR")
	if user == "" || address == "" {
		t.Skip("database environment is not configured")
	}

	suffix := fmt.Sprintf("%d", time.Now().UnixNano())
	autoDatabase := "bar_model_auto_" + suffix
	ddlDatabase := "bar_model_ddl_" + suffix
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
	if err := autoDB.Set("gorm:table_options", "ENGINE=InnoDB DEFAULT CHARSET=utf8mb4").AutoMigrate(BarModels()...); err != nil {
		t.Fatal(err)
	}

	coreDDL, err := os.ReadFile("../migrations/002_bar_core.sql")
	if err != nil {
		t.Fatal(err)
	}
	maintenanceDDL, err := os.ReadFile("../migrations/003_bar_stock_maintenance.sql")
	if err != nil {
		t.Fatal(err)
	}
	extraDDL, err := os.ReadFile("../migrations/004_bar_extra_ingredients.sql")
	if err != nil {
		t.Fatal(err)
	}
	backpackDDL, err := os.ReadFile("../migrations/005_bar_user_backpack.sql")
	if err != nil {
		t.Fatal(err)
	}
	collectionDDL, err := os.ReadFile("../migrations/006_bar_collection_loop.sql")
	if err != nil {
		t.Fatal(err)
	}
	ddlDB := openBarSchemaDatabase(t, user, password, address, ddlDatabase)
	if err := ddlDB.Exec(string(coreDDL) + "\n" + string(maintenanceDDL) + "\n" + string(extraDDL) + "\n" + string(backpackDDL) + "\n" + string(collectionDDL)).Error; err != nil {
		t.Fatal(err)
	}

	autoColumns, autoIndexes := readBarSchema(t, root, autoDatabase)
	ddlColumns, ddlIndexes := readBarSchema(t, root, ddlDatabase)
	if len(tableNames(autoColumns)) != 16 {
		t.Fatalf("AutoMigrate created %d bar tables, want 16", len(tableNames(autoColumns)))
	}
	if !reflect.DeepEqual(autoColumns, ddlColumns) {
		t.Fatalf("AutoMigrate columns differ from DDL: %s", firstSchemaDifference(autoColumns, ddlColumns))
	}
	if !reflect.DeepEqual(autoIndexes, ddlIndexes) {
		t.Fatalf("AutoMigrate indexes differ from DDL: %s", firstSchemaDifference(autoIndexes, ddlIndexes))
	}
}

func firstSchemaDifference(left, right interface{}) string {
	leftValue := reflect.ValueOf(left)
	rightValue := reflect.ValueOf(right)
	limit := leftValue.Len()
	if rightValue.Len() < limit {
		limit = rightValue.Len()
	}
	for index := 0; index < limit; index++ {
		if !reflect.DeepEqual(leftValue.Index(index).Interface(), rightValue.Index(index).Interface()) {
			return fmt.Sprintf("at %d: auto=%+v ddl=%+v", index, leftValue.Index(index).Interface(), rightValue.Index(index).Interface())
		}
	}
	return fmt.Sprintf("length: auto=%d ddl=%d", leftValue.Len(), rightValue.Len())
}

func openBarSchemaDatabase(t *testing.T, user, password, address, database string) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?charset=utf8mb4&multiStatements=true", user, password, address, database)
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatal(err)
	}
	return db
}

func readBarSchema(t *testing.T, db *gorm.DB, database string) ([]barColumnSpec, []barIndexSpec) {
	t.Helper()
	var columns []barColumnSpec
	if err := db.Raw(`
		SELECT table_name, column_name, ordinal_position AS ordinal, column_type,
		       is_nullable, COALESCE(column_default, '<NULL>') AS column_default, extra
		FROM information_schema.columns
		WHERE table_schema = ? AND LEFT(table_name, 4) = 'bar_'
		ORDER BY table_name, ordinal_position`, database).Scan(&columns).Error; err != nil {
		t.Fatal(err)
	}
	var indexes []barIndexSpec
	if err := db.Raw(`
		SELECT table_name, index_name, non_unique, seq_in_index AS sequence, column_name
		FROM information_schema.statistics
		WHERE table_schema = ? AND LEFT(table_name, 4) = 'bar_'
		ORDER BY table_name, index_name, seq_in_index`, database).Scan(&indexes).Error; err != nil {
		t.Fatal(err)
	}
	return columns, indexes
}

func tableNames(columns []barColumnSpec) []string {
	seen := make(map[string]bool)
	for _, column := range columns {
		seen[column.TableName] = true
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		if strings.HasPrefix(name, "bar_") {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}
