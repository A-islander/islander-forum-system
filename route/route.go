package route

import (
	"fmt"
	"net/http"

	"github.com/hedykan/httpHelper"
)

type HandleFunc func(w http.ResponseWriter, r *http.Request)

func Init() *http.ServeMux {
	port := ":12345"
	forumServer := http.NewServeMux()

	plateHandleArr := httpHelper.HandleArr{
		{Url: "get", Handler: http.HandlerFunc(getForumPlate)},
	}.AddGroup("plate").AddMiddleward(calcVisitTimeMiddleware)
	httpHelper.SetMuxHandle(forumServer, plateHandleArr)

	forumHandleArr := httpHelper.HandleArr{
		{Url: "get", Handler: http.HandlerFunc(getForumPost)},
		{Url: "index", Handler: http.HandlerFunc(getForumPostIndex)},
		{Url: "list", Handler: http.HandlerFunc(getForumPostList)},
		{Url: "indexLast", Handler: http.HandlerFunc(getForumIndexLastTime)},
		{Url: "userList", Handler: http.HandlerFunc(getUserForumPostList)},
		{Url: "post", Handler: http.HandlerFunc(postForumPost)},
		{Url: "reply", Handler: http.HandlerFunc(replyForumPost)},
		{Url: "sage/add", Handler: http.HandlerFunc(sageAdd)},
		{Url: "sage/sub", Handler: http.HandlerFunc(sageSub)},
		{Url: "sage/list", Handler: http.HandlerFunc(sageList)},
		{Url: "delete/ownPost", Handler: http.HandlerFunc(deleteOwnPost)},
		{Url: "recover/ownPost", Handler: http.HandlerFunc(recoverOwnPost)},
	}.AddGroup("forum").AddMiddleward(calcVisitTimeMiddleware)
	httpHelper.SetMuxHandle(forumServer, forumHandleArr)

	imgHandleArr := httpHelper.HandleArr{
		{Url: "token", Handler: http.HandlerFunc(getImgToken)},
		{Url: "upload", Handler: http.HandlerFunc(postImgUpload)},
	}.AddGroup("img").AddMiddleward(calcVisitTimeMiddleware)
	httpHelper.SetMuxHandle(forumServer, imgHandleArr)

	fmt.Printf("listen to port %s", port)
	http.ListenAndServe(port, forumServer)

	return forumServer
}
