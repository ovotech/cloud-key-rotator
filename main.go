package main

import (
	"bytes"
	b64 "encoding/base64"
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
	circleci "github.com/jszwedko/go-circleci"
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
	rotationAgeThresholdMins = 53
)

type circleCIDeleteResp struct {
	Message string
}

type circleCIUpdate struct {
	Name  string
	Value string
}

type Previous struct {
	Status   string
	BuildNum int `json:"build_num"`
}

type cloudProvider struct {
	Name    string
	Project string
}

type circleCI struct {
	UsernameProject string
	EnvVar          string
}

type gitHub struct {
	Filepath              string
	OrgRepo               string
	VerifyCircleCISuccess bool
	CircleCIDeployJobName string
}

type keySource struct {
	ServiceAccountName string
	CircleCI           []circleCI
	GitHub             gitHub
}

type config struct {
	AkrPass              string
	AgeMetricGranularity string
	IncludeAwsUserKeys   bool
	DatadogAPIKey        string
	RotationMode         bool
	CloudProviders       []cloudProvider
	ExcludeSAs           []string
	IncludeSAs           []string
	Blacklist            []string
	Whitelist            []string
	KeySources           []keySource
	CircleCIAPIToken     string
	GitHubAccessToken    string
	GitName              string
	GitEmail             string
	KmsKey               string
}

//getConfig returns the application config
func getConfig() (c config) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix(envVarPrefix)
	viper.SetDefault("agemetricgranularity", "min")
	viper.SetDefault("envconfigpath", "/etc/cloud-key-rotator/")
	viper.SetDefault("envconfigname", "ckr")
	// viper.SetConfigName(viper.GetString("envconfigname"))
	// viper.AddConfigPath(viper.GetString("envconfigpath"))
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
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
	fmt.Println(keySlice)
	if c.RotationMode {
		rotateKeys(keySlice, c.KeySources, c.CircleCIAPIToken, c.GitHubAccessToken,
			c.GitName, c.GitEmail, c.KmsKey, c.AkrPass)
	} else {
		postMetric(keySlice, c.DatadogAPIKey)
	}
}

func rotateKeys(keySlice []keys.Key, keySources []keySource, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail, kmsKey, akrPass string) {
	processedItems := make([]string, 0)
	for _, key := range keySlice {
		if !contains(processedItems, key.Account) {
			accountKeySources, err := accountKeySources(key.Account, keySources)
			check(err)
			processedItems = append(processedItems, key.Account)
			if key.Age > rotationAgeThresholdMins {
				//TODO: alert
				newKey, err := keys.CreateKey(key)
				//TODO: alert
				check(err)
				fmt.Println("created new key")
				updateKeySources(accountKeySources, newKey, circleCIAPIToken,
					gitHubAccessToken, gitName, gitEmail, kmsKey, akrPass)
				//TODO: alert
				//TODO: don't delete the old key yet..needs proper testing first..
				//check(keys.DeleteKey(key))
				//TODO: alert
			} else {
				fmt.Println("Skipping SA: " + key.Account + ", key: " + key.ID +
					" as it's only " + fmt.Sprintf("%f", key.Age) + " minutes old.")
			}
		}
	}
}

func accountKeySources(account string, keySources []keySource) (accountKeySources []keySource,
	err error) {
	err = errors.New("No account key sources (in config) mapped to SA: " + account)
	for _, keySource := range keySources {
		fmt.Println(account)
		fmt.Println(keySource.ServiceAccountName)
		if account == keySource.ServiceAccountName {
			fmt.Println(keySource)
			err = nil
			accountKeySources = append(accountKeySources, keySource)
		}
	}
	fmt.Println(accountKeySources)
	return
}

func updateKeySources(keySources []keySource, newKey, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail, kmsKey, akrPass string) {
	for _, keySource := range keySources {
		log.Println(keySource)
		for _, circleCI := range keySource.CircleCI {
			updateCircleCI(circleCI, newKey, circleCIAPIToken)
		}
		if keySource.GitHub.OrgRepo != "" {
			orgRepo := keySource.GitHub.OrgRepo
			fmt.Println(orgRepo)
			updateGitHubRepo(keySource, gitHubAccessToken, gitName, gitEmail,
				circleCIAPIToken, newKey, kmsKey, akrPass)
		}
	}
}

func updateGitHubRepo(gitHubSource keySource,
	gitHubAccessToken, gitName, gitEmail, circleCIAPIToken, newKey, kmsKey, akrPass string) {
	singleLine := false
	//TODO: Get the last string from config (it's Mantle's -n flag)
	decodedKey, err := b64.StdEncoding.DecodeString(newKey)
	check(err)
	encKey := enc.CipherBytesFromPrimitives([]byte(decodedKey),
		singleLine, "", "", "", "", kmsKey)
	localDir := "cloud-key-rotator-tmp-repo"
	orgRepo := gitHubSource.GitHub.OrgRepo
	repo := cloneGitRepo(localDir, orgRepo, gitHubAccessToken)
	fmt.Println("cloned git repo: " + orgRepo)
	var gitCommentBuff bytes.Buffer
	gitCommentBuff.WriteString("CKR updating ")
	gitCommentBuff.WriteString(gitHubSource.ServiceAccountName)
	w, err := repo.Worktree()
	check(err)
	fullFilePath := localDir + "/" + gitHubSource.GitHub.Filepath
	check(ioutil.WriteFile(fullFilePath, encKey, 0644))
	w.Add(fullFilePath)
	signKey, err := commitSignKey(gitName, gitEmail, akrPass)
	check(err)
	commit, err := w.Commit(gitCommentBuff.String(), &git.CommitOptions{
		Author: &object.Signature{
			Name:  gitName,
			Email: gitEmail,
			When:  time.Now(),
		},
		All:     true,
		SignKey: signKey,
	})
	check(err)
	committed, err := repo.CommitObject(commit)
	check(err)
	fmt.Println("committed to local git repo: " + orgRepo)
	check(repo.Push(&git.PushOptions{Auth: &gitHttp.BasicAuth{
		Username: "abc123", // yes, this can be anything except an empty string
		Password: gitHubAccessToken,
	},
		Progress: os.Stdout}))
	fmt.Println("pushed to remote git repo: " + orgRepo)
	if gitHubSource.GitHub.VerifyCircleCISuccess {
		verifyCircleCIJobSuccess(gitHubSource.GitHub.OrgRepo,
			fmt.Sprintf("%s", committed.ID()),
			gitHubSource.GitHub.CircleCIDeployJobName, circleCIAPIToken)
	}
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

func verifyCircleCIJobSuccess(orgRepo, gitHash, circleCIDeployJobName, circleCIAPIToken string) {
	client := &circleci.Client{Token: circleCIAPIToken} // Token not required to query info for public projects
	splitOrgRepo := strings.Split(orgRepo, "/")
	org := splitOrgRepo[0]
	repo := splitOrgRepo[1]
	targetBuildNum := obtainBuildNum(org, repo, gitHash, circleCIDeployJobName, client)
	checkForJobSuccess(org, repo, targetBuildNum, client)
}

func checkForJobSuccess(org, repo string, targetBuildNum int, client *circleci.Client) {
	checkAttempts := 0
	checkLimit := 60
	checkInterval := 5 * time.Second
	for {
		build, err := client.GetBuild(org, repo, targetBuildNum)
		check(err)
		fmt.Println("checking status of build")
		if build.Status == "success" {
			fmt.Println("detected build success")
			break
		}
		checkAttempts++
		if checkAttempts == checkLimit {
			panic("Unable to verify CircleCI job was a success. You may need to" +
				" increase the check limit. https://circleci.com/gh/" + org + "/" + repo + "/" +
				strconv.Itoa(targetBuildNum))
		}
		time.Sleep(checkInterval)
	}
}

func obtainBuildNum(org, repo, gitHash, circleCIDeployJobName string, client *circleci.Client) (targetBuildNum int) {
	checkAttempts := 0
	checkLimit := 60
	checkInterval := 5 * time.Second
	for {
		builds, err := client.ListRecentBuildsForProject(org, repo, "master", "running", -1, 0)
		check(err)
		for _, build := range builds {
			fmt.Println("checking for target job in build: " + strconv.Itoa(build.BuildNum))
			if build.VcsRevision == gitHash && build.BuildParameters["CIRCLE_JOB"] == circleCIDeployJobName {
				fmt.Println(build.BuildParameters["CIRCLE_JOB"])
				targetBuildNum = build.BuildNum
				break
			}
		}
		if targetBuildNum > 0 {
			break
		}
		checkAttempts++
		if checkAttempts == checkLimit {
			panic("Unable to determine CircleCI build number from target job name: " + circleCIDeployJobName)
		}
		time.Sleep(checkInterval)
	}
	return
}

func updateCircleCI(circleCISource circleCI, newKey, circleCIAPIToken string) {
	fmt.Println("starting cirlceci key rotate process")
	client := &circleci.Client{Token: circleCIAPIToken}
	usernameProject := circleCISource.UsernameProject
	envVarName := circleCISource.EnvVar
	splitUsernameProject := strings.Split(usernameProject, "/")
	username := splitUsernameProject[0]
	project := splitUsernameProject[1]
	if verifyCircleCiEnvVar(username, project, envVarName, client) {
		check(client.DeleteEnvVar(username, project, envVarName))
		fmt.Println("Deleted env var: " + envVarName + " from: " + usernameProject)
		if !verifyCircleCiEnvVar(username, project, envVarName, client) {
			fmt.Println("Verified env var: " + envVarName + " deletion on: " + usernameProject)
		} else {
			panic("Env var: " + envVarName +
				" deletion failed on username/project: " + usernameProject)
		}
		_, err := client.AddEnvVar(username, project, envVarName, newKey)
		check(err)
		fmt.Println("Added env var: " + envVarName + " to: " + usernameProject)
		if verifyCircleCiEnvVar(username, project, envVarName, client) {
			fmt.Println("Verified new env var: " + envVarName + " on: " + usernameProject)
		} else {
			panic("New env var: " + envVarName +
				" not detected on username/project: " + usernameProject)
		}
	}
}

func verifyCircleCiEnvVar(username, project, envVarName string, client *circleci.Client) (exists bool) {
	envVars, err := client.ListEnvVars(username, project)
	check(err)
	for _, envVar := range envVars {
		if envVar.Name == envVarName {
			exists = true
			break
		}
	}
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
		if valid && contains(config.IncludeSAs, key.Account) {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return
}

func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
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

func commitSignKey(name, email, passphrase string) (entity *openpgp.Entity, err error) {
	reader, err := os.Open("akr.asc")
	check(err)
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
