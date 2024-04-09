package controller

import (
	"context"
	"fmt"
	"log"

	chatmodel "github.com/forum_server/model/chatModel"
)

// 岛民娘回复
func posterReply(str string, ctx context.Context) {
	resp, err := chatmodel.GetChat(str)
	fmt.Println("discussOperate", resp, err)
	if err != nil {
		log.Println(err)
		return
	}
	followId, err := GetFollowId(ctx)
	if err != nil {
		log.Println(err)
		return
	}

	ReplyForumPost(resp.Data, followId, []int{}, 7, "", "岛民娘")
}
