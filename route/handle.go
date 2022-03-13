package route

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/forum_server/controller"
	"github.com/forum_server/model"
)

func getForumPlate(w http.ResponseWriter, r *http.Request) {
	res, err := controller.GetForumPlate()
	if err != nil {
		writeError(w, 404, err.Error())
	} else {
		write(w, res)
	}
}

func getForumPost(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, err := strconv.Atoi(query["postId"])
	if err != nil {
		log.Println(err)
	}
	res, err := controller.GetForumPost(postId)
	if err != nil {
		writeError(w, 404, err.Error())
	} else {
		write(w, res)
	}
}

func getForumPostIndex(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	plateId, _ := strconv.Atoi(query["plateId"])
	page, _ := strconv.Atoi(query["page"])
	size, _ := strconv.Atoi(query["size"])
	list := controller.GetForumPostIndex(plateId, page, size)
	count := controller.GetForumPostIndexCount(plateId)
	writeList(w, list, count)
}

func getForumPostList(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, _ := strconv.Atoi(query["postId"])
	page, _ := strconv.Atoi(query["page"])
	size, _ := strconv.Atoi(query["size"])
	list, err := controller.GetForumPostList(postId, page, size)
	if err != nil {
		writeError(w, 404, err.Error())
	} else {
		count := controller.GetForumPostListCount(postId)
		writeList(w, list, count)
	}
}

func postForumPost(w http.ResponseWriter, r *http.Request) {
	var query struct {
		Value    string
		Title    string
		ReplyArr []int
		PlateId  int
		MediaUrl string
	}
	postJson(r, &query)
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, 403, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, 403, err.Error())
		return
	}
	err = controller.PostForumPost(query.Value, query.Title, query.ReplyArr, query.PlateId, user.Id, query.MediaUrl, user.Name)
	if err != nil {
		writeError(w, 404, err.Error())
	} else {
		write(w, nil)
	}
}

func replyForumPost(w http.ResponseWriter, r *http.Request) {
	var query struct {
		Value    string
		FollowId int
		ReplyArr []int
		MediaUrl string
	}
	postJson(r, &query)
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, 403, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, 403, err.Error())
		return
	}
	err = controller.ReplyForumPost(query.Value, query.FollowId, query.ReplyArr, user.Id, query.MediaUrl, user.Name)
	if err != nil {
		writeError(w, 404, err.Error())
	} else {
		write(w, nil)
	}
}

func sageAdd(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, _ := strconv.Atoi(query["postId"])
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, 403, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, 403, err.Error())
		return
	}
	err = controller.SageAdd(postId, user.Id)
	if err != nil {
		writeError(w, 403, err.Error())
	} else {
		write(w, nil)
	}
}

func sageSub(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, _ := strconv.Atoi(query["postId"])
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, 403, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, 403, err.Error())
		return
	}
	err = controller.SageSub(postId, user.Id)
	if err != nil {
		writeError(w, 403, err.Error())
	} else {
		write(w, nil)
	}
}

func sageList(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	page, _ := strconv.Atoi(query["page"])
	size, _ := strconv.Atoi(query["size"])
	list := controller.GetAlreadySagePost(page, size)
	count := model.GetAlreadySageCount()
	writeList(w, list, count)
}
