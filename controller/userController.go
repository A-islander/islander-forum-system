package controller

import (
	"crypto/rand"
	"errors"
	"log"
	"time"

	"github.com/forum_server/model"
)

// User 合并自 user-system 的 controller.User（去掉 RPC，本地直查 model）。
type User struct {
	Id           int    `json:"id"`
	Name         string `json:"name"`
	RegisterTime int    `json:"registerTime"`
}

func NewUser(name string) *User {
	res := User{
		Name:         name,
		RegisterTime: int(time.Now().Unix()),
	}
	res.Id = model.CreateUser(model.User{Name: res.Name, RegisterTime: res.RegisterTime})
	return &res
}

func GetUserById(id int) model.User {
	return model.GetUser(id)
}

func GetUserByToken(token string) (model.User, error) {
	if err := checkToken(token); err != nil {
		return model.User{}, err
	}
	userId, err := model.Token2UserId(token)
	if err != nil {
		return model.User{}, err
	}
	return model.GetUser(userId), nil
}

func GetUserArr(idArr []int) []model.User {
	return model.GetUserArr(idArr)
}

func checkToken(token string) error {
	if len(token) != 32 {
		return errors.New("token is field")
	}
	return nil
}

// ── 注册（IP 免密，合并自 user-system 的 loginController）──────────
// 原 LoginByPassword / RegisterByPassword 为死代码，不迁移。

// RegisterByIpAddress 按 IP 注册/登录：IP 已存在则返回其 token，否则新建用户 + 发 token。
func RegisterByIpAddress(ipAddr string) (string, error) {
	userId, err := model.GetOauthByIp(ipAddr)
	if err == nil {
		return model.GetTokenByUserId(userId), nil
	}
	user := NewUser(newRandName(7))
	model.SaveOauth(user.Id, ipAddr)
	return model.NewToken(user.Id), nil
}

// newRandName 生成 n 位随机用户名（IP 注册用）。
// 用 crypto/rand 替换原 math/rand + 手动 Seed，避免并发下 Seed 竞争与可预测性。
func newRandName(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		log.Println("newRandName crypto/rand failed:", err)
		return "islander"
	}
	for i := range b {
		b[i] = letters[int(b[i])%len(letters)]
	}
	return string(b)
}
