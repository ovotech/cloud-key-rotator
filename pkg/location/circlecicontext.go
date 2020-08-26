package location

import (
	"github.com/ovotech/cloud-key-rotator/pkg/cred"

	"github.com/CircleCI-Public/circleci-cli/api"
)

// CircleCIContext type
type CircleCIContext struct {
	ContextID   string
	KeyIDEnvVar string
	KeyEnvVar   string
}

func (circleContext CircleCIContext) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	logger.Info("Starting CircleCI context env var updates")

	gqlclient := api.NewContextGraphqlClient(
		"https://circleci.com",
		"graphql",
		creds.CircleCIAPIToken,
		false,
	)

	provider := keyWrapper.KeyProvider
	contextID := circleContext.ContextID

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
		if err = updateCircleCIContext(contextID, keyIDEnvVar, keyWrapper.KeyID, gqlclient); err != nil {
			return
		}
	}

	if err = updateCircleCIContext(contextID, keyEnvVar, keyWrapper.Key, gqlclient); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "CircleCIContext",
		LocationURI:  contextID,
		LocationIDs:  []string{keyIDEnvVar, keyEnvVar}}

	return updated, nil

}

func updateCircleCIContext(contextID, envVarName, envVarValue string,
	gqlclient *api.GraphQLContextClient) (err error) {

	if err = gqlclient.DeleteEnvironmentVariable(contextID, envVarName); err != nil {
		return
	}
	return gqlclient.CreateEnvironmentVariable(contextID, envVarName, envVarValue)
}
