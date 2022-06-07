package controller

import (
	"errors"
	"fmt"
	"math/rand"
	"strconv"
	"time"
)

type Value struct {
	Num  int
	Str  string
	Atom string
	Type int // 1:num,2:str,3:atom
}

type ExprNode struct {
	Atom  string
	Value Value
	Param []*ExprNode
}

type ExprStr struct {
	Str   string
	Start int
	End   int
}

// 查找表达式
func FindExpression(str string) []ExprStr {
	length := len(str)
	// 栈堆
	stack := 0
	// 记录开始
	start := 0
	end := 0
	ExprArr := make([]ExprStr, 0)
	for i := 0; i < length; i++ {
		// 通过栈堆匹配
		if str[i] == '[' {
			start = i
			stack += 1
			for stack != 0 {
				i++
				// 找到头了
				if i >= length {
					return ExprArr
				}
				if str[i] == '[' {
					stack += 1
				}
				if str[i] == ']' {
					stack -= 1
				}
			}
			end = i
			ExprArr = append(ExprArr, ExprStr{Str: str[start : end+1], Start: start, End: end})
		}
	}
	// fmt.Println(ExprArr)

	return ExprArr
}

// 传入待求值字符串
func Eval(expression string) string {
	exprArr := FindExpression(expression)
	var newStr string
	start := 0
	for i := 0; i < len(exprArr); i++ {
		node, _ := parseValue(exprArr[i].Str, 0)
		_, err := evalValue(node)
		if err != nil {
			fmt.Println(err)
		}
		newStr += expression[start:exprArr[i].End+1] + " = " + node.Value.transferValue()
		start = exprArr[i].End + 1
	}
	if len(exprArr) == 0 {
		return expression
	}
	return newStr
}

// 回值和类型
func evalValue(expr *ExprNode) (Value, error) {
	// 没有子节点了
	if len(expr.Param) == 0 {
		// 字符串
		if expr.Atom[0] == '"' {
			expr.Value.Str = expr.Atom[1 : len(expr.Atom)-1]
			expr.Value.Type = 1
			return expr.Value, nil
		}
		// 数字
		if checkNum(expr.Atom) {
			expr.Value.Num, _ = strconv.Atoi(expr.Atom)
			expr.Value.Type = 2
			return expr.Value, nil
		} else {
			expr.Value.Atom = expr.Atom
			expr.Value.Type = 4
			return expr.Value, nil
		}
	}
	// expr.param 都要求值
	for i := 0; i < len(expr.Param); i++ {
		value, err := evalValue(expr.Param[i])
		if err != nil {
			return value, err
		}
	}
	value, err := evalValueOper(expr)
	if err != nil {
		return expr.Value, errors.New("eval error")
	}
	expr.Value = value
	return expr.Value, nil
}

// 求值操作
func evalValueOper(expr *ExprNode) (Value, error) {
	var param []Value
	for i := 0; i < len(expr.Param); i++ {
		param = append(param, expr.Param[i].Value)
	}
	return operate(expr.Atom, param)
}

// 操作对应表
func operate(atom string, param []Value) (Value, error) {
	rand.Seed(time.Now().UnixNano())
	switch atom {
	case "+":
		return addOperate(param)
	case "roll":
		return rollOperate(param)
	case "decide":
		return decideOperate(param)
	}
	return Value{}, errors.New("eval error")
}

func addOperate(param []Value) (Value, error) {
	ret := Value{}
	sum := 0
	for i := 0; i < len(param); i++ {
		if param[i].Type != 2 {
			return ret, errors.New("eval error")
		}
		sum += param[i].Num
	}
	ret.setValue(sum, 2)

	return ret, nil
}

func rollOperate(param []Value) (Value, error) {
	ret := Value{}
	// 验参
	if len(param) != 2 {
		return ret, errors.New("eval error")
	}
	for i := 0; i < len(param); i++ {
		if param[i].Type != 2 {
			return ret, errors.New("eval error")
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
			return ret, errors.New("eval error")
		}
	}
	ret.setValue(param[rand.Intn(len(param))].Str, 1)
	return ret, nil
}

// 回查找的字符串，和游标index
func parseValue(str string, index int) (*ExprNode, int) {
	// 跳过空格
	index = skipSpace(str, index)
	// 表达式
	if str[index] == '[' {
		return parseExpression(str, index)
	} else if str[index] == '"' { // 字符串
		return parseStr(str, index)
	} else { // 数字和原子
		return parseAtom(str, index)
	}
}

// 返回复合表达式
func parseExpression(str string, index int) (*ExprNode, int) {
	index += 1
	rootStatus := false
	var node *ExprNode
	for {
		index = skipSpace(str, index)
		if str[index] == ']' {
			index += 1
			return node, index
		} else if !rootStatus {
			node, index = parseValue(str, index)
			rootStatus = true
		} else {
			childNode, i := parseValue(str, index)
			node.Param = append(node.Param, childNode)
			index = i
		}
	}
	// return node
}

// 返回原子表达式
func parseAtom(str string, index int) (*ExprNode, int) {
	index = skipSpace(str, index)
	buff := make([]byte, 0)
	node := ExprNode{}
	for {
		if str[index] == ' ' || str[index] == ']' {
			if str[index] == ' ' {
				index += 1 // 跳过最后的空格
			}
			node.Atom = string(buff)
			return &node, index
		}
		buff = append(buff, str[index])
		index += 1
	}
}

// 返回字符串表达式
func parseStr(str string, index int) (*ExprNode, int) {
	buff := make([]byte, 1)
	buff[0] = str[index]
	index += 1
	node := ExprNode{}
	for {
		if str[index] == '"' {
			buff = append(buff, str[index])
			node.Atom = string(buff)
			index += 1
			return &node, index
		}
		buff = append(buff, str[index])
		index += 1
	}
}

func skipSpace(str string, index int) int {
	// 跳过空格
	for {
		if str[index] == ' ' && index < len(str) {
			index += 1
		} else {
			break
		}
	}
	return index
}

func printTree(node *ExprNode) {
	fmt.Printf("%p ", node)
	fmt.Println(node)
	for i := 0; i < len(node.Param); i++ {
		printTree(node.Param[i])
	}
}

func (v *Value) getValue() interface{} {
	switch v.Type {
	case 1:
		return v.Str
	case 2:
		return v.Num
	case 4:
		return v.Atom
	}
	return nil
}

func (v *Value) transferValue() string {
	switch v.Type {
	case 1:
		return v.Str
	case 2:
		return strconv.Itoa(v.Num)
	case 4:
		return v.Atom
	}
	return "求值错误，请检查表达式"
}

func (v *Value) setValue(value interface{}, valueType int) {
	switch valueType {
	case 1:
		v.Str = value.(string)
		v.Type = valueType
		break
	case 2:
		v.Num = value.(int)
		v.Type = valueType
		break
	case 4:
		v.Atom = value.(string)
		v.Type = valueType
		break
	}
}

func checkNum(str string) bool {
	for i := 0; i < len(str); i++ {
		if str[i] < '0' || str[i] > '9' {
			return false
		}
	}
	return true
}
