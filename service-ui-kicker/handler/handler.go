package handler

import (
	"bytes"
	"os/exec"
	"strings"
	"sync"

	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3"
	"gopkg.in/go-playground/webhooks.v3/github"
)

// HookServer handles webhook
type HookServer struct {
	latest string
	mu     sync.Mutex
	tasks  chan string
}

// New returns new hook server
func New(repo string) *HookServer {
	latest := getLastSha(repo)
	log.Infof("Set the latest version for hook server to %q", latest)
	tasks := make(chan string, 8)
	return &HookServer{latest: latest, tasks: tasks}
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

// Tasks returns channel of tasks
func (hs *HookServer) Tasks() <-chan string {
	return hs.tasks
}

// HandlePush handles GitHub push events
func (hs *HookServer) HandlePush(payload interface{}, header webhooks.Header) {
	pl := payload.(github.PushPayload)
	if pl.Ref != "refs/heads/master" {
		return
	}
	hs.mu.Lock()
	defer hs.mu.Unlock()
	hs.latest = pl.HeadCommit.ID
	log.Infof("Push to weaveworks/scope master detected, latest version of weave-scope is %v", pl.HeadCommit.ID)
	log.Info("Waiting till the build finishes successfully")
}

// HandleStatus handles GitHub Commit status updated from the API
func (hs *HookServer) HandleStatus(payload interface{}, header webhooks.Header) {
	pl := payload.(github.StatusPayload)
	if !hs.checkSha(pl) {
		return
	}
	shortSha := pl.Sha[:8]
	log.Infoln("weaveworks/scope build has finished successfully")
	hs.tasks <- shortSha
	log.Infof("New version of weave-scope: %v added to tasks", shortSha)
}

func (hs *HookServer) checkSha(pl github.StatusPayload) bool {
	hs.mu.Lock()
	defer hs.mu.Unlock()
	if !(pl.Sha == hs.latest && pl.State == "success") {
		return false
	}
	hs.latest = ""
	return true
}
