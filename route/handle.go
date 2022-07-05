package route

import (
	"errors"
	"log"
	"net/http"
	"strconv"

	"github.com/forum_server/controller"
)

func getForumPlate(w http.ResponseWriter, r *http.Request) {
	res, err := controller.GetForumPlate()
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
	} else {
		write(w, res)
	}
}

func getForumIndexLastTime(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	page, _ := strconv.Atoi(query["page"])
	size, _ := strconv.Atoi(query["size"])
	list, count := controller.GetForumIndexLastTime(page, size, []int{5})
	writeList(w, list, count)
}

func getUserForumPostList(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	page, _ := strconv.Atoi(query["page"])
	size, _ := strconv.Atoi(query["size"])
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	list, count := controller.GetForumPostByUid(user.Id, page, size)
	writeList(w, list, count)
}

func getForumPost(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, err := strconv.Atoi(query["postId"])
	if err != nil {
		log.Println(err)
	}
	res, err := controller.GetForumPost(postId)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
	} else {
		write(w, res)
	}
}

func getForumPostIndex(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	plateId, _ := strconv.Atoi(query["plateId"])
	page, _ := strconv.Atoi(query["page"])
	size, _ := strconv.Atoi(query["size"])
	list, count := controller.GetForumPostIndex(plateId, page, size)
	writeList(w, list, count)
}

func getForumPostList(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, _ := strconv.Atoi(query["postId"])
	page, _ := strconv.Atoi(query["page"])
	size, _ := strconv.Atoi(query["size"])
	list, count, err := controller.GetForumPostList(postId, page, size)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
	} else {
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
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	err = controller.PostForumPost(query.Value, query.Title, query.ReplyArr, query.PlateId, user.Id, query.MediaUrl, user.Name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
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
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	err = controller.ReplyForumPost(query.Value, query.FollowId, query.ReplyArr, user.Id, query.MediaUrl, user.Name)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
	} else {
		write(w, nil)
	}
}

func deleteOwnPost(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, _ := strconv.Atoi(query["postId"])
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	status := controller.DeleteOwnPost(token[0], postId)
	write(w, struct {
		Status bool `json:"status"`
	}{Status: status})
}

func recoverOwnPost(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, _ := strconv.Atoi(query["postId"])
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	status := controller.RecoverOwnPost(token[0], postId)
	write(w, struct {
		Status bool `json:"status"`
	}{Status: status})
}

func sageAdd(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, _ := strconv.Atoi(query["postId"])
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	sageStatus, err := controller.SageAdd(postId, user.Id)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
	} else {
		write(w, sageStatus)
	}
}

func sageSub(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	postId, _ := strconv.Atoi(query["postId"])
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	user, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	sageStatus, err := controller.SageSub(postId, user.Id)
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
	} else {
		write(w, sageStatus)
	}
}

func sageList(w http.ResponseWriter, r *http.Request) {
	query := get(r)
	page, _ := strconv.Atoi(query["page"])
	size, _ := strconv.Atoi(query["size"])
	list, count := controller.GetAlreadySagePost(page, size)
	writeList(w, list, count)
}

func getImgToken(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	_, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	imgToken, err := controller.GetImgToken()
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	write(w, struct {
		Token string `json:"token"`
	}{Token: imgToken})
}

func postImgUpload(w http.ResponseWriter, r *http.Request) {
	token, ok := r.Header["Authorization"]
	if !ok {
		writeError(w, http.StatusForbidden, errors.New("without Authorization").Error())
		return
	}
	_, err := controller.GetUserByToken(token[0])
	if err != nil {
		writeError(w, http.StatusForbidden, err.Error())
		return
	}
	res := controller.PostImgUpload(r)
	write(w, res)
}
