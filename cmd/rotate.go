package cmd

import (
	"bytes"
	b64 "encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"go.uber.org/zap"

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
	datadogURL            = "https://api.datadoghq.com/api/v1/series?api_key="
	envVarPrefix          = "ckr"
	slackInlineCodeMarker = "```"
)

//cloudProvider type
type cloudProvider struct {
	Name    string
	Project string
}

//circleCI type
type circleCI struct {
	UsernameProject string
	KeyIDEnvVar     string
	KeyEnvVar       string
}

//datadog type
type datadog struct {
	MetricEnv  string
	MetricTeam string
	MetricName string
}

//gitHub type
type gitHub struct {
	Filepath              string
	OrgRepo               string
	VerifyCircleCISuccess bool
	CircleCIDeployJobName string
}

//keySource type
type keySource struct {
	RotationAgeThresholdMins int
	ServiceAccountName       string
	CircleCI                 []circleCI
	GitHub                   gitHub
}

//serviceAccount type
type providerServiceAccounts struct {
	Provider cloudProvider
	Accounts []string
}

//config type
type config struct {
	AkrPass                         string
	IncludeAwsUserKeys              bool
	Datadog                         datadog
	DatadogAPIKey                   string
	RotationMode                    bool
	CloudProviders                  []cloudProvider
	ExcludeSAs                      []providerServiceAccounts
	IncludeSAs                      []providerServiceAccounts
	Blacklist                       []string
	Whitelist                       []string
	KeySources                      []keySource
	CircleCIAPIToken                string
	GitHubAccessToken               string
	GitName                         string
	GitEmail                        string
	KmsKey                          string
	SlackWebhook                    string
	DefaultRotationAgeThresholdMins int
}

// rotateCmd represents the save command
var rotateCmd = &cobra.Command{
	Use:   "rotate",
	Short: "Rotate some cloud keys",
	Long:  `Rotate some cloud keys`,
	Run: func(cmd *cobra.Command, args []string) {
		logger.Info("cloud-key-rotator rotate called")
		rotate()
	},
}

var account string
var provider string
var project string

var logger = stdoutLogger().Sugar()

func init() {
	defaultAccount := ""
	defaultProvider := ""
	defaultProject := ""
	rotateCmd.Flags().StringVarP(&account, "account", "a", defaultAccount,
		"Account to rotate")
	rotateCmd.Flags().StringVarP(&provider, "provider", "p", defaultProvider,
		"Provider of account to rotate")
	rotateCmd.Flags().StringVarP(&project, "project", "j", defaultProject,
		"Project of account to rotate")
	rootCmd.AddCommand(rotateCmd)
}

func rotate() {
	defer logger.Sync()
	c := getConfig()
	providers := make([]keys.Provider, 0)
	if len(provider) > 0 {
		providers = append(providers, keys.Provider{GcpProject: project,
			Provider: provider})
	} else {
		for _, provider := range c.CloudProviders {
			providers = append(providers, keys.Provider{GcpProject: provider.Project,
				Provider: provider.Name})
		}
	}
	keySlice := keys.Keys(providers)
	logger.Infof("Found %d keys in total", len(keySlice))
	keySlice = filterKeys(keySlice, c, account)
	if c.RotationMode {
		logger.Infof("Filtered down to %d keys to rotate", len(keySlice))
		rotateKeys(keySlice, c.KeySources, c.CircleCIAPIToken, c.GitHubAccessToken,
			c.GitName, c.GitEmail, c.KmsKey, c.AkrPass, c.SlackWebhook,
			c.DefaultRotationAgeThresholdMins)
	} else {
		postMetric(keySlice, c.DatadogAPIKey, c.Datadog)
	}
}

//getConfig returns the application config
func getConfig() (c config) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix(envVarPrefix)
	viper.AddConfigPath("/etc/cloud-key-rotator/")
	viper.SetEnvPrefix("ckr")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.ReadInConfig()
	check(viper.Unmarshal(&c))
	if !viper.IsSet("cloudProviders") {
		logger.Panic("cloudProviders is not set")
	}
	return
}

//rotatekeys runs through the end to end process of rotating a slice of keys:
//filter down to subset of target keys, generate new key for each, update the
//key's sources and finally delete the existing/old key
func rotateKeys(keySlice []keys.Key, keySources []keySource, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail, kmsKey, akrPass, slackWebhook string,
	defaultRotationAgeThresholdMins int) {
	processedItems := make([]string, 0)
	for _, key := range keySlice {
		keyAccount := key.Account
		if !contains(processedItems, keyAccount) {
			accountKeySource, err := accountKeySource(keyAccount, keySources)
			check(err)
			rotationAgeThresholdMins := defaultRotationAgeThresholdMins
			if accountKeySource.RotationAgeThresholdMins > 0 {
				rotationAgeThresholdMins = accountKeySource.RotationAgeThresholdMins
			}
			processedItems = append(processedItems, keyAccount)
			if key.Age > float64(rotationAgeThresholdMins) {
				keyProvider := key.Provider.Provider
				informProcessStart(key, keyProvider, rotationAgeThresholdMins)
				//*****************************************************
				//  create key
				//*****************************************************
				newKeyID, newKey := createKey(key, keyProvider, slackWebhook)
				//*****************************************************
				//  update sources
				//*****************************************************
				updateKeySources(accountKeySource, newKeyID, newKey, keyProvider, circleCIAPIToken,
					gitHubAccessToken, gitName, gitEmail, kmsKey,
					akrPass, slackWebhook)
				//*****************************************************
				//  delete old key
				//*****************************************************
				deleteKey(key, keyProvider)
			} else {
				logger.Infof("Skipping SA: %s, key: %s as it's only %s minutes old",
					account, key.ID, fmt.Sprintf("%f", key.Age))
			}
		} else {
			logger.Infof("Skipping SA: %s, key: %s as this account has already been processed",
				account, key.ID)
		}
	}
}

//informProcessStart informs of the rotation process starting
func informProcessStart(key keys.Key, keyProvider string, rotationAgeThresholdMins int) {
	logger.Infow("Rotation process started", map[string]string{
		"keyProvider":     keyProvider,
		"account":         account,
		"keyID":           key.ID,
		"keyAge":          fmt.Sprintf("%f", key.Age),
		"keyAgeThreshold": strconv.Itoa(rotationAgeThresholdMins)})
}

//createKey creates a new key with the provider specified
func createKey(key keys.Key, keyProvider, slackWebhook string) (newKeyID, newKey string) {
	var err error
	newKeyID, newKey, err = keys.CreateKey(key)
	check(err)
	logger.Infow("New key created",
		"keyProvider", keyProvider,
		"account", account,
		"keyID", newKeyID)
	return
}

//updateKeySources updates the sources of the key
func updateKeySources(keySource keySource, keyID, key, keyProvider, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail, kmsKey,
	akrPass, slackWebhook string) (updateSuccessMap map[string][]string) {
	updateSuccessMap = updateKeySource(keySource, keyID, key,
		circleCIAPIToken, gitHubAccessToken, gitName, gitEmail, kmsKey, akrPass)
	logger.Infow("Key sources updated",
		"keyProvider", keyProvider,
		"account", account,
		"keyID", keyID,
		"keySourceUpdates", updateSuccessMap)
	return
}

//deletekey deletes the key
func deleteKey(key keys.Key, keyProvider string) {
	check(keys.DeleteKey(key))
	logger.Infow("Old key deleted",
		"keyProvider", keyProvider,
		"account", account,
		"keyID", key.ID)
}

//stdoutLogger creates a stdout logger
func stdoutLogger() (logger *zap.Logger) {
	config := zap.NewProductionConfig()
	config.OutputPaths = []string{"stdout"}
	config.ErrorOutputPaths = []string{"stdout"}
	logger, _ = config.Build()
	return
}

//accountKeySource gets the keySource element defined in config for the
//specified account
func accountKeySource(account string,
	keySources []keySource) (accountKeySource keySource, err error) {
	err = errors.New("No account key sources (in config) mapped to SA: " + account)
	for _, keySource := range keySources {
		if account == keySource.ServiceAccountName {
			err = nil
			accountKeySource = keySource
			break
		}
	}
	return
}

//updateKeySource updates the keySources specified with the new key
func updateKeySource(keySource keySource, keyID, key, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail, kmsKey,
	akrPass string) (updateSuccessMap map[string][]string) {
	updateSuccessMap = make(map[string][]string)
	for _, circleCI := range keySource.CircleCI {
		updateCircleCI(circleCI, keyID, key, circleCIAPIToken)
		registerUpdateSuccess("CircleCI", circleCI.UsernameProject, []string{circleCI.KeyIDEnvVar, circleCI.KeyEnvVar},
			updateSuccessMap)
	}
	if len(keySource.GitHub.OrgRepo) > 0 {
		if len(kmsKey) > 0 {
			updateGitHubRepo(keySource, gitHubAccessToken, gitName, gitEmail,
				circleCIAPIToken, key, kmsKey, akrPass)
			registerUpdateSuccess("GitHub",
				keySource.GitHub.OrgRepo, []string{keySource.GitHub.Filepath}, updateSuccessMap)
		} else {
			logger.Panic("Not updating un-encrypted new key in a Git repository. Use the" +
				"'KmsKey' field in config to specify the KMS key to use for encryption")
		}
	}
	return
}

func registerUpdateSuccess(sourceType, sourceRepo string, sourceIDs []string,
	updateSuccessMap map[string][]string) {
	updateSlice := make([]string, 0)
	if updateSuccessSlice, ok := updateSuccessMap[sourceType]; ok {
		updateSlice = updateSuccessSlice
	}
	first := true
	var sourceIDBuff bytes.Buffer
	sourceIDBuff.WriteString(sourceRepo)
	sourceIDBuff.WriteString(": ")
	for _, sourceID := range sourceIDs {
		if len(sourceID) > 0 {
			if !first {
				sourceIDBuff.WriteString(", ")
			}
			first = false
			sourceIDBuff.WriteString(sourceID)
		}
	}
	updateSlice = append(updateSlice, sourceIDBuff.String())
	updateSuccessMap[sourceType] = updateSlice
}

//updateGitHubRepo updates the new key in the specified gitHubSource
func updateGitHubRepo(gitHubSource keySource,
	gitHubAccessToken, gitName, gitEmail, circleCIAPIToken, newKey, kmsKey,
	akrPass string) {
	singleLine := false
	disableValidation := false
	decodedKey, err := b64.StdEncoding.DecodeString(newKey)
	check(err)
	encKey := enc.CipherBytesFromPrimitives([]byte(decodedKey),
		singleLine, disableValidation, "", "", "", "", kmsKey)
	localDir := "/etc/cloud-key-rotator/cloud-key-rotator-tmp-repo"
	orgRepo := gitHubSource.GitHub.OrgRepo
	repo := cloneGitRepo(localDir, orgRepo, gitHubAccessToken)
	logger.Infof("Cloned git repo: %s", orgRepo)
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
	logger.Infof("Committed to local git repo: %s", orgRepo)
	check(repo.Push(&git.PushOptions{Auth: &gitHttp.BasicAuth{
		Username: "abc123", // yes, this can be anything except an empty string
		Password: gitHubAccessToken,
	},
		Progress: os.Stdout}))
	logger.Infof("Pushed to remote git repo: %s", orgRepo)
	if gitHubSource.GitHub.VerifyCircleCISuccess {
		verifyCircleCIJobSuccess(gitHubSource.GitHub.OrgRepo,
			fmt.Sprintf("%s", committed.ID()),
			gitHubSource.GitHub.CircleCIDeployJobName, circleCIAPIToken)
	}
	os.RemoveAll(localDir)
}

//cloneGitRepo clones the specified Git repository into a local directory
func cloneGitRepo(localDir, orgRepo, token string) (repo *git.Repository) {
	url := strings.Join([]string{"https://github.com/", orgRepo, ".git"}, "")
	repo, err := git.PlainClone(localDir, false, &git.CloneOptions{
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

//verifyCircleCIJobSuccess uses the specified gitHash to track down the circleCI
//build number, which it then uses to determine the status of the circleCI build
func verifyCircleCIJobSuccess(orgRepo, gitHash, circleCIDeployJobName,
	circleCIAPIToken string) {
	client := &circleci.Client{Token: circleCIAPIToken}
	splitOrgRepo := strings.Split(orgRepo, "/")
	org := splitOrgRepo[0]
	repo := splitOrgRepo[1]
	targetBuildNum := obtainBuildNum(org, repo, gitHash, circleCIDeployJobName,
		client)
	checkForJobSuccess(org, repo, targetBuildNum, client)
}

//checkForJobSuccess polls the circleCI API until the build is successful or
//failed, or a timeout is reached, whichever happens first
func checkForJobSuccess(org, repo string, targetBuildNum int,
	client *circleci.Client) {
	checkAttempts := 0
	checkLimit := 60
	checkInterval := 5 * time.Second
	logger.Infof("Polling CircleCI for status of build: %d", targetBuildNum)
	for {
		build, err := client.GetBuild(org, repo, targetBuildNum)
		check(err)
		if build.Status == "success" {
			logger.Infof("Detected success of CircleCI build: %d", targetBuildNum)
			break
		} else if build.Status == "failed" {
			logger.Panicf("CircleCI job: %d has failed", targetBuildNum)
		}
		checkAttempts++
		if checkAttempts == checkLimit {
			logger.Panicf("Unable to verify CircleCI job was a success: https://circleci.com/gh/%s/repo/%d",
				targetBuildNum)
		}
		time.Sleep(checkInterval)
	}
}

//obtainBuildNum gets the number of the circleCI build by matching up the gitHash
func obtainBuildNum(org, repo, gitHash, circleCIDeployJobName string,
	client *circleci.Client) (targetBuildNum int) {
	checkAttempts := 0
	checkLimit := 60
	checkInterval := 5 * time.Second
	for {
		builds, err := client.ListRecentBuildsForProject(org, repo, "master",
			"running", -1, 0)
		check(err)
		for _, build := range builds {
			logger.Infof("Checking for target job in CircleCI build: %s", build.BuildNum)
			if build.VcsRevision == gitHash &&
				build.BuildParameters["CIRCLE_JOB"] == circleCIDeployJobName {
				targetBuildNum = build.BuildNum
				break
			}
		}
		if targetBuildNum > 0 {
			break
		}
		checkAttempts++
		if checkAttempts == checkLimit {
			logger.Panicf("Unable to determine CircleCI build number from target job name: %s",
				circleCIDeployJobName)
		}
		time.Sleep(checkInterval)
	}
	return
}

//updateCircleCI updates the circleCI environment variable by deleting and
//then creating it again with the new key
func updateCircleCI(circleCISource circleCI, keyID, key, circleCIAPIToken string) {
	logger.Info("Starting CircleCI env var updates")
	client := &circleci.Client{Token: circleCIAPIToken}
	keyIDEnvVarName := circleCISource.KeyIDEnvVar
	splitUsernameProject := strings.Split(circleCISource.UsernameProject, "/")
	username := splitUsernameProject[0]
	project := splitUsernameProject[1]
	if len(keyIDEnvVarName) > 0 {
		updateCircleCIEnvVar(username, project, keyIDEnvVarName, keyID, client)
	}
	updateCircleCIEnvVar(username, project, circleCISource.KeyEnvVar, key, client)
}

func updateCircleCIEnvVar(username, project, envVarName, envVarValue string,
	client *circleci.Client) {
	verifyCircleCiEnvVar(username, project, envVarName, client)
	check(client.DeleteEnvVar(username, project, envVarName))
	logger.Infof("Deleted CircleCI env var: %s from %s/%s", envVarName, username, project)
	_, err := client.AddEnvVar(username, project, envVarName, envVarValue)
	check(err)
	logger.Infof("Added CircleCI env var: %s to %s/%s", envVarName, username, project)
	verifyCircleCiEnvVar(username, project, envVarName, client)
}

func verifyCircleCiEnvVar(username, project, envVarName string,
	client *circleci.Client) {
	var exists bool
	envVars, err := client.ListEnvVars(username, project)
	check(err)
	for _, envVar := range envVars {
		if envVar.Name == envVarName {
			exists = true
			break
		}
	}
	if exists {
		logger.Infof("Verified CircleCI env var: %s on %s/%s",
			envVarName, username, project)
	} else {
		logger.Panicf("CircleCI env var: %s not detected on %s/%s",
			envVarName, username, project)
	}
}

//filterKeys returns a keys.Key slice created by filtering the provided
// keys.Key slice based on specific rules for each provider
func filterKeys(keys []keys.Key, config config, account string) (filteredKeys []keys.Key) {
	for _, key := range keys {
		//valid bool is used to filter out keys early, e.g. if config says don't
		//include AWS user keys, and the current key happens to be a user key
		valid := true
		if key.Provider.Provider == "aws" {
			valid = validAwsKey(key, config)
		}
		var eligible bool
		if valid {
			if len(account) > 0 {
				eligible = key.Account == account
			} else {
				includeSASlice := config.IncludeSAs
				excludeSASlice := config.ExcludeSAs
				if len(includeSASlice) > 0 {
					eligible = keyDefinedInFiltering(includeSASlice, key)
				} else if len(excludeSASlice) > 0 {
					eligible = !keyDefinedInFiltering(excludeSASlice, key)
				} else {
					//if no include or exclude filters have been set, we still want to include
					//ALL keys if we're NOT operating in rotation mode, i.e., just posting key
					//ages out to external places
					eligible = !config.RotationMode
				}
			}
		}
		if eligible {
			filteredKeys = append(filteredKeys, key)
		}
	}
	return
}

func keyDefinedInFiltering(providerServiceAccounts []providerServiceAccounts,
	key keys.Key) (defined bool) {
	for _, psa := range providerServiceAccounts {
		if psa.Provider.Name == key.Provider.Provider &&
			psa.Provider.Project == key.Provider.GcpProject {
			for _, sa := range psa.Accounts {
				defined = sa == key.Account
				if defined {
					break
				}
			}
			if defined {
				break
			}
		}
	}
	return defined
}

//contains returns true if the string slice contains the specified string
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

//postMetric posts details of each keys.Key to a metrics api
func postMetric(keys []keys.Key, apiKey string, datadog datadog) {
	if len(apiKey) > 0 {
		url := strings.Join([]string{datadogURL, apiKey}, "")
		for _, key := range keys {
			var jsonString = []byte(
				`{ "series" :[{"metric":"` + datadog.MetricName + `",` +
					`"points":[[` +
					strconv.FormatInt(time.Now().Unix(), 10) +
					`, ` + strconv.FormatFloat(key.Age, 'f', 2, 64) +
					`]],` +
					`"type":"count",` +
					`"tags":[` +
					`"team:` + datadog.MetricTeam + `",` +
					`"environment:` + datadog.MetricEnv + `",` +
					`"key:` + key.Name + `",` +
					`"provider:` + key.Provider.Provider + `",` +
					`"account:` + key.Account +
					`"]}]}`)
			req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonString))
			check(err)
			req.Header.Set("Content-type", "application/json")
			client := &http.Client{}
			resp, err := client.Do(req)
			check(err)
			defer resp.Body.Close()
			if resp.StatusCode != 202 {
				logger.Panicf("non-202 status code (%d) returned by Datadog", resp.StatusCode)
			}
		}
	}
}

//check panics if error is not nil
func check(e error) {
	if e != nil {
		logger.Panic(e.Error())
	}
}

//commitSignKey creates an openPGP Entity based on a user's name, email,
//armoredKeyRing and passphrase for the key ring. This commitSignKey can then
//be used to GPG sign Git commits
func commitSignKey(name, email, passphrase string) (entity *openpgp.Entity,
	err error) {
	if passphrase == "" {
		logger.Panic("ArmouredKeyRing passphrase must not be empty")
	}
	reader, err := os.Open("/etc/cloud-key-rotator/akr.asc")
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
