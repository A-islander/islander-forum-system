-- 海浪之歌：用户背包物料批次。
-- 所有用户共用一张表；名称、单位、风味等定义继续引用 bar_ingredient_type。

CREATE TABLE bar_user_ingredient_instance (
    id          BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '用户物料批次 id',
    user_id     BIGINT UNSIGNED NOT NULL COMMENT '所属用户 id',
    type_id     BIGINT UNSIGNED NOT NULL COMMENT '物料种类 id → bar_ingredient_type.id',
    qty_total   DECIMAL(10,2) NOT NULL COMMENT '获得时总数量',
    qty_remain  DECIMAL(10,2) NOT NULL COMMENT '当前剩余数量',
    produced_at BIGINT NOT NULL COMMENT '采集或生产时间（Unix 秒）',
    expire_at   BIGINT NOT NULL COMMENT '过期时间（Unix 秒）',
    attrs       JSON NULL COMMENT '该批次属性，如 freshness / acidity',
    source      VARCHAR(16) NOT NULL DEFAULT 'collect' COMMENT 'collect / gift / reward / return',
    source_id   BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '来源业务记录 id',
    status      TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=可用 1=耗尽 2=过期 3=已上交',
    created_at  BIGINT NOT NULL COMMENT '创建时间（Unix 秒）',
    updated_at  BIGINT NOT NULL COMMENT '更新时间（Unix 秒）',
    PRIMARY KEY (id),
    KEY idx_user_type_status_expire (user_id, type_id, status, expire_at),
    KEY idx_user_status (user_id, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='用户背包物料批次表';
