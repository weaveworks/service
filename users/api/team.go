package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/weaveworks/service/users"
	"github.com/weaveworks/service/users/render"
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
		render.Error(w, r, err)
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

func (a *API) ensureUserPartOfTeam(ctx context.Context, currentUser *users.User, teamExternalID, teamName string) (*users.Team, error) {
	if teamName == "" && teamExternalID == "" {
		return nil, fmt.Errorf("At least one of teamExternalID, teamName needs to be provided")
	}
	if teamName != "" && teamExternalID != "" {
		return nil, fmt.Errorf("Only one of teamExternalID, teamName has to be provided")
	}

	teams, err := a.db.ListTeamsForUserID(ctx, currentUser.ID)
	if err != nil {
		return nil, err
	}
	for _, team := range teams {
		// case insensitive
		if teamName != "" {
			if strings.ToLower(team.Name) == strings.ToLower(teamName) {
				return team, nil
			}
		} else {
			if team.ExternalID == teamExternalID {
				return team, nil
			}
		}
	}

	if teamName == "" {
		return nil, fmt.Errorf("Team name should not be blank. currentUser: %v, teamExternalID %v, teamName %v", currentUser.ID, teamExternalID, teamName)
	}

	// teams does not exists for the user, create it!
	team, err := a.db.CreateTeam(ctx, teamName)
	if err != nil {
		return nil, err
	}

	err = a.db.AddUserToTeam(ctx, currentUser.ID, team.ID)
	if err != nil {
		return nil, err
	}

	return team, nil
}
