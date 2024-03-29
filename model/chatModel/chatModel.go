package chatmodel

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"

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
	url, err := GetChatUrl()
	if err != nil {
		return ChatResp{}, err
	}
	url = url + "defaultSend?str=" + str

	resp, err := http.Get(url)
	// fmt.Println("GetChat", url, resp, err)
	if resp.StatusCode != 200 {
		return ChatResp{}, errors.New("request status fault " + strconv.Itoa(resp.StatusCode))
	}
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
