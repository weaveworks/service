package update

import (
	"bytes"
	"errors"
	"fmt"
	"strings"

	"github.com/go-kit/kit/log"

	"github.com/weaveworks/flux"
	"github.com/weaveworks/flux/image"
	"github.com/weaveworks/flux/resource"
)

var zeroImageRef = image.Ref{}

// ContainerSpecs defines the spec for a `containers` manifest update.
type ContainerSpecs struct {
	Kind           ReleaseKind
	ContainerSpecs map[flux.ResourceID][]ContainerUpdate
	SkipMismatches bool
}

// CalculateRelease computes required controller updates to satisfy this specification.
// It returns an error if any spec calculation fails unless `SkipMismatches` is true.
func (s ContainerSpecs) CalculateRelease(rc ReleaseContext, logger log.Logger) ([]*ControllerUpdate, Result, error) {
	all, results, err := s.selectServices(rc)
	if err != nil {
		return nil, results, err
	}
	updates := s.controllerUpdates(results, all)

	failures := 0
	successes := 0
	for _, res := range results {
		switch res.Status {
		case ReleaseStatusFailed:
			failures++
		case ReleaseStatusSuccess:
			successes++
		}
	}
	if failures > 0 {
		return updates, results, errors.New("cannot satisfy specs")
	}
	if successes == 0 {
		return updates, results, errors.New("no changes found")
	}

	return updates, results, nil
}

func (s ContainerSpecs) selectServices(rc ReleaseContext) ([]*ControllerUpdate, Result, error) {
	results := Result{}
	var rids []flux.ResourceID
	for rid := range s.ContainerSpecs {
		rids = append(rids, rid)
	}
	all, err := rc.SelectServices(results, []ControllerFilter{&IncludeFilter{IDs: rids}}, nil)
	if err != nil {
		return nil, results, err
	}
	return all, results, nil
}

func (s ContainerSpecs) controllerUpdates(results Result, all []*ControllerUpdate) []*ControllerUpdate {
	var updates []*ControllerUpdate
	for _, u := range all {
		cs, err := u.Controller.ContainersOrError()
		if err != nil {
			results[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusFailed,
				Error:  err.Error(),
			}
			continue
		}

		containers := map[string]resource.Container{}
		for _, spec := range cs {
			containers[spec.Name] = spec
		}

		var mismatch, notfound []string
		var containerUpdates []ContainerUpdate
		for _, spec := range s.ContainerSpecs[u.ResourceID] {
			container, ok := containers[spec.Container]
			if !ok {
				notfound = append(notfound, spec.Container)
				continue
			}

			// An empty spec for the current image skips the precondition
			if spec.Current != zeroImageRef && container.Image != spec.Current {
				mismatch = append(mismatch, spec.Container)
				continue
			}

			if container.Image == spec.Target {
				// Nothing to update
				continue
			}

			containerUpdates = append(containerUpdates, spec)
		}

		mismatchError := fmt.Sprintf(ContainerTagMismatch, strings.Join(mismatch, ", "))

		var rerr string
		skippedMismatches := s.SkipMismatches && len(mismatch) > 0
		switch {
		case len(notfound) > 0:
			// Always fail if container disappeared or was misspelled
			results[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusFailed,
				Error:  fmt.Sprintf(ContainerNotFound, strings.Join(notfound, ", ")),
			}
		case !s.SkipMismatches && len(mismatch) > 0:
			// Only fail if we do not skip for mismatches. Otherwise we either succeed
			// with partial updates or then mark it as skipped because no precondition
			// fulfilled.
			results[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusFailed,
				Error:  mismatchError,
			}
		case len(containerUpdates) == 0:
			rerr = ImageUpToDate
			if skippedMismatches {
				rerr = mismatchError
			}
			results[u.ResourceID] = ControllerResult{
				Status: ReleaseStatusSkipped,
				Error:  rerr,
			}
		default:
			rerr = ""
			if skippedMismatches {
				// While we succeed here, we still want the client to know that some
				// container mismatched.
				rerr = mismatchError
			}
			u.Updates = containerUpdates
			updates = append(updates, u)
			results[u.ResourceID] = ControllerResult{
				Status:       ReleaseStatusSuccess,
				Error:        rerr,
				PerContainer: u.Updates,
			}
		}
	}

	return updates
}

func (s ContainerSpecs) ReleaseKind() ReleaseKind {
	return s.Kind
}

func (s ContainerSpecs) ReleaseType() ReleaseType {
	return "containers"
}

func (s ContainerSpecs) CommitMessage(result Result) string {
	buf := &bytes.Buffer{}
	fmt.Fprintln(buf, "Release containers")
	for _, res := range result.AffectedResources() {
		fmt.Fprintf(buf, "\n%s", res)
		for _, upd := range result[res].PerContainer {
			fmt.Fprintf(buf, "\n- %s", upd.Target)
		}
		fmt.Fprintln(buf)
	}
	if err := result.Error(); err != "" {
		fmt.Fprintf(buf, "\n%s", result.Error())
	}
	return buf.String()
}
