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
	"sort"
	"strings"
	"syscall"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/google/go-github/github"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/weaveworks/common/logging"
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
	Repositories map[string]candidateList
}

// implements sort.Interface lexiographically
type candidateList []string

type mainState struct {
	client         *github.Client
	repositories   map[repository]candidateList
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

func init() {
	prometheus.MustRegister(pullsAssigned)
}

func (list candidateList) Len() int {
	return len(list)
}

func (list candidateList) Swap(i, j int) {
	list[i], list[j] = list[j], list[i]
}

func (list candidateList) Less(i, j int) bool {
	return list[i] < list[j]
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

		// special case: treat WIP in the subject same as ignore directive
		if strings.Contains(*prInfo.Title, "WIP") {
			log.Infof("ignoring %v due to WIP in title", pr)
			continue
		}

		if _, ignore := pr.directives["ignore"]; ignore {
			log.Infof("ignoring %v due to ignore directive", pr)
			continue
		}
		results = append(results, pr)
	}

	return results, nil
}

func assignPullRequest(client *github.Client, pr pullRequest, candidates candidateList) error {
	// contains returns whether haystack contains needle. haystack must be sorted.
	contains := func(haystack candidateList, needle string) bool {
		index := sort.Search(len(haystack), func(i int) bool { return haystack[i] >= needle })
		return index < len(haystack) && haystack[index] == needle
	}

	// removeList returns a slice containing all elements of slice a which are not in slice b.
	// b must be sorted.
	removeList := func(a, b candidateList) candidateList {
		result := candidateList{}
		for _, v := range a {
			if !contains(b, v) {
				result = append(result, v)
			}
		}
		return result
	}

	logger := log.WithFields(log.Fields{
		"repo":        pr.repo,
		"pullrequest": pr,
	})

	logger.Debugf("Got candidates: %v", candidates)

	if excludes, ok := pr.directives["exclude"]; ok {
		logger.Debugf("Excluding: %v", excludes)
		sort.Sort(candidateList(excludes))
		candidates = removeList(candidates, excludes)
	}

	logger.Debugf("Excluding owner, %v", pr.owner)
	candidates = removeList(candidates, candidateList{pr.owner})

	logger.Debugf("Candidates after all excludes: %v", candidates)

	if len(candidates) == 0 {
		logger.Infof("No candidates to assign to")
		return nil
	}

	assignee := candidates[rand.Intn(len(candidates))]

	logger.Debugf("Assigning candidate %v", assignee)
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
	logger.Infof("Assigned user %v", assignee)
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

func parseRepositories(conf *config) (map[repository]candidateList, error) {
	result := map[repository]candidateList{}
	for repoPath, value := range conf.Repositories {
		parts := strings.Split(repoPath, "/")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Bad repository name %v", repoPath)
		}
		sort.Sort(value)
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

	flag.StringVar(&oauthToken, "token", "", "An oauth token to access github. May alternately be passed as GITHUB_TOKEN env var.")
	flag.StringVar(&confPath, "conf_path", "/etc/pr-assigner.json", "Where to find the config file.")
	flag.StringVar(&logLevel, "log.level", "info", "Logging level to use: debug | info | warn | error")
	flag.DurationVar(&state.period, "period", time.Minute, "How often to poll github for new PRs.")
	flag.StringVar(&httpListen, "listen", ":80", "host:port for HTTP server to listen on.")
	flag.Parse()

	if len(oauthToken) == 0 {
		oauthToken = os.Getenv("GITHUB_TOKEN")
	}

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
					logger.WithFields(log.Fields{
						"pullrequest": pr,
					}).Errorf("error assigning to PR: %v", err)
				}
			}
		}
	}
}
