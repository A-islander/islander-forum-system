package chatmodel

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"
)

type ChatResp struct {
	Code int    `json:"code"`
	Data string `json:"data"`
	Msg  string `json:"msg"`
}

func GetChat(str string) (ChatResp, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("http://swap-api.juhuan.top/chat/defaultSend?str=" + str)
	if err != nil {
		return ChatResp{}, err
	}
	defer resp.Body.Close()
	var buf [512]byte
	res := bytes.NewBuffer(nil)

	for {
		n, err := resp.Body.Read(buf[0:])
		res.Write(buf[0:n])
		if err != nil && err == io.EOF {
			break
		} else if err != nil {
			return ChatResp{}, err
		}
	}

	var chatResp ChatResp
	json.Unmarshal(res.Bytes(), &chatResp)
	return chatResp, err
}
