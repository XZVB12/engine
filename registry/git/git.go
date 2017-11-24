package git

import (
	"os"
	"path/filepath"

	"github.com/maliceio/engine/cli/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	git "gopkg.in/src-d/go-git.v4"
)

var (
	regDir     = os.Getenv("MALICE_REGISTRY_PATH")
	regURL     = os.Getenv("MALICE_REGISTRY_URL")
	regRepoDir = "registry"
)

func init() {
	if regDir == "" {
		regDir = filepath.Join(config.Dir(), regRepoDir)
	}
	logrus.Debugf("malice registry directory is %s", regDir)
	if regURL == "" {
		regURL = "https://github.com/maliceio/registry"
	}
	logrus.Debugf("malice registry URL is %s", regURL)
}

// CloneRegistry clones the maliceio/registry repo and verifies it's signature
func CloneRegistry() error {
	repo, err := git.PlainClone(regDir, false, &git.CloneOptions{
		URL:      regURL,
		Depth:    1,
		Progress: os.Stdout,
	})
	if err != nil {
		return errors.Wrap(err, "git clone failed")
	}
	logrus.Debugf("malice registry has been cloned into: %s", regDir)
	// get HEAD commit
	currentBranch, err := repo.Head()
	if err != nil {
		return errors.Wrap(err, "get repo HEAD failed")
	}

	commit, err := repo.CommitObject(currentBranch.Hash())
	if err != nil {
		return errors.Wrap(err, "get latest commit failed")
	}
	// verify malice registry
	pgpKeys, err := Asset("registry/fixture/pgp_keys.asc")
	if err != nil {
		return errors.Wrap(err, "open pgp_keys.asc failed")
	}
	// keyRingReader := bytes.NewReader(string())
	_, err = commit.Verify(string(pgpKeys))
	if err != nil {
		return errors.Wrap(err, "git verify-commit failed")
	}
	return nil
}

// PullRegistry updates registry repo
func PullRegistry() error {
	repo, err := git.PlainOpen(regDir)
	if err != nil {
		return errors.Wrap(err, "open registry repo failed")
	}
	w, err := repo.Worktree()
	if err != nil {
		return errors.Wrap(err, "get the working directory for the repository failed")
	}
	err = w.Pull(&git.PullOptions{RemoteName: "origin"})
	if err == git.NoErrAlreadyUpToDate {
		logrus.Debug("malice registry already up-to-date")
		return nil
	}
	if err != nil {
		return errors.Wrap(err, "connect fetch failed")
	}
	logrus.Debug("malice registry has been updated")
	return nil
}
