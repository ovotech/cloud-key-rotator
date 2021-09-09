package location

import (
	"encoding/base64"
	"net/http"

	"github.com/ovotech/cloud-key-rotator/pkg/cred"

	"github.com/CircleCI-Public/circleci-cli/api"
	"github.com/CircleCI-Public/circleci-cli/settings"
)

// CircleCIContext type
type CircleCIContext struct {
	ContextID    string
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
	var restClient *api.ContextRestClient
	if restClient, err = api.NewContextRestClient(cfg); err != nil {
		return
	}

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
	restClient *api.ContextRestClient) (err error) {

	if err = restClient.DeleteEnvironmentVariable(contextID, envVarName); err != nil {
		return
	}
	return restClient.CreateEnvironmentVariable(contextID, envVarName, envVarValue)
}
