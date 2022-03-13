package main

import (
	"log"
	"os"

	"github.com/forum_server/route"
)

func init() {
	file := "./log/" + "message" + ".log"
	logFile, err := os.OpenFile(file, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0766)
	if err != nil {
		panic(err)
	}
	log.SetOutput(logFile) // 将文件设置为log输出的文件
	log.SetPrefix("[ForumServer]")
	log.SetFlags(log.LstdFlags | log.Lshortfile | log.LUTC)
	return
}

func main() {
	go route.Init()
	select {}
}
