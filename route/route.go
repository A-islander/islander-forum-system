package route

import (
	"fmt"
	"net/http"
)

type HandleFunc func(w http.ResponseWriter, r *http.Request)

func Init() *http.ServeMux {
	port := ":12345"
	forumServer := http.NewServeMux()
	mid := totalMiddleware
	forumServer.Handle("/plate/get", mid(getForumPlate))
	forumServer.Handle("/forum/get", mid(getForumPost))
	forumServer.Handle("/forum/index", mid(getForumPostIndex))
	forumServer.Handle("/forum/list", mid(getForumPostList))
	forumServer.Handle("/forum/indexLast", mid(getForumIndexLastTime))
	forumServer.Handle("/forum/userList", mid(getUserForumPostList))
	forumServer.Handle("/forum/post", mid(postForumPost))
	forumServer.Handle("/forum/reply", mid(replyForumPost))
	forumServer.Handle("/forum/sage/add", mid(sageAdd))
	forumServer.Handle("/forum/sage/sub", mid(sageSub))
	forumServer.Handle("/forum/sage/list", mid(sageList))
	forumServer.Handle("/forum/delete/ownPost", mid(deleteOwnPost))
	forumServer.Handle("/forum/recover/ownPost", mid(recoverOwnPost))
	forumServer.Handle("/img/token", mid(getImgToken))
	forumServer.Handle("/img/upload", mid(postImgUpload))

	fmt.Printf("listen to port %s", port)
	http.ListenAndServe(port, forumServer)

	return forumServer
}
