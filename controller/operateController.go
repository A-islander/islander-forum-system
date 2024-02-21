package controller

import (
	"errors"
	"math/rand"
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
func operate(atom string, param []Value) (Value, error) {
	// rand.Seed(time.Now().UnixNano())
	switch atom {
	case "+":
		return addOperate(param)
	case "-":
		return subOperate(param)
	case "roll":
		return rollOperate(param)
	case "decide":
		return decideOperate(param)
	case "和岛民娘聊会：":
		return discussOperate(param)
	}
	return Value{}, errors.New("eval error: operate not found")
}

func addOperate(param []Value) (Value, error) {
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

func subOperate(param []Value) (Value, error) {
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

func rollOperate(param []Value) (Value, error) {
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

func decideOperate(param []Value) (Value, error) {
	ret := Value{}
	for i := 0; i < len(param); i++ {
		if param[i].Type != 1 {
			return ret, errors.New("eval error: value is not string")
		}
	} // 如果这样注释有bug
	ret.setValue(param[rand.Intn(len(param))].Str, 1)
	return ret, nil
}

func discussOperate(param []Value) (Value, error) {
	ret := Value{}
	if param[0].Type != 1 {
		return ret, errors.New("eval error: value is not string")
	}
	str := param[0].Str

	// 回复str
	resStr := str

	ret.setValue(resStr, 1)

	go func(str string) {

	}(resStr)

	return ret, nil
}
