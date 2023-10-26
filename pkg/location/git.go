// Copyright 2019 OVO Technology
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package location

import (
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

// Git type
type Git struct {
	Filepath              string
	FileType              string
	OrgRepo               string
	VerifyCircleCISuccess bool
	CircleCIDeployJobName string
}

func (git Git) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {

	if len(creds.KmsKey) == 0 {
		err = errors.New("Not updating un-encrypted new key in a Git repository. Use the" +
			"'KmsKey' field in config to specify the KMS key to use for encryption")
		return
	}
	var key string
	if key, err = getKeyForFileBasedLocation(keyWrapper, git.FileType); err != nil {
		return
	}

	const localDir = "/etc/cloud-key-rotator/cloud-key-rotator-tmp-repo"

	defer os.RemoveAll(localDir)

	var signKey *openpgp.Entity
	if signKey, err = crypt.CommitSignKey(creds.GitAccount.GitName, creds.GitAccount.GitEmail, creds.AkrPass, creds.AkrPath); err != nil {
		return
	}

	var committed *object.Commit
	const singleLine = false
	const disableValidation = true
	if committed, err = writeKeyToRemoteGitRepo(git, serviceAccountName,
		crypt.EncryptedServiceAccountKey(key, creds.KmsKey),
		localDir, signKey, creds); err != nil {
		return
	}

	if git.VerifyCircleCISuccess {
		err = verifyCircleCIJobSuccess(git.OrgRepo,
			fmt.Sprintf("%s", committed.ID()),
			git.CircleCIDeployJobName, creds.CircleCIAPIToken)
	}

	updated = UpdatedLocation{
		LocationType: "Git",
		LocationURI:  git.OrgRepo,
		LocationIDs:  []string{git.Filepath}}

	return
}

// writeKeyToRemoteGitRepo handles the writing of the supplied key to the *remote*
// Git repo defined in the Git struct
func writeKeyToRemoteGitRepo(gitt Git, serviceAccountName string,
	key []byte, localDir string, signKey *openpgp.Entity, creds cred.Credentials) (committed *object.Commit, err error) {
	var repo *git.Repository
	if repo, err = cloneGitRepo(localDir, gitt.OrgRepo,
		creds.GitAccount.GitAccessToken); err != nil {
		return
	}
	logger.Infof("Cloned git repo: %s", gitt.OrgRepo)
	var commit plumbing.Hash
	if commit, err = writeKeyToLocalGitRepo(gitt, repo, key, serviceAccountName,
		localDir, signKey, creds); err != nil {
		return
	}
	if committed, err = repo.CommitObject(commit); err != nil {
		return
	}
	logger.Infof("Committed to local git repo: %s", gitt.OrgRepo)
	if err = repo.Push(&git.PushOptions{Auth: &gitHttp.BasicAuth{
		Username: "abc123", // yes, this can be anything except an empty string
		Password: creds.GitAccount.GitAccessToken,
	},
		Progress: os.Stdout}); err != nil {
		return
	}
	logger.Infof("Pushed to remote git repo: %s", gitt.OrgRepo)
	return
}

// writeKeyToLocalGitRepo handles the writing of the supplied key to the *local*
// Git repo defined in the Git struct
func writeKeyToLocalGitRepo(gitt Git, repo *git.Repository, key []byte,
	serviceAccountName, localDir string, signKey *openpgp.Entity, creds cred.Credentials) (commmit plumbing.Hash, err error) {
	var w *git.Worktree
	if w, err = repo.Worktree(); err != nil {
		return
	}
	fullFilePath := localDir + "/" + gitt.Filepath
	if err = ioutil.WriteFile(fullFilePath, key, 0644); err != nil {
		return
	}
	w.Add(fullFilePath)
	autoStage := true
	return w.Commit(fmt.Sprintf("CKR updating %s", serviceAccountName), &git.CommitOptions{
		Author: &object.Signature{
			Name:  creds.GitAccount.GitName,
			Email: creds.GitAccount.GitEmail,
			When:  time.Now(),
		},
		All:     autoStage,
		SignKey: signKey,
	})
}

// cloneGitRepo clones the specified Git repository into a local directory
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
