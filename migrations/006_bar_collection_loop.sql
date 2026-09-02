-- 海浪之歌：随机搜集、每日次数、地点掉落规则与定制酒名。

CREATE TABLE bar_collect_location (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code VARCHAR(32) NOT NULL,
    name VARCHAR(64) NOT NULL,
    description VARCHAR(255) NOT NULL DEFAULT '',
    weight INT UNSIGNED NOT NULL DEFAULT 1,
    status TINYINT UNSIGNED NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (id), UNIQUE KEY uk_code (code), UNIQUE KEY uk_name (name), KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='岛内搜集地点';

CREATE TABLE bar_collect_loot (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    location_id BIGINT UNSIGNED NOT NULL,
    type_id BIGINT UNSIGNED NOT NULL,
    weight INT UNSIGNED NOT NULL DEFAULT 1,
    min_qty DECIMAL(10,2) NOT NULL,
    max_qty DECIMAL(10,2) NOT NULL,
    attrs_rule JSON NULL,
    shelf_life_days INT UNSIGNED NULL DEFAULT NULL,
    status TINYINT UNSIGNED NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_location_type (location_id, type_id),
    KEY idx_location_status (location_id, status), KEY idx_type (type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='地点可产出物料及属性范围';

CREATE TABLE bar_collect_daily (
    user_id BIGINT UNSIGNED NOT NULL,
    day_key INT UNSIGNED NOT NULL COMMENT 'Asia/Shanghai 日期 YYYYMMDD',
    used_count TINYINT UNSIGNED NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    updated_at BIGINT NOT NULL,
    PRIMARY KEY (user_id, day_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户每日搜集计数';

CREATE TABLE bar_collect_log (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    day_key INT UNSIGNED NOT NULL,
    daily_seq TINYINT UNSIGNED NOT NULL,
    location_id BIGINT UNSIGNED NOT NULL,
    type_id BIGINT UNSIGNED NOT NULL,
    quantity DECIMAL(10,2) NOT NULL,
    attrs JSON NULL,
    instance_id BIGINT UNSIGNED NOT NULL DEFAULT 0,
    created_at BIGINT NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_day_seq (user_id, day_key, daily_seq),
    KEY idx_user_time (user_id, created_at),
    KEY idx_location_time (location_id, created_at),
    KEY idx_type_time (type_id, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户搜集结果日志';

ALTER TABLE bar_drink ADD COLUMN name VARCHAR(128) NOT NULL DEFAULT '' AFTER made_by;

INSERT INTO bar_collect_location (id,code,name,description,weight,status,created_at,updated_at) VALUES
(1,'east_cliff','东边悬崖','海风最先抵达的石壁，酸香植物长在岩缝里。',40,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()),
(2,'island_greenhouse','岛心温室','岛民们照料热带果实的暖棚。',40,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()),
(3,'tidal_salt_flat','潮汐盐滩','退潮后留下会发亮的细盐。',20,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP());

INSERT INTO bar_collect_loot (location_id,type_id,weight,min_qty,max_qty,attrs_rule,shelf_life_days,status,created_at,updated_at) VALUES
(1,11,45,50,100,JSON_OBJECT('freshness',JSON_ARRAY(90,100),'acidity',JSON_ARRAY(75,90),'aroma',JSON_ARRAY(60,80)),NULL,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()),
(1,12,35,40,80, JSON_OBJECT('freshness',JSON_ARRAY(88,100),'acidity',JSON_ARRAY(80,95),'aroma',JSON_ARRAY(65,85)),NULL,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()),
(1,14,20,1,3,   JSON_OBJECT('freshness',JSON_ARRAY(85,100),'coolness',JSON_ARRAY(75,95)),NULL,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()),
(2,13,35,10,30, JSON_OBJECT('freshness',JSON_ARRAY(82,98),'acidity',JSON_ARRAY(50,70),'aroma',JSON_ARRAY(75,95)),NULL,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()),
(2,18,35,40,100,JSON_OBJECT('freshness',JSON_ARRAY(85,100),'sweetness',JSON_ARRAY(65,85),'acidity',JSON_ARRAY(25,45)),NULL,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()),
(2,19,30,30,80, JSON_OBJECT('freshness',JSON_ARRAY(80,98),'tartness',JSON_ARRAY(70,90),'acidity',JSON_ARRAY(50,75)),NULL,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP()),
(3,15,100,2,8,  JSON_OBJECT('salinity',JSON_ARRAY(80,100)),NULL,0,UNIX_TIMESTAMP(),UNIX_TIMESTAMP());
