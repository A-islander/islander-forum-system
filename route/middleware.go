package route

import (
	"errors"
	"net/http"

	"github.com/forum_server/model"
)

func methodMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 解决跨域问题
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		switch r.Method {
		// 复杂POST处理
		case "OPTIONS":
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type,AccessToken,X-CSRF-Token, Authorization")
			w.Header().Set("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
			w.Header().Set("Access-Control-Allow-Credentials", "true")
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// 计算访问次数
func calcVisitTimeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := remoteIp(r)
		// forumServer:ipVisitCount
		key := "fS:ipVC:" + ip
		// 十秒内访问次数
		count := model.AddCount(key, 1, 10)
		if count > 10 {
			// writeError(w, 403, errors.New("visited too much").Error())
			writeError(w, http.StatusTooManyRequests, errors.New("too many request").Error())
			return
		}
		next.ServeHTTP(w, r)
	})
}

func totalMiddleware(next func(http.ResponseWriter, *http.Request)) http.Handler {
	return methodMiddleware(calcVisitTimeMiddleware(http.HandlerFunc(next)))
}
