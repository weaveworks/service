package grpc

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	strings "strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/net/context"

	"github.com/weaveworks/service/common/collections"
)

// Server implements KubectlServer.
type Server struct {
	versions *collections.Trie
	runner   KubectlRunner
}

const (
	latest     = "latest"
	kubectlDir = "/kubectl"
)

// NewServer creates... a new server.
func NewServer(runner KubectlRunner) (*Server, error) {
	versions, err := listSupportedVersions()
	if err != nil {
		return nil, err
	}
	log.Infof("Supported Kubernetes versions: {%v}", strings.Join(versions, ", "))
	return &Server{
		versions: versionsTrie(versions),
		runner:   runner,
	}, nil
}

func listSupportedVersions() ([]string, error) {
	kubectlBinaries, err := ioutil.ReadDir(kubectlDir)
	if err != nil {
		return nil, err
	}
	var supportedVersions []string
	for _, f := range kubectlBinaries {
		if f.Name() != latest {
			supportedVersions = append(supportedVersions, f.Name())
		}
	}
	return supportedVersions, nil
}

func versionsTrie(versions []string) *collections.Trie {
	trie := collections.NewTrie()
	for _, version := range versions {
		trie.Add(version)
	}
	return trie
}

// RunKubectlCmd executes the provided kubectl command against the specified cluster.
func (s Server) RunKubectlCmd(ctx context.Context, req *KubectlRequest) (*KubectlReply, error) {
	kubeCfgFile, err := writeToFile(req.Kubeconfig)
	if err != nil {
		return nil, err
	}
	defer os.Remove(kubeCfgFile.Name())
	version := s.findCompatibleVersionOrDefaultToLatest(req.Version)
	out, err := s.runner.RunCmd(kubeCfgFile.Name(), version, req.Args)
	log.Infof("Out: \n%v\nErr: \n%v\n", string(out), err)
	if err != nil {
		return nil, err
	}
	return &KubectlReply{
		Output: string(out),
	}, nil
}

func writeToFile(kubeconfig []byte) (*os.File, error) {
	tmpfile, err := ioutil.TempFile("/tmp", "kubeconfig")
	if err != nil {
		return nil, err
	}
	if _, err := tmpfile.Write(kubeconfig); err != nil {
		return nil, err
	}
	if err := tmpfile.Close(); err != nil {
		return nil, err
	}
	return tmpfile, nil
}

func (s Server) findCompatibleVersionOrDefaultToLatest(version string) string {
	if compatVersion, ok := s.versions.BestMatch(version); ok {
		return compatVersion
	}
	return latest
}

// KubectlRunner is the interface for kubectl commands' runners.
// This abstraction is mainly to be able to do DI for testing.
type KubectlRunner interface {
	RunCmd(kubeCfgFileName, version string, args []string) ([]byte, error)
}

// DefaultKubectlRunner is the canonical implementation of KubectlRunner.
type DefaultKubectlRunner struct {
}

// RunCmd runs the provided command on the Kubernetes cluster targeted by the provided kubeconfig using the provided version of kubectl.
func (r DefaultKubectlRunner) RunCmd(kubeCfgFileName, version string, rawArgs []string) ([]byte, error) {
	cmd, args := cmdAndArgs(kubeCfgFileName, version, rawArgs)
	log.Infof("Running: %v %v", cmd, args)
	// TODO: instrument.
	command := exec.Command(cmd, args...)
	return command.CombinedOutput()
}

func cmdAndArgs(kubeCfgFileName, version string, rawArgs []string) (string, []string) {
	var args []string
	args = append(args, fmt.Sprintf("--kubeconfig=%v", kubeCfgFileName))
	args = append(args, rawArgs...)
	cmd := fmt.Sprintf("%v/%v", kubectlDir, version)
	return cmd, args
}

// NoOpKubectlRunner is a no-op implementation of KubectlRunner.
// This implementation is mostly useful for testing.
type NoOpKubectlRunner struct {
}

// RunCmd actually does NOT run anything for this implementation. It just logs the command it could have run with a different runner.
func (r NoOpKubectlRunner) RunCmd(kubeCfgFileName, version string, rawArgs []string) ([]byte, error) {
	cmd, args := cmdAndArgs(kubeCfgFileName, version, rawArgs)
	msg := fmt.Sprintf("Dry run: %v %v", cmd, args)
	log.Infof(msg)
	return []byte(msg), nil
}
