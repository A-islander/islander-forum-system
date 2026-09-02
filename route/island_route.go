package route

import (
	"net/http"

	"github.com/forum_server/controller"
)

func registerIslandRoutes(mux *http.ServeMux) {
	mux.Handle("/api/island/environment", methodMiddleware(http.HandlerFunc(islandEnvironmentHandler)))
}

func islandEnvironmentHandler(w http.ResponseWriter, r *http.Request) {
	if !requireMethod(w, r, http.MethodGet) {
		return
	}
	environment, err := controller.GetIslandEnvironment(r.Context())
	if err != nil {
		writeError(w, http.StatusServiceUnavailable, err.Error())
		return
	}
	write(w, environment)
}
