package config

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"
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
	if err != nil && os.Getenv("ISLANDER_DB_ADDR") == "" {
		log.Println(err)
	}
	var res Config
	if err == nil {
		json.Unmarshal(file, &res)
	}
	applyEnv(&res)
	return res
}

// applyEnv allows local/test deployments to select an isolated database
// without rewriting conf/config.json. Empty environment variables are ignored.
func applyEnv(conf *Config) {
	overrides := []struct {
		key string
		set func(string)
	}{
		{"ISLANDER_DB_USER", func(v string) { conf.UserName = v }},
		{"ISLANDER_DB_PASSWORD", func(v string) { conf.PassWord = v }},
		{"ISLANDER_DB_ADDR", func(v string) { conf.Ip = v }},
		{"ISLANDER_DB_NAME", func(v string) { conf.Database = v }},
		{"ISLANDER_USER_DB_NAME", func(v string) { conf.UserDatabase = v }},
		{"ISLANDER_REDIS_ADDR", func(v string) { conf.RedisIp = v }},
	}
	for _, override := range overrides {
		if value := os.Getenv(override.key); value != "" {
			override.set(value)
		}
	}
}
