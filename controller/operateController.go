package controller

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

// 字符串操作
// TODO 根据前缀表达式构建语法分析树
// [operate 123 123] 或 [operate?123&123]
// func strOperate(str string) string {
// 	rand.Seed(time.Now().UnixNano())
// 	// 通过[]查找操作
// 	reg := regexp.MustCompile(`\[(.*?)\]`)
// 	if reg == nil {
// 		return str
// 	}
// 	res := reg.FindAllStringSubmatch(str, -1)
// 	// 操作数大于10不允许操作
// 	if len(res) > 10 {
// 		return str
// 	}
// 	// 通过?分割操作和参数
// 	for key, value := range res {
// 		index := reg.FindAllIndex([]byte(str), -1)
// 		repl, _ := strOperateCase(value[1])
// 		str = str[:index[key][0]] + repl + str[index[key][1]:]
// 	}
// 	// 通过&分割参数

// 	return str
// }

// // 字符串操作选择
// func strOperateCase(str string) (string, error) {
// 	split := strings.Split(str, " ")
// 	operate := split[0]
// 	param := split[1:]
// 	switch operate {
// 	case "roll":
// 		num, err := rollOperate(param)
// 		return "[" + str + "] = " + strconv.Itoa(num), err
// 	case "+":
// 		num, err := addOperate(param)
// 		return "[" + str + "] = " + strconv.Itoa(num), err
// 	case "decide":
// 		choice, err := decideOperate(param)
// 		return "[" + str + "] = " + choice, err
// 	}
// 	return "[" + str + "]", nil
// }

// func rollOperate(param []string) (int, error) {
// 	// 验参
// 	if len(param) != 2 {
// 		return 0, errors.New("illegal param")
// 	}
// 	start, err := strconv.Atoi(param[0])
// 	if err != nil {
// 		return 0, errors.New("illegal param")
// 	}
// 	end, err := strconv.Atoi(param[1])
// 	if err != nil {
// 		return 0, errors.New("illegal param")
// 	}
// 	num := rand.Intn(end-start+1) + start
// 	return num, nil
// }

// func addOperate(param []string) (int, error) {
// 	sum := 0
// 	for i := 0; i < len(param); i++ {
// 		num, err := strconv.Atoi(param[i])
// 		if err != nil {
// 			return 0, errors.New("illegal param")
// 		}
// 		sum += num
// 	}
// 	return sum, nil
// }

// func decideOperate(param []string) (string, error) {
// 	return param[rand.Intn(len(param))], nil
// }
