package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"github.com/weaveworks/common/logging"
	commonuser "github.com/weaveworks/common/user"
	billing_grpc "github.com/weaveworks/service/common/billing/grpc"
	"github.com/weaveworks/service/common/featureflag"
	"github.com/weaveworks/service/common/orgs"
	"github.com/weaveworks/service/common/render"
	"github.com/weaveworks/service/users"
	users_sync "github.com/weaveworks/service/users-sync/api"
	"github.com/weaveworks/service/users/client"
	"github.com/weaveworks/service/users/db/filter"
	"github.com/weaveworks/service/users/login"
	"github.com/weaveworks/service/users/weeklyreports"
)

// AdminTeamView represents a team to display in the admin listing.
type AdminTeamView struct {
	*users.Team
	BillingAccount billing_grpc.BillingAccount
}

func (a *API) admin(w http.ResponseWriter, r *http.Request) {
	w.Header().Add("Content-Type", "text/html")
	fmt.Fprintf(w, `
<!doctype html>
<html>
	<head><title>Users Service</title></head>
	<body>
		<h1>Users Service</h1>
		<ul>
			<li><a href="/admin/users/users">Users</a></li>
			<li><a href="/admin/users/organizations">Organizations</a></li>
			<li><a href="/admin/users/teams">Teams</a></li>
			<li><a href="/admin/users/weeklyreports">Weekly Reports</a></li>
		</ul>
	</body>
</html>
`)
}

type listUsersView struct {
	Users []privateUserView `json:"organizations"`
}

type privateUserView struct {
	ID           string `json:"id"`
	Email        string `json:"email"`
	CreatedAt    string `json:"created_at"`
	FirstLoginAt string `json:"first_login_at"`
	LastLoginAt  string `json:"last_login_at"`
	Admin        bool   `json:"admin"`
}

type tableHeaderView struct {
	Label template.HTML
	Link  template.URL
}

func (a *API) adminListUsers(w http.ResponseWriter, r *http.Request) {
	page := filter.ParsePageValue(r.FormValue("page"))
	query := r.FormValue("query")
	f := filter.And(filter.ParseUserQuery(query))
	us, err := a.db.ListUsers(r.Context(), f, page)
	if err != nil {
		renderError(w, r, err)
		return
	}
	switch render.Format(r) {
	case render.FormatJSON:
		view := listUsersView{}
		for _, user := range us {
			view.Users = append(view.Users, privateUserView{
				ID:           user.ID,
				Email:        user.Email,
				CreatedAt:    user.FormatCreatedAt(),
				FirstLoginAt: user.FormatFirstLoginAt(),
				LastLoginAt:  user.FormatLastLoginAt(),
				Admin:        user.Admin,
			})
		}
		render.JSON(w, http.StatusOK, view)
	default: // render.FormatHTML
		b, err := a.templates.Bytes("list_users.html", map[string]interface{}{
			"Users":    us,
			"Query":    r.FormValue("query"),
			"Page":     page,
			"NextPage": page + 1,
			"Message":  r.FormValue("msg"),
		})
		if err != nil {
			renderError(w, r, err)
			return
		}
		if _, err := w.Write(b); err != nil {
			commonuser.LogWith(r.Context(), logging.Global()).Warnf("list users: %v", err)
		}
	}
}

func (a *API) adminListUsersForOrganization(w http.ResponseWriter, r *http.Request) {
	orgID, ok := mux.Vars(r)["orgExternalID"]
	if !ok {
		renderError(w, r, users.ErrNotFound)
		return
	}

	org, err := a.db.FindOrganizationByID(r.Context(), orgID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	usersRoles, err := a.db.ListTeamUsersWithRoles(r.Context(), org.TeamID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	// Generate an ordered array of roles with aligned indices to users array.
	us := []users.User{}
	roles := []users.Role{}
	for _, userRole := range usersRoles {
		us = append(us, userRole.User)
		roles = append(roles, userRole.Role)
	}

	b, err := a.templates.Bytes("list_users.html", map[string]interface{}{
		"Users":         us,
		"Roles":         roles,
		"OrgExternalID": orgID,
		"Message":       r.FormValue("msg"),
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		commonuser.LogWith(r.Context(), logging.Global()).Warnf("list users: %v", err)
	}
}

func (a *API) adminRemoveUserFromOrganization(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID := vars["orgExternalID"]
	userID := vars["userID"]

	if members, err := a.db.ListOrganizationUsers(r.Context(), orgExternalID, false, false); err != nil {
		renderError(w, r, err)
		return
	} else if len(members) == 1 {
		// An organization cannot be with zero members
		renderError(w, r, users.ErrForbidden)
		return
	}

	user, err := a.db.FindUserByID(r.Context(), userID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	if err := a.db.RemoveUserFromOrganization(r.Context(), orgExternalID, user.Email); err != nil {
		renderError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/users/organizations/"+orgExternalID+"/users", http.StatusFound)
}

func (a *API) adminWeeklyReportsControlPanel(w http.ResponseWriter, r *http.Request) {
	b, err := a.templates.Bytes("weekly_report_emails_panel.html", map[string]interface{}{
		"UserEmail":     r.FormValue("UserEmail"),
		"OrgExternalID": r.FormValue("OrgExternalID"),
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		commonuser.LogWith(r.Context(), logging.Global()).Warnf("weekly report email: %v", err)
	}
}

func (a *API) adminWeeklyReportsTriggerJob(w http.ResponseWriter, r *http.Request) {
	if _, err := a.usersSyncClient.EnforceWeeklyReporterJob(r.Context(), &users_sync.EnforceWeeklyReporterJobRequest{}); err != nil {
		renderError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/users/weeklyreports", http.StatusFound)
}

func (a *API) adminWeeklyReportsSendSingle(w http.ResponseWriter, r *http.Request) {
	if _, err := a.grpc.SendOutWeeklyReport(r.Context(), &users.SendOutWeeklyReportRequest{
		Now:        time.Now(),
		ExternalID: r.FormValue("OrgExternalID"),
	}); err != nil {
		renderError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/users/weeklyreports", http.StatusFound)
}

func (a *API) adminWeeklyReportsPreview(w http.ResponseWriter, r *http.Request) {
	user, err := a.db.FindUserByEmail(r.Context(), r.FormValue("UserEmail"))
	if err != nil {
		renderError(w, r, err)
		return
	}
	org, err := a.db.FindOrganizationByID(r.Context(), r.FormValue("OrgExternalID"))
	if err != nil {
		renderError(w, r, err)
		return
	}
	weeklyReport, err := weeklyreports.GenerateReport(org, time.Now())
	if err != nil {
		renderError(w, r, err)
		return
	}
	err = a.emailer.WeeklyReportEmail([]*users.User{user}, weeklyReport)
	if err != nil {
		renderError(w, r, err)
		return
	}
	http.Redirect(w, r, "/admin/users/weeklyreports", http.StatusFound)
}

func getTableHeaders(requestURL url.URL, sortBy string, sortAscending bool) []tableHeaderView {
	tableHeaders := []string{
		"ID",
		"Name",
		"Team",
		"CreatedAt",
		"LastSentWeeklyReportAt",
		"DeletedAt",
		"Platform",
		"PlatformVersion",
		"Fields",
		"TrialRemaining",
		"Billing",
		"Admin",
	}

	labels := map[string]template.HTML{
		"ID":                     "ID",
		"Name":                   "Name<br />Instance",
		"Team":                   "Team",
		"CreatedAt":              "CreatedAt<br />FirstSeenConnectedAt",
		"LastSentWeeklyReportAt": "LastSentWeeklyReportAt",
		"DeletedAt":              "DeletedAt",
		"Platform":               "Platform / Env",
		"PlatformVersion":        "K8s version",
		"Fields":                 "Fields",
		"TrialRemaining":         "Trial days remaining",
		"Billing":                "Billing",
		"Admin":                  "Admin",
	}

	headers := []tableHeaderView{}
	sortableColumns := getSortableColumns()
	for _, id := range tableHeaders {
		label := labels[id]
		link := ""
		if sortableColumns[id] != "" {
			linkURL := requestURL
			q := linkURL.Query()
			q.Set("sortBy", id)
			q.Del("sortAscending")
			if sortBy == id {
				if !sortAscending {
					q.Set("sortAscending", "1")
					label = template.HTML(fmt.Sprintf("▼ %s", label))
				} else {
					label = template.HTML(fmt.Sprintf("▲ %s", label))
				}
			}
			linkURL.RawQuery = q.Encode()
			link = linkURL.String()
		}
		headers = append(headers, tableHeaderView{
			Label: label,
			Link:  template.URL(link),
		})
	}

	return headers
}

func getSortableColumns() map[string]string {
	// PlatformVersion
	//  - Sort versions nicely
	//  - nullif converts empty list to NULL. So we can put them at last when combined with "nulls last".
	return map[string]string{
		"ID":                     "organizations.id::int",
		"Name":                   "organizations.name",
		"Team":                   "teams.external_id",
		"CreatedAt":              "organizations.created_at",
		"LastSentWeeklyReportAt": "organizations.last_sent_weekly_report_at",
		"DeletedAt":              "organizations.deleted_at",
		"Platform":               "organizations.platform",
		"PlatformVersion":        "nullif(string_to_array(regexp_replace(organizations.platform_version, '[^0-9.]', '', 'g'), '.')::int[],'{}')",
		"TrialRemaining":         "organizations.trial_expires_at",
	}
}

func getNextPageLink(requestURL url.URL) template.URL {
	q := requestURL.Query()
	nextPage := filter.ParsePageValue(q.Get("page")) + 1
	q.Set("page", fmt.Sprintf("%v", nextPage))
	requestURL.RawQuery = q.Encode()
	return template.URL(requestURL.String())
}

func parseSortParams(r http.Request) (string, bool) {
	sortByFormValue := r.FormValue("sortBy")
	sortAscendingFormValue := r.FormValue("sortAscending")

	sortBy := "CreatedAt"
	if sortByFormValue != "" && getSortableColumns()[sortByFormValue] != "" {
		sortBy = sortByFormValue
	}

	sortAscending := false
	if sortAscendingFormValue != "" {
		sortAscending = true
	}

	return sortBy, sortAscending
}

func getOrderClause(sortBy string, sortAscending bool) string {
	sortableColumns := getSortableColumns()
	sortDirection := "desc"
	if sortAscending {
		sortDirection = "asc"
	}
	return fmt.Sprintf("%s %s nulls last", sortableColumns[sortBy], sortDirection)
}

func (a *API) adminListOrganizations(w http.ResponseWriter, r *http.Request) {
	page := filter.ParsePageValue(r.FormValue("page"))
	query := r.FormValue("query")

	sortBy, sortAscending := parseSortParams(*r)
	organizations, err := a.db.ListAllOrganizations(r.Context(), filter.ParseOrgQuery(query), getOrderClause(sortBy, sortAscending), page)
	if err != nil {
		renderError(w, r, err)
		return
	}

	orgUsers, moreUsersCount, err := a.GetOrganizationsUsers(r.Context(), organizations, 3)
	if err != nil {
		renderError(w, r, err)
		return
	}

	b, err := a.templates.Bytes("list_organizations.html", map[string]interface{}{
		"Organizations":      organizations,
		"OrganizationUsers":  orgUsers,
		"MoreUsersCount":     moreUsersCount,
		"Query":              r.FormValue("query"),
		"Page":               page,
		"Message":            r.FormValue("msg"),
		"BillingFeatureFlag": featureflag.Billing,
		"Headers":            getTableHeaders(*r.URL, sortBy, sortAscending),
		"NextPageLink":       getNextPageLink(*r.URL),
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		commonuser.LogWith(r.Context(), logging.Global()).Warnf("list organizations: %v", err)
	}
}

func (a *API) adminListOrganizationsForUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		renderError(w, r, users.ErrNotFound)
		return
	}
	user, err := a.db.FindUserByID(r.Context(), userID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	sortBy, sortAscending := parseSortParams(*r)
	organizations, err := a.db.ListAllOrganizationsForUserIDs(r.Context(), getOrderClause(sortBy, sortAscending), userID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	orgUsers, moreUsersCount, err := a.GetOrganizationsUsers(r.Context(), organizations, 3)
	if err != nil {
		renderError(w, r, err)
		return
	}

	b, err := a.templates.Bytes("list_organizations.html", map[string]interface{}{
		"Organizations":      organizations,
		"OrganizationUsers":  orgUsers,
		"MoreUsersCount":     moreUsersCount,
		"UserEmail":          user.Email,
		"BillingFeatureFlag": featureflag.Billing,
		"Headers":            getTableHeaders(*r.URL, sortBy, sortAscending),
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		commonuser.LogWith(r.Context(), logging.Global()).Warnf("list organizations: %v", err)
	}
}

func (a *API) adminListTeams(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	page := filter.ParsePageValue(r.FormValue("page"))
	query := r.FormValue("query")

	teams, err := a.db.ListAllTeams(ctx, filter.ParseTeamQuery(query), "", page)
	if err != nil {
		renderError(w, r, err)
		return
	}

	teamViews, err := a.adminTeamViews(ctx, teams)
	if err != nil {
		renderError(w, r, err)
		return
	}

	b, err := a.templates.Bytes("list_teams.html", map[string]interface{}{
		"Teams":        teamViews,
		"Query":        r.FormValue("query"),
		"Page":         page,
		"Message":      r.FormValue("msg"),
		"NextPageLink": getNextPageLink(*r.URL),
	})
	if err != nil {
		renderError(w, r, err)
		return
	}
	if _, err := w.Write(b); err != nil {
		commonuser.LogWith(r.Context(), logging.Global()).Warnf("list teams: %v", err)
	}
}

func (a *API) adminTeamViews(ctx context.Context, teams []*users.Team) ([]AdminTeamView, error) {
	var views []AdminTeamView
	for _, t := range teams {
		ba, err := a.billingClient.FindBillingAccountByTeamID(ctx, &billing_grpc.BillingAccountByTeamIDRequest{TeamID: t.ID})
		if err != nil {
			return nil, err
		}
		views = append(views, AdminTeamView{Team: t, BillingAccount: *ba})
	}
	return views, nil
}

func (a *API) adminChangeTeamBilling(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	teamID, ok := vars["teamID"]
	if !ok {
		renderError(w, r, users.ErrNotFound)
		return
	}

	provider := r.FormValue("provider")
	_, err := a.billingClient.SetTeamBillingAccountProvider(r.Context(),
		&billing_grpc.BillingAccountProviderRequest{TeamID: teamID, Provider: provider})
	if err != nil {
		renderError(w, r, users.ErrNotFound)
		return
	}
	redirectWithMessage(w, r, fmt.Sprintf("Updated billing provider to %q for team %s", provider, teamID))
}

func (a *API) adminChangeOrgFields(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID, ok := vars["orgExternalID"]
	if !ok {
		renderError(w, r, users.ErrNotFound)
		return
	}

	// Single value `field=foo, value=bar`
	if r.FormValue("field") != "" {
		if err := a.setOrganizationField(r.Context(), orgExternalID, r.FormValue("field"), r.FormValue("value")); err != nil {
			renderError(w, r, err)
			return
		}
	} else { // Multi value `foo=bar, moo=zar`
		fields := [...]string{"FeatureFlags", orgs.RefuseDataAccess, orgs.RefuseDataUpload, "RefuseDataReason"}
		var errs []string
		for _, field := range fields {
			if err := a.setOrganizationField(r.Context(), orgExternalID, field, r.FormValue(field)); err != nil {
				errs = append(errs, err.Error())
			}
		}

		if len(errs) > 0 {
			renderError(w, r, errors.New(strings.Join(errs, "; ")))
			return
		}
	}

	redirectWithMessage(w, r, fmt.Sprintf("Saved config for %s", orgExternalID))
}

func (a *API) setOrganizationField(ctx context.Context, orgExternalID, field, value string) error {
	var err error
	switch field {
	case "FirstSeenConnectedAt":
		now := time.Now()
		err = a.db.SetOrganizationFirstSeenConnectedAt(ctx, orgExternalID, &now)
	case orgs.RefuseDataAccess:
		deny := value == "on"
		err = a.db.SetOrganizationRefuseDataAccess(ctx, orgExternalID, deny)
	case orgs.RefuseDataUpload:
		deny := value == "on"
		err = a.db.SetOrganizationRefuseDataUpload(ctx, orgExternalID, deny)
	case "RefuseDataReason":
		err = a.db.SetOrganizationRefuseDataReason(ctx, orgExternalID, value)
	case "FeatureFlags":
		err = a.setOrgFeatureFlags(ctx, orgExternalID, strings.Fields(value))
	default:
		err = users.ValidationErrorf("Invalid field %v", field)
	}
	return err
}

func (a *API) adminTrial(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orgExternalID, ok := vars["orgExternalID"]
	if !ok {
		renderError(w, r, users.ErrNotFound)
		return
	}

	remaining, err := strconv.Atoi(r.FormValue("remaining"))
	if err != nil {
		renderError(w, r, err)
		return
	}

	org, err := a.db.FindOrganizationByID(r.Context(), orgExternalID)
	if err != nil {
		renderError(w, r, err)
		return
	}

	email := r.FormValue("email") == "on"
	if err := a.extendOrgTrialPeriod(r.Context(), org, time.Now().UTC().Add(time.Duration(remaining)*24*time.Hour), email); err != nil {
		renderError(w, r, err)
		return
	}
	redirectWithMessage(w, r, fmt.Sprintf("Extended trial to %d remaining days for %s", remaining, orgExternalID))
}

func (a *API) adminMakeUserAdmin(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID, ok := vars["userID"]
	if !ok {
		renderError(w, r, users.ErrNotFound)
		return
	}
	admin := r.FormValue("admin") == "true"
	if err := a.MakeUserAdmin(r.Context(), userID, admin); err != nil {
		renderError(w, r, err)
		return
	}
	redirectTo := r.FormValue("redirect_to")
	if redirectTo == "" {
		redirectTo = "/admin/users/users"
	}
	http.Redirect(w, r, redirectTo, http.StatusFound)
}

func (a *API) adminBecomeUser(w http.ResponseWriter, r *http.Request) {
	commonuser.LogWith(r.Context(), logging.Global()).Infoln(r)
	vars := mux.Vars(r)
	becomeID, ok := vars["userID"]
	if !ok {
		renderError(w, r, users.ErrNotFound)
		return
	}
	u, err := a.db.FindUserByID(r.Context(), becomeID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	session, err := a.sessions.Get(r)
	if err != nil {
		return
	}
	// If we are already impersonating we will get the impersonating id
	// here which we then keep as impersonator for the new user.
	userID := session.GetActingUserID()
	if err := a.sessions.Set(w, r, u.ID, userID); err != nil {
		renderError(w, r, users.ErrInvalidAuthenticationData)
		return
	}
	http.Redirect(w, r, "/", http.StatusFound)
}

func (a *API) adminDeleteUser(w http.ResponseWriter, r *http.Request) {
	userID := mux.Vars(r)["userID"]
	if userID == "" {
		renderError(w, r, users.ErrNotFound)
		return
	}

	session, err := a.sessions.Get(r)
	if err != nil {
		return
	}
	actingID := session.GetActingUserID()
	if err := a.db.DeleteUser(r.Context(), userID, actingID); err != nil {
		renderError(w, r, err)
		return
	}

	redirectWithMessage(w, r, fmt.Sprintf("Deleted user with id %s", userID))
}

func (a *API) adminDeleteOrganization(w http.ResponseWriter, r *http.Request) {
	externalID := mux.Vars(r)["orgExternalID"]
	if externalID == "" {
		renderError(w, r, users.ErrNotFound)
		return
	}

	session, err := a.sessions.Get(r)
	if err != nil {
		return
	}
	userID := session.GetActingUserID()
	if err := a.db.DeleteOrganization(r.Context(), externalID, userID); err != nil {
		renderError(w, r, err)
		return
	}

	redirectWithMessage(w, r, fmt.Sprintf("Deleted organization with id %s", externalID))
}

func (a *API) adminGetUserToken(w http.ResponseWriter, r *http.Request) {
	// Get User ID from path
	vars := mux.Vars(r)
	userIDOrEmail, ok := vars["userID"]
	if !ok {
		renderError(w, r, users.ErrProviderParameters)
		return
	}
	provider, ok := vars["provider"]
	if !ok {
		renderError(w, r, users.ErrProviderParameters)
		return
	}

	// Does user exist?
	userID := ""
	user, err := a.db.FindUserByID(r.Context(), userIDOrEmail)
	if err == nil {
		userID = user.ID
	} else {
		if err == users.ErrNotFound {
			user, err := a.db.FindUserByEmail(r.Context(), userIDOrEmail)
			if err != nil {
				renderError(w, r, err)
				return
			}
			userID = user.ID
		} else {
			renderError(w, r, err)
			return
		}
	}

	// Get logins for user
	logins, err := a.db.ListLoginsForUserIDs(r.Context(), userID)
	if err != nil {
		renderError(w, r, err)
		return
	}
	l, err := getSpecificLogin(provider, logins)
	if err != nil {
		renderError(w, r, err)
		return
	}

	// Parse session information to get token
	tok, err := parseTokenFromSession(l.Session)
	if err != nil {
		renderError(w, r, err)
		return
	}
	render.JSON(w, 200, client.ProviderToken{
		Token: tok,
	})
	return
}

func getSpecificLogin(login string, logins []*login.Login) (*login.Login, error) {
	for _, l := range logins {
		if l.Provider == login {
			return l, nil
		}
	}
	return nil, users.ErrLoginNotFound
}

func parseTokenFromSession(session json.RawMessage) (string, error) {
	b, err := session.MarshalJSON()
	if err != nil {
		return "", err
	}
	var sess struct {
		Token struct {
			AccessToken string `json:"access_token"`
		} `json:"token"`
	}
	err = json.Unmarshal(b, &sess)
	return sess.Token.AccessToken, err
}

func redirectWithMessage(w http.ResponseWriter, r *http.Request, msg string) {
	u := r.URL
	query := u.Query()
	query.Set("msg", msg)
	path := strings.Join(strings.Split(u.Path, "/")[:4], "/")
	http.Redirect(w, r, fmt.Sprintf("%s?%s", path, query.Encode()), http.StatusFound)
}

// MakeUserAdmin makes a user an admin
func (a *API) MakeUserAdmin(ctx context.Context, userID string, admin bool) error {
	return a.db.SetUserAdmin(ctx, userID, admin)
}

// GetOrganizationsUsers gets the user meta
func (a *API) GetOrganizationsUsers(ctx context.Context, organizations []*users.Organization, limit int) (map[string][]*users.User, map[string]int, error) {
	orgUsers := make(map[string][]*users.User)
	moreUsersCount := make(map[string]int)
	for _, org := range organizations {
		us, err := a.db.ListOrganizationUsers(ctx, org.ExternalID, true, false)
		if err != nil {
			return nil, nil, err
		}
		// If more than n users then return the first (n - 1) users
		// leaving a spare line for a "and x more" message.
		more := 0
		if len(us) > limit {
			displayedUserCount := limit - 1
			more = len(us) - displayedUserCount
			us = us[:displayedUserCount]
		}

		orgUsers[org.ExternalID] = us
		moreUsersCount[org.ExternalID] = more
	}

	return orgUsers, moreUsersCount, nil
}
