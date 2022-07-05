package model

import (
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
	end := first + size
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
	db.Limit(size).Offset(first).Where("(follow_id = ? and status = 0) or id = ?", postId, postId).Order("time asc").Find(&res).Limit(-1).Offset(-1).Count(&count)
	return res, int(count)
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
func GetLastPostList(followIdArr []int, count int) []ForumPost {
	var res []ForumPost
	db.Raw("select fp.* from (select fp1.*, (select count(*) + 1 from forum_post fp2 where fp2.follow_id = fp1.follow_id and fp2.time > fp1.time) top from forum_post fp1 where follow_id in ?) fp where top < (? + 1) order by fp.follow_id, top", followIdArr, count).Scan(&res)
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

func UpdateSageAdd(post ForumPost) {
	db.Model(&post).Update("sage_add_id", post.SageAddId)
	indexKey := "fS:pI:" + strconv.Itoa(post.PlateId)
	delKey(indexKey)
}

func UpdateSageSub(post ForumPost) {
	db.Model(&post).Update("sage_sub_id", post.SageSubId)
	indexKey := "fS:pI:" + strconv.Itoa(post.PlateId)
	delKey(indexKey)
}

func UpdateForumPostStatus(post ForumPost, status int) {
	db.Model(&post).Update("status", status)
	// 更新缓存
	indexKey := "fS:pI:" + strconv.Itoa(post.PlateId)
	countKey := "fS:pIC:" + strconv.Itoa(post.PlateId)
	// setForumIndexBuff(post)
	delKey(indexKey)
	delKey(countKey)
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
