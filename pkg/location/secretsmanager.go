package location

import (
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	awsSm "github.com/aws/aws-sdk-go/service/secretsmanager"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
)

// SecretsManager type
type SecretsManager struct {
	KeyParamName   string
	KeyIDParamName string
	Region         string
	ConvertToFile  bool
	FileType       string
}

func (sm SecretsManager) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	provider := keyWrapper.KeyProvider
	var key string
	var keyEnvVar string
	var keyIDEnvVar string
	var idValue bool

	if keyEnvVar, err = getVarNameFromProvider(provider, sm.KeyParamName, idValue); err != nil {
		return
	}

	if sm.ConvertToFile || provider == "gcp" {
		if key, err = getKeyForFileBasedLocation(keyWrapper, sm.FileType); err != nil {
			return
		}
	} else {
		key = keyWrapper.Key
		idValue = true
		if keyIDEnvVar, err = getVarNameFromProvider(provider, sm.KeyIDParamName, idValue); err != nil {
			return
		}
	}
	svc := awsSm.New(
		session.New(),
		aws.NewConfig().
			WithRegion(sm.Region).
			WithEndpoint(fmt.Sprintf("secretsmanager.%s.amazonaws.com", sm.Region)),
	)

	if len(keyIDEnvVar) > 0 {
		if err = updateSecretsManagerSecret(keyIDEnvVar, keyWrapper.KeyID, *svc); err != nil {
			return
		}
	}
	if err = updateSecretsManagerSecret(keyEnvVar, key, *svc); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "secretsmanager",
		LocationURI:  sm.Region,
		LocationIDs:  []string{keyIDEnvVar, keyEnvVar}}
	return
}

func updateSecretsManagerSecret(paramName, paramValue string, svc awsSm.SecretsManager) (err error) {
	input := &awsSm.PutSecretValueInput{SecretId: &paramName, SecretString: &paramValue}
	_, err = svc.PutSecretValue(input)
	return
}
