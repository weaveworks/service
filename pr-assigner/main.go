package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"
)

type repository struct {
	owner string
	name  string
}

type pullRequest struct {
	repo       repository
	id         int
	owner      string
	directives map[string][]string
}

type config struct {
	Repositories map[string][]string
}

func searchPullRequests(client *github.Client, repo repository) ([]pullRequest, error) {
	prList, _, err := client.PullRequests.List(repo.owner, repo.name, &github.PullRequestListOptions{
		State: "open",
	})
	if err != nil {
		return nil, err
	}

	var results []pullRequest
	for _, prInfo := range prList {
		if prInfo.Assignee != nil {
			continue
		}
		pr := pullRequest{
			repo:       repo,
			id:         *prInfo.Number,
			owner:      *prInfo.User.Login,
			directives: parseDirectives(prInfo.Body),
		}
		if _, ignore := pr.directives["ignore"]; ignore {
			continue
		}
		results = append(results, pr)
	}

	return results, nil
}

func assignPullRequest(client *github.Client, users map[string]*github.User, pr pullRequest, candidates []string) error {
	contains := func(haystack []string, needle string) bool {
		for _, item := range haystack {
			if item == needle {
				return true
			}
		}
		return false
	}

	if excludes, present := pr.directives["exclude"]; present {
		var newCandidates []string
		for _, candidate := range candidates {
			if !contains(excludes, candidate) {
				newCandidates = append(newCandidates, candidate)
			}
		}
		candidates = newCandidates
	}

	var newCandidates []string
	for _, candidate := range candidates {
		if candidate != pr.owner {
			newCandidates = append(newCandidates, candidate)
		}
	}
	candidates = newCandidates

	if len(candidates) > 0 {
		assignee := users[candidates[rand.Intn(len(candidates))]]

		url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", pr.repo.owner, pr.repo.name, pr.id)
		reqBody, err := json.Marshal(map[string]string{
			"assignee": *assignee.Login,
		})
		if err != nil {
			return err
		}
		req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(reqBody))
		if err != nil {
			return err
		}
		_, err = client.Do(req, nil)
		if err != nil {
			return err
		}
	}
	return nil
}

func parseDirectives(body *string) map[string][]string {
	// A directive is a line like: "pr-assigner: WORD ARG1, ARG2, ..."
	// This function will only parse the word and args, it won't validate
	// the values, number of args, etc.

	// Currently defined directives:
	// ignore (no args) - Do not assign this PR to anyone
	// exclude {ARGS} - Do not assign PR to this person

	result := make(map[string][]string)

	if body == nil {
		return result
	}

	for _, line := range strings.Split(*body, "\n") {
		// split into "pr-assigner:", WORD, ARGS
		parts := strings.SplitN(line, " ", 3)
		if len(parts) < 2 || !strings.EqualFold(parts[0], "pr-assigner:") {
			continue
		}
		key := strings.ToLower(parts[1])
		var values []string
		if len(parts) == 3 {
			for _, arg := range strings.Split(parts[2], ",") {
				values = append(values, strings.Trim(arg, " "))
			}
		}
		result[key] = values
	}
	return result
}

func createClient(token string) *github.Client {
	if len(token) == 0 {
		return github.NewClient(nil)
	}
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(oauth2.NoContext, ts)
	return github.NewClient(tc)
}

func getAllUsers(client *github.Client, repositories map[repository][]string) (map[string]*github.User, error) {
	// return a mapping of usernames to Users for all users referenced by repositories config
	result := make(map[string]*github.User)
	for _, users := range repositories {
		for _, username := range users {
			user, _, err := client.Users.Get(username)
			if err != nil {
				return nil, err
			}
			result[username] = user
		}
	}
	return result, nil
}

func loadConfig(path string) (*config, error) {
	var conf config

	content, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(content, &conf); err != nil {
		return nil, err
	}

	return &conf, nil
}

func parseRepositories(conf *config) (map[repository][]string, error) {
	result := make(map[repository][]string)
	for repoPath, value := range conf.Repositories {
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Bad repository name %v", repoPath)
		}
		result[repository{
			owner: parts[0],
			name:  parts[1],
		}] = value
	}
	return result, nil
}

func main() {
	var (
		oauthToken string
		confPath   string
	)
	flag.StringVar(&oauthToken, "token", "", "oauth token to access github")
	flag.StringVar(&confPath, "conf_path", "/etc/pr-assigner.json", "where to find the config file")
	flag.Parse()

	config, err := loadConfig(confPath)
	if err != nil {
		fmt.Printf("Unable to load config file from %v: %v\n", confPath, err)
		os.Exit(1)
	}
	repositories, err := parseRepositories(config)
	if err != nil {
		fmt.Printf("Error parsing config: %v\n", err)
	}

	client := createClient(oauthToken)
	users, err := getAllUsers(client, repositories)
	if err != nil {
		fmt.Printf("Unable to verify all given usernames: %v\n", err)
		os.Exit(1)
	}

	for {
		for repo, candidates := range repositories {
			prs, err := searchPullRequests(client, repo)
			if err != nil {
				fmt.Printf("error fetching PRs: %v\n", err)
			}
			for _, pr := range prs {
				err := assignPullRequest(client, users, pr, candidates)
				if err != nil {
					fmt.Printf("error assigning to PR: %v\n", err)
				}
			}
		}
		time.Sleep(60 * time.Second)
	}
}
