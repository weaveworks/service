package scope

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/pkg/errors"
)

const (
	serviceUIRepo = "git@github.com:weaveworks/service-ui.git"
	packagePath   = "client"
	commitTimeout = 10 * time.Minute
)

// yarnUpdateScopeVersion updates package.json and yarn.lock files to specified version of weave-scope
// `yarn add package-name@version` installs a specific version of a package from the registry
func yarnUpdateScopeVersion(version string, path string) error {
	weaveScopePackage := fmt.Sprintf("weave-scope@https://s3.amazonaws.com/weaveworks-js-modules/weave-scope/%s/weave-scope.tgz", version)

	cmd := exec.Command("yarn", "add", "--ignore-engines", weaveScopePackage)
	cmd.Dir = path
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.Wrapf(err, "yarn add failed: %s", out)
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

	if err := yarnUpdateScopeVersion(version, filepath.Join(tmpdir, packagePath)); err != nil {
		return errors.Wrapf(err, "cannot update weave-scope to version %s", version)
	}

	commitMsg := fmt.Sprintf("Bump Scope version to %s", version)
	if err := execGitCommand(ctx, tmpdir, "commit", "--author=\"weaveworksbot <team+gitbot@weave.works>\"", "-a", "-m", commitMsg); err != nil {
		return errors.Wrapf(err, "commit failed")
	}

	return execGitCommand(ctx, tmpdir, "push", "origin", "master")
}
