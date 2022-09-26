package controller

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"

	"github.com/aliyun/aliyun-oss-go-sdk/oss"
	"github.com/forum_server/config"
	"github.com/spf13/viper"
)

var bucket *oss.Bucket
var ossUrl string

func init() {
	viper.SetConfigFile("./conf/config.json")
	err := viper.ReadInConfig()
	if err != nil {
		log.Println(err)
	}
	ossUrl = viper.GetString("ossUrl")

	client, err := oss.New(viper.GetString("endPoint"), viper.GetString("accessKeyId"), viper.GetString("accessKeySecret"))
	if err != nil {
		log.Println(err)
	}

	bucket, err = client.Bucket(viper.GetString("bucketName"))
	if err != nil {
		log.Println(err)
	}
}

// 使用https://sm.ms图床
func GetImgToken() (string, error) {
	config := config.GetConfig()
	res, err := http.PostForm(
		"https://sm.ms/api/v2/token",
		url.Values{
			"username": {config.UserName},
			"password": {config.PassWord},
		})
	if err != nil {
		log.Println(err)
	}
	defer res.Body.Close()
	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		log.Println(err)
	}
	var resJson map[string]interface{}
	json.Unmarshal(body, &resJson)
	if resJson["success"] != true {
		return "", errors.New("get token field")
	}
	token := resJson["data"].(map[string]interface{})["token"].(string)
	return token, nil
}

func PostImgUpload(r *http.Request) interface{} {
	// 获取文件
	file, header, err := r.FormFile("file")
	if err != nil {
		log.Println(err)
	}

	buff := new(bytes.Buffer)
	buff.ReadFrom(file)

	// 大小限制5mb
	if buff.Len() > 5*1024*1024 {
		return Response{
			Success: false,
		}
	}

	// 获取去重图片md5并且连接后缀
	nameHash := makeMd5Hash([]byte(buff.Bytes()))
	name := nameHash + path.Ext(header.Filename)
	err = upload(name, buff)
	if err != nil {
		log.Println(err)
		return Response{
			Success: false,
		}
	}

	res := Response{
		Success:   true,
		RequestId: name,
		Data: struct {
			Url string "json:\"url\""
		}{
			Url: ossUrl + name,
		},
	}
	return res
}

type Response struct {
	Success   bool   `json:"success"`
	RequestId string `json:"RequestId"`
	Data      struct {
		Url string `json:"url"`
	} `json:"data"`
}

func upload(objectName string, fp io.Reader) (err error) {
	// 重名则不提交， TODO 哈希碰撞处理
	isExist, err := bucket.IsObjectExist(objectName)
	if isExist {
		return err
	}
	err = bucket.PutObject(objectName, fp)
	return err
}

func makeMd5Hash(file []byte) string {
	hasher := md5.New()
	hasher.Write(file)
	return hex.EncodeToString(hasher.Sum(nil))
}
