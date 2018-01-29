package preview

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"gopkg.in/go-playground/webhooks.v3/github"

	"github.com/weaveworks/service/service-ui-kicker/hookdispatcher"
)

const (
	repo = "https://github.com/weaveworks/service-ui.git"
)

// LinkPoster updates commits in github with build preview URLs
type LinkPoster struct {
	token string
}

// New creates a new LinkPoster
func New(token string) *LinkPoster {
	return &LinkPoster{token: token}
}

// Start starts a LinkPoster
func (l *LinkPoster) Start(hs *hookdispatcher.HookDispatcher) {
	events := hs.Listen(repo)
	go func() {
		for payload := range events {
			switch pl := payload.(type) {
			case github.PushPayload:
				continue
			case github.StatusPayload:
				l.HandleStatus(pl)
			default:
				log.Warnf("Unhandled service-ui update hook payload %T!", pl)
				continue
			}
		}
	}()
}

// HandleStatus handles GitHub Commit status updates from the API
func (l *LinkPoster) HandleStatus(pl github.StatusPayload) {
	if !(pl.State == "success" && pl.Context == "ci/circleci: upload") {
		log.WithFields(log.Fields{
			"state": pl.State, "context": pl.Context,
		}).Debugf("Payload not eligible")
		return
	}
	// "target_url": "https://circleci.com/gh/weaveworks/service-ui/5100?utm_campaign=vcs-integration-link&utm_medium=referral&utm_source=github-build-link",
	url, err := url.Parse(*pl.TargetURL)
	if err != nil {
		log.WithField("url", *pl.TargetURL).Errorf("Failed to parse URL")
		return
	}
	parts := strings.Split(url.Path, "/")
	buildID, err := strconv.ParseInt(parts[len(parts)-1], 10, 32)
	if err != nil {
		log.WithField("url", *pl.TargetURL).Errorf("Failed to extract build ID")
		return
	}
	previewURL := fmt.Sprintf("https://%d.build.dev.weave.works/", buildID)
	statusURL := strings.Replace(pl.Repository.StatusesURL, "{sha}", pl.Sha, 1)
	values := map[string]string{
		"state":       "success",
		"target_url":  previewURL,
		"context":     "preview",
		"description": "Interactive preview of this commit",
	}
	jsonValue, _ := json.Marshal(values)

	log.WithFields(log.Fields{
		"sha": pl.Sha, "Preview URL": previewURL,
	}).Info("Posting preview URL")

	req, err := http.NewRequest("POST", statusURL, bytes.NewBuffer(jsonValue))
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", l.token))
	resp, err := http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != 201 {
		log.Warnf("Error posting status link %v %v", resp, err)
	}
}
