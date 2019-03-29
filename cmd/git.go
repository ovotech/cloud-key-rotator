package cmd

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"golang.org/x/crypto/openpgp"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	gitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

// gitHubAccount type
type gitHubAccount struct {
	GitHubAccessToken string
	GitName           string
	GitEmail          string
}

//gitHub type
type gitHub struct {
	Filepath              string
	OrgRepo               string
	VerifyCircleCISuccess bool
	CircleCIDeployJobName string
}

func (gitHub gitHub) write(serviceAccountName, keyID, key string, creds credentials) (updated updatedLocation, err error) {

	if len(creds.KmsKey) == 0 {
		err = errors.New("Not updating un-encrypted new key in a Git repository. Use the" +
			"'KmsKey' field in config to specify the KMS key to use for encryption")
		return
	}

	const localDir = "/etc/cloud-key-rotator/cloud-key-rotator-tmp-repo"

	defer os.RemoveAll(localDir)

	// TODO Move me out of git-specific code
	var encKey []byte
	if encKey, err = encryptedServiceAccountKey(key, creds.KmsKey); err != nil {
		return
	}

	var signKey *openpgp.Entity
	if signKey, err = commitSignKey(creds.GitHubAccount.GitName, creds.GitHubAccount.GitEmail, creds.AkrPass); err != nil {
		return
	}

	var committed *object.Commit
	if committed, err = writeKeyToRemoteGitRepo(gitHub, serviceAccountName, encKey, localDir, signKey, creds); err != nil {
		return
	}

	if gitHub.VerifyCircleCISuccess {
		err = verifyCircleCIJobSuccess(gitHub.OrgRepo,
			fmt.Sprintf("%s", committed.ID()),
			gitHub.CircleCIDeployJobName, creds.CircleCIAPIToken)
	}

	updated = updatedLocation{
		LocationType: "GitHub",
		LocationURI:  gitHub.OrgRepo,
		LocationIDs:  []string{gitHub.Filepath}}

	return
}

//writeKeyToRemoteGitRepo handles the writing of the supplied key to the *remote*
// Git repo defined in the GitHub struct
func writeKeyToRemoteGitRepo(gitHub gitHub, serviceAccountName string,
	key []byte, localDir string, signKey *openpgp.Entity, creds credentials) (committed *object.Commit, err error) {
	var repo *git.Repository
	if repo, err = cloneGitRepo(localDir, gitHub.OrgRepo,
		creds.GitHubAccount.GitHubAccessToken); err != nil {
		return
	}
	logger.Infof("Cloned git repo: %s", gitHub.OrgRepo)
	var commit plumbing.Hash
	if commit, err = writeKeyToLocalGitRepo(gitHub, repo, key, serviceAccountName,
		localDir, signKey, creds); err != nil {
		return
	}
	if committed, err = repo.CommitObject(commit); err != nil {
		return
	}
	logger.Infof("Committed to local git repo: %s", gitHub.OrgRepo)
	if err = repo.Push(&git.PushOptions{Auth: &gitHttp.BasicAuth{
		Username: "abc123", // yes, this can be anything except an empty string
		Password: creds.GitHubAccount.GitHubAccessToken,
	},
		Progress: os.Stdout}); err != nil {
		return
	}
	logger.Infof("Pushed to remote git repo: %s", gitHub.OrgRepo)
	return
}

//writeKeyToLocalGitRepo handles the writing of the supplied key to the *local*
// Git repo defined in the GitHub struct
func writeKeyToLocalGitRepo(gitHub gitHub, repo *git.Repository, key []byte,
	serviceAccountName, localDir string, signKey *openpgp.Entity, creds credentials) (commmit plumbing.Hash, err error) {
	var w *git.Worktree
	if w, err = repo.Worktree(); err != nil {
		return
	}
	fullFilePath := localDir + "/" + gitHub.Filepath
	if err = ioutil.WriteFile(fullFilePath, key, 0644); err != nil {
		return
	}
	w.Add(fullFilePath)
	autoStage := true
	return w.Commit(fmt.Sprintf("CKR updating %s", serviceAccountName), &git.CommitOptions{
		Author: &object.Signature{
			Name:  creds.GitHubAccount.GitName,
			Email: creds.GitHubAccount.GitEmail,
			When:  time.Now(),
		},
		All:     autoStage,
		SignKey: signKey,
	})
}

//cloneGitRepo clones the specified Git repository into a local directory
func cloneGitRepo(localDir, orgRepo, token string) (repo *git.Repository, err error) {
	url := strings.Join([]string{"https://github.com/", orgRepo, ".git"}, "")
	return git.PlainClone(localDir, false, &git.CloneOptions{
		Auth: &gitHttp.BasicAuth{
			Username: "abc123", // yes, this can be anything except an empty string
			Password: token,
		},
		URL:      url,
		Progress: os.Stdout,
	})
}
