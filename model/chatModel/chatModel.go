package chatmodel

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"time"

	"github.com/spf13/viper"
)

type ChatResp struct {
	Code int    `json:"code"`
	Data string `json:"data"`
	Msg  string `json:"msg"`
}

func GetChatUrl() (string, error) {
	var url string
	viper.SetConfigFile("./conf/config.json")
	err := viper.ReadInConfig()
	if err != nil {
		return url, err
	}
	url = viper.GetString("chatUrl")
	return url, nil
}

func GetChat(str string) (ChatResp, error) {
	client := &http.Client{Timeout: 5 * time.Second}
	url, err := GetChatUrl()
	if err != nil {
		return ChatResp{}, err
	}
	resp, err := client.Get(url + "defaultSend?str=" + str)
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
