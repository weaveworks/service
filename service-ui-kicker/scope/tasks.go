package scope

import (
	"bytes"
	"context"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3/github"

	"github.com/weaveworks/service/service-ui-kicker/handler"
)

const (
	scopeRepo = "https://github.com/weaveworks/scope.git"
)

var tasksCompleted = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "tasks_completed",
		Help: "Number of completed tasks for scope version update.",
	},
	[]string{},
)
var tasksFailed = prometheus.NewCounterVec(
	prometheus.CounterOpts{
		Name: "tasks_failed",
		Help: "Number of failed tasks for scope version update.",
	},
	[]string{},
)
var completedDuration = prometheus.NewHistogram(
	prometheus.HistogramOpts{
		Name: "completed_task_duration",
		Help: "Histogram for the completed tasks duration.",
	},
)

// Init registers prometheus metrics
func Init() {
	prometheus.MustRegister(tasksCompleted, tasksFailed, completedDuration)
}

// getLastSha returns last sha for master branch repository repo
func getLastSha(repo string) string {
	cmd := exec.Command("git", "ls-remote", repo, "refs/heads/master")
	stdout := bytes.NewBuffer(nil)
	stderr := bytes.NewBuffer(nil)
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		log.Errorf("Cannot get last sha in repo %s: %s\n stdout: %s, stderr: %s", repo, err, stdout.String(), stderr.String())
		return ""
	}

	return strings.Fields(stdout.String())[0]
}

// Updater pushes updated scope (and dependencies) into service-ui
type Updater struct {
	mu     sync.Mutex
	latest string
}

// NewUpdater creates a new updater
func NewUpdater() *Updater {
	latest := getLastSha(scopeRepo)
	log.Infof("Set the latest version for Scope updater to %q", latest)
	return &Updater{latest: latest}
}

// Start starts the updater
func (u *Updater) Start(hs *handler.HookServer) {
	events := hs.Listen(scopeRepo)
	go func() {
		log.Info("Start waiting for new tasks...")
		for payload := range events {

			switch pl := payload.(type) {
			case github.PushPayload:
				u.HandlePush(pl)
			case github.StatusPayload:
				u.HandleStatus(pl)
			default:
				log.Warnf("Unhandled scope update hook payload %T!", pl)
				continue
			}

		}
	}()
}

// HandlePush handles GitHub push events
func (u *Updater) HandlePush(pl github.PushPayload) {
	if pl.Ref != "refs/heads/master" {
		return
	}
	u.mu.Lock()
	defer u.mu.Unlock()
	u.latest = pl.HeadCommit.ID
	log.Infof("Push to weaveworks/scope master detected, latest version of weave-scope is %v", pl.HeadCommit.ID)
	log.Info("Waiting till the build finishes successfully")
}

// HandleStatus handles GitHub Commit status updated from the API
func (u *Updater) HandleStatus(pl github.StatusPayload) {
	u.mu.Lock()
	defer u.mu.Unlock()
	if !(pl.Sha == u.latest && pl.State == "success") {
		return
	}
	u.latest = ""

	shortSha := pl.Sha[:8]
	log.Infoln("weaveworks/scope build has finished successfully")

	begin := time.Now()
	if err := PushUpdatedFile(context.Background(), shortSha); err != nil {
		log.Errorf("Cannot clone, commit and push new version: %v", err)
		tasksFailed.With(prometheus.Labels{}).Inc()
	} else {
		log.Infof("Task done: version %q pushed to weaveworks/service-ui", shortSha)
		tasksCompleted.With(prometheus.Labels{}).Inc()
		completedDuration.Observe(time.Since(begin).Seconds())
	}
}
