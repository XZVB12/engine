package git

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/maliceio/engine/cli/config"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/openpgp"
	git2go "gopkg.in/libgit2/git2go.v26"
)

var (
	regDir     = os.Getenv("MALICE_REGISTRY")
	regRepoDir = "registry"
)

func init() {
	if regDir == "" {
		regDir = filepath.Join(config.Dir(), regRepoDir)
	}
	logrus.Debugf("malice registry directory is %s", regDir)
}

// CloneRegistry clones the maliceio/registry repo and verifies it's signature
func CloneRegistry() error {
	opts := &git2go.CloneOptions{
		CheckoutBranch: "master",
	}
	repo, err := git2go.Clone("https://github.com/maliceio/registry.git", regDir, opts)
	if err != nil {
		return errors.Wrap(err, "git clone failed")
	}
	logrus.Debugf("malice registry has been cloned into: %s", regDir)
	// get HEAD commit
	currentBranch, err := repo.Head()
	if err != nil {
		return errors.Wrap(err, "get repo HEAD failed")
	}

	commit, err := repo.LookupCommit(currentBranch.Target())
	if err != nil {
		return errors.Wrap(err, "get latest commit failed")
	}
	// verify malice registry
	err = VerifyCommit(commit)
	if err != nil {
		return errors.Wrap(err, "git verify-commit failed")
	}
	return nil
}

// PullRegistry updates registry repo
func PullRegistry() error {
	repo, err := git2go.OpenRepository(regDir)
	if err != nil {
		return errors.Wrap(err, "open registry repo failed")
	}
	origin, err := repo.Remotes.Lookup("origin")
	if err != nil {
		return errors.Wrap(err, "repo remotes lookup origin failed")
	}
	err = origin.ConnectFetch(nil, nil, nil)
	if err != nil {
		return errors.Wrap(err, "connect fetch failed")
	}
	logrus.Debug("malice registry has been updated")
	return nil
}

// VerifyCommit performs a `git verify-commit head`
func VerifyCommit(commit *git2go.Commit) error {
	pgpKeys, err := Asset("registry/fixture/pgp_keys.asc")
	if err != nil {
		return errors.Wrap(err, "open pgp_keys.asc failed")
	}
	keyRingReader := bytes.NewReader(pgpKeys)

	if commit == nil {
		repo, err := git2go.OpenRepository(regDir)
		if err != nil {
			return errors.Wrap(err, "open registry repo failed")
		}

		currentBranch, err := repo.Head()
		if err != nil {
			return errors.Wrap(err, "get repo HEAD failed")
		}

		commit, err = repo.LookupCommit(currentBranch.Target())
		if err != nil {
			return errors.Wrap(err, "get latest commit failed")
		}
	}

	signature, signed, err := commit.ExtractSignature()
	if err != nil {
		return errors.Wrap(err, "extract signature failed")
	}

	keyring, err := openpgp.ReadArmoredKeyRing(keyRingReader)
	if err != nil {
		return errors.Wrap(err, "read armored key-ring failed")
	}

	entity, err := openpgp.CheckArmoredDetachedSignature(keyring, strings.NewReader(signed), strings.NewReader(signature))
	if err != nil {
		return errors.Wrap(err, "check detached signature failed")
	}
	for ident := range entity.Identities {
		logrus.Debugf("good signature from: %s", entity.Identities[ident].Name)
	}
	return nil
}
