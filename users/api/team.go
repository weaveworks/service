package api

import (
	"context"
	"net/http"

	"github.com/gorilla/mux"

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

func (a *API) userCanAccessTeam(ctx context.Context, currentUser *users.User, teamExternalID string) (*users.Team, error) {
	teams, err := a.db.ListTeamsForUserID(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}
	for _, t := range teams {
		if t.ExternalID == teamExternalID {
			return t, nil
		}
	}
	return nil, users.ErrForbidden
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

func (a *API) deleteTeam(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	teamExternalID := mux.Vars(r)["teamExternalID"]
	team, err := a.userCanAccessTeam(r.Context(), currentUser, teamExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	if err := a.db.DeleteTeam(r.Context(), team.ID); err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
