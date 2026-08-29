-- ============================================================================
-- 海浪之家 · 调酒核心系统 DDL（V1）
-- ----------------------------------------------------------------------------
-- 文档配套：
--   设计决策：《海浪之家调酒核心系统.md》
--   端到端案例：《海浪之家一杯酒的诞生.md》（字段取值可对照该案例理解）
--
-- 表清单（10 张）：
--   风味层  bar_flavor_node           风味树（自引用分层 taxonomy）
--           bar_ingredient_flavor     原料种类 → 风味叶子的映射
--   原料层  bar_ingredient_type       原料种类（纯定义层，不存库存）
--           bar_ingredient_instance   原料实例（通用库存单元，万物皆批次）
--           bar_restock_log           补货日志（溯源链的一环）
--           bar_process               加工方案（官方 + 岛民投稿）
--           bar_process_log           加工日志
--   配方层  bar_recipe                酒单/配方（官方 + 岛民投稿）
--           bar_recipe_item           配方组成条目
--   产出层  bar_drink                 产出记录（一杯杯真实调出的酒）
--
-- 数据库约定：
--   - 库：forum 库，表前缀 bar_
--   - 主键：统一自增 id
--   - 时间：统一 BIGINT 存 Unix 时间戳（秒），沿用 forum_post 惯例
--   - JSON 列：需 MySQL >= 5.7.8；若版本更低，将全文 JSON 类型替换为 TEXT 即可
--     （属性匹配与风味计算全部在应用层完成，不依赖 SQL 层 JSON 函数）
--   - 字符集：utf8mb4（故事、留言要支持 emoji）
--
-- 核心设计原则（读表前必看）：
--   1. 配方引用"种类 + 品质区间"，产出引用"实例"
--   2. 库存 = 实例剩余量之和；扣减默认 FEFO，可请求级指定实例，允许跨批次凑单
--   3. 衰变双轨：freshness（连续，影响风味，存 attrs JSON）与 expire_at（死线，独立字段）
--   4. 存输入不存输出：bar_drink 以 inputs_snapshot 为真相，flavor_snapshot 为缓存
--   5. 加行优先于加列：新原料/新风味/新手法都是插数据，不是改表
-- ============================================================================

-- ============================================================================
-- 1. 风味树
--    自引用分层结构。叶子 = 风味的具体人格（咸·海盐 / 咸·酱油麴），
--    中层汇合（咸味），上层汇总（矿物感 → 清新系）。
--    原料只映射到叶子；父节点强度 = 子节点递归求和（V1 纯求和，展示层归一化）。
--    加新风味 = 插节点 + 给原料补映射行，旧数据零迁移。
-- ============================================================================
CREATE TABLE bar_flavor_node (
    id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    parent_id           BIGINT UNSIGNED NULL DEFAULT NULL COMMENT '父节点 id；NULL = 顶级分类（如 清新系）',
    name                VARCHAR(64)     NOT NULL COMMENT '风味名，如：清新系 / 柑橘调 / 柠檬感 / 咸·海盐',
    level               TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '层级冗余（仅作展示排序参考，代码不得依赖具体深度）。树深度不设限：约定原料绑"能准确描述它的最深节点"，节点强度 = 直接贡献 + 子节点递归之和，任意深度自洽',
    description         VARCHAR(512)    NOT NULL DEFAULT '' COMMENT '图鉴文案：这个风味的描述，图鉴页展示用',
    sensitivity_default DECIMAL(3,2)    NOT NULL DEFAULT 0.00 COMMENT '新鲜度敏感度默认值 s∈[-1,1]；>0 怕不新鲜，<0 越陈越香，0 无感。原料映射表可覆盖',
    is_hidden           TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '是否隐藏风味：1=常规原料永远映射不到，只能被仪式等特殊输入点亮（图鉴钩子，V2 用，先留位）',
    sort                INT UNSIGNED    NOT NULL DEFAULT 0 COMMENT '同级排序，图鉴/雷达图展示顺序',
    status              TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=启用 1=停用（停用的节点不参与计算与展示）',
    created_at          BIGINT          NOT NULL COMMENT '创建时间（Unix 秒）',
    updated_at          BIGINT          NOT NULL COMMENT '更新时间（Unix 秒）',
    PRIMARY KEY (id),
    KEY idx_parent (parent_id),
    UNIQUE KEY uk_parent_name (parent_id, name) COMMENT '同一父节点下名称唯一，防止运营手滑建重'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='风味树节点表';

-- ============================================================================
-- 2. 原料种类（纯定义层）
--    只存"这种东西是什么"，不存库存。库存全部由实例表承载。
-- ============================================================================
CREATE TABLE bar_ingredient_type (
    id                  BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    code                VARCHAR(16)     NOT NULL COMMENT '种类短码，用于实例对外编号：LEM / SYR / SLT…（见 bar_ingredient_instance.code）',
    name                VARCHAR(64)     NOT NULL COMMENT '种类名：柠檬 / 蓝柑糖浆 / 海盐…',
    category            VARCHAR(32)     NOT NULL DEFAULT '' COMMENT '分类：base=基酒 juice=果汁 syrup=糖浆 garnish=装饰 spice=香料 craft=工艺品…（字符串便于扩展，不用枚举）',
    mixable             TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '可否用于调酒：1=酒料 0=非食用物料（香囊/风铃/纪念品）。配方与加工 inputs 匹配时按此过滤，防止"往酒里加香囊"',
    unit                VARCHAR(8)      NOT NULL DEFAULT 'ml' COMMENT '计量单位：ml / g / 个。实例 qty 以此为准',
    default_batch_qty   DECIMAL(10,2)   NOT NULL DEFAULT 1.00 COMMENT '默认进货规格：自动补货时一批的量（基酒=500ml/瓶，柠檬=10颗/批），人工/岛民进货可覆盖',
    shelf_life_days     INT UNSIGNED    NOT NULL DEFAULT 30 COMMENT '默认保质期天数：补货时 expire_at = produced_at + 此值，批次可覆盖',
    freshness_decay_per_day DECIMAL(5,2) NOT NULL DEFAULT 0.00 COMMENT '新鲜度每日衰减点数（0~100 制）：0=不衰减（糖浆这类只走保质期轨），>0 走双轨（柠檬这类生鲜）',
    default_attrs       JSON            NULL COMMENT '默认属性模板，如 {"freshness":100,"acidity":70}；补货未显式给属性时按此初始化。平铺数值 JSON，新属性只加 key',
    appearance          JSON            NULL COMMENT '外观贡献：{"color":"#RRGGBB","opacity":0.0~1.0,"gloss":0.0~1.0}。最终酒体颜色/透明度/光泽由各原料按 qty 加权混合；手法再调整质感（摇和浑浊、搅和清澈、分层多层）',
    mouthfeel           JSON            NULL COMMENT '口感维度贡献：{"body":醇厚,"crisp":清爽,"creamy":绵密,"effervescent":气泡刺激,"heat":灼烧感,"astringent":涩感}。最终口感按 qty 加权聚合，手法再做加减',
    codex               TEXT            NULL COMMENT '图鉴文案：这种原料的介绍、产地故事',
    status              TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=启用 1=停用',
    created_at          BIGINT          NOT NULL COMMENT '创建时间（Unix 秒）',
    updated_at          BIGINT          NOT NULL COMMENT '更新时间（Unix 秒）',
    PRIMARY KEY (id),
    UNIQUE KEY uk_code (code),
    UNIQUE KEY uk_name (name)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='物料种类表（定义层，无库存；mixable 区分酒料与非食用物料）';

-- ============================================================================
-- 3. 原料 → 风味叶子映射
--    只绑叶子节点，父级强度由聚合时沿树汇总得出。
--    加新风味维度（如泥煤）：插风味节点 + 给本表补行，然后可重算历史酒。
-- ============================================================================
CREATE TABLE bar_ingredient_flavor (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    type_id         BIGINT UNSIGNED NOT NULL COMMENT '原料种类 id → bar_ingredient_type.id',
    node_id         BIGINT UNSIGNED NOT NULL COMMENT '风味节点 id → bar_flavor_node.id（约定绑能准确描述该原料的最深节点；节点日后长出子层不影响老映射）',
    base_intensity  DECIMAL(4,1)    NOT NULL COMMENT '基础强度 0.0~10.0：该原料满状态（新鲜度100）时对此风味的贡献基数',
    sensitivity     DECIMAL(3,2)    NULL DEFAULT NULL COMMENT '新鲜度敏感度覆盖值；NULL=用节点的 sensitivity_default。例：柠檬的柑橘调 s=0.9，陈皮的柑橘调 s=0',
    created_at      BIGINT          NOT NULL COMMENT '创建时间（Unix 秒）',
    updated_at      BIGINT          NOT NULL COMMENT '更新时间（Unix 秒）',
    PRIMARY KEY (id),
    UNIQUE KEY uk_type_node (type_id, node_id) COMMENT '同一对(原料,叶子)只一条；但一种原料可映射多个叶子（柠檬=柠檬感+酸味+清香），一原料=一束风味贡献',
    KEY idx_node (node_id) COMMENT '反向查"哪些原料带这个风味"（图鉴、推荐用）'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='原料种类-风味叶子映射表';

-- ============================================================================
-- 4. 原料实例（通用库存单元，万物皆批次）
--    一瓶糖浆、一颗悬崖柠檬，都是一行实例。
--    通贩货 = attrs 为空的实例；独特货 = attrs 丰富的实例，同一模型。
-- ============================================================================
CREATE TABLE bar_ingredient_instance (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键（内部引用用）',
    code            VARCHAR(32)     NOT NULL COMMENT '对外编号：{种类短码}-{批次日期}-{批次内序号}，如 LEM-260629-0042。只负责好认，深度溯源走 source 链',
    type_id         BIGINT UNSIGNED NOT NULL COMMENT '原料种类 id → bar_ingredient_type.id',
    qty_total       DECIMAL(10,2)   NOT NULL COMMENT '批次初始量（单位见种类表 unit），叙事/统计用',
    qty_remain      DECIMAL(10,2)   NOT NULL COMMENT '剩余量。调制/加工扣减此字段；归零 → status=1 耗尽',
    produced_at     BIGINT          NOT NULL COMMENT '生产日期（Unix 秒）。叙事用："6月的柠檬"',
    expire_at       BIGINT          NOT NULL COMMENT '过期死线（Unix 秒）= produced_at + 保质期天数。巡检任务据此置 status=2。查询只查此字段，索引友好',
    attrs           JSON            NULL COMMENT '个性属性，平铺数值 JSON：{"freshness":85,"acidity":70}。freshness 随时间衰减（双轨之一）；通贩货可为 NULL。新属性只加 key',
    source          VARCHAR(16)     NOT NULL DEFAULT 'restock' COMMENT '来源类型：restock=补货（含岛民带料） process=加工产出。未来新来源只加取值',
    source_id       BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '来源记录 id：source=restock → bar_restock_log.id；source=process → bar_process_log.id',
    status          TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '0=在库 1=耗尽 2=过期退回（巡检任务置位，保留数据供溯源/审计）',
    created_at      BIGINT          NOT NULL COMMENT '创建时间（Unix 秒）',
    updated_at      BIGINT          NOT NULL COMMENT '更新时间（Unix 秒）',
    PRIMARY KEY (id),
    UNIQUE KEY uk_code (code),
    KEY idx_type_status_expire (type_id, status, expire_at) COMMENT 'FEFO 匹配主索引：按种类筛在库、按过期时间升序取货',
    KEY idx_expire_status (expire_at, status) COMMENT '过期巡检索引：扫 expire_at < now 的在库批次'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='原料实例表（库存单元）';

-- ============================================================================
-- 5. 补货日志（溯源链起点）
--    统一补货入口：系统定时补货与岛民带料上岛都写这里。
--    每次补货创建一批实例，实例 source=restock, source_id=本表 id。
-- ============================================================================
CREATE TABLE bar_restock_log (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    type_id         BIGINT UNSIGNED NOT NULL COMMENT '补的原料种类 → bar_ingredient_type.id',
    quantity        DECIMAL(10,2)   NOT NULL COMMENT '补货量（种类单位）',
    instance_id     BIGINT UNSIGNED NOT NULL COMMENT '本次补货生成的批次实例 → bar_ingredient_instance.id',
    source_type     TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '来源类型：0=岛民娘日常进货（系统固定补给） 1=岛民带料上岛 2=外来商船限定货 3=岛民搜集队采集。后期新来源只加取值',
    source_uid      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '相关人 uid：岛民带料/采集时为用户 id；贸易进货为 0。溯源叙事："这颗柠檬是某某从东边悬崖摘的"',
    attrs           JSON            NULL COMMENT '批次属性快照（写入实例 attrs 的原始值，留档）',
    expire_at       BIGINT          NOT NULL COMMENT '本批次使用的过期死线（默认=当天+种类保质期，此处留档覆盖值）',
    note            VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '备注/叙事文案："东边悬崖晨采"之类',
    created_at      BIGINT          NOT NULL COMMENT '创建时间（Unix 秒）',
    PRIMARY KEY (id),
    KEY idx_type_time (type_id, created_at),
    KEY idx_source_uid (source_type, source_uid) COMMENT '查某岛民带过哪些料（个人图鉴/成就用）'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='补货日志表';

-- ============================================================================
-- 6. 加工方案（官方 + 岛民投稿）
--    加工 = 不进酒杯的转化：消耗输入实例 → 产出新种类实例。
--    与酒单（bar_recipe）同构：creator + 审核状态的 UGC 双来源。
--    附加价值：快过期原料的救赎出口（柠檬 → 耐放的柠檬蜜）。
-- ============================================================================
CREATE TABLE bar_process (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    name            VARCHAR(64)     NOT NULL COMMENT '加工方案名：手榨柠檬汁 / 盐渍柠檬…',
    story           TEXT            NULL COMMENT '方案故事文案（与配方的 story 对齐）',
    creator_id      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创作者 uid；0=官方（岛民娘）',
    inputs          JSON            NOT NULL COMMENT '输入清单：[{"type_id":7,"qty":100,"requirement":{"freshness":[50,100]}}]。结构与 bar_recipe_item 同构（种类+用量+品质区间）',
    output_type_id  BIGINT UNSIGNED NOT NULL COMMENT '产出物种类 → bar_ingredient_type.id',
    output_qty      DECIMAL(10,2)   NOT NULL COMMENT '单次加工产出量（产出物单位）',
    attribute_rule  JSON            NULL COMMENT '输出属性推导规则，如 {"freshness":"min*0.9"}：从输入实例属性推导输出实例属性。V1 支持 min/max/avg × 系数即可',
    shelf_life_days INT UNSIGNED    NULL DEFAULT NULL COMMENT '产出物保质期覆盖值；NULL=用产出种类的默认值',
    status          TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '0=正常 1=审核中 2=屏蔽（官方投稿直接 0；岛民投稿默认 1，审核流同论坛帖子）',
    created_at      BIGINT          NOT NULL COMMENT '创建时间（Unix 秒）',
    updated_at      BIGINT          NOT NULL COMMENT '更新时间（Unix 秒）',
    PRIMARY KEY (id),
    KEY idx_creator_status (creator_id, status),
    KEY idx_output_type (output_type_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='加工方案表';

-- ============================================================================
-- 7. 加工日志
--    每次执行加工写一行；产出实例 source=process, source_id=本表 id。
-- ============================================================================
CREATE TABLE bar_process_log (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    process_id      BIGINT UNSIGNED NOT NULL COMMENT '执行的加工方案 → bar_process.id',
    operator_uid    BIGINT UNSIGNED NOT NULL COMMENT '操作者 uid（岛民自己加工，或系统/岛民娘代加工）',
    inputs_snapshot JSON            NOT NULL COMMENT '实际消耗的输入定格：[{"type_id":7,"portions":[{"instance_id":4201,"qty":100,"freshness":62}]}]。与 bar_drink.inputs_snapshot 同构',
    output_instance_id BIGINT UNSIGNED NOT NULL COMMENT '产出的实例 → bar_ingredient_instance.id',
    created_at      BIGINT          NOT NULL COMMENT '创建时间（Unix 秒）',
    PRIMARY KEY (id),
    KEY idx_process_time (process_id, created_at),
    KEY idx_operator (operator_uid, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='加工日志表';

-- ============================================================================
-- 8. 酒单/配方（官方 + 岛民投稿）
--    配方只描述"要什么样的原料"（种类+份数+品质区间），永不绑定具体实例。
-- ============================================================================
CREATE TABLE bar_recipe (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    name            VARCHAR(64)     NOT NULL COMMENT '酒名：海角黄昏…',
    story           TEXT            NULL COMMENT '配方故事：创作者留下的那句话，酒单页展示',
    creator_id      BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '创作者 uid；0=官方（岛民娘特调）',
    technique       VARCHAR(32)     NOT NULL DEFAULT '' COMMENT '手法文案：摇和/搅和/直调…。V1 纯展示不参与计算；V2 手法表落地后此处迁移为关联（inputs_snapshot 中 kind=technique 已留扩展位）',
    status          TINYINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '0=正常（上酒单） 1=审核中 2=屏蔽。官方直接 0，岛民投稿默认 1',
    order_count     INT UNSIGNED    NOT NULL DEFAULT 0 COMMENT '被点次数（冗余计数，酒单排序/称号用）',
    created_at      BIGINT          NOT NULL COMMENT '创建时间（Unix 秒）',
    updated_at      BIGINT          NOT NULL COMMENT '更新时间（Unix 秒）',
    PRIMARY KEY (id),
    KEY idx_status_order (status, order_count) COMMENT '酒单页：正常状态按热度排序',
    KEY idx_creator (creator_id, status) COMMENT '"我投稿的配方"列表'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='酒单/配方表';

-- ============================================================================
-- 9. 配方组成条目
--    品质区间 requirement：{"freshness":[80,100]}，匹配实例 attrs 中
--    对应属性必须存在且落在闭区间内；NULL = 无要求，是个柠檬就行。
-- ============================================================================
CREATE TABLE bar_recipe_item (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    recipe_id       BIGINT UNSIGNED NOT NULL COMMENT '所属配方 → bar_recipe.id',
    type_id         BIGINT UNSIGNED NOT NULL COMMENT '原料种类 → bar_ingredient_type.id',
    qty             DECIMAL(10,2)   NOT NULL COMMENT '绝对用量（种类单位：ml/g/个），写多少扣多少。风味权重 = 该项 qty / 全配方 Σqty（ml/g 混合求和虽不纯，但液体占大头，实用主义过关）；前端的"2:1:0.5"比例展示由 qty 归一化派生',
    requirement     JSON            NULL COMMENT '品质区间：{"freshness":[80,100],"acidity":[60,100]}；NULL=无要求。新属性维度只加 key',
    step            TINYINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '调制步骤顺序（叙事/动画用："先冰杯，再…"）',
    PRIMARY KEY (id),
    KEY idx_recipe (recipe_id),
    KEY idx_type (type_id) COMMENT '反向查"哪些配方用到这种原料"（缺料影响分析用）'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='配方组成条目表';

-- ============================================================================
-- 10. 产出记录（一杯杯真实调出的酒）
--     存储原则：存输入不存输出。
--       inputs_snapshot = 真相来源（出生证明）
--       flavor_snapshot = 展示缓存（可全量重算刷新）
--       config_version  = 计算规则版本（可复现）
-- ============================================================================
CREATE TABLE bar_drink (
    id              BIGINT UNSIGNED NOT NULL AUTO_INCREMENT COMMENT '主键',
    recipe_id       BIGINT UNSIGNED NOT NULL COMMENT '使用的配方 → bar_recipe.id',
    ordered_by      BIGINT UNSIGNED NOT NULL COMMENT '点单者 uid',
    made_by         BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '调制者：0=岛民娘（NPC）；后期岛民可当调酒师',
    message         VARCHAR(255)    NOT NULL DEFAULT '' COMMENT '点单留言："用我自己的柠檬！"',
    inputs_snapshot JSON            NOT NULL COMMENT '输入快照：[{"kind":"ingredient","type_id":7,"qty":15,"portions":[{"instance_id":4201,"qty":15,"freshness":60,"code":"LEM-260629-0042"}]},{"kind":"technique","name":"摇和"}]。kind 即输入处理器注册表的 key，新输入种类只加 kind 值',
    flavor_snapshot JSON            NULL COMMENT '风味快照（缓存）：{"leaves":{"201":3.53},"rolled":{"1":5.63,"20":5.04}}。leaves=叶子层（标签云），rolled=沿树汇总（雷达图）。新增风味后跑重算脚本刷新',
    appearance_snapshot JSON          NULL COMMENT '外观快照（缓存）：{"color":"#RRGGBB","opacity":0.5,"texture":"cloudy","layers":1,"garnish":null}。LLM 描述外形和 UI 渲染都用它，重算逻辑与风味重算同步',
    mouthfeel_snapshot  JSON          NULL COMMENT '口感快照（缓存）：{"base":{"body":0.3,"crisp":0.1,...},"texture":"cloudy","dominant":["body","heat"]}。LLM 描述口感用',
    description     TEXT            NULL COMMENT '本杯实况描述（上酒解说文案）：优先 LLM 生成（风味档案+外观+口感+溯源+偶然事件 → 岛民娘口吻，出品时生成一次永久缓存，调制动画掩盖延迟）；LLM 超时/失败降级为规则分档模板拼装。与配方的静态 story 互补：story 是灵魂，description 是实况',
    config_version  INT UNSIGNED    NOT NULL DEFAULT 1 COMMENT '计算规则版本号：新鲜度公式/腐坏阈值/汇总方式变更时 +1，保证每杯酒的算法可复现',
    created_at      BIGINT          NOT NULL COMMENT '出品时间（Unix 秒）',
    PRIMARY KEY (id),
    KEY idx_recipe_time (recipe_id, created_at) COMMENT '"这款酒最近的出品"列表',
    KEY idx_ordered_by (ordered_by, created_at) COMMENT '"我点过的酒"（个人酒单/图鉴）'
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='产出记录表';

-- ============================================================================
-- 附：V1 明确不建、但已在数据格式中留位的扩展
--   - 手法表 / 熟练度表 / 仪式表 → inputs_snapshot 的 kind 与处理器注册表
--   - 风味相性（协同/压制）       → flavor_snapshot 结构不变，计算层扩展
--   - 岛民娘情景文案表            → 可先放配置，量级上来再入库
-- ============================================================================
