-- 海浪之家 V1：自动库存维护策略。
-- 实际库存仍以 bar_ingredient_instance 为准，本表只保存补充方式和水位。

CREATE TABLE bar_stock_policy (
    type_id                    BIGINT UNSIGNED NOT NULL COMMENT '原料种类 id → bar_ingredient_type.id',
    min_qty                    DECIMAL(10,2) NOT NULL COMMENT '有效库存低于该值时触发补充',
    max_qty                    DECIMAL(10,2) NOT NULL COMMENT '自动补充的库存上限',
    replenish_mode             VARCHAR(16) NOT NULL DEFAULT 'restock' COMMENT 'restock=直接补货 process=加工产生 none=不自动补充',
    process_id                 BIGINT UNSIGNED NULL DEFAULT NULL COMMENT 'process 模式使用的官方加工方案 id',
    retire_freshness_below     DECIMAL(5,2) NULL DEFAULT NULL COMMENT '低于该有效新鲜度时退回；NULL=不检查',
    enabled                    TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '0=停用 1=启用',
    created_at                 BIGINT NOT NULL COMMENT '创建时间（Unix 秒）',
    updated_at                 BIGINT NOT NULL COMMENT '更新时间（Unix 秒）',
    PRIMARY KEY (type_id),
    KEY idx_enabled (enabled),
    KEY idx_process (process_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='原料自动维护策略';

INSERT INTO bar_stock_policy
    (type_id, min_qty, max_qty, replenish_mode, process_id, retire_freshness_below, enabled, created_at, updated_at)
VALUES
    ( 1, 100,  500, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    ( 2, 100,  500, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    ( 3, 100,  500, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    ( 4, 100,  500, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    ( 5, 100,  500, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    ( 6, 100,  500, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    ( 7,  60,  300, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    ( 8,  80,  400, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    ( 9, 180, 1000, 'process',     2,   30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (10, 200, 1000, 'process',     3,   30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (11, 250, 1000, 'restock', NULL,   30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (12, 200,  800, 'restock', NULL,   30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (13, 150,  600, 'restock', NULL,   30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (14,  12,   50, 'restock', NULL,   30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (15,  40,  200, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (16, 100,  330, 'restock', NULL, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (17,  30,  240, 'process',     1,   20, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (18, 250, 1000, 'restock', NULL,   30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (19, 200,  800, 'restock', NULL,   30, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP()),
    (20, 190,  760, 'process',     4, NULL, 1, UNIX_TIMESTAMP(), UNIX_TIMESTAMP());
