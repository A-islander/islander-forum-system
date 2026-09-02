-- 岛民岛：全岛统一天气时间线与节日配置。
-- 时间统一使用 Unix 秒；slot_at 按整点对齐。天气是全岛能力，不使用 bar_ 前缀。

CREATE TABLE island_weather_slot (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    slot_at BIGINT NOT NULL COMMENT '小时锚点（Unix 秒）',
    condition_code VARCHAR(32) NOT NULL COMMENT 'clear/partly_cloudy/cloudy/light_rain/heavy_rain/storm/fog',
    season_code VARCHAR(16) NOT NULL COMMENT 'spring/summer/autumn/winter',
    cloudiness DECIMAL(5,4) NOT NULL DEFAULT 0.0000,
    precipitation DECIMAL(5,4) NOT NULL DEFAULT 0.0000,
    precipitation_probability DECIMAL(5,4) NOT NULL DEFAULT 0.0000,
    precipitation_mm_per_hour DECIMAL(7,2) NOT NULL DEFAULT 0.00,
    temperature_c DECIMAL(5,2) NOT NULL,
    humidity DECIMAL(5,4) NOT NULL DEFAULT 0.0000,
    visibility_km DECIMAL(7,2) NOT NULL DEFAULT 20.00,
    wind_speed_mps DECIMAL(6,2) NOT NULL DEFAULT 0.00,
    wind_direction_deg SMALLINT UNSIGNED NOT NULL DEFAULT 0,
    wave_level DECIMAL(5,4) NOT NULL DEFAULT 0.0000,
    generation_context JSON NULL COMMENT '季节混合、节日与生成参数快照',
    source VARCHAR(16) NOT NULL DEFAULT 'auto' COMMENT 'auto/manual',
    generator_version INT UNSIGNED NOT NULL DEFAULT 1,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_slot_at (slot_at),
    KEY idx_source_slot (source, slot_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='岛民岛全局小时天气时间线';

CREATE TABLE island_calendar_event (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code VARCHAR(64) NOT NULL,
    name VARCHAR(64) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    starts_at BIGINT NOT NULL,
    ends_at BIGINT NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    weather_mode VARCHAR(16) NOT NULL DEFAULT 'prefer' COMMENT 'prefer/restrict/force',
    weather_modifier JSON NULL COMMENT '天气权重、温度偏移和天气限制',
    theme_code VARCHAR(64) NOT NULL DEFAULT '',
    content_config JSON NULL COMMENT '装饰、环境音和角色台词配置',
    status TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=启用 1=停用',
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_code_start (code, starts_at),
    KEY idx_status_time (status, starts_at, ends_at),
    KEY idx_priority (priority)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='岛民岛节日与特殊日期配置';
