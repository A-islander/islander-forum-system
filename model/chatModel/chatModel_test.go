package chatmodel

import (
	"fmt"
	"testing"
)

func TestGet(t *testing.T) {
	resp, err := GetChat("你好啊，请问这里是那里")
	if err != nil {
		t.Error(err.Error())
	}
	fmt.Println(resp)
}
