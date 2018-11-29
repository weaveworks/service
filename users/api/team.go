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

// PermissionsView describes an array of permissions
type PermissionsView struct {
	Permissions []PermissionView `json:"permissions,omitempty"`
}

// PermissionView describes a permission
type PermissionView struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description"`
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

func (a *API) listPermissions(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	user, err := a.db.FindUserByEmail(r.Context(), mux.Vars(r)["userEmail"])
	if err != nil {
		renderError(w, r, err)
		return
	}

	team, err := a.db.FindTeamByExternalID(r.Context(), mux.Vars(r)["teamExternalID"])
	if err != nil {
		renderError(w, r, err)
		return
	}

	role, err := a.db.GetUserRoleInTeam(r.Context(), user.ID, team.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	permissions, err := a.db.ListPermissionsForRoleID(r.Context(), role.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	view := PermissionsView{Permissions: make([]PermissionView, 0, len(permissions))}
	for _, permission := range permissions {
		view.Permissions = append(view.Permissions, PermissionView{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
		})
	}
	render.JSON(w, http.StatusOK, view)
}
