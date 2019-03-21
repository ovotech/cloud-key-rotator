package cmd

import (
	"bytes"
	"context"
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
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	gkev1 "google.golang.org/api/container/v1"
	git "gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
	gitHttp "gopkg.in/src-d/go-git.v4/plumbing/transport/http"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
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

//k8s type
type k8s struct {
	Project     string
	Location    string
	ClusterName string
	Namespace   string
	SecretName  string
	DataName    string
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
	K8s                      []k8s
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

//googleAuthProvider type
type googleAuthProvider struct {
	tokenSource oauth2.TokenSource
}

var (
	googleScopes = []string{
		"https://www.googleapis.com/auth/cloud-platform",
		"https://www.googleapis.com/auth/userinfo.email"}
	_         rest.AuthProvider = &googleAuthProvider{}
	rotateCmd                   = &cobra.Command{
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
	account         string
	provider        string
	project         string
	defaultAccount  string
	defaultProvider string
	defaultProject  string
	logger          = stdoutLogger().Sugar()
)

const (
	datadogURL       = "https://api.datadoghq.com/api/v1/series?api_key="
	envVarPrefix     = "ckr"
	googleAuthPlugin = "google" // so that this is different than "gcp" that's already in client-go tree.
)

func init() {
	rotateCmd.Flags().StringVarP(&account, "account", "a", defaultAccount,
		"Account to rotate")
	rotateCmd.Flags().StringVarP(&provider, "provider", "p", defaultProvider,
		"Provider of account to rotate")
	rotateCmd.Flags().StringVarP(&project, "project", "j", defaultProject,
		"Project of account to rotate")
	rootCmd.AddCommand(rotateCmd)
	if err := rest.RegisterAuthProviderPlugin(googleAuthPlugin, newGoogleAuthProvider); err != nil {
		logger.Fatalf("Failed to register %s auth plugin: %v", googleAuthPlugin, err)
	}
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
		if err = rotateKeys(keySlice, c.KeySources, c.CircleCIAPIToken, c.GitHubAccessToken,
			c.GitName, c.GitEmail, c.KmsKey, c.AkrPass,
			c.DefaultRotationAgeThresholdMins); err != nil {
			return
		}
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
	gitHubAccessToken, gitName, gitEmail, kmsKey, akrPass string,
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
				if newKeyID, newKey, err = createKey(key, keyProvider); err != nil {
					return
				}
				//*****************************************************
				//  update sources
				//*****************************************************
				if err = updateKeySources(source, newKeyID, newKey, keyProvider, circleCIAPIToken,
					gitHubAccessToken, gitName, gitEmail, kmsKey,
					akrPass); err != nil {
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
func createKey(key keys.Key, keyProvider string) (newKeyID, newKey string, err error) {
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
	akrPass string) (err error) {
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
		if err = updateCircleCI(circleCI, keyID, key, circleCIAPIToken); err != nil {
			return
		}
		updatedSources = append(updatedSources, updatedSource{
			SourceType: "CircleCI",
			SourceURI:  circleCI.UsernameProject,
			SourceIDs:  []string{circleCI.KeyIDEnvVar, circleCI.KeyEnvVar}})
	}
	if len(keySource.GitHub.OrgRepo) > 0 {
		if len(kmsKey) > 0 {
			if err = updateGitHubRepo(keySource, gitHubAccessToken, gitName, gitEmail,
				circleCIAPIToken, key, kmsKey, akrPass); err != nil {
				return
			}
			updatedSources = append(updatedSources, updatedSource{
				SourceType: "GitHub",
				SourceURI:  keySource.GitHub.OrgRepo,
				SourceIDs:  []string{keySource.GitHub.Filepath}})
		} else {
			err = errors.New("Not updating un-encrypted new key in a Git repository. Use the" +
				"'KmsKey' field in config to specify the KMS key to use for encryption")
			return
		}
	}
	for _, k8sSecret := range keySource.K8s {
		var cluster *gkev1.Cluster
		if cluster, err = gkeCluster(k8sSecret.Project, k8sSecret.Location,
			k8sSecret.ClusterName); err != nil {
			return
		}
		var k8sClient *kubernetes.Clientset
		if k8sClient, err = kubernetesClient(cluster); err != nil {
			return
		}
		if _, err = updateK8sSecret(k8sSecret.SecretName, k8sSecret.DataName,
			k8sSecret.Namespace, key, k8sClient); err != nil {
			return
		}
	}
	return
}

//gkeCluster creates a GKE cluster struct
func gkeCluster(project, location, clusterName string) (cluster *gkev1.Cluster, err error) {
	ctx := context.Background()
	var httpClient *http.Client
	if httpClient, err = google.DefaultClient(ctx, gkev1.CloudPlatformScope); err != nil {
		return
	}
	var gkeService *gkev1.Service
	if gkeService, err = gkev1.New(httpClient); err != nil {
		return
	}
	cluster, err = gkeService.Projects.Locations.Clusters.
		Get(fmt.Sprintf("projects/%s/locations/%s/clusters/%s", project, location, clusterName)).
		Do()
	return
}

//kubernetesClient creates a kubernetes clientset
func kubernetesClient(cluster *gkev1.Cluster) (k8sclient *kubernetes.Clientset, err error) {
	var decodedClientCertificate []byte
	if decodedClientCertificate, err = b64.StdEncoding.
		DecodeString(cluster.MasterAuth.ClientCertificate); err != nil {
		return
	}
	var decodedClientKey []byte
	if decodedClientKey, err = b64.StdEncoding.
		DecodeString(cluster.MasterAuth.ClientKey); err != nil {
		return
	}
	var decodedClusterCaCertificate []byte
	if decodedClusterCaCertificate, err = b64.StdEncoding.
		DecodeString(cluster.MasterAuth.ClusterCaCertificate); err != nil {
		return
	}
	return kubernetes.NewForConfig(&rest.Config{
		Username: cluster.MasterAuth.Username,
		Password: cluster.MasterAuth.Password,
		Host:     "https://" + cluster.Endpoint,
		TLSClientConfig: rest.TLSClientConfig{
			Insecure: false,
			CertData: decodedClientCertificate,
			KeyData:  decodedClientKey,
			CAData:   decodedClusterCaCertificate,
		},
		AuthProvider: &clientcmdapi.AuthProviderConfig{Name: googleAuthPlugin},
	})
}

func (g *googleAuthProvider) WrapTransport(rt http.RoundTripper) http.RoundTripper {
	return &oauth2.Transport{
		Base:   rt,
		Source: g.tokenSource,
	}
}

func (g *googleAuthProvider) Login() error { return nil }

func newGoogleAuthProvider(addr string, config map[string]string,
	persister rest.AuthProviderConfigPersister) (authProvider rest.AuthProvider, err error) {
	var ts oauth2.TokenSource
	if ts, err = google.DefaultTokenSource(context.TODO(), googleScopes...); err != nil {
		return
	}
	return &googleAuthProvider{tokenSource: ts}, nil
}

//updateK8sSecret updates a specific namespace/secret/data with the key string
func updateK8sSecret(secretName, dataName, namespace, key string,
	k8sclient *kubernetes.Clientset) (newSecret *v1.Secret, err error) {
	logger.Info("Starting k8s secret updates")
	var secret *v1.Secret
	if secret, err = k8sclient.CoreV1().Secrets(namespace).Get(secretName,
		metav1.GetOptions{}); err != nil {
		return
	}
	var decodedKey []byte
	if decodedKey, err = b64.StdEncoding.DecodeString(key); err != nil {
		return
	}
	secret.Data = map[string][]byte{dataName: decodedKey}
	return k8sclient.CoreV1().Secrets(namespace).Update(secret)
}

//updateGitHubRepo updates the new key in the specified gitHubSource
func updateGitHubRepo(gitHubSource keySource,
	gitHubAccessToken, gitName, gitEmail, circleCIAPIToken, newKey, kmsKey,
	akrPass string) (err error) {
	singleLine := false
	disableValidation := true
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
		err = verifyCircleCIJobSuccess(gitHubSource.GitHub.OrgRepo,
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
	circleCIAPIToken string) (err error) {
	client := &circleci.Client{Token: circleCIAPIToken}
	splitOrgRepo := strings.Split(orgRepo, "/")
	org := splitOrgRepo[0]
	repo := splitOrgRepo[1]
	var targetBuildNum int
	if targetBuildNum, err = obtainBuildNum(org, repo, gitHash, circleCIDeployJobName,
		client); err != nil {
		return
	}
	return checkForJobSuccess(org, repo, targetBuildNum, client)
}

//checkForJobSuccess polls the circleCI API until the build is successful or
//failed, or a timeout is reached, whichever happens first
func checkForJobSuccess(org, repo string, targetBuildNum int,
	client *circleci.Client) (err error) {
	checkAttempts := 0
	checkLimit := 60
	checkInterval := 5 * time.Second
	logger.Infof("Polling CircleCI for status of build: %d", targetBuildNum)
	for {
		var build *circleci.Build
		if build, err = client.GetBuild(org, repo, targetBuildNum); err != nil {
			return
		}
		if build.Status == "success" {
			logger.Infof("Detected success of CircleCI build: %d", targetBuildNum)
			break
		} else if build.Status == "failed" {
			return fmt.Errorf("CircleCI job: %d has failed", targetBuildNum)
		}
		checkAttempts++
		if checkAttempts == checkLimit {
			return fmt.Errorf("Unable to verify CircleCI job was a success: https://circleci.com/gh/%s/%s/%d",
				org, repo, targetBuildNum)
		}
		time.Sleep(checkInterval)
	}
	return
}

//obtainBuildNum gets the number of the circleCI build by matching up the gitHash
func obtainBuildNum(org, repo, gitHash, circleCIDeployJobName string,
	client *circleci.Client) (targetBuildNum int, err error) {
	checkAttempts := 0
	checkLimit := 60
	checkInterval := 5 * time.Second
	for {
		var builds []*circleci.Build
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
			err = fmt.Errorf("Unable to determine CircleCI build number from target job name: %s",
				circleCIDeployJobName)
			return
		}
		time.Sleep(checkInterval)
	}
	return
}

//updateCircleCI updates the circleCI environment variable by deleting and
//then creating it again with the new key
func updateCircleCI(circleCISource circleCI, keyID, key, circleCIAPIToken string) (err error) {
	logger.Info("Starting CircleCI env var updates")
	client := &circleci.Client{Token: circleCIAPIToken}
	keyIDEnvVarName := circleCISource.KeyIDEnvVar
	splitUsernameProject := strings.Split(circleCISource.UsernameProject, "/")
	username := splitUsernameProject[0]
	project := splitUsernameProject[1]
	if len(keyIDEnvVarName) > 0 {
		if err = updateCircleCIEnvVar(username, project, keyIDEnvVarName, keyID,
			client); err != nil {
			return
		}
	}
	return updateCircleCIEnvVar(username, project, circleCISource.KeyEnvVar, key, client)
}

func updateCircleCIEnvVar(username, project, envVarName, envVarValue string,
	client *circleci.Client) (err error) {
	if err = verifyCircleCiEnvVar(username, project, envVarName, client); err != nil {
		return
	}
	if err = client.DeleteEnvVar(username, project, envVarName); err != nil {
		return
	}
	logger.Infof("Deleted CircleCI env var: %s from %s/%s", envVarName, username, project)
	if _, err = client.AddEnvVar(username, project, envVarName, envVarValue); err != nil {
		return
	}
	logger.Infof("Added CircleCI env var: %s to %s/%s", envVarName, username, project)
	return verifyCircleCiEnvVar(username, project, envVarName, client)
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
		err = fmt.Errorf("CircleCI env var: %s not detected on %s/%s",
			envVarName, username, project)
		return
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
				err = fmt.Errorf("non-202 status code (%d) returned by Datadog", resp.StatusCode)
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
		err = errors.New("ArmouredKeyRing passphrase must not be empty")
		return
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
