package location

import (
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/ovotech/cloud-key-rotator/pkg/cred"
	"github.com/ovotech/cloud-key-rotator/pkg/crypt"
	"golang.org/x/crypto/openpgp"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	gitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

//GitHub type
type GitHub struct {
	Filepath              string
	OrgRepo               string
	VerifyCircleCISuccess bool
	CircleCIDeployJobName string
}

func (gitHub GitHub) Write(serviceAccountName, keyWrapper keyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {

	if len(creds.KmsKey) == 0 {
		err = errors.New("Not updating un-encrypted new key in a Git repository. Use the" +
			"'KmsKey' field in config to specify the KMS key to use for encryption")
		return
	}
	var base64Decode bool
	var key string
	if keyWrapper.keyProvider == "aws" {
		key = fmt.Sprintf("[default]\naws_access_key_id = %s\naws_secret_access_key = %s", keyWrapper.keyID, keyWrapper.key)
	} else {
		if key, err = b64.StdEncoding.DecodeString(keyWrapper.key); err != nil {
			return
		}
	}

	const localDir = "/etc/cloud-key-rotator/cloud-key-rotator-tmp-repo"

	defer os.RemoveAll(localDir)

	var signKey *openpgp.Entity
	if signKey, err = crypt.CommitSignKey(creds.GitHubAccount.GitName, creds.GitHubAccount.GitEmail, creds.AkrPass); err != nil {
		return
	}

	var committed *object.Commit
	const singleLine = false
	const disableValidation = true
	if committed, err = writeKeyToRemoteGitRepo(gitHub, serviceAccountName,
		crypt.EncryptedServiceAccountKey(key, creds.KmsKey),
		localDir, signKey, creds); err != nil {
		return
	}

	if gitHub.VerifyCircleCISuccess {
		err = verifyCircleCIJobSuccess(gitHub.OrgRepo,
			fmt.Sprintf("%s", committed.ID()),
			gitHub.CircleCIDeployJobName, creds.CircleCIAPIToken)
	}

	updated = UpdatedLocation{
		LocationType: "GitHub",
		LocationURI:  gitHub.OrgRepo,
		LocationIDs:  []string{gitHub.Filepath}}

	return
}

//writeKeyToRemoteGitRepo handles the writing of the supplied key to the *remote*
// Git repo defined in the GitHub struct
func writeKeyToRemoteGitRepo(gitHub GitHub, serviceAccountName string,
	key []byte, localDir string, signKey *openpgp.Entity, creds cred.Credentials) (committed *object.Commit, err error) {
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
func writeKeyToLocalGitRepo(gitHub GitHub, repo *git.Repository, key []byte,
	serviceAccountName, localDir string, signKey *openpgp.Entity, creds cred.Credentials) (commmit plumbing.Hash, err error) {
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
