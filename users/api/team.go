package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"

	"github.com/weaveworks/service/common/permission"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/common/validation"
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

// RolesView describes an array of user roles
type RolesView struct {
	Roles []RoleView `json:"roles"`
}

// RoleView describes a user role
type RoleView struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// PermissionsView describes an array of permissions
type PermissionsView struct {
	Permissions []PermissionView `json:"permissions"`
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

func (a *API) listRoles(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	roles, err := a.db.ListRoles(r.Context())
	if err != nil {
		renderError(w, r, err)
		return
	}
	view := RolesView{Roles: make([]RoleView, 0, len(roles))}
	for _, role := range roles {
		view.Roles = append(view.Roles, RoleView{
			ID:   role.ID,
			Name: role.Name,
		})
	}
	render.JSON(w, http.StatusOK, view)
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

func (a *API) updateUserRoleInTeam(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	teamExternalID := mux.Vars(r)["teamExternalID"]
	userEmail := mux.Vars(r)["userEmail"]
	ctx := r.Context()

	team, err := a.userCanAccessTeam(ctx, currentUser, teamExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	user, err := a.db.FindUserByEmail(ctx, userEmail)
	if err != nil {
		renderError(w, r, err)
		return
	}

	defer r.Body.Close()
	var update users.TeamMembershipWriteView
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}

	// Users are never allowed to change their roles - even if they're
	// admins, they must promote other admins first who can then downgrade
	// their role - this is to prevent instances ending up with no admins!
	if userEmail == currentUser.Email {
		renderError(w, r, users.ErrForbidden)
		return
	}

	if err := RequireTeamMemberPermissionTo(ctx, a.db, currentUser.ID, teamExternalID, permission.UpdateTeamMemberRole); err != nil {
		renderError(w, r, err)
		return
	}
	// This query might fail for a couple of reasons:
	//   1. The user is not part of the team
	//   2. Role ID is not valid (`admin`, `editor`, `viewer`)
	//      - this check is done implicitly on the DB level
	err = a.db.UpdateUserRoleInTeam(ctx, user.ID, team.ID, update.RoleID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func renderPermissions(permissions []*users.Permission) *PermissionsView {
	view := PermissionsView{Permissions: make([]PermissionView, 0, len(permissions))}
	for _, permission := range permissions {
		view.Permissions = append(view.Permissions, PermissionView{
			ID:          permission.ID,
			Name:        permission.Name,
			Description: permission.Description,
		})
	}
	return &view
}

func (a *API) listTeamPermissions(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	teamExternalID := mux.Vars(r)["teamExternalID"]
	userEmail := mux.Vars(r)["userEmail"]
	ctx := r.Context()

	team, err := a.userCanAccessTeam(ctx, currentUser, teamExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	user, err := a.db.FindUserByEmail(ctx, userEmail)
	if err != nil {
		renderError(w, r, err)
		return
	}

	role, err := a.db.GetUserRoleInTeam(ctx, user.ID, team.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	permissions, err := a.db.ListPermissionsForRoleID(ctx, role.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, renderPermissions(permissions))
}

func (a *API) removeUserFromTeam(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	teamExternalID := mux.Vars(r)["teamExternalID"]
	userEmail := mux.Vars(r)["userEmail"]

	team, err := a.userCanAccessTeam(ctx, currentUser, teamExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	user, err := a.db.FindUserByEmail(ctx, userEmail)
	if err != nil {
		renderError(w, r, err)
		return
	}

	members, err := a.db.ListTeamUsersWithRoles(ctx, team.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	if len(members) == 1 {
		renderError(w, r, users.ErrForbidden)
		return
	}

	role, err := a.db.GetUserRoleInTeam(ctx, user.ID, team.ID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	// A team always has to have at least one admin present,
	// so if the user to be removed is the only admin, deny it.
	// TODO(fbarl): Consider extracting "admin", "editor", "viewer" into constants.
	if role.ID == "admin" {
		adminCount := 0
		for _, m := range members {
			if m.Role.ID == "admin" {
				adminCount++
			}
		}
		if adminCount == 1 {
			renderError(w, r, users.ErrForbidden)
			return
		}
	}

	// All users should be able to remove themselves from the team regardless of their role,
	// so we skip the permission check if a user is trying to remove themselves.
	if userEmail != currentUser.Email {
		if err := RequireTeamMemberPermissionTo(ctx, a.db, currentUser.ID, teamExternalID, permission.RemoveTeamMember); err != nil {
			renderError(w, r, err)
			return
		}
	}

	if err = a.db.RemoveUserFromTeam(ctx, user.ID, team.ID); err != nil {
		renderError(w, r, err)
		return
	}

	render.JSON(w, http.StatusOK, nil)
}

// InviteUserToTeamResponse is the message sent as the result of an invite request
type InviteUserToTeamResponse struct {
	Email  string `json:"email"`
	RoleID string `json:"roleId"`
}

func (a *API) inviteUserToTeam(currentUser *users.User, w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	teamExternalID := mux.Vars(r)["teamExternalID"]

	defer r.Body.Close()

	var resp InviteUserToTeamResponse
	if err := json.NewDecoder(r.Body).Decode(&resp); err != nil {
		renderError(w, r, users.NewMalformedInputError(err))
		return
	}

	email := strings.TrimSpace(resp.Email)
	if email == "" || !validation.ValidateEmail(email) {
		renderError(w, r, users.ErrEmailIsInvalid)
		return
	}

	roleID := strings.TrimSpace(resp.RoleID)
	if roleID == "" {
		roleID = users.DefaultRoleID
	}
	resp.RoleID = roleID

	team, err := a.userCanAccessTeam(ctx, currentUser, teamExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	if err := RequireTeamMemberPermissionTo(ctx, a.db, currentUser.ID, teamExternalID, permission.InviteTeamMember); err != nil {
		renderError(w, r, err)
		return
	}

	invitee, created, err := a.db.InviteUserToTeam(ctx, email, teamExternalID, roleID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	// We always do this so that the timing difference can't be used to infer a user's existence.
	token, err := a.generateUserToken(ctx, invitee)
	if err != nil {
		renderError(w, r, fmt.Errorf("cannot generate user token: %s", err))
		return
	}

	if created {
		err = a.emailer.InviteToTeamEmail(currentUser, invitee, teamExternalID, team.Name, token)
	} else {
		err = a.emailer.GrantAccessToTeamEmail(currentUser, invitee, teamExternalID, team.Name)
	}

	if err != nil {
		renderError(w, r, fmt.Errorf("cannot send invite email: %s", err))
		return
	}

	render.JSON(w, http.StatusOK, resp)
}
