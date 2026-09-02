package model

import (
	"encoding/json"

	"gorm.io/gorm"
)

type IslandWeatherSlot struct {
	Id                       uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	SlotAt                   int64           `gorm:"column:slot_at;type:bigint;not null;uniqueIndex:uk_slot_at;index:idx_source_slot,priority:2" json:"slot_at"`
	ConditionCode            string          `gorm:"column:condition_code;type:varchar(32);not null" json:"condition_code"`
	SeasonCode               string          `gorm:"column:season_code;type:varchar(16);not null" json:"season_code"`
	Cloudiness               float64         `gorm:"column:cloudiness;type:decimal(5,4);not null;default:0.0000" json:"cloudiness"`
	Precipitation            float64         `gorm:"column:precipitation;type:decimal(5,4);not null;default:0.0000" json:"precipitation"`
	PrecipitationProbability float64         `gorm:"column:precipitation_probability;type:decimal(5,4);not null;default:0.0000" json:"precipitation_probability"`
	PrecipitationMmPerHour   float64         `gorm:"column:precipitation_mm_per_hour;type:decimal(7,2);not null;default:0.00" json:"precipitation_mm_per_hour"`
	TemperatureC             float64         `gorm:"column:temperature_c;type:decimal(5,2);not null" json:"temperature_c"`
	Humidity                 float64         `gorm:"column:humidity;type:decimal(5,4);not null;default:0.0000" json:"humidity"`
	VisibilityKm             float64         `gorm:"column:visibility_km;type:decimal(7,2);not null;default:20.00" json:"visibility_km"`
	WindSpeedMps             float64         `gorm:"column:wind_speed_mps;type:decimal(6,2);not null;default:0.00" json:"wind_speed_mps"`
	WindDirectionDeg         uint16          `gorm:"column:wind_direction_deg;type:smallint unsigned;not null;default:0" json:"wind_direction_deg"`
	WaveLevel                float64         `gorm:"column:wave_level;type:decimal(5,4);not null;default:0.0000" json:"wave_level"`
	GenerationContext        json.RawMessage `gorm:"column:generation_context;type:json;default:null" json:"generation_context"`
	Source                   string          `gorm:"column:source;type:varchar(16);not null;default:auto;index:idx_source_slot,priority:1" json:"source"`
	GeneratorVersion         uint            `gorm:"column:generator_version;type:int unsigned;not null;default:1" json:"generator_version"`
	CreatedAt                int64           `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt                int64           `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (IslandWeatherSlot) TableName() string { return "island_weather_slot" }

type IslandCalendarEvent struct {
	Id              uint64          `gorm:"column:id;primaryKey;autoIncrement:true;not null" json:"id"`
	Code            string          `gorm:"column:code;type:varchar(64);not null;uniqueIndex:uk_code_start,priority:1" json:"code"`
	Name            string          `gorm:"column:name;type:varchar(64);not null" json:"name"`
	Description     string          `gorm:"column:description;type:varchar(255);not null;default:''" json:"description"`
	StartsAt        int64           `gorm:"column:starts_at;type:bigint;not null;uniqueIndex:uk_code_start,priority:2;index:idx_status_time,priority:2" json:"starts_at"`
	EndsAt          int64           `gorm:"column:ends_at;type:bigint;not null;index:idx_status_time,priority:3" json:"ends_at"`
	Priority        int32           `gorm:"column:priority;type:int;not null;default:0;index:idx_priority" json:"priority"`
	WeatherMode     string          `gorm:"column:weather_mode;type:varchar(16);not null;default:prefer" json:"weather_mode"`
	WeatherModifier json.RawMessage `gorm:"column:weather_modifier;type:json;default:null" json:"weather_modifier"`
	ThemeCode       string          `gorm:"column:theme_code;type:varchar(64);not null;default:''" json:"theme_code"`
	ContentConfig   json.RawMessage `gorm:"column:content_config;type:json;default:null" json:"content_config"`
	Status          uint8           `gorm:"column:status;type:tinyint unsigned;not null;default:0;index:idx_status_time,priority:1" json:"status"`
	CreatedAt       int64           `gorm:"column:created_at;type:bigint;not null" json:"created_at"`
	UpdatedAt       int64           `gorm:"column:updated_at;type:bigint;not null" json:"updated_at"`
}

func (IslandCalendarEvent) TableName() string { return "island_calendar_event" }

func IslandModels() []interface{} {
	return []interface{}{&IslandWeatherSlot{}, &IslandCalendarEvent{}}
}

func IslandDatabase() *gorm.DB { return db }
