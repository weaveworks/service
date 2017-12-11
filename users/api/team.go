package api

import (
	"net/http"

	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
)

// TeamsView describes an array of teams
type TeamsView struct {
	Teams []TeamView `json:"teams,omitempty"`
}

// TeamView describes a team
type TeamView struct {
	ExternalID string `json:"id"`
	Name       string `json:"name"`
}

func (a *API) listTeams(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	teams, err := a.db.ListTeamsForUserID(r.Context(), currentUser.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	view := TeamsView{Teams: make([]TeamView, 0, len(teams))}
	for _, team := range teams {
		view.Teams = append(view.Teams, TeamView{
			ExternalID: team.ExternalID,
			Name:       team.Name,
		})
	}
	render.JSON(w, http.StatusOK, view)
}
