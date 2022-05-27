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

func TestStrOperate(t *testing.T) {
	// for i := 0; i < 10; i++ {
	// 	fmt.Println(strOperate("你好，我现在在决定 [decide 吃饭 睡觉 coding]"))
	// }
	// str := "123123 [+  abc   [+ 234 456] \"[123]\"] 2314123 [+  abc   [+ 234 456] \"[123]\"]"
	str := "123123 [+  abc   [+ 234 456] \"[123]\"] 123 [+  abc   [+ 234 456] \"[123]"
	exprArr := FindExpression(str)
	for i := 0; i < len(exprArr); i++ {
		node, _ := parseValue(exprArr[i].Str, 0)
		printTree(node)
	}
	fmt.Println(checkNum("123123123"))
}
