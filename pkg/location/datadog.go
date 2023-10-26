package location

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"github.com/DataDog/datadog-api-client-go/api/v1/datadog"
	"github.com/ovotech/cloud-key-rotator/pkg/cred"
	"net/http"
)

// Datadog type
type Datadog struct {
	Project     string
	ClientEmail string
}

var (
	// ErrMissingDatadogCredentials is returned when the datadog authentication details are missing
	ErrMissingDatadogCredentials = errors.New("missing datadog credentials")
	// ErrInvalidDatadogCredentials is returned when the datadog authentication details are invalid
	ErrInvalidDatadogCredentials = errors.New("invalid datadog credentials")
	// ErrDatadogBadRequest is returned when the datadog API rejects the update request
	ErrDatadogBadRequest = errors.New("bad request")
	// ErrDatadogIntegrationNotFound is returned when the existing integration
	ErrDatadogIntegrationNotFound = errors.New("existing datadog integration not found")
	// ErrIncorrectGCPKeyProvider is returned when attempting to use this location with a non-GCP key
	ErrIncorrectGCPKeyProvider = errors.New("this location only supports GCP service account keys")
)

// Write
func (dd Datadog) Write(serviceAccountName string, wrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {
	logger.Infof("Starting Datadog GCP integration update for %s in project %s", dd.ClientEmail, dd.Project)

	if wrapper.KeyProvider != "gcp" {
		err = ErrIncorrectGCPKeyProvider
		return
	}

	var ctx context.Context
	if ctx, err = createDatadogContext(context.Background(), creds); err != nil {
		return
	}
	client := datadog.NewAPIClient(datadog.NewConfiguration())

	var integration datadog.GCPAccount
	if integration, err = dd.getDatadogGCPIntegration(ctx, client); err != nil {
		return
	}

	if integration, err = updateDatadogGCPAccount(integration, wrapper); err != nil {
		return
	}

	var r *http.Response
	if _, r, err = client.GCPIntegrationApi.UpdateGCPIntegration(ctx, integration); err != nil {
		return
	}

	switch r.StatusCode {
	case 403:
		err = ErrInvalidDatadogCredentials
	case 400:
		err = ErrDatadogBadRequest
	case 200:
		updated = UpdatedLocation{
			LocationType: "DatadogGCPIntegration",
			LocationURI:  *integration.ProjectId,
			LocationIDs:  []string{*integration.ClientEmail},
		}
	}
	return
}

func (dd Datadog) getDatadogGCPIntegration(ctx context.Context, client *datadog.APIClient) (datadog.GCPAccount, error) {
	accs, _, err := client.GCPIntegrationApi.ListGCPIntegration(ctx)
	if err != nil {
		return datadog.GCPAccount{}, err
	}

	for _, acc := range accs {
		if *acc.ClientEmail == dd.ClientEmail && *acc.ProjectId == dd.Project {
			return acc, nil
		}
	}

	return datadog.GCPAccount{}, ErrDatadogIntegrationNotFound
}

func createDatadogContext(ctx context.Context, creds cred.Credentials) (context.Context, error) {
	if creds.Datadog.APIKey == "" || creds.Datadog.AppKey == "" {
		return nil, ErrMissingDatadogCredentials
	}

	ctx = context.WithValue(ctx, datadog.ContextAPIKeys, map[string]datadog.APIKey{
		"apiKeyAuth": {Key: creds.Datadog.APIKey},
		"appKeyAuth": {Key: creds.Datadog.AppKey},
	})

	return ctx, nil
}

func updateDatadogGCPAccount(integration datadog.GCPAccount, wrapper KeyWrapper) (datadog.GCPAccount, error) {
	keyBytes, err := base64.StdEncoding.DecodeString(wrapper.Key)
	if err != nil {
		return integration, err
	}

	type gcpPrivateKeyData struct {
		PrivateKeyID string `json:"private_key_id"`
		PrivateKey   string `json:"private_key"`
	}
	key := &gcpPrivateKeyData{}
	if err = json.Unmarshal(keyBytes, key); err != nil {
		return integration, err
	}

	integration.PrivateKey = &key.PrivateKey
	integration.PrivateKeyId = &key.PrivateKeyID

	return integration, nil
}
