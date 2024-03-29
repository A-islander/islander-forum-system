package controller

import (
	"context"
	"errors"
	"log"
	"math/rand"

	chatmodel "github.com/forum_server/model/chatModel"
)

// 操作
type Operate struct {
	Id       int `json:"id"`
	Type     int `json:"type"`
	AgreeNum int `json:"agreeNum"`
	DenyNum  int `json:"denyNum"`
	Status   int `json:"status"`
}

// 操作的类型
type OperateType struct {
	Id      int
	Value   string
	Operate func(postId int, userIdArr []int)
}

// 操作对应表
func operate(atom string, param []Value, ctx context.Context) (Value, error) {
	// rand.Seed(time.Now().UnixNano())
	switch atom {
	case "+":
		return addOperate(param, ctx)
	case "-":
		return subOperate(param, ctx)
	case "roll":
		return rollOperate(param, ctx)
	case "decide":
		return decideOperate(param, ctx)
	case "和岛民娘聊会":
		return discussOperate(param, ctx)
	}
	return Value{}, errors.New("eval error: operate not found")
}

func addOperate(param []Value, ctx context.Context) (Value, error) {
	ret := Value{}
	sum := 0
	for i := 0; i < len(param); i++ {
		if param[i].Type != 2 {
			return ret, errors.New("eval error: value is not number")
		}
		sum += param[i].Num
	}
	ret.setValue(sum, 2)

	return ret, nil
}

func subOperate(param []Value, ctx context.Context) (Value, error) {
	ret := Value{}
	sum := 0
	for i := 0; i < len(param); i++ {
		if param[i].Type != 2 {
			return ret, errors.New("eval error: value is not number")
		}
		sum -= param[i].Num
	}
	ret.setValue(sum, 2)

	return ret, nil
}

func rollOperate(param []Value, ctx context.Context) (Value, error) {
	ret := Value{}
	// 验参
	if len(param) != 2 {
		return ret, errors.New("eval error: param error")
	}
	for i := 0; i < len(param); i++ {
		if param[i].Type != 2 {
			return ret, errors.New("eval error: value is not number")
		}
	}
	start := param[0].Num
	end := param[1].Num
	roll := rand.Intn(end - start + 1)
	ret.setValue(roll+start, 2)

	return ret, nil
}

func decideOperate(param []Value, ctx context.Context) (Value, error) {
	ret := Value{}
	for i := 0; i < len(param); i++ {
		if param[i].Type != 1 {
			return ret, errors.New("eval error: value is not string")
		}
	} // 如果这样注释有bug
	ret.setValue(param[rand.Intn(len(param))].Str, 1)
	return ret, nil
}

func discussOperate(param []Value, ctx context.Context) (Value, error) {
	ret := Value{}
	if param[0].Type != 1 {
		return ret, errors.New("eval error: value is not string")
	}
	str := param[0].Str

	ret.setValue("岛民娘回复中...", 1)

	foo := func(str string, ctx context.Context) {
		resp, err := chatmodel.GetChat(str)
		// fmt.Println("discussOperate", resp, err)
		if err != nil {
			log.Println(err)
			return
		}
		followId, err := GetFollowId(ctx)
		if err != nil {
			log.Println(err)
			return
		}

		ReplyForumPost(resp.Data, followId, []int{}, 7, "", "岛民娘")
	}
	go foo(str, ctx)

	return ret, nil
}
