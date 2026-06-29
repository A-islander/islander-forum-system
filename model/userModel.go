package model

import (
	"crypto/rand"
	"encoding/hex"
	"log"
)

// User 对应 user 表（由 user-system 合并而来，去 RPC 后改为本地直查）。
type User struct {
	Id           int
	Name         string
	RegisterTime int
}

// TableName 显式指定表名 user（GORM 默认会复数化为 users，与实际表名不符）。
func (User) TableName() string {
	return "user"
}

// ── User CRUD ────────────────────────────────────────────────

func CreateUser(data User) int {
	return data.create()
}

func UpdateUser(data User) int {
	return data.update()
}

func (user *User) create() int {
	userDb().Create(user)
	return user.Id
}

func (user *User) update() int {
	userDb().Model(user).Update("name", user.Name)
	return user.Id
}

func GetUser(id int) User {
	var res User
	userDb().Where("id = ?", id).Take(&res)
	return res
}

// GetUserByToken 由 token 直查 User（不做格式校验，供不需要区分"格式错"与"未命中"的调用方使用）。
func GetUserByToken(token string) User {
	userId, err := Token2UserId(token)
	if err != nil {
		return User{}
	}
	return GetUser(userId)
}

func GetUserArr(userIdArr []int) []User {
	var res []User
	if len(userIdArr) > 0 {
		userDb().Find(&res, userIdArr)
	}
	return res
}

// ── Token（表 user_token）──────────────────────────────────────
// token 发放 + 校验能力，原由 user-system 经 RPC 提供，合并后改为本地直查。

type Token struct {
	Id     int
	UserId int
	Token  string
}

func (Token) TableName() string {
	return "user_token"
}

// Token2UserId 由 token 查 userId，未命中返回 error（鉴权的真实依据）。
func Token2UserId(token string) (int, error) {
	var res Token
	err := userDb().Where("token = ?", token).Take(&res).Error
	return res.UserId, err
}

func GetTokenByUserId(userId int) string {
	var res Token
	userDb().Where("user_id = ?", userId).Take(&res)
	return res.Token
}

// SaveToken 落库新 token。user 表冗余 token 镜像写入已废弃（只保留 user_token 主查询源）。
func SaveToken(userId int, token string) int {
	res := Token{Token: token, UserId: userId}
	userDb().Create(&res)
	return res.Id
}

// NewToken 用 crypto/rand 生成 16 字节随机数 → hex 编码 = 32 字符。
// 保持 checkToken len==32 契约，彻底消除旧 md5(秒级时间戳) 的可预测与同秒碰撞问题。
func NewToken(userId int) string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// crypto/rand 失败极罕见（系统熵不足）；失败则进程不应继续签发不可靠凭证。
		log.Fatalf("crypto/rand failed: %v", err)
	}
	token := hex.EncodeToString(b)
	SaveToken(userId, token)
	return token
}

// ── Oauth（表 user_oauth，仅用 ip 维度做免密注册）─────────────────

type Oauth struct {
	Id        int
	IpAddress string
}

func (Oauth) TableName() string {
	return "user_oauth"
}

func GetOauthByIp(ipAddr string) (int, error) {
	var res Oauth
	err := userDb().Where("ip_address = ?", ipAddr).Take(&res).Error
	return res.Id, err
}

func SaveOauth(userId int, ipAddr string) int {
	res := Oauth{Id: userId, IpAddress: ipAddr}
	userDb().Create(&res)
	return res.Id
}
