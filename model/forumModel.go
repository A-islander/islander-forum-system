package model

import (
	"encoding/json"
	"strconv"

	"gorm.io/gorm"
)

type ForumPost struct {
	Id            int    `json:"id"`
	Title         string `json:"title"`
	Value         string `json:"value"`
	FollowId      int    `json:"followId"`
	PlateId       int    `json:"plateId"`
	Status        int    `json:"status"`
	ReplyArr      string `json:"replyArr"`
	UserId        int    `json:"userId"`
	Time          int    `json:"time"`
	MediaUrl      string `json:"mediaUrl"`
	ReplyCount    int    `json:"replyCount"`
	TopStatus     int    `json:"topStatus"`
	LastReplyTime int    `json:"listReplayTime"`
	SageAddId     string `json:"sageAddId"`
	SageSubId     string `json:"sageSubId"`
	Name          string `json:"name"`
}

type ForumPlate struct {
	Id     int
	Name   string
	Status int
	Value  string
}

// 获取该条post在整个串中的位置
type ForumPostId struct {
	Id int `json:"id"`
}

func GetForumPlate() ([]ForumPlate, error) {
	var res []ForumPlate
	err := db.Where("status = ?", 0).Find(&res).Error
	return res, err
}

func GetForumIndexLastTime(page, size int, notInArr []int) ([]ForumPost, int) {
	first := page * size
	var res []ForumPost
	var count int64
	if len(notInArr) > 0 {
		db.Limit(size).Offset(first).Where("follow_id = 0 and status = 0 and plate_id not in (?)", notInArr).Order("last_reply_time desc").Find(&res).Limit(-1).Offset(-1).Count(&count)
	} else {
		db.Limit(size).Offset(first).Where("follow_id = 0 and status = 0").Order("last_reply_time desc").Find(&res).Limit(-1).Offset(-1).Count(&count)
	}
	return res, int(count)
}

func GetForumPost(postId int) (ForumPost, error) {
	var res ForumPost
	err := db.Where("id = ?", postId).Take(&res).Error
	return res, err
}

func GetForumPostIndexBuff(plateId int, page, size int) ([]ForumPost, int) {
	// forumServer:postIndex:plateId
	key := "fS:pI:" + strconv.Itoa(plateId)
	// forumServer:postIndexCount:plateId
	countKey := "fS:pIC:" + strconv.Itoa(plateId)
	first := page * size
	end := first + size - 1
	var res []ForumPost
	var count int
	if checkKey(key) { // 读时更新
		if end < getZsetCount(key) { // 超过指定缓存
			buffRes := getZsetArr(key, int64(first), int64(end))
			res = tranPost(buffRes)
			if checkKey(countKey) {
				count = getCount(countKey)
			} else {
				count = getForumPostIndexCount(plateId)
				setCount(countKey, count)
			}
		} else {
			res, count = GetForumPostIndex(plateId, page, size)
		}
	} else { // 读入前一百缓存
		res, count = GetForumPostIndex(plateId, 0, 100)
		setCount(countKey, count)
		initForumIndexBuff(res)
		res, count = GetForumPostIndex(plateId, page, size)
	}
	return res, count
}

// 获取帖子最后回复
func GetLastPostListBuff(postIdArr []int, count int) []ForumPost {
	var res []ForumPost
	var missKey []int
	for i := 0; i < len(postIdArr); i++ {
		// forumServer:postLastReply:postId
		key := "fS:pLR:" + strconv.Itoa(postIdArr[i])
		if checkKey(key) {
			// fmt.Println("succ", postIdArr[i])
			buffRes := getZsetArr(key, int64(0), int64(count-1))
			res = append(res, tranPost(buffRes)...)
		} else {
			// fmt.Println("miss", postIdArr[i])
			missKey = append(missKey, postIdArr[i])
		}
	}
	if missKey != nil { // 读时更新
		updateRes := GetLastPostList(missKey, count)
		initLastPostBuff(updateRes)
		res = append(res, updateRes...)
	}
	return res
}

// 增加buff版本
func GetForumPostIndex(plateId int, page int, size int) ([]ForumPost, int) {
	first := page * size
	var res []ForumPost
	var count int64
	db.Limit(size).Offset(first).Where("plate_id = ? and follow_id = 0 and status = 0", plateId).Order("last_reply_time desc").Find(&res).Limit(-1).Offset(-1).Count(&count)
	return res, int(count)
}

func getForumPostIndexCount(plateId int) int {
	var count int64
	db.Model(&ForumPost{}).Where("plate_id = ? and follow_id = 0 and status = 0", plateId).Count(&count)
	return int(count)
}

func GetForumPostList(postId int, page int, size int) ([]ForumPost, int) {
	first := page * size
	var res []ForumPost
	var count int64
	db.Limit(size).Offset(first).Where("(follow_id = ? and status = 0) or id = ?", postId, postId).Order("time asc, id asc").Find(&res).Limit(-1).Offset(-1).Count(&count)
	return res, int(count)
}

func GetForumPostCount(postId int) (int, error) {
	var count int64
	err := db.Model(&ForumPost{}).Where("(follow_id = ? and status = 0) or id = ?", postId, postId).Count(&count).Error
	return int(count), err
}

// GetForumPostFloor 计算 replyId 这条回复在其所属主帖 postId 的楼层号（1-based）。
// 口径必须与 GetForumPostList 逐字一致：(follow_id=postId AND status=0) OR id=postId，按 (time asc, id asc) 排序。
// floor = 排在它前面的条数 + 1。用 (time,id) 二元组比较，消除同秒并列时的顺序歧义：
//
//	排在前 = time 更早，或 time 相同但 id 更小。
//
// 这样算出的页号与 list 接口实际分页位置完全一致。
// 先 Take 校验目标回复属于该帖且 status=0（被 sage/自删/不属于 → record not found 错误，触发前端降级）。
func GetForumPostFloor(postId, replyId int) (int, error) {
	var reply ForumPost
	err := db.Where("id = ? and follow_id = ? and status = 0", replyId, postId).Take(&reply).Error
	if err != nil {
		return 0, err
	}
	var floor int64
	db.Model(&ForumPost{}).
		Where("((follow_id = ? and status = 0) or id = ?) and (time < ? or (time = ? and id < ?))",
			postId, postId, reply.Time, reply.Time, reply.Id).
		Count(&floor)
	return int(floor) + 1, nil
}

// TODO 获取单个帖子
func GetForumPostByPostId(postId int) (ForumPost, error) {
	var res ForumPost
	err := db.Where("id = ?", postId).Take(&res).Error
	return res, err
}

func getForumPostListCount(postId int) int {
	var count int64
	db.Where("(follow_id = ? and status = 0) or id = ?", postId, postId).Count(&count)
	return int(count)
}

// 根据用户uid获取过往发言
func GetForumPostListByUid(uid int, page, size int) ([]ForumPost, int) {
	first := page * size
	var res []ForumPost
	var count int64
	db.Limit(size).Offset(first).Where("user_id = ?", uid).Order("time desc").Find(&res).Limit(-1).Offset(-1).Count(&count)
	return res, int(count)
}

// 增加buff版本
// GetLastPostList 取每个 followId 串内最新的 count 条回复（按 time 倒序的前 count 条）。
//
// 旧实现用相关子查询算楼层 top（O(N²)，外层每条都跑内层 count），热帖首页直接爆炸。
// 现改为：一次查询取出这些串的全部正常回复（ORDER BY follow_id, time DESC 走索引），
// 应用层按 follow_id 分组各截取前 count 条 —— 单次扫描 O(N)，且结果顺序与旧实现等价。
// 配合 GetLastPostListBuff 缓存，命中缓存时连这次查询都省了。
func GetLastPostList(followIdArr []int, count int) []ForumPost {
	if len(followIdArr) == 0 || count <= 0 {
		return []ForumPost{}
	}
	var all []ForumPost
	db.Where("follow_id IN ? AND status = 0", followIdArr).Order("follow_id, time DESC").Find(&all)

	// 按 follow_id 分组，每组只保留最新的 count 条；整体保持按 follow_id 升序、组内 time DESC。
	res := make([]ForumPost, 0, len(all))
	curId := 0
	curCnt := 0
	first := true
	for i := 0; i < len(all); i++ {
		p := all[i]
		if first || p.FollowId != curId {
			curId = p.FollowId
			curCnt = 0
			first = false
		}
		if curCnt < count {
			res = append(res, p)
			curCnt++
		}
	}
	return res
}

func GetAlreadySagePost(page int, size int) ([]ForumPost, int) {
	first := page * size
	var res []ForumPost
	var count int64
	db.Limit(size).Offset(first).Where("status = 1").Order("time desc").Find(&res).Limit(-1).Offset(-1).Count(&count)
	return res, int(count)
}

// 新增buff版本，删除首页缓存
func SaveForumPost(post ForumPost) int {
	db.Create(&post)
	// 存入数据库后删除缓存
	indexkey := "fS:pI:" + strconv.Itoa(post.PlateId)
	delKey(indexkey)
	return post.Id
}

// 新增buff版本，删除最晚回复缓存，更新或删除首页缓存
func SaveForumReply(post ForumPost) int {
	db.Create(&post)
	// 存入数据库后删除缓存
	postKey := "fS:pLR:" + strconv.Itoa(post.FollowId)
	delKey(postKey)
	return post.Id
}

// 更新帖子数据
func UpdateForumPostCount(postId int, time int) {
	db.Model(&ForumPost{}).Where("id = ?", postId).Updates(ForumPost{LastReplyTime: time})
	db.Model(&ForumPost{}).Where("id = ?", postId).UpdateColumn("reply_count", gorm.Expr("reply_count + ?", 1))
	// 存入数据库后删除缓存，可以升级为更新
	var post ForumPost
	db.Where("id = ?", postId).Find(&post)
	indexKey := "fS:pI:" + strconv.Itoa(post.PlateId)
	countKey := "fS:pIC:" + strconv.Itoa(post.PlateId)
	// setForumIndexBuff(post)
	delKey(indexKey)
	delKey(countKey)
}

// UpdateSageAdd / UpdateSageSub 保留兼容（非原子，仅用于无并发场景）。
func UpdateSageAdd(post ForumPost) {
	db.Model(&post).Update("sage_add_id", post.SageAddId)
	delSageIndexBuff(post.PlateId)
}

func UpdateSageSub(post ForumPost) {
	db.Model(&post).Update("sage_sub_id", post.SageSubId)
	delSageIndexBuff(post.PlateId)
}

// delSageIndexBuff 清除板块首页缓存（sage 变动影响排序）。
func delSageIndexBuff(plateId int) {
	indexKey := "fS:pI:" + strconv.Itoa(plateId)
	delKey(indexKey)
}

// SageAddPostUser 原子地把 userId 加入 sageAdd 阵营，并从 sageSub 阵营移除（互斥）。
// 返回更新后的 sageAddId / sageSubId 数组（供上层判断是否触发 SageSet）。
// 利用 MySQL JSON 函数在单条 UPDATE 内完成，消除 read-modify-write 竞态：
//   - CAST(? AS JSON) 保证 userId 作为 JSON 数字参与比对/插入，类型一致；
//   - JSON_CONTAINS 判存在，NOT 取反 → 天然防重复追加；
//   - 两阵营互斥移除用 JSON_REMOVE + JSON_UNQUOTE(JSON_SEARCH) 定位。
func SageAddPostUser(postId, userId int) (addArr, subArr []int) {
	jv := jsonValue(userId)
	// 1) 互斥：先从 sub 阵营移除（若存在）
	db.Model(&ForumPost{}).Where("id = ? AND JSON_CONTAINS(sage_sub_id, CAST(? AS JSON))", postId, jv).
		Update("sage_sub_id", gorm.Expr("JSON_REMOVE(sage_sub_id, JSON_UNQUOTE(JSON_SEARCH(sage_sub_id, 'one', CAST(? AS JSON))))", jv))
	// 2) 加入 add 阵营（仅当不存在时）
	db.Model(&ForumPost{}).Where("id = ? AND NOT JSON_CONTAINS(sage_add_id, CAST(? AS JSON))", postId, jv).
		Update("sage_add_id", gorm.Expr("JSON_ARRAY_APPEND(sage_add_id, '$', CAST(? AS JSON))", jv))
	return readSageArr(postId)
}

// SageRemovePostUser 原子地把 userId 从 sageAdd 阵营移除（取消赞）。
func SageRemovePostUser(postId, userId int) (addArr, subArr []int) {
	jv := jsonValue(userId)
	db.Model(&ForumPost{}).Where("id = ? AND JSON_CONTAINS(sage_add_id, CAST(? AS JSON))", postId, jv).
		Update("sage_add_id", gorm.Expr("JSON_REMOVE(sage_add_id, JSON_UNQUOTE(JSON_SEARCH(sage_add_id, 'one', CAST(? AS JSON))))", jv))
	return readSageArr(postId)
}

// SageSubPostUser 原子地把 userId 加入 sageSub 阵营，并从 sageAdd 阵营移除（互斥）。
func SageSubPostUser(postId, userId int) (addArr, subArr []int) {
	jv := jsonValue(userId)
	// 1) 互斥：先从 add 阵营移除（若存在）
	db.Model(&ForumPost{}).Where("id = ? AND JSON_CONTAINS(sage_add_id, CAST(? AS JSON))", postId, jv).
		Update("sage_add_id", gorm.Expr("JSON_REMOVE(sage_add_id, JSON_UNQUOTE(JSON_SEARCH(sage_add_id, 'one', CAST(? AS JSON))))", jv))
	// 2) 加入 sub 阵营（仅当不存在时）
	db.Model(&ForumPost{}).Where("id = ? AND NOT JSON_CONTAINS(sage_sub_id, CAST(? AS JSON))", postId, jv).
		Update("sage_sub_id", gorm.Expr("JSON_ARRAY_APPEND(sage_sub_id, '$', CAST(? AS JSON))", jv))
	return readSageArr(postId)
}

// SageRemoveSubUser 原子地把 userId 从 sageSub 阵营移除（取消踩）。
func SageRemoveSubUser(postId, userId int) (addArr, subArr []int) {
	jv := jsonValue(userId)
	db.Model(&ForumPost{}).Where("id = ? AND JSON_CONTAINS(sage_sub_id, CAST(? AS JSON))", postId, jv).
		Update("sage_sub_id", gorm.Expr("JSON_REMOVE(sage_sub_id, JSON_UNQUOTE(JSON_SEARCH(sage_sub_id, 'one', CAST(? AS JSON))))", jv))
	return readSageArr(postId)
}

// jsonValue 把 int 转成十进制字符串；SQL 侧用 CAST(? AS JSON) 转为 JSON 数字文档，
// 保证 JSON_CONTAINS / JSON_ARRAY_APPEND 与数组元素（数字）类型一致。
func jsonValue(v int) string {
	return strconv.Itoa(v)
}

// json2intArr 把 JSON 数组字符串解析为 []int，解析失败返回空切片（稳健，不 panic）。
func json2intArr(data string) []int {
	res := make([]int, 0)
	if data == "" {
		return res
	}
	err := json.Unmarshal([]byte(data), &res)
	if err != nil {
		return make([]int, 0)
	}
	return res
}

// readSageArr 读取更新后的两阵营数组，供上层判断是否触发 sage 状态。
func readSageArr(postId int) (addArr, subArr []int) {
	var post ForumPost
	db.Select("sage_add_id", "sage_sub_id", "plate_id").Where("id = ?", postId).Take(&post)
	addArr = json2intArr(post.SageAddId)
	subArr = json2intArr(post.SageSubId)
	delSageIndexBuff(post.PlateId)
	return
}

func UpdateForumPostStatus(post ForumPost, status int) {
	db.Model(&post).Update("status", status)
	// 更新缓存
	indexKey := "fS:pI:" + strconv.Itoa(post.PlateId)
	countKey := "fS:pIC:" + strconv.Itoa(post.PlateId)
	postKey := "fS:pLR:" + strconv.Itoa(post.FollowId)
	// setForumIndexBuff(post)
	delKey(indexKey)
	delKey(countKey)
	delKey(postKey)
}

// UpdateForumPostStatusById 按帖子 id 更新状态，并自行补查 plate_id/follow_id 以清缓存。
// 供原子 sage 投票后触发隐藏使用（此时调用方只有 postId）。
func UpdateForumPostStatusById(postId, status int) {
	db.Model(&ForumPost{}).Where("id = ?", postId).Update("status", status)
	var post ForumPost
	db.Select("plate_id", "follow_id").Where("id = ?", postId).Take(&post)
	indexKey := "fS:pI:" + strconv.Itoa(post.PlateId)
	countKey := "fS:pIC:" + strconv.Itoa(post.PlateId)
	postKey := "fS:pLR:" + strconv.Itoa(post.FollowId)
	delKey(indexKey)
	delKey(countKey)
	delKey(postKey)
}

func initForumIndexBuff(postArr []ForumPost) {
	for i := 0; i < len(postArr); i++ {
		setForumIndexBuff(postArr[i])
	}
}

func initForumReplyBuff(postArr []ForumPost) {
	for i := 0; i < len(postArr); i++ {
		setForumReplyBuff(postArr[i])
	}
}

func initLastPostBuff(postArr []ForumPost) {
	for i := 0; i < len(postArr); i++ {
		setLastReplyBuff(postArr[i])
	}
}

// 设置首页缓存
func setForumIndexBuff(post ForumPost) {
	// rdb := newRdb()
	score := post.LastReplyTime
	// post.LastReplyTime = 0
	key := "fS:pI:" + strconv.Itoa(post.PlateId)
	addZsetBuff(key, score, post)
	rdb.Expire(ctx, key, buffTime)
}

// 设置回复缓存
func setForumReplyBuff(post ForumPost) {
	// rdb := newRdb()
	key := "fS:pR:" + strconv.Itoa(post.FollowId)
	addZsetBuff(key, post.Time, post)
	rdb.Expire(ctx, key, buffTime)
}

// 设置最晚回复缓存
func setLastReplyBuff(post ForumPost) {
	// rdb := newRdb()
	key := "fS:pLR:" + strconv.Itoa(post.FollowId)
	addZsetBuff(key, post.Time, post)
	rdb.Expire(ctx, key, buffTime)
}

// 更改主贴板块
func ChangePostPlate(postId int, plateId int) {
	db.Model(&ForumPost{}).Where("id = ?", postId).Updates(ForumPost{PlateId: plateId})
}

// 更改从贴板块
func ChangeFollowPostPlate(followId int, plateId int) {
	db.Model(&ForumPost{}).Where("follow_id = ?", followId).Updates(ForumPost{PlateId: plateId})
}

func (ForumPost) TableName() string {
	return "forum_post"
}

func (ForumPlate) TableName() string {
	return "forum_plate"
}
