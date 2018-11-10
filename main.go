package main

import (
	"bytes"
	b64 "encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	keys "github.com/eversc/cloud-key-client"
	enc "github.com/ovotech/mantle/crypt"
	"github.com/spf13/viper"
	"golang.org/x/crypto/openpgp"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	gitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
)

const (
	datadogURL               = "https://api.datadoghq.com/api/v1/series?api_key="
	gcpAgeThresholdMins      = 43200 //30 days
	envVarPrefix             = "ckr"
	rotationAgeThresholdMins = 1000000000
)

type circleCIDeleteResp struct {
	Message string
}

type circleCIUpdate struct {
	Name  string
	Value string
}

type cloudProvider struct {
	Name    string
	Project string
}

type circleCI struct {
	usernameProjectEnvVarMap map[string]string
}

type gitHub struct {
	filepath string
	orgRepo  string
}

type keySource struct {
	serviceAccountName string
	circleCI           circleCI
	gitHub             gitHub
}

type config struct {
	AgeMetricGranularity  string
	IncludeAwsUserKeys    bool
	verifyCircleCISuccess bool
	DatadogAPIKey         string
	RotationMode          bool
	CloudProviders        []cloudProvider
	IgnoreAccounts        []string
	KeySources            []keySource
	CircleCIAPIToken      string
	GitHubAccessToken     string
	GitName               string
	GitEmail              string
}

//getConfig returns the application config
func getConfig() (c config) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix(envVarPrefix)
	viper.SetDefault("agemetricgranularity", "min")
	viper.SetDefault("envconfigpath", "/etc/cloud-key-rotator/")
	viper.SetDefault("envconfigname", "ckr")
	viper.SetConfigName(viper.GetString("envconfigname"))
	viper.AddConfigPath(viper.GetString("envconfigpath"))
	viper.ReadInConfig()
	err := viper.Unmarshal(&c)
	if err != nil {
		log.Println(err)
	}
	if !viper.IsSet("cloudProviders") {
		panic("cloudProviders is not set")
	}
	return
}

func main() {
	c := getConfig()
	providers := make([]keys.Provider, 0)
	for _, provider := range c.CloudProviders {
		providers = append(providers, keys.Provider{GcpProject: provider.Project,
			Provider: provider.Name})
	}
	keySlice := keys.Keys(providers)
	keySlice = filterKeys(keySlice, c)
	keySlice = adjustAges(keySlice, c)
	if c.RotationMode {
		rotateKeys(keySlice, c.KeySources, c.CircleCIAPIToken, c.GitHubAccessToken,
			c.GitName, c.GitEmail, c.verifyCircleCISuccess)
	} else {
		postMetric(keySlice, c.DatadogAPIKey)
	}
}

func rotateKeys(keySlice []keys.Key, keySources []keySource, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail string, verifyCircleCISuccess bool) {
	//for each key:

	//if above a certain age threshold:

	//alert that process has started

	//create new key

	//update the source(s)

	//verify the source update(s) has worked?

	//delete the key

	//alert that key has been deleted

	for _, key := range keySlice {
		accountKeySources, err := accountKeySources(key.Account, keySources)
		check(err)
		if key.Age > rotationAgeThresholdMins {
			//TODO: alert
			privateKey, err := keys.CreateKey(key)
			check(err)
			updateKeySources(accountKeySources, privateKey, circleCIAPIToken,
				gitHubAccessToken, gitName, gitEmail, verifyCircleCISuccess)
			//TODO: don't delete the old key yet..needs proper testing first..
			//check(keys.DeleteKey(key))
			//TODO: alert
		}
	}
}

func accountKeySources(account string, keySources []keySource) (accountKeySources []keySource,
	err error) {
	err = errors.New("")
	for _, keySource := range keySources {
		if account == keySource.serviceAccountName {
			err = nil
			accountKeySources = append(accountKeySources, keySource)
		}
	}
	return
}

func updateKeySources(keySources []keySource, newKey, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail string, verifyCircleCISuccess bool) {
	repoSourceStructMap := make(map[string][]keySource)
	for _, keySource := range keySources {
		log.Println(keySource)
		if &keySource.circleCI != nil {
			rotateKeyInCircleCI(keySource.circleCI, newKey, circleCIAPIToken)
		}
		if &keySource.gitHub != nil {
			orgRepo := keySource.gitHub.orgRepo
			//group gitHub sources together so multiple keys can be updated in a single commit
			repoSourceStructMap[orgRepo] = append(repoSourceStructMap[orgRepo], keySource)
		}
	}
	singleLine := false
	encKey := enc.CipherBytes([]byte(newKey), singleLine)
	for orgRepo, gitHubSources := range repoSourceStructMap {
		updateGitHubRepo(gitHubSources, orgRepo, gitHubAccessToken,
			gitName, gitEmail, encKey, verifyCircleCISuccess)
	}
}

func updateGitHubRepo(gitHubSources []keySource, orgRepo,
	gitHubAccessToken, gitName, gitEmail string, encKey []byte,
	verifyCircleCISuccess bool) {
	keyNum := len(gitHubSources)
	if keyNum > 0 {
		//TODO: fix localDir
		localDir := ""
		repo := cloneGitRepo(localDir, orgRepo, gitHubAccessToken)
		var gitCommentBuff bytes.Buffer
		gitCommentBuff.WriteString("CKR updating ")

		if keyNum > 1 {
			gitCommentBuff.WriteString(strconv.Itoa(keyNum))
			gitCommentBuff.WriteString(" service-account keys")
			//iterate over sources, update each key/file
			for _, gitHubSource := range gitHubSources {
				gitCommentBuff.WriteString("\n")
				gitCommentBuff.WriteString(gitHubSource.serviceAccountName)
				writeBytesToFile(encKey, gitHubSource.gitHub.filepath)
			}
		} else {
			gitHubSource := gitHubSources[0]
			gitCommentBuff.WriteString(gitHubSource.serviceAccountName)
			writeBytesToFile(encKey, gitHubSource.gitHub.filepath)
		}
		check(repo.Push(&git.PushOptions{}))
		if verifyCircleCISuccess {
			verifyCircleCIJobSuccess(orgRepo)
		}
	}
}

func verifyCircleCIJobSuccess(orgRepo string) {
	//get list of currently running builds
	//https://circleci.com/docs/api/#recent-builds-for-a-single-project

	//iterate over the list, only match on a build which has the desired git commit hash

	//grab the buildNum

	//poll https://circleci.com/docs/api/#single-job until it has a status of success
}

func commitToGitRepo(repo *git.Repository, gitName, gitEmail, commitComment string) {
	worktree, err := repo.Worktree()
	check(err)
	signKey, err := commitSignKey("", gitName, gitEmail, "")
	check(err)

	commit, err := worktree.Commit(commitComment, &git.CommitOptions{
		Author: &object.Signature{
			Name:  gitName,
			Email: gitEmail,
			When:  time.Now(),
		},
		SignKey: signKey,
	})
	check(err)
	_, err = repo.CommitObject(commit)
	check(err)
}

func writeBytesToFile(encKey []byte, filepath string) {
	err := ioutil.WriteFile(filepath, encKey, 0644)
	check(err)
}

func cloneGitRepo(localDir, orgRepo, token string) (repo *git.Repository) {
	url := strings.Join([]string{"https://github.com/", orgRepo, ".git"}, "")
	repo, err := git.PlainClone(localDir, false, &git.CloneOptions{
		// The intended use of a GitHub personal access token is in replace of your password
		// because access tokens can easily be revoked.
		// https://help.github.com/articles/creating-a-personal-access-token-for-the-command-line/
		Auth: &gitHttp.BasicAuth{
			Username: "abc123", // yes, this can be anything except an empty string
			Password: token,
		},
		URL:      url,
		Progress: os.Stdout,
	})
	check(err)
	return
}

func rotateKeyInCircleCI(circleci circleCI, newKey, circleCIAPIToken string) {
	base64Key := b64.StdEncoding.EncodeToString([]byte(newKey))
	for usernameProject, envVarName := range circleci.usernameProjectEnvVarMap {
		postURL := circleCIURL(usernameProject)
		envVarURL := postURL + "/" + envVarName
		circleciDeleteBytes, err := json.Marshal(&circleCIDeleteResp{Message: "ok"})
		check(err)
		client := &http.Client{}

		//Delete existing circleCI env var
		postHTTPRequest("DELETE", postURL, circleCIAPIToken, circleciDeleteBytes,
			nil, 200, client)

		//Construct payload and expected response, and post new env var to circleCI
		circleciPostBytes, err := json.Marshal(&circleCIUpdate{Name: envVarName,
			Value: base64Key})
		check(err)
		circleciGetBytes, err := json.Marshal(&circleCIUpdate{Name: envVarName,
			Value: hiddenCircleValue(base64Key)})
		check(err)
		postHTTPRequest("POST", envVarURL, circleCIAPIToken, circleciGetBytes,
			circleciPostBytes, 201, client)

		//sleep for 10s, to protect against any replication delay
		time.Sleep(time.Duration(10) * time.Second)

		//Check the new env var is still returned
		postHTTPRequest("GET", envVarURL, circleCIAPIToken, circleciGetBytes,
			nil, 200, client)
	}
}

func hiddenCircleValue(value string) (hiddenValue string) {
	hiddenValue = strings.Join([]string{"xxxx", value[len(value)-4:]}, "")
	return
}

func circleCIURL(usernameProject string) (url string) {
	url = strings.Join([]string{"https://circleci.com/api/v1.1/project/github/",
		usernameProject, "/envvar"}, "")
	return
}

func postHTTPRequest(method, url, apiToken string, expectedRespBody,
	postBody []byte, expectedStatusCode int, client *http.Client) {
	req, err := http.NewRequest(method, url, bytes.NewReader(postBody))
	check(err)
	req.Header.Set("Content-Type", "application/json")
	queryParams := req.URL.Query()
	queryParams.Add("circle-token", apiToken)
	req.URL.RawQuery = queryParams.Encode()
	resp, err := client.Do(req)
	check(err)
	defer resp.Body.Close()
	if resp.StatusCode != expectedStatusCode {
		panic(strings.Join([]string{
			"Unexpected status code returned from the circleCI API. Want: ",
			strconv.Itoa(expectedStatusCode),
			", Got: ", strconv.Itoa(resp.StatusCode),
			". method: ", method, ", URL: ", url}, ""))
	}
	responseBody, err := ioutil.ReadAll(resp.Body)
	check(err)
	if string(responseBody) != string(expectedRespBody) {
		panic(strings.Join([]string{
			"Unexpected response body returned by the circleCI API. Want: ",
			string(expectedRespBody),
			", Got: ", string(responseBody)}, ""))
	}

}

func rotateKeyInGithub(keySource keySource) (err error) {

	return
}

//adjustAges returns a keys.Key slice containing the same keys but with keyAge
// changed to whatever's been configured (min/day/hour)
func adjustAges(keys []keys.Key, config config) (adjustedKeys []keys.Key) {
	for _, key := range keys {
		key.Age = adjustAgeScale(key.Age, config)
		adjustedKeys = append(adjustedKeys, key)
	}
	return adjustedKeys
}

//filterKeys returns a keys.Key slice created by filtering the provided
// keys.Key slice based on specific rules for each provider
func filterKeys(keys []keys.Key, config config) (filteredKeys []keys.Key) {
	for _, key := range keys {
		valid := true
		if key.Provider.Provider == "aws" {
			valid = validAwsKey(key, config)
		}
		if valid {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return
}

//validAwsKey returns a bool that reflects whether the provided keys.Key is
// valid, based on aws-specific rules
func validAwsKey(key keys.Key, config config) (valid bool) {
	if config.IncludeAwsUserKeys {
		valid = true
	} else {
		match, _ := regexp.MatchString("[a-zA-Z]\\.[a-zA-Z]", key.Name)
		valid = !match
	}

	return
}

//adjustAgeScale converts the provided keyAge into the desired age
// granularity, based on the 'AgeMetricGranularity' in the provided
// Specification
func adjustAgeScale(keyAge float64, config config) (adjustedAge float64) {
	switch config.AgeMetricGranularity {
	case "day":
		adjustedAge = keyAge / 60 / 24
	case "hour":
		adjustedAge = keyAge / 60
	case "min":
		adjustedAge = keyAge
	default:
		panic("Unsupported age metric granularity: " +
			config.AgeMetricGranularity)
	}
	return
}

//postMetric posts details of each keys.Key to a metrics api
func postMetric(keys []keys.Key, apiKey string) {
	if len(apiKey) > 0 {
		url := strings.Join([]string{datadogURL, apiKey}, "")
		for _, key := range keys {
			var jsonString = []byte(
				`{ "series" :[{"metric":"key-dater.age","points":[[` +
					strconv.FormatInt(time.Now().Unix(), 10) +
					`, ` + strconv.FormatFloat(key.Age, 'f', 2, 64) +
					`]],"type":"count","tags":["team:cepheus","environment:prod","key:` +
					key.Name + `","provider:` + key.Provider.Provider + `"]}]}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonString))
			check(err)
			req.Header.Set("Content-type", "application/json")
			client := &http.Client{}
			resp, err := client.Do(req)
			check(err)
			defer resp.Body.Close()
			if resp.StatusCode != 202 {
				body, err := ioutil.ReadAll(resp.Body)
				check(err)
				bodyString := string(body)
				fmt.Println(string(bodyString))
				panic("non-202 status code (" + strconv.Itoa(resp.StatusCode) +
					") returned by Datadog")
			}
		}
	}
}

//check panics if error is not nil
func check(e error) {
	if e != nil {
		panic(e.Error())
	}
}

func commitSignKey(armoredKeyRing, name, email, passphrase string) (entity *openpgp.Entity, err error) {
	reader := strings.NewReader(armoredKeyRing)
	entityList, err := openpgp.ReadArmoredKeyRing(reader)
	check(err)
	_, ok := entityList[0].Identities[strings.Join([]string{name, " <", email, ">"}, "")]
	if !ok {
		err = errors.New("Failed to add Identity to EntityList")
	}
	check(entityList[0].PrivateKey.Decrypt([]byte(passphrase)))
	entity = entityList[0]
	return
}
