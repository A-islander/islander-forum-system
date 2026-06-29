package model

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/forum_server/config"
	"github.com/go-redis/redis/v8"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

var ctx = context.Background()
var db = newDB()
var rdb = newRdb()
var userDB = newUserDB()

// 时间从设置里拿
var buffTime = time.Second * time.Duration(config.GetConfig().BuffTime)

func getDsn() string {
	conf := config.GetConfig()
	dsn := conf.UserName + ":" + conf.PassWord + "@tcp(" + conf.Ip + ")/" + conf.Database + "?charset=utf8mb4&timeout=30s"
	return dsn
}

func newRdb() *redis.Client {
	rdb := redis.NewClient(&redis.Options{
		Addr:     config.GetConfig().RedisIp,
		Password: "",
		DB:       0,
	})
	return rdb
}

func newDB() *gorm.DB {
	dsn := getDsn()
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	sqlDb, _ := db.DB()
	sqlDb.SetMaxIdleConns(10)
	sqlDb.SetMaxOpenConns(50)
	sqlDb.SetConnMaxLifetime(time.Hour)
	return db
}

// newUserDB 初始化 user 相关表（user / user_token / user_oauth）所在的 DB 连接。
// 合并自 user-system：默认与 forum 同库（user 表与 forum 表共库时直接复用 db），
// 若配置了独立的 UserDatabase 则连独立库（支持分库部署，零破坏）。
func newUserDB() *gorm.DB {
	conf := config.GetConfig()
	if conf.UserDatabase == "" {
		return db
	}
	dsn := conf.UserName + ":" + conf.PassWord + "@tcp(" + conf.Ip + ")/" + conf.UserDatabase + "?charset=utf8mb4&timeout=30s"
	udb, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatal(err)
	}
	sqlDb, _ := udb.DB()
	sqlDb.SetMaxIdleConns(10)
	sqlDb.SetMaxOpenConns(50)
	sqlDb.SetConnMaxLifetime(time.Hour)
	return udb
}

// userDb 返回 user 相关表所用的 DB（默认即全局 db）。
func userDb() *gorm.DB {
	return userDB
}

func addZsetBuff(key string, score int, data interface{}) {
	value, _ := json.Marshal(data)
	// rdb := newRdb()
	err := rdb.ZAdd(ctx, key, &redis.Z{
		Score:  float64(score),
		Member: value,
	}).Err()
	if err != nil {
		log.Println(err)
	}
}

func getZsetArr(key string, first int64, end int64) []string {
	// rdb := newRdb()
	res, err := rdb.ZRevRange(ctx, key, first, end).Result()
	if err != nil {
		log.Println(err)
	}
	rdb.Expire(ctx, key, buffTime)
	return res
}

func getZsetCount(key string) int {
	// rdb := newRdb()
	num, err := rdb.ZCard(ctx, key).Result()
	if err != nil {
		log.Println(err)
	}
	return int(num)
}

func delKey(key string) int {
	// rdb := newRdb()
	num, err := rdb.Del(ctx, key).Result()
	if err != nil {
		log.Println(err)
	}
	return int(num)
}

func checkKey(key string) bool {
	// rdb := newRdb()
	num, err := rdb.Exists(ctx, key).Result()
	if err != nil {
		log.Println(err)
	}
	if num > 0 {
		return true
	} else {
		return false
	}
}

func tranPost(buffArr []string) []ForumPost {
	res := make([]ForumPost, len(buffArr))
	for i := 0; i < len(buffArr); i++ {
		json.Unmarshal([]byte(buffArr[i]), &res[i])
	}
	return res
}

func setCount(key string, count int) {
	// rdb := newRdb()
	rdb.Set(ctx, key, count, buffTime)
}

func getCount(key string) int {
	// rdb := newRdb()
	buff, err := rdb.Get(ctx, key).Result()
	if err != nil {
		log.Println(err)
	}
	count, err := strconv.Atoi(buff)
	if err != nil {
		log.Println(err)
	}
	return count
}

// 记录key次数
func AddCount(key string, count int, recordTime int) int {
	val, err := rdb.IncrBy(ctx, key, int64(count)).Result()
	if err != nil {
		log.Println(err)
	}
	rdb.Expire(ctx, key, time.Second*time.Duration(recordTime))
	return int(val)
}

// func get(query string, res ...interface{}) {
// 	db := newDB()
// 	defer db.Close()
// 	rows, err := db.Query(query)
// 	if err != nil {
// 		panic(err)
// 	}
// 	defer rows.Close()
// 	for rows.Next() {
// 		rows.Scan(res...)
// 	}
// }

// func insert(query string, data ...interface{}) int {
// 	db := newDB()
// 	defer db.Close()
// 	res, err := db.Exec(query, data...)
// 	if err != nil {
// 		panic(err)
// 	}
// 	lastId, err := res.LastInsertId()
// 	return int(lastId)
// }
