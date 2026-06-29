package controller

import (
	"context"
	"errors"
	"time"

	"github.com/forum_server/config"
	"github.com/forum_server/model"
)

type ForumPost struct {
	Id            int         `json:"id"`
	Title         string      `json:"title"`
	Value         string      `json:"value"`
	FollowId      int         `json:"followId"`
	PlateId       int         `json:"plateId"`
	Status        int         `json:"status"`
	ReplyArr      []int       `json:"replyArr"`
	UserId        int         `json:"userId"`
	Time          int         `json:"time"`
	MediaUrl      string      `json:"mediaUrl"`
	ReplyCount    int         `json:"replyCount"`
	TopStatus     int         `json:"topStatus"`
	LastReplyTime int         `json:"lastReplyTime"`
	SageAddId     []int       `json:"sageAddId"`
	SageSubId     []int       `json:"sageSubId"`
	SageAddCount  int         `json:"sageAddCount"`
	SageSubCount  int         `json:"sageSubCount"`
	Name          string      `json:"name"`
	LastReplyArr  []ForumPost `json:"lastReplyArr"`
	SageAddUser   []string    `json:"sageAddUser"`
	SageSubUser   []string    `json:"sageSubUser"`
}

type ForumPlate struct {
	Id     int    `json:"id"`
	Name   string `json:"name"`
	Status int    `json:"status"`
	Value  string `json:"value"`
}

type FollowIdKey struct{}
type PlateIdKey struct{}

func GetFollowId(ctx context.Context) (int, error) {
	followId, ok := ctx.Value(FollowIdKey{}).(int)
	if !ok {
		return 0, errors.New("followId not found")
	}
	return followId, nil
}

func GetPlateId(ctx context.Context) (int, error) {
	plateId, ok := ctx.Value(PlateIdKey{}).(int)
	if !ok {
		return 0, errors.New("followId not found")
	}
	return plateId, nil
}

func NewForumPost(value string, plateId int, followId int, userId int) *ForumPost {
	ctx := context.WithValue(context.Background(), PlateIdKey{}, plateId)
	ctx = context.WithValue(ctx, FollowIdKey{}, followId)
	// 传入字符串操作
	value = Eval(value, ctx)
	return &ForumPost{
		Value:     value,
		PlateId:   plateId,
		UserId:    userId,
		Status:    0,
		Time:      int(time.Now().Unix()),
		SageAddId: make([]int, 0),
		SageSubId: make([]int, 0),
	}
}

func GetForumPlate() ([]ForumPlate, error) {
	res, err := model.GetForumPlate()
	if err != nil {
		return nil, err
	}
	return TransferForumPlateArrModel(res), nil
}

// 时间线
func GetForumIndexLastTime(page, size int, exceptPlateArr []int) ([]ForumPost, int) {
	modelRes, count := model.GetForumIndexLastTime(page, size, exceptPlateArr)
	res := TransferForumPostListModel(modelRes)
	getLastReply(res)
	return res, count
}

// 获取单个串
func GetForumPost(postId int) (ForumPost, error) {
	resModel, err := model.GetForumPost(postId)
	res := transferForumPostModel(resModel)
	if err != nil {
		return res, err
	}
	res.Name = model.GetUser(res.UserId).Name
	if res.Status > 0 {
		res.Value = "该串已被隐藏或sage"
	}
	return res, nil
}

// 获取板块首页
func GetForumPostIndex(plateId int, page int, size int) ([]ForumPost, int) {
	// 缓存版
	modelRes, count := model.GetForumPostIndexBuff(plateId, page, size)
	// 无缓存版
	// modelRes, count := model.GetForumPostIndex(plateId, page, size)
	res := TransferForumPostListModel(modelRes)
	getLastReply(res)
	return res, count
}

func getLastReply(res []ForumPost) {
	// 获取最晚回复，最晚回复时间非自己
	// 用一次遍历获取所需内存分配
	replyIndexCount := 0
	for i := 0; i < len(res); i++ {
		if res[i].Time != res[i].LastReplyTime {
			replyIndexCount += 1
		}
	}
	followIdArr := make([]int, replyIndexCount)
	replyIndex := 0
	resMap := make(map[int]*ForumPost)
	for i := 0; i < len(res); i++ {
		if res[i].Time != res[i].LastReplyTime {
			followIdArr[replyIndex] = res[i].Id
			resMap[res[i].Id] = &res[i]
			replyIndex += 1
		}
	}
	// 缓存版
	lastRes := TransferForumPostListModel(model.GetLastPostListBuff(followIdArr, 5))
	// 无缓存版
	// lastRes := TransferForumPostListModel(model.GetLastPostList(followIdArr, 5))
	for i := len(lastRes) - 1; i > -1; i-- {
		resMap[lastRes[i].FollowId].LastReplyArr = append(resMap[lastRes[i].FollowId].LastReplyArr, lastRes[i])
	}
}

// 获取该串回帖数
func GetForumPostCount(postId int) (int, error) {
	return model.GetForumPostCount(postId)
}

// 获取串页
func GetForumPostList(postId int, page int, size int) ([]ForumPost, int, error) {
	if _, err := GetForumPost(postId); err != nil {
		return nil, 0, err
	}
	modelRes, count := model.GetForumPostList(postId, page, size)
	res := TransferForumPostListModel(modelRes)
	return res, count, nil
}

// 通过uid获取发言
func GetForumPostByUid(uid int, page, size int) ([]ForumPost, int) {
	modelRes, count := model.GetForumPostListByUid(uid, page, size)
	res := TransferForumPostListModel(modelRes)
	return res, count
}

// 发串
func PostForumPost(value string, title string, replyArr []int, plateId int, userId int, mediaUrl string, name string) error {
	if value == "" || len(value) > 8192 || len(title) > 128 || len(mediaUrl) > 2048 {
		return errors.New("input too long or value is null")
	}
	post := NewForumPost(value, plateId, 0, userId)
	post.Title = title
	post.MediaUrl = mediaUrl
	post.LastReplyTime = post.Time
	post.Name = name
	if replyArr == nil {
		post.ReplyArr = make([]int, 0)
	} else {
		post.ReplyArr = replyArr
	}
	model.SaveForumPost(transferForumPost(*post))

	return nil
}

// 回串
func ReplyForumPost(value string, followId int, replyArr []int, userId int, mediaUrl string, name string) error {
	if value == "" || len(value) > 8192 || len(mediaUrl) > 2048 {
		return errors.New("input too long")
	}
	mainPost, err := GetForumPost(followId)
	if err != nil || mainPost.FollowId != 0 {
		return errors.New("replyId is illegal")
	}
	post := NewForumPost(value, mainPost.PlateId, followId, userId)
	post.FollowId = mainPost.Id
	post.MediaUrl = mediaUrl
	post.LastReplyTime = post.Time
	post.Name = name
	if replyArr == nil {
		post.ReplyArr = make([]int, 0)
	} else {
		post.ReplyArr = replyArr
	}
	model.SaveForumReply(transferForumPost(*post))
	if mainPost.Status == 0 { // SAGE贴自沉
		model.UpdateForumPostCount(mainPost.Id, post.Time)
	} else {
		model.UpdateForumPostCount(mainPost.Id, mainPost.Time)
	}

	return nil
}

// 删除拥有贴
func DeleteOwnPost(token string, postId int) bool {
	user := model.GetUserByToken(token)
	post, err := model.GetForumPostByPostId(postId)
	if err != nil {
		return false
	}
	if post.UserId == user.Id {
		model.UpdateForumPostStatus(post, 2)
		return true
	}
	return false
}

// 恢复拥有贴
func RecoverOwnPost(token string, postId int) bool {
	user := model.GetUserByToken(token)
	post, err := model.GetForumPostByPostId(postId)
	if err != nil {
		return false
	}
	if post.UserId == user.Id {
		model.UpdateForumPostStatus(post, 0)
		return true
	}
	return false
}

// sage添加（原子版）
// 用 MySQL JSON 原子函数完成投票变更，消除并发 read-modify-write 丢票问题。
// 返回值 sageStatus 含义：true=执行了「加入/取消对侧」，false=本次是「取消」。
func SageAdd(postId int, userId int) (bool, error) {
	post, err := GetForumPost(postId)
	if err != nil {
		return false, err
	}
	if post.Status != 0 {
		return false, errors.New("it's sage")
	}

	addOk := findIntArr(post.SageAddId, userId)
	if !addOk {
		// 加入 add 阵营（model 层会原子地从 sub 阵营互斥移除）
		addArr, subArr := model.SageAddPostUser(postId, userId)
		SageSetByCount(postId, addArr, subArr)
		return true, nil
	}
	// 已赞 → 取消赞
	model.SageRemovePostUser(postId, userId)
	return false, nil
}

// 反sage添加（原子版）
func SageSub(postId int, userId int) (bool, error) {
	post, err := GetForumPost(postId)
	if err != nil {
		return false, err
	}
	addOk := findIntArr(post.SageAddId, userId)
	if addOk {
		// 已赞 → 取消赞（取消对侧时不需要触发 sage 判定，赞数只减不增）
		model.SageRemovePostUser(postId, userId)
		return false, nil
	}

	subOk := findIntArr(post.SageSubId, userId)
	if !subOk {
		// 加入 sub 阵营（model 层会原子地从 add 阵营互斥移除）
		addArr, subArr := model.SageSubPostUser(postId, userId)
		SageSetByCount(postId, addArr, subArr)
		return true, nil
	}
	// 已踩 → 取消踩
	model.SageRemoveSubUser(postId, userId)
	return false, nil
}

// SageSetByCount 根据最新投票数判断是否触发 sage 隐藏。
// 使用投票后从 DB 重读的真实数组，避免基于脏数据判定。
func SageSetByCount(postId int, addArr, subArr []int) {
	conf := config.GetConfig()
	if len(addArr)-len(subArr) > conf.SageNum {
		var post model.ForumPost
		post.Id = postId
		// 需 plateId 以便清缓存：补查一次拿 plate_id
		model.UpdateForumPostStatusById(postId, 1)
	}
}

func GetAlreadySagePost(page int, size int) ([]ForumPost, int) {
	modelRes, count := model.GetAlreadySagePost(page, size)
	res := TransferForumPostListModel(modelRes)
	for i := 0; i < len(res); i++ {
		userArr := model.GetUserArr(res[i].SageAddId)
		pushSageUser(&res[i], userArr, 1)
		userArr = model.GetUserArr(res[i].SageSubId)
		pushSageUser(&res[i], userArr, 2)
	}
	return res, count
}

func pushSageUser(post *ForumPost, userArr []model.User, pushType int) {
	switch pushType {
	case 1:
		post.SageAddUser = make([]string, len(userArr))
		for i := 0; i < len(userArr); i++ {
			post.SageAddUser[i] = userArr[i].Name
		}
		return
	case 2:
		post.SageSubUser = make([]string, len(userArr))
		for i := 0; i < len(userArr); i++ {
			post.SageSubUser[i] = userArr[i].Name
		}
		return
	}
}

// 移版
func ChangePostPlate(postId int, plateId int) {
	model.ChangePostPlate(postId, plateId)
	model.ChangeFollowPostPlate(postId, plateId)
}

func findIntArr(arr []int, id int) bool {
	for i := 0; i < len(arr); i++ {
		if arr[i] == id {
			return true
		}
	}
	return false
}

func delIntArr(arr []int, id int) []int {
	for i := 0; i < len(arr); i++ {
		if arr[i] == id {
			arr = append(arr[:i], arr[i+1:]...)
			return arr
		}
	}
	return arr
}
