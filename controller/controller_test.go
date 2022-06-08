package controller

import (
	"fmt"
	"testing"

	"github.com/forum_server/model"
)

func TestGetForumIndex(t *testing.T) {
	fmt.Println(GetForumPostIndex(1, 0, 10))
}

// func TestGetForumPost(t *testing.T) {
// 	ReplyForumPost("test", 1, nil, 1, "", "name")
// 	PostForumPost("test", "test", nil, 1, 1, "", "name")
// }

func TestSage(t *testing.T) {
	// fmt.Println(SageAdd(1, 3))
}

func TestForum(t *testing.T) {
	fmt.Println(GetForumPlate())
}

func TestGetLast(t *testing.T) {
	fmt.Println(GetForumIndexLastTime(0, 10, []int{}))
}

func TestDelIntArr(t *testing.T) {
	arr := delIntArr([]int{1, 2, 3}, 1)
	fmt.Println(arr)
}

func TestGetUserArr(t *testing.T) {
	fmt.Println(model.GetUserArr([]int{}))
}

func TestGetForumPostByUid(t *testing.T) {
	fmt.Println(GetForumPostByUid(1, 0, 10))
}

func TestGetImgToken(t *testing.T) {
	fmt.Println(GetImgToken())
}

func TestChangePost(t *testing.T) {
	ChangePostPlate(57, 1)
}

// 还有错误没处理
func TestStrOperate(t *testing.T) {
	str := `2022-06-08日更新`
	newStr := Eval(str)
	fmt.Println(newStr)
}
