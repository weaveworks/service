package committer

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"time"

	"github.com/pkg/errors"
)

const (
	serviceUIRepo = "git@github.com:weaveworks/service-ui.git"
	packageFile   = "client/package.json"
	commitTimeout = 1 * time.Minute
)

func editScopeVersion(file string, version string) error {
	weaveScopeLink := fmt.Sprintf("https://s3.amazonaws.com/weaveworks-js-modules/weave-scope/%s/weave-scope.tgz", version)
	// Edit json file with jq instead of encoding/json package to preserve key order

	app := "jq"
	arg := fmt.Sprintf(`.dependencies."weave-scope" = "%s"`, weaveScopeLink)
	cmd := exec.Command(app, arg, file)

	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "jq failed: %s", out)
	}

	content := out
	if err = ioutil.WriteFile(file, content, 0666); err != nil {
		return errors.Wrapf(err, "cannot write new content to file %s", file)
	}

	return nil
}

func execGitCommand(ctx context.Context, dir string, args ...string) error {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "output: %s", string(out))
	}
	return nil
}

// PushUpdatedFile adds a commit with new version of scope to service-ui repo
func PushUpdatedFile(ctx context.Context, version string) error {
	ctx, cancel := context.WithTimeout(ctx, commitTimeout)
	defer cancel()

	tmpdir, err := ioutil.TempDir("", "service-ui-kicker-")
	if err != nil {
		return errors.Wrap(err, "failed to create temp dir")
	}

	defer func() {
		_ = os.RemoveAll(tmpdir)
	}()

	if err := execGitCommand(ctx, tmpdir, "clone", serviceUIRepo, tmpdir); err != nil {
		return errors.Wrapf(err, "clone failed")
	}

	pathToFile := path.Join(tmpdir, packageFile)
	if err := editScopeVersion(pathToFile, version); err != nil {
		return errors.Wrapf(err, "cannot edit file %s", pathToFile)
	}

	commitMsg := fmt.Sprintf("Bump Scope version to %s", version)
	if err := execGitCommand(ctx, tmpdir, "commit", "-a", "-m", commitMsg); err != nil {
		return errors.Wrapf(err, "commit failed")
	}

	if err := execGitCommand(ctx, tmpdir, "push", "origin", "master"); err != nil {
		return err
	}
	return nil
}
