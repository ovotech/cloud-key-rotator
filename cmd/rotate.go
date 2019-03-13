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

	circleci "github.com/jszwedko/go-circleci"
	keys "github.com/ovotech/cloud-key-client"
	enc "github.com/ovotech/mantle/crypt"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"golang.org/x/crypto/openpgp"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
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

//updatedSource type
type updatedSource struct {
	SourceType string
	SourceURI  string
	SourceIDs  []string
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
		if err := rotate(); err != nil {
			logger.Error(err)
		}
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

func rotate() (err error) {
	defer logger.Sync()
	var c config
	if c, err = getConfig(); err != nil {
		return
	}
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
	var keySlice []keys.Key
	if keySlice, err = keys.Keys(providers); err != nil {
		return
	}
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
	return
}

//getConfig returns the application config
func getConfig() (c config, err error) {
	viper.AutomaticEnv()
	viper.SetEnvPrefix(envVarPrefix)
	viper.AddConfigPath("/etc/cloud-key-rotator/")
	viper.SetEnvPrefix("ckr")
	viper.SetConfigName("config")
	viper.AddConfigPath(".")
	viper.ReadInConfig()
	if err = viper.Unmarshal(&c); err != nil {
		return
	}
	if !viper.IsSet("cloudProviders") {
		err = errors.New("cloudProviders is not set")
		return
	}
	return
}

//rotatekeys runs through the end to end process of rotating a slice of keys:
//filter down to subset of target keys, generate new key for each, update the
//key's sources and finally delete the existing/old key
func rotateKeys(keySlice []keys.Key, keySources []keySource, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail, kmsKey, akrPass, slackWebhook string,
	defaultRotationAgeThresholdMins int) (err error) {
	processedItems := make([]string, 0)
	for _, key := range keySlice {
		keyAccount := key.Account
		if !contains(processedItems, keyAccount) {
			var source keySource
			if source, err = accountKeySource(keyAccount, keySources); err != nil {
				return
			}
			rotationAgeThresholdMins := defaultRotationAgeThresholdMins
			if source.RotationAgeThresholdMins > 0 {
				rotationAgeThresholdMins = source.RotationAgeThresholdMins
			}
			processedItems = append(processedItems, keyAccount)
			if key.Age > float64(rotationAgeThresholdMins) {
				keyProvider := key.Provider.Provider
				informProcessStart(key, keyProvider, rotationAgeThresholdMins)
				//*****************************************************
				//  create key
				//*****************************************************
				var newKeyID string
				var newKey string
				if newKeyID, newKey, err = createKey(key, keyProvider, slackWebhook); err != nil {
					return
				}
				//*****************************************************
				//  update sources
				//*****************************************************
				if err = updateKeySources(source, newKeyID, newKey, keyProvider, circleCIAPIToken,
					gitHubAccessToken, gitName, gitEmail, kmsKey,
					akrPass, slackWebhook); err != nil {
					return
				}
				//*****************************************************
				//  delete old key
				//*****************************************************
				deleteKey(key, keyProvider)
			} else {
				logger.Infof("Skipping SA: %s, key: %s as it's only %f minutes old",
					account, key.ID, key.Age)
			}
		} else {
			logger.Infof("Skipping SA: %s, key: %s as this account has already been processed",
				account, key.ID)
		}
	}
	return
}

//informProcessStart informs of the rotation process starting
func informProcessStart(key keys.Key, keyProvider string, rotationAgeThresholdMins int) {
	logger.Infow("Rotation process started",
		"keyProvider", keyProvider,
		"account", account,
		"keyID", key.ID,
		"keyAge", fmt.Sprintf("%f", key.Age),
		"keyAgeThreshold", strconv.Itoa(rotationAgeThresholdMins))
}

//createKey creates a new key with the provider specified
func createKey(key keys.Key, keyProvider, slackWebhook string) (newKeyID, newKey string, err error) {
	if newKeyID, newKey, err = keys.CreateKey(key); err != nil {
		logger.Error(err)
		return
	}
	logger.Infow("New key created",
		"keyProvider", keyProvider,
		"account", account,
		"keyID", newKeyID)
	return
}

//updateKeySources updates the sources of the key
func updateKeySources(keySource keySource, keyID, key, keyProvider, circleCIAPIToken,
	gitHubAccessToken, gitName, gitEmail, kmsKey,
	akrPass, slackWebhook string) (err error) {
	var updatedSources []updatedSource
	if updatedSources, err = updateKeySource(keySource, keyID, key,
		circleCIAPIToken, gitHubAccessToken, gitName, gitEmail, kmsKey, akrPass); err != nil {
		return
	}
	logger.Infow("Key sources updated",
		"keyProvider", keyProvider,
		"account", account,
		"keyID", keyID,
		"keySourceUpdates", updatedSources)
	return
}

//deletekey deletes the key
func deleteKey(key keys.Key, keyProvider string) (err error) {
	if err = keys.DeleteKey(key); err != nil {
		return
	}
	logger.Infow("Old key deleted",
		"keyProvider", keyProvider,
		"account", account,
		"keyID", key.ID)
	return
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
	akrPass string) (updatedSources []updatedSource, err error) {

	for _, circleCI := range keySource.CircleCI {
		updateCircleCI(circleCI, keyID, key, circleCIAPIToken)
		updatedSources = append(updatedSources, updatedSource{
			SourceType: "CircleCI",
			SourceURI:  circleCI.UsernameProject,
			SourceIDs:  []string{circleCI.KeyIDEnvVar, circleCI.KeyEnvVar}})
	}

	if len(keySource.GitHub.OrgRepo) > 0 {
		if len(kmsKey) > 0 {
			updateGitHubRepo(keySource, gitHubAccessToken, gitName, gitEmail,
				circleCIAPIToken, key, kmsKey, akrPass)
			updatedSources = append(updatedSources, updatedSource{
				SourceType: "GitHub",
				SourceURI:  keySource.GitHub.OrgRepo,
				SourceIDs:  []string{keySource.GitHub.Filepath}})
		} else {
			logger.Panic("Not updating un-encrypted new key in a Git repository. Use the" +
				"'KmsKey' field in config to specify the KMS key to use for encryption")
		}
	}
	return
}

//updateGitHubRepo updates the new key in the specified gitHubSource
func updateGitHubRepo(gitHubSource keySource,
	gitHubAccessToken, gitName, gitEmail, circleCIAPIToken, newKey, kmsKey,
	akrPass string) (err error) {
	singleLine := false
	disableValidation := false
	var decodedKey []byte
	if decodedKey, err = b64.StdEncoding.DecodeString(newKey); err != nil {
		return
	}
	encKey := enc.CipherBytesFromPrimitives([]byte(decodedKey),
		singleLine, disableValidation, "", "", "", "", kmsKey)
	localDir := "/etc/cloud-key-rotator/cloud-key-rotator-tmp-repo"
	orgRepo := gitHubSource.GitHub.OrgRepo
	var repo *git.Repository
	if repo, err = cloneGitRepo(localDir, orgRepo, gitHubAccessToken); err != nil {
		return
	}
	logger.Infof("Cloned git repo: %s", orgRepo)
	var gitCommentBuff bytes.Buffer
	gitCommentBuff.WriteString("CKR updating ")
	gitCommentBuff.WriteString(gitHubSource.ServiceAccountName)
	var w *git.Worktree
	if w, err = repo.Worktree(); err != nil {
		return
	}
	fullFilePath := localDir + "/" + gitHubSource.GitHub.Filepath
	if err = ioutil.WriteFile(fullFilePath, encKey, 0644); err != nil {
		return
	}
	w.Add(fullFilePath)
	var signKey *openpgp.Entity
	if signKey, err = commitSignKey(gitName, gitEmail, akrPass); err != nil {
		return
	}
	autoStage := true
	var commit plumbing.Hash
	if commit, err = w.Commit(gitCommentBuff.String(), &git.CommitOptions{
		Author: &object.Signature{
			Name:  gitName,
			Email: gitEmail,
			When:  time.Now(),
		},
		All:     autoStage,
		SignKey: signKey,
	}); err != nil {
		return
	}
	var committed *object.Commit
	if committed, err = repo.CommitObject(commit); err != nil {
		return
	}
	logger.Infof("Committed to local git repo: %s", orgRepo)
	if err = repo.Push(&git.PushOptions{Auth: &gitHttp.BasicAuth{
		Username: "abc123", // yes, this can be anything except an empty string
		Password: gitHubAccessToken,
	},
		Progress: os.Stdout}); err != nil {
		return
	}
	logger.Infof("Pushed to remote git repo: %s", orgRepo)
	if gitHubSource.GitHub.VerifyCircleCISuccess {
		verifyCircleCIJobSuccess(gitHubSource.GitHub.OrgRepo,
			fmt.Sprintf("%s", committed.ID()),
			gitHubSource.GitHub.CircleCIDeployJobName, circleCIAPIToken)
	}
	os.RemoveAll(localDir)
	return
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
		var build *circleci.Build
		var err error
		if build, err = client.GetBuild(org, repo, targetBuildNum); err != nil {
			return
		}
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
		var builds []*circleci.Build
		var err error
		if builds, err = client.ListRecentBuildsForProject(org, repo, "master",
			"running", -1, 0); err != nil {
			return
		}
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
	client *circleci.Client) (err error) {
	verifyCircleCiEnvVar(username, project, envVarName, client)
	if err = client.DeleteEnvVar(username, project, envVarName); err != nil {
		return
	}
	logger.Infof("Deleted CircleCI env var: %s from %s/%s", envVarName, username, project)
	if _, err = client.AddEnvVar(username, project, envVarName, envVarValue); err != nil {
		return
	}
	logger.Infof("Added CircleCI env var: %s to %s/%s", envVarName, username, project)
	verifyCircleCiEnvVar(username, project, envVarName, client)
	return
}

func verifyCircleCiEnvVar(username, project, envVarName string,
	client *circleci.Client) (err error) {
	var exists bool
	var envVars []circleci.EnvVar
	if envVars, err = client.ListEnvVars(username, project); err != nil {
		return
	}
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
	return
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
func postMetric(keys []keys.Key, apiKey string, datadog datadog) (err error) {
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
			var req *http.Request
			if req, err = http.NewRequest("POST", url, bytes.NewBuffer(jsonString)); err != nil {
				return
			}
			req.Header.Set("Content-type", "application/json")
			client := &http.Client{}
			var resp *http.Response
			if resp, err = client.Do(req); err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != 202 {
				logger.Panicf("non-202 status code (%d) returned by Datadog", resp.StatusCode)
			}
		}
	}
	return
}

//commitSignKey creates an openPGP Entity based on a user's name, email,
//armoredKeyRing and passphrase for the key ring. This commitSignKey can then
//be used to GPG sign Git commits
func commitSignKey(name, email, passphrase string) (entity *openpgp.Entity,
	err error) {
	if passphrase == "" {
		logger.Panic("ArmouredKeyRing passphrase must not be empty")
	}
	var reader *os.File
	if reader, err = os.Open("/etc/cloud-key-rotator/akr.asc"); err != nil {
		return
	}
	var entityList openpgp.EntityList
	if entityList, err = openpgp.ReadArmoredKeyRing(reader); err != nil {
		return
	}
	_, ok := entityList[0].Identities[strings.Join([]string{name, " <", email, ">"}, "")]
	if !ok {
		err = errors.New("Failed to add Identity to EntityList")
	}
	if err = entityList[0].PrivateKey.Decrypt([]byte(passphrase)); err != nil {
		return
	}
	entity = entityList[0]
	return
}
