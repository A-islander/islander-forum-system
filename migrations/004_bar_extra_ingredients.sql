-- 海浪之歌：客人自选加料策略。
-- 数量均使用物料自身 unit；库存仍由 bar_ingredient_instance 按 FEFO 扣减。

ALTER TABLE bar_ingredient_type
    ADD COLUMN extra_enabled TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否允许客人在配方外额外选择' AFTER unit,
    ADD COLUMN extra_default_qty DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '前端推荐的单杯加料量' AFTER extra_enabled,
    ADD COLUMN extra_max_qty DECIMAL(10,2) NOT NULL DEFAULT 0.00 COMMENT '单杯该物料允许的最大加料量；0=不可加' AFTER extra_default_qty;

UPDATE bar_ingredient_type SET extra_enabled=1, extra_default_qty=5,  extra_max_qty=15 WHERE id IN (6,7);
UPDATE bar_ingredient_type SET extra_enabled=1, extra_default_qty=10, extra_max_qty=30 WHERE id IN (8,11,12,17);
UPDATE bar_ingredient_type SET extra_enabled=1, extra_default_qty=20, extra_max_qty=60 WHERE id IN (9,10);
UPDATE bar_ingredient_type SET extra_enabled=1, extra_default_qty=15, extra_max_qty=40 WHERE id=13;
UPDATE bar_ingredient_type SET extra_enabled=1, extra_default_qty=1,  extra_max_qty=3  WHERE id=14;
UPDATE bar_ingredient_type SET extra_enabled=1, extra_default_qty=1,  extra_max_qty=2  WHERE id=15;
UPDATE bar_ingredient_type SET extra_enabled=1, extra_default_qty=30, extra_max_qty=100 WHERE id=16;
UPDATE bar_ingredient_type SET extra_enabled=1, extra_default_qty=20, extra_max_qty=50 WHERE id IN (18,19);
