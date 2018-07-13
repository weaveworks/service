package http

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/service/common/constants/webhooks"
)

func TestHandleWebhook(t *testing.T) {
	mockDaemon := &remote.MockServer{
		NotifyChangeError: nil,
	}
	s := Server{daemonProxy: mockDaemon}

	{ // Invalid integration type
		req, err := http.NewRequest("GET", "https://weave.test/webhooks/secret-abc/", nil)
		assert.NoError(t, err)
		req.Header.Set(webhooks.WebhooksIntegrationTypeHeader, "invalid")

		rr := httptest.NewRecorder()
		s.handleWebhook(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	}

	{ // Github success
		payload := []byte(`
			{
				"ref": "refs/tags/simple-tag",
				"before": "a10867b14bb761a232cd80139fbd4c0d33264240",
				"after": "0000000000000000000000000000000000000000",
				"created": false,
				"deleted": true,
				"forced": false,
				"base_ref": null,
				"compare": "https://github.com/Codertocat/Hello-World/compare/a10867b14bb7...000000000000",
				"commits": [],
				"head_commit": null,
				"repository": {
				  "id": 135493233,
				  "node_id": "MDEwOlJlcG9zaXRvcnkxMzU0OTMyMzM=",
				  "name": "Hello-World",
				  "full_name": "Codertocat/Hello-World",
				  "owner": {
					"name": "Codertocat",
					"email": "21031067+Codertocat@users.noreply.github.com",
					"login": "Codertocat",
					"id": 21031067,
					"node_id": "MDQ6VXNlcjIxMDMxMDY3",
					"avatar_url": "https://avatars1.githubusercontent.com/u/21031067?v=4",
					"gravatar_id": "",
					"url": "https://api.github.com/users/Codertocat",
					"html_url": "https://github.com/Codertocat",
					"followers_url": "https://api.github.com/users/Codertocat/followers",
					"following_url": "https://api.github.com/users/Codertocat/following{/other_user}",
					"gists_url": "https://api.github.com/users/Codertocat/gists{/gist_id}",
					"starred_url": "https://api.github.com/users/Codertocat/starred{/owner}{/repo}",
					"subscriptions_url": "https://api.github.com/users/Codertocat/subscriptions",
					"organizations_url": "https://api.github.com/users/Codertocat/orgs",
					"repos_url": "https://api.github.com/users/Codertocat/repos",
					"events_url": "https://api.github.com/users/Codertocat/events{/privacy}",
					"received_events_url": "https://api.github.com/users/Codertocat/received_events",
					"type": "User",
					"site_admin": false
				  },
				  "private": false,
				  "html_url": "https://github.com/Codertocat/Hello-World",
				  "description": null,
				  "fork": false,
				  "url": "https://github.com/Codertocat/Hello-World",
				  "forks_url": "https://api.github.com/repos/Codertocat/Hello-World/forks",
				  "keys_url": "https://api.github.com/repos/Codertocat/Hello-World/keys{/key_id}",
				  "collaborators_url": "https://api.github.com/repos/Codertocat/Hello-World/collaborators{/collaborator}",
				  "teams_url": "https://api.github.com/repos/Codertocat/Hello-World/teams",
				  "hooks_url": "https://api.github.com/repos/Codertocat/Hello-World/hooks",
				  "issue_events_url": "https://api.github.com/repos/Codertocat/Hello-World/issues/events{/number}",
				  "events_url": "https://api.github.com/repos/Codertocat/Hello-World/events",
				  "assignees_url": "https://api.github.com/repos/Codertocat/Hello-World/assignees{/user}",
				  "branches_url": "https://api.github.com/repos/Codertocat/Hello-World/branches{/branch}",
				  "tags_url": "https://api.github.com/repos/Codertocat/Hello-World/tags",
				  "blobs_url": "https://api.github.com/repos/Codertocat/Hello-World/git/blobs{/sha}",
				  "git_tags_url": "https://api.github.com/repos/Codertocat/Hello-World/git/tags{/sha}",
				  "git_refs_url": "https://api.github.com/repos/Codertocat/Hello-World/git/refs{/sha}",
				  "trees_url": "https://api.github.com/repos/Codertocat/Hello-World/git/trees{/sha}",
				  "statuses_url": "https://api.github.com/repos/Codertocat/Hello-World/statuses/{sha}",
				  "languages_url": "https://api.github.com/repos/Codertocat/Hello-World/languages",
				  "stargazers_url": "https://api.github.com/repos/Codertocat/Hello-World/stargazers",
				  "contributors_url": "https://api.github.com/repos/Codertocat/Hello-World/contributors",
				  "subscribers_url": "https://api.github.com/repos/Codertocat/Hello-World/subscribers",
				  "subscription_url": "https://api.github.com/repos/Codertocat/Hello-World/subscription",
				  "commits_url": "https://api.github.com/repos/Codertocat/Hello-World/commits{/sha}",
				  "git_commits_url": "https://api.github.com/repos/Codertocat/Hello-World/git/commits{/sha}",
				  "comments_url": "https://api.github.com/repos/Codertocat/Hello-World/comments{/number}",
				  "issue_comment_url": "https://api.github.com/repos/Codertocat/Hello-World/issues/comments{/number}",
				  "contents_url": "https://api.github.com/repos/Codertocat/Hello-World/contents/{+path}",
				  "compare_url": "https://api.github.com/repos/Codertocat/Hello-World/compare/{base}...{head}",
				  "merges_url": "https://api.github.com/repos/Codertocat/Hello-World/merges",
				  "archive_url": "https://api.github.com/repos/Codertocat/Hello-World/{archive_format}{/ref}",
				  "downloads_url": "https://api.github.com/repos/Codertocat/Hello-World/downloads",
				  "issues_url": "https://api.github.com/repos/Codertocat/Hello-World/issues{/number}",
				  "pulls_url": "https://api.github.com/repos/Codertocat/Hello-World/pulls{/number}",
				  "milestones_url": "https://api.github.com/repos/Codertocat/Hello-World/milestones{/number}",
				  "notifications_url": "https://api.github.com/repos/Codertocat/Hello-World/notifications{?since,all,participating}",
				  "labels_url": "https://api.github.com/repos/Codertocat/Hello-World/labels{/name}",
				  "releases_url": "https://api.github.com/repos/Codertocat/Hello-World/releases{/id}",
				  "deployments_url": "https://api.github.com/repos/Codertocat/Hello-World/deployments",
				  "created_at": 1527711484,
				  "updated_at": "2018-05-30T20:18:35Z",
				  "pushed_at": 1527711528,
				  "git_url": "git://github.com/Codertocat/Hello-World.git",
				  "ssh_url": "git@github.com:Codertocat/Hello-World.git",
				  "clone_url": "https://github.com/Codertocat/Hello-World.git",
				  "svn_url": "https://github.com/Codertocat/Hello-World",
				  "homepage": null,
				  "size": 0,
				  "stargazers_count": 0,
				  "watchers_count": 0,
				  "language": null,
				  "has_issues": true,
				  "has_projects": true,
				  "has_downloads": true,
				  "has_wiki": true,
				  "has_pages": true,
				  "forks_count": 0,
				  "mirror_url": null,
				  "archived": false,
				  "open_issues_count": 2,
				  "license": null,
				  "forks": 0,
				  "open_issues": 2,
				  "watchers": 0,
				  "default_branch": "master",
				  "stargazers": 0,
				  "master_branch": "master"
				},
				"pusher": {
				  "name": "Codertocat",
				  "email": "21031067+Codertocat@users.noreply.github.com"
				},
				"sender": {
				  "login": "Codertocat",
				  "id": 21031067,
				  "node_id": "MDQ6VXNlcjIxMDMxMDY3",
				  "avatar_url": "https://avatars1.githubusercontent.com/u/21031067?v=4",
				  "gravatar_id": "",
				  "url": "https://api.github.com/users/Codertocat",
				  "html_url": "https://github.com/Codertocat",
				  "followers_url": "https://api.github.com/users/Codertocat/followers",
				  "following_url": "https://api.github.com/users/Codertocat/following{/other_user}",
				  "gists_url": "https://api.github.com/users/Codertocat/gists{/gist_id}",
				  "starred_url": "https://api.github.com/users/Codertocat/starred{/owner}{/repo}",
				  "subscriptions_url": "https://api.github.com/users/Codertocat/subscriptions",
				  "organizations_url": "https://api.github.com/users/Codertocat/orgs",
				  "repos_url": "https://api.github.com/users/Codertocat/repos",
				  "events_url": "https://api.github.com/users/Codertocat/events{/privacy}",
				  "received_events_url": "https://api.github.com/users/Codertocat/received_events",
				  "type": "User",
				  "site_admin": false
				}
			  }
		`)

		req, err := http.NewRequest("POST", "https://weave.test/webhooks/secret-abc/", bytes.NewReader(payload))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(webhooks.WebhooksIntegrationTypeHeader, webhooks.GithubPushIntegrationType)
		req.Header.Set("X-Github-Event", "push")

		rr := httptest.NewRecorder()
		s.handleWebhook(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// Same thing but as application/x-www-form-urlencoded
		form := url.Values{}
		form.Add("payload", string(payload))
		req, err = http.NewRequest("POST", "https://weave.test/webhooks/secret-abc/", strings.NewReader(form.Encode()))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set(webhooks.WebhooksIntegrationTypeHeader, webhooks.GithubPushIntegrationType)
		req.Header.Set("X-Github-Event", "push")

		rr = httptest.NewRecorder()
		s.handleWebhook(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)

		// TODO: Assert call to NotifyChange once flux has a method for this.
		// assert.Equal(t, len(mockDaemon.NotifyChangeCalls), 1)
		// expectedChange := v9.Change{
		// 	Kind: v9.GitChange,
		// 	Source: v9.GitUpdate{
		// 		URL:    "git@github.com:Codertocat/Hello-World.git",
		// 		Branch: "refs/tags/simple-tag",
		// 	},
		// }
		// assert.Equal(t, mockDaemon.NotifyChangeCalls[0].Change, expectedChange)
	}
}
