package http

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/weaveworks/flux/api/v9"
	"github.com/weaveworks/flux/remote"
	"github.com/weaveworks/service/common/constants/webhooks"
)

type notifyingServer struct {
	notified []v9.Change
	*remote.MockServer
}

func (s *notifyingServer) NotifyChange(_ context.Context, c v9.Change) error {
	s.notified = append(s.notified, c)
	return nil
}

func (s *notifyingServer) take() []v9.Change {
	wasNotified := s.notified
	s.notified = nil
	return wasNotified
}

func TestHandleWebhook(t *testing.T) {
	mockDaemon := &notifyingServer{
		MockServer: &remote.MockServer{},
	}
	s := Server{daemonProxy: mockDaemon}

	// Make sure this works the way it's supposed to
	mockDaemon.NotifyChange(context.TODO(), v9.Change{})
	assert.Len(t, mockDaemon.take(), 1)
	assert.Len(t, mockDaemon.take(), 0)

	t.Run("invalid integration type", func(t *testing.T) {
		req, err := http.NewRequest("GET", "https://weave.test/webhooks/secret-abc/", nil)
		assert.NoError(t, err)
		req.Header.Set(webhooks.WebhooksIntegrationTypeHeader, "invalid")

		rr := httptest.NewRecorder()
		s.handleWebhook(rr, req)

		assert.Equal(t, http.StatusBadRequest, rr.Code)
	})

	t.Run("github success case", func(t *testing.T) {
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
		assert.Equal(t, []v9.Change{
			{
				Kind: v9.GitChange,
				Source: v9.GitUpdate{
					URL:    "git@github.com:Codertocat/Hello-World.git",
					Branch: "refs/tags/simple-tag",
				},
			},
		}, mockDaemon.take())

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
		assert.Len(t, mockDaemon.take(), 1)

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
	})

	t.Run("DockerHub success case", func(t *testing.T) {
		payload := []byte(`
		{
			"callback_url": "https://registry.hub.docker.com/u/svendowideit/testhook/hook/2141b5bi5i5b02bec211i4eeih0242eg11000a/",
			"push_data": {
			  "images": [
				  "27d47432a69bca5f2700e4dff7de0388ed65f9d3fb1ec645e2bc24c223dc1cc3",
				  "51a9c7c1f8bb2fa19bcd09789a34e63f35abb80044bc10196e304f6634cc582c",
				  "..."
			  ],
			  "pushed_at": 1.417566161e+09,
			  "pusher": "trustedbuilder",
			  "tag": "latest"
			},
			"repository": {
			  "comment_count": 0,
			  "date_created": 1.417494799e+09,
			  "description": "",
			  "dockerfile": "",
			  "full_description": "Docker Hub based automated build from a GitHub repo",
			  "is_official": false,
			  "is_private": true,
			  "is_trusted": true,
			  "name": "testhook",
			  "namespace": "svendowideit",
			  "owner": "svendowideit",
			  "repo_name": "svendowideit/testhook",
			  "repo_url": "https://registry.hub.docker.com/u/svendowideit/testhook/",
			  "star_count": 0,
			  "status": "Active"
			}
		}
		`)

		req, err := http.NewRequest("POST", "https://weave.test/webhooks/secret-abc/", bytes.NewReader(payload))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(webhooks.WebhooksIntegrationTypeHeader, webhooks.DockerHubIntegrationType)

		rr := httptest.NewRecorder()
		s.handleWebhook(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Len(t, mockDaemon.take(), 1)

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
	})

	t.Run("quay.io success case", func(t *testing.T) {
		payload := []byte(`
		{
			"name": "repository",
			"repository": "mynamespace/repository",
			"namespace": "mynamespace",
			"docker_url": "quay.io/mynamespace/repository",
			"homepage": "https://quay.io/repository/mynamespace/repository",
			"updated_tags": [
			  "latest"
			]
		}
		`)

		req, err := http.NewRequest("POST", "https://weave.test/webhooks/secret-abc/", bytes.NewReader(payload))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set(webhooks.WebhooksIntegrationTypeHeader, webhooks.QuayIntegrationType)

		rr := httptest.NewRecorder()
		s.handleWebhook(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Len(t, mockDaemon.take(), 1)

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
	})

	t.Run("bitbucket.org success case", func(t *testing.T) { // Bitbucket.org success
		payload := []byte(`
        {
          "push": {
            "changes": [
              {
                "forced": false,
                "old": {
                  "target": {
                    "hash": "2269f06c06bebc4f4da528a4144390fddd2cc190",
                    "links": {
                      "self": {
                        "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commit/2269f06c06bebc4f4da528a4144390fddd2cc190"
                      },
                      "html": {
                        "href": "https://bitbucket.org/mbridgen/dummy/commits/2269f06c06bebc4f4da528a4144390fddd2cc190"
                      }
                    },
                    "author": {
                      "raw": "Michael Bridgen <michael@weave.works>",
                      "type": "author",
                      "user": {
                        "username": "mbridgen",
                        "display_name": "Michael Bridgen",
                        "account_id": "5bf2cda767e4d1175166c7f6",
                        "links": {
                          "self": {
                            "href": "https://api.bitbucket.org/2.0/users/mbridgen"
                          },
                          "html": {
                            "href": "https://bitbucket.org/mbridgen/"
                          },
                          "avatar": {
                            "href": "https://bitbucket.org/account/mbridgen/avatar/"
                          }
                        },
                        "type": "user",
                        "nickname": "mbridgen",
                        "uuid": "{3995cc2b-9848-486a-908a-8b77534761b5}"
                      }
                    },
                    "summary": {
                      "raw": "Initial commit",
                      "markup": "markdown",
                      "html": "<p>Initial commit</p>",
                      "type": "rendered"
                    },
                    "parents": [],
                    "date": "2018-11-19T14:51:37+00:00",
                    "message": "Initial commit",
                    "type": "commit"
                  },
                  "links": {
                    "commits": {
                      "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commits/master"
                    },
                    "self": {
                      "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/refs/branches/master"
                    },
                    "html": {
                      "href": "https://bitbucket.org/mbridgen/dummy/branch/master"
                    }
                  },
                  "default_merge_strategy": "merge_commit",
                  "merge_strategies": [
                    "merge_commit",
                    "squash",
                    "fast_forward"
                  ],
                  "type": "branch",
                  "name": "master"
                },
                "links": {
                  "commits": {
                    "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commits?include=6db247ef432063e73512b86edf7b4a72da927355&exclude=2269f06c06bebc4f4da528a4144390fddd2cc190"
                  },
                  "html": {
                    "href": "https://bitbucket.org/mbridgen/dummy/branches/compare/6db247ef432063e73512b86edf7b4a72da927355..2269f06c06bebc4f4da528a4144390fddd2cc190"
                  },
                  "diff": {
                    "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/diff/6db247ef432063e73512b86edf7b4a72da927355..2269f06c06bebc4f4da528a4144390fddd2cc190"
                  }
                },
                "truncated": false,
                "commits": [
                  {
                    "hash": "6db247ef432063e73512b86edf7b4a72da927355",
                    "links": {
                      "self": {
                        "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commit/6db247ef432063e73512b86edf7b4a72da927355"
                      },
                      "comments": {
                        "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commit/6db247ef432063e73512b86edf7b4a72da927355/comments"
                      },
                      "patch": {
                        "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/patch/6db247ef432063e73512b86edf7b4a72da927355"
                      },
                      "html": {
                        "href": "https://bitbucket.org/mbridgen/dummy/commits/6db247ef432063e73512b86edf7b4a72da927355"
                      },
                      "diff": {
                        "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/diff/6db247ef432063e73512b86edf7b4a72da927355"
                      },
                      "approve": {
                        "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commit/6db247ef432063e73512b86edf7b4a72da927355/approve"
                      },
                      "statuses": {
                        "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commit/6db247ef432063e73512b86edf7b4a72da927355/statuses"
                      }
                    },
                    "author": {
                      "raw": "Michael Bridgen <mikeb@squaremobius.net>",
                      "type": "author"
                    },
                    "summary": {
                      "raw": "Hello world\n",
                      "markup": "markdown",
                      "html": "<p>Hello world</p>",
                      "type": "rendered"
                    },
                    "parents": [
                      {
                        "type": "commit",
                        "hash": "2269f06c06bebc4f4da528a4144390fddd2cc190",
                        "links": {
                          "self": {
                            "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commit/2269f06c06bebc4f4da528a4144390fddd2cc190"
                          },
                          "html": {
                            "href": "https://bitbucket.org/mbridgen/dummy/commits/2269f06c06bebc4f4da528a4144390fddd2cc190"
                          }
                        }
                      }
                    ],
                    "date": "2018-11-19T17:13:36+00:00",
                    "message": "Hello world\n",
                    "type": "commit"
                  }
                ],
                "created": false,
                "closed": false,
                "new": {
                  "target": {
                    "hash": "6db247ef432063e73512b86edf7b4a72da927355",
                    "links": {
                      "self": {
                        "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commit/6db247ef432063e73512b86edf7b4a72da927355"
                      },
                      "html": {
                        "href": "https://bitbucket.org/mbridgen/dummy/commits/6db247ef432063e73512b86edf7b4a72da927355"
                      }
                    },
                    "author": {
                      "raw": "Michael Bridgen <mikeb@squaremobius.net>",
                      "type": "author"
                    },
                    "summary": {
                      "raw": "Hello world\n",
                      "markup": "markdown",
                      "html": "<p>Hello world</p>",
                      "type": "rendered"
                    },
                    "parents": [
                      {
                        "type": "commit",
                        "hash": "2269f06c06bebc4f4da528a4144390fddd2cc190",
                        "links": {
                          "self": {
                            "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commit/2269f06c06bebc4f4da528a4144390fddd2cc190"
                          },
                          "html": {
                            "href": "https://bitbucket.org/mbridgen/dummy/commits/2269f06c06bebc4f4da528a4144390fddd2cc190"
                          }
                        }
                      }
                    ],
                    "date": "2018-11-19T17:13:36+00:00",
                    "message": "Hello world\n",
                    "type": "commit"
                  },
                  "links": {
                    "commits": {
                      "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/commits/master"
                    },
                    "self": {
                      "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy/refs/branches/master"
                    },
                    "html": {
                      "href": "https://bitbucket.org/mbridgen/dummy/branch/master"
                    }
                  },
                  "default_merge_strategy": "merge_commit",
                  "merge_strategies": [
                    "merge_commit",
                    "squash",
                    "fast_forward"
                  ],
                  "type": "branch",
                  "name": "master"
                }
              }
            ]
          },
          "repository": {
            "scm": "git",
            "website": "",
            "name": "dummy",
            "links": {
              "self": {
                "href": "https://api.bitbucket.org/2.0/repositories/mbridgen/dummy"
              },
              "html": {
                "href": "https://bitbucket.org/mbridgen/dummy"
              },
              "avatar": {
                "href": "https://bytebucket.org/ravatar/%7B6cf7f1fc-003b-4e82-9a31-352d506530b0%7D?ts=go"
              }
            },
            "full_name": "mbridgen/dummy",
            "owner": {
              "username": "mbridgen",
              "display_name": "Michael Bridgen",
              "account_id": "5bf2cda767e4d1175166c7f6",
              "links": {
                "self": {
                  "href": "https://api.bitbucket.org/2.0/users/mbridgen"
                },
                "html": {
                  "href": "https://bitbucket.org/mbridgen/"
                },
                "avatar": {
                  "href": "https://bitbucket.org/account/mbridgen/avatar/"
                }
              },
              "type": "user",
              "nickname": "mbridgen",
              "uuid": "{3995cc2b-9848-486a-908a-8b77534761b5}"
            },
            "type": "repository",
            "is_private": false,
            "uuid": "{6cf7f1fc-003b-4e82-9a31-352d506530b0}"
          },
          "actor": {
            "username": "mbridgen",
            "display_name": "Michael Bridgen",
            "account_id": "5bf2cda767e4d1175166c7f6",
            "links": {
              "self": {
                "href": "https://api.bitbucket.org/2.0/users/mbridgen"
              },
              "html": {
                "href": "https://bitbucket.org/mbridgen/"
              },
              "avatar": {
                "href": "https://bitbucket.org/account/mbridgen/avatar/"
              }
            },
            "type": "user",
            "nickname": "mbridgen",
            "uuid": "{3995cc2b-9848-486a-908a-8b77534761b5}"
          }
        }`)

		req, err := http.NewRequest("POST", "https://weave.test/webhooks/secret-abc/", bytes.NewReader(payload))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Event-Key", "repo:push")
		req.Header.Set(webhooks.WebhooksIntegrationTypeHeader, webhooks.BitbucketOrgPushIntegrationType)

		rr := httptest.NewRecorder()
		s.handleWebhook(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, []v9.Change{
			{
				Kind: v9.GitChange,
				Source: v9.GitUpdate{
					URL:    "git@bitbucket.org:mbridgen/dummy.git",
					Branch: "master",
				},
			},
		}, mockDaemon.take())
	})

	t.Run("gitlab success", func(t *testing.T) {
		payload := []byte(`
        {
          "object_kind": "push",
          "before": "95790bf891e76fee5e1747ab589903a6a1f80f22",
          "after": "da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
          "ref": "refs/heads/master",
          "checkout_sha": "da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
          "user_id": 4,
          "user_name": "John Smith",
          "user_username": "jsmith",
          "user_email": "john@example.com",
          "user_avatar": "https://s.gravatar.com/avatar/d4c74594d841139328695756648b6bd6?s=8://s.gravatar.com/avatar/d4c74594d841139328695756648b6bd6?s=80",
          "project_id": 15,
          "project":{
            "id": 15,
            "name":"Diaspora",
            "description":"",
            "web_url":"http://example.com/mike/diaspora",
            "avatar_url":null,
            "git_ssh_url":"git@example.com:mike/diaspora.git",
            "git_http_url":"http://example.com/mike/diaspora.git",
            "namespace":"Mike",
            "visibility_level":0,
            "path_with_namespace":"mike/diaspora",
            "default_branch":"master",
            "homepage":"http://example.com/mike/diaspora",
            "url":"git@example.com:mike/diaspora.git",
            "ssh_url":"git@example.com:mike/diaspora.git",
            "http_url":"http://example.com/mike/diaspora.git"
          },
          "repository":{
            "name": "Diaspora",
            "url": "git@example.com:mike/diaspora.git",
            "description": "",
            "homepage": "http://example.com/mike/diaspora",
            "git_http_url":"http://example.com/mike/diaspora.git",
            "git_ssh_url":"git@example.com:mike/diaspora.git",
            "visibility_level":0
          },
          "commits": [
            {
              "id": "b6568db1bc1dcd7f8b4d5a946b0b91f9dacd7327",
              "message": "Update Catalan translation to e38cb41.",
              "timestamp": "2011-12-12T14:27:31+02:00",
              "url": "http://example.com/mike/diaspora/commit/b6568db1bc1dcd7f8b4d5a946b0b91f9dacd7327",
              "author": {
                "name": "Jordi Mallach",
                "email": "jordi@softcatala.org"
              },
              "added": ["CHANGELOG"],
              "modified": ["app/controller/application.rb"],
              "removed": []
            },
            {
              "id": "da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
              "message": "fixed readme",
              "timestamp": "2012-01-03T23:36:29+02:00",
              "url": "http://example.com/mike/diaspora/commit/da1560886d4f094c3e6c9ef40349f7d38b5d27d7",
              "author": {
                "name": "GitLab dev user",
                "email": "gitlabdev@dv6700.(none)"
              },
              "added": ["CHANGELOG"],
              "modified": ["app/controller/application.rb"],
              "removed": []
            }
          ],
          "total_commits_count": 4
        }`)

		req, err := http.NewRequest("POST", "https://weave.test/webhooks/secret-abc/", bytes.NewReader(payload))
		assert.NoError(t, err)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Gitlab-Event", "Push Hook")
		req.Header.Set(webhooks.WebhooksIntegrationTypeHeader, webhooks.GitlabPushIntegrationType)

		rr := httptest.NewRecorder()
		s.handleWebhook(rr, req)

		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, []v9.Change{
			{
				Kind: v9.GitChange,
				Source: v9.GitUpdate{
					URL:    "git@example.com:mike/diaspora.git",
					Branch: "master",
				},
			},
		}, mockDaemon.take())

	})

}
