package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/service/common/logging"
	"golang.org/x/oauth2"
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

type mainState struct {
	client         *github.Client
	repositories   map[repository][]string
	period         time.Duration
	cancelMainLoop chan bool
}

var pullsAssigned = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "pulls_assigned",
		Help: "Number of pull requests assigned.",
	},
	[]string{"repo", "user"},
)

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

func assignPullRequest(client *github.Client, pr pullRequest, candidates []string) error {
	contains := func(haystack []string, needle string) bool {
		for _, item := range haystack {
			if item == needle {
				return true
			}
		}
		return false
	}

	if excludes, ok := pr.directives["exclude"]; ok {
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

	if len(candidates) == 0 {
		return nil
	}

	assignee := candidates[rand.Intn(len(candidates))]

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s/issues/%d", pr.repo.owner, pr.repo.name, pr.id)
	reqBody, err := json.Marshal(struct {
		Assignee string `json:"assignee"`
	}{
		Assignee: assignee,
	})
	if err != nil {
		return err
	}
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer(reqBody))
	if err != nil {
		return err
	}
	if _, err = client.Do(req, nil); err != nil {
		return err
	}
	pullsAssigned.With(prometheus.Labels{
		"repo": fmt.Sprintf("%s/%s", pr.repo.owner, pr.repo.name),
		"user": assignee,
	}).Inc()
	return nil
}

func parseDirectives(body *string) map[string][]string {
	// A directive is a line like: "pr-assigner: WORD ARG1, ARG2, ..."
	// This function will only parse the word and args, it won't validate
	// the values, number of args, etc.

	// Currently defined directives:
	// ignore (no args) - Do not assign this PR to anyone
	// exclude {ARGS} - Do not assign PR to this person

	result := map[string][]string{}

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
	result := map[repository][]string{}
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
		oauthToken  string
		confPath    string
		logLevel    string
		httpListen  string
		state       mainState
		httpErrored chan error
	)
	sig := make(chan os.Signal)

	flag.StringVar(&oauthToken, "token", "", "An oauth token to access github.")
	flag.StringVar(&confPath, "conf_path", "/etc/pr-assigner.json", "Where to find the config file.")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.DurationVar(&state.period, "period", time.Minute, "How often to poll github for new PRs.")
	flag.StringVar(&httpListen, "listen", ":80", "host:port for HTTP server to listen on.")
	flag.Parse()

	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)

	if err := logging.Setup(logLevel); err != nil {
		log.Fatalf("Error configuring logging: %v", err)
	}

	config, err := loadConfig(confPath)
	if err != nil {
		log.Fatalf("Unable to load config file from %v: %v", confPath, err)
	}
	state.repositories, err = parseRepositories(config)
	if err != nil {
		log.Fatalf("Error parsing config: %v", err)
	}

	state.client = createClient(oauthToken)
	state.cancelMainLoop = make(chan bool)
	go mainLoop(state)
	defer close(state.cancelMainLoop)

	prometheus.MustRegister(pullsAssigned)

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "pr-assigner running with config: %v\n", state.repositories)
	})
	http.Handle("/metrics", prometheus.Handler())

	go func() {
		httpErrored <- http.ListenAndServe(httpListen, nil)
	}()

	select {
	case err := <-httpErrored:
		log.Errorf("http serve error: %v", err)
	case <-sig:
		log.Info("Shutting down")
	}
}

func mainLoop(state mainState) {
	ticker := time.Tick(state.period)
	for {
		select {
		case <-ticker:
		case <-state.cancelMainLoop:
			return
		}
		for repo, candidates := range state.repositories {
			logger := log.WithFields(log.Fields{
				"repo": repo,
			})
			prs, err := searchPullRequests(state.client, repo)
			if err != nil {
				logger.Errorf("error fetching PRs: %v", err)
			}
			logger.Infof("got %d eligible PRs", len(prs))
			for _, pr := range prs {
				if err := assignPullRequest(state.client, pr, candidates); err != nil {
					temp := logger.WithFields(log.Fields{
						"pullrequest": pr,
					})
					temp.Errorf("error assigning to PR: %v", err)
				}
			}
		}
	}
}
