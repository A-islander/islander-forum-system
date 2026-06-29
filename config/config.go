package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
)

type Config struct {
	UserName     string
	PassWord     string
	Ip           string
	Database     string
	UserDatabase string // user 相关表所在库；留空则与 forum 共库（合并自 user-system）
	SageNum      int
	RedisIp      string
	BuffTime     int
	ImgUserName  string
	ImgPassWord  string
}

func GetConfig() Config {
	file, err := ioutil.ReadFile("./conf/config.json")
	// file, err := ioutil.ReadFile("../conf/config.json")
	if err != nil {
		log.Println(err)
	}
	var res Config
	json.Unmarshal(file, &res)
	return res
}
