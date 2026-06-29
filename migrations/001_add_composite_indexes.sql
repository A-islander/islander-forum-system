-- Islander forum-system 复合索引迁移
-- 目的：缓解深翻页（P2）+ 支撑 P1 楼层查询的单次索引扫描。
-- 现状：本项目表结构由 DBA 手动维护（GORM 无 AutoMigrate），故索引以 SQL 脚本管理。
-- 执行：在 forum 库手动执行；索引已存在时各语句幂等（使用非唯一普通索引，重复执行报错可忽略）。
-- 时间：2026-06-29

-- 1) 楼层 / 最后回复列表：GetLastPostList 的 WHERE follow_id IN ? AND status=0 ORDER BY follow_id, time DESC
--    覆盖 P1 改写后的单次扫描路径。
ALTER TABLE forum_post ADD INDEX idx_follow_time_status (follow_id, time, status);

-- 2) 板块首页：GetForumPostIndex 的 plate_id + follow_id=0 + status=0 ORDER BY last_reply_time DESC
ALTER TABLE forum_post ADD INDEX idx_plate_lastreply_status (plate_id, last_reply_time, status);

-- 3) 用户历史发言：GetForumPostListByUid 的 user_id ORDER BY time DESC
ALTER TABLE forum_post ADD INDEX idx_user_time (user_id, time);

-- 4) 时间线主帖：follow_id=0 + status=0 ORDER BY last_reply_time DESC（与 idx_plate_lastreply_status 互补）
--    已被 idx_plate_lastreply_status 部分覆盖，如时间线慢可酌情补：
-- ALTER TABLE forum_post ADD INDEX idx_follow_lastreply_status (follow_id, last_reply_time, status);
