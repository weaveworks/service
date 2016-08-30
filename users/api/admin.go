package api

import (
	"fmt"
	"net/http"

	"github.com/Sirupsen/logrus"
	"github.com/gorilla/mux"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
)

func (a *API) admin(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!doctype html>
<html>
	<head><title>User service</title></head>
	<body>
		<h1>User service</h1>
		<ul>
			<li><a href="private/api/users">Users</a></li>
			<li><a href="private/api/pardot">Sync User-Creation with Pardot</a></li>
		</ul>
	</body>
</html>
`)
}

func (a *API) listUsers(w http.ResponseWriter, r *http.Request) {
	users, err := a.db.ListUsers()
	if err != nil {
		render.Error(w, r, err)
		return
	}
	b, err := a.templates.Bytes("list_users.html", map[string]interface{}{
		"Users": users,
	})
	if err != nil {
		render.Error(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		logrus.Warn("list users: %v", err)
	}
}

func (a *API) pardotRefresh(w http.ResponseWriter, r *http.Request) {
	users, err := a.db.ListUsers()
	if err != nil {
		render.Error(w, r, err)
		return
	}

	for _, user := range users {
		// tell pardot about the users
		a.pardotClient.UserCreated(user.Email, user.CreatedAt)
		if !user.ApprovedAt.IsZero() {
			a.pardotClient.UserApproved(user.Email, user.ApprovedAt)
		}
	}
}

func (a *API) makeUserAdmin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		render.Error(w, r, users.ErrNotFound)
		return
	}
	admin := r.URL.Query().Get("admin") == "true"
	if err := a.db.SetUserAdmin(userID, admin); err != nil {
		render.Error(w, r, err)
		return
	}
	redirectTo := r.FormValue("redirect_to")
	if redirectTo == "" {
		redirectTo = "/private/api/users"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}
