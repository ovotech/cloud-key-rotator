package location

import (
	"encoding/base64"
	"net/http"
	"time"

	"github.com/ovotech/cloud-key-rotator/pkg/cred"

	"github.com/CircleCI-Public/circleci-cli/api/context"
	"github.com/CircleCI-Public/circleci-cli/settings"

	"github.com/cenkalti/backoff/v4"
)

// CircleCIContext type
type CircleCIContext struct {
	ContextID    string
	OrgID        string
	VcsType      string
	OrgName      string
	KeyIDEnvVar  string
	KeyEnvVar    string
	Base64Decode bool
}

func (circleContext CircleCIContext) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	logger.Info("Starting CircleCI context env var updates")
	cfg := settings.Config{
		Host:         "https://circleci.com",
		HTTPClient:   http.DefaultClient,
		RestEndpoint: "api/v2",
		Token:        creds.CircleCIAPIToken,
	}
	restClient := context.NewContextClient(&cfg, circleContext.OrgID, circleContext.VcsType, circleContext.OrgName)

	provider := keyWrapper.KeyProvider
	contextID := circleContext.ContextID
	key := keyWrapper.Key
	// if configured, base64 decode the key (GCP return encoded keys)
	if circleContext.Base64Decode {
		var keyb []byte
		keyb, err = base64.StdEncoding.DecodeString(key)
		if err != nil {
			return
		}
		key = string(keyb)
	}

	var keyEnvVar string
	var idValue bool
	if keyEnvVar, err = getVarNameFromProvider(provider, circleContext.KeyEnvVar, idValue); err != nil {
		return
	}

	var keyIDEnvVar string
	idValue = true
	if keyIDEnvVar, err = getVarNameFromProvider(provider, circleContext.KeyIDEnvVar, idValue); err != nil {
		return
	}

	if len(keyIDEnvVar) > 0 {
		if err = updateCircleCIContext(contextID, keyIDEnvVar, keyWrapper.KeyID, restClient); err != nil {
			return
		}
	}

	if err = updateCircleCIContext(contextID, keyEnvVar, key, restClient); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "CircleCIContext",
		LocationURI:  contextID,
		LocationIDs:  []string{keyIDEnvVar, keyEnvVar}}

	return updated, nil

}

func updateCircleCIContext(contextID, envVarName, envVarValue string,
	restClient context.ContextInterface) (err error) {

	maxElapsedTimeSecs := 500
	backoffMultiplier := 5

	deleteOp := func() error {
		logger.Infof("Deleting env var %s on contextID: %s", envVarName, contextID)
		return restClient.DeleteEnvironmentVariable(contextID, envVarName)
	}
	err = callCircleCIWithExpBackoff(deleteOp,
		time.Duration(maxElapsedTimeSecs)*time.Second,
		float64(backoffMultiplier))
	if err != nil {
		return
	}
	createOp := func() error {
		logger.Infof("Creating env var %s on contextID: %s", envVarName, contextID)
		return restClient.CreateEnvironmentVariable(contextID, envVarName, envVarValue)
	}
	err = callCircleCIWithExpBackoff(createOp,
		time.Duration(maxElapsedTimeSecs)*time.Second,
		float64(backoffMultiplier))
	return
}

// callCircleCIWithExpBackoff calls the CircleCI API with exponential backoff
func callCircleCIWithExpBackoff(operation backoff.Operation,
	maxElapsedTime time.Duration, multiplier float64) (err error) {
	b := backoff.NewExponentialBackOff()
	b.MaxElapsedTime = maxElapsedTime
	b.Multiplier = multiplier
	err = backoff.Retry(operation, b)
	return
}
