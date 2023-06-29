package location

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-github/v45/github"
)

// Mock functions
type mockGitHubActionsService struct {
	getPublicKeyResponse struct {
		publicKey *github.PublicKey
		response  *github.Response
		error     error
	}
	updateSecretResponse struct {
		response *github.Response
		error    error
	}
}

func (m mockGitHubActionsService) GetRepoPublicKey(context.Context, string, string) (*github.PublicKey, *github.Response, error) {
	return m.getPublicKeyResponse.publicKey, m.getPublicKeyResponse.response, m.getPublicKeyResponse.error
}

func (m mockGitHubActionsService) CreateOrUpdateEnvSecret(context.Context, int, string, *github.EncryptedSecret) (*github.Response, error) {
	return m.updateSecretResponse.response, m.updateSecretResponse.error
}

func (m mockGitHubActionsService) CreateOrUpdateRepoSecret(context.Context, string, string, *github.EncryptedSecret) (*github.Response, error) {
	return m.updateSecretResponse.response, m.updateSecretResponse.error
}

func TestAddEnvSecretSuccess(t *testing.T) {
	actionsService := mockGitHubActionsService{}
	err := addEnvOrRepoSecret(nil, actionsService, "", "", "my_env", "", "", 1234)
	if err != nil {
		t.Error("Expected nil, got error")
	}
}

func TestAddEnvSecretPublicKeyFail(t *testing.T) {
	actionsService := mockGitHubActionsService{
		getPublicKeyResponse: struct {
			publicKey *github.PublicKey
			response  *github.Response
			error     error
		}{error: errors.New("error getting repo public key")},
	}
	err := addEnvOrRepoSecret(nil, actionsService, "", "", "my_env", "", "", 1234)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestAddEnvSecretAddSecretFail(t *testing.T) {
	actionsService := mockGitHubActionsService{
		updateSecretResponse: struct {
			response *github.Response
			error    error
		}{error: errors.New("error adding secret")},
	}
	err := addEnvOrRepoSecret(nil, actionsService, "", "", "my_env", "", "", 1234)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestAddEnvSecretParseRepoIDFail(t *testing.T) {
	actionsService := mockGitHubActionsService{
		updateSecretResponse: struct {
			response *github.Response
			error    error
		}{error: errors.New("error adding secret")},
	}
	err := addEnvOrRepoSecret(nil, actionsService, "", "", "my_env", "", "", 1234)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestAddRepoSecretSuccess(t *testing.T) {
	actionsService := mockGitHubActionsService{}
	err := addEnvOrRepoSecret(nil, actionsService, "", "", "", "", "", 0)
	if err != nil {
		t.Error("Expected nil, got error")
	}
}

func TestAddRepoSecretPublicKeyFail(t *testing.T) {
	actionsService := mockGitHubActionsService{
		getPublicKeyResponse: struct {
			publicKey *github.PublicKey
			response  *github.Response
			error     error
		}{error: errors.New("error getting repo public key")},
	}
	err := addEnvOrRepoSecret(nil, actionsService, "", "", "", "", "", 0)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestAddRepoSecretAddSecretFail(t *testing.T) {
	actionsService := mockGitHubActionsService{
		updateSecretResponse: struct {
			response *github.Response
			error    error
		}{error: errors.New("error adding secret")},
	}
	err := addEnvOrRepoSecret(nil, actionsService, "", "", "", "", "", 0)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

// we can only check whether the error is not nil, the returned encrypted
// value will be different each time so we can't check it's value
func TestEncryptSecretWithPublicKeySuccess(t *testing.T) {
	tmp := ""
	_, err := encryptSecretWithPublicKey(&github.PublicKey{Key: &tmp, KeyID: &tmp}, "", "")
	if err != nil {
		t.Error("Expected nil, got error")
	}
}

// should fail as @ is invalid for base64 decoding
func TestEncryptSecretWithPublicKeyB64Fail(t *testing.T) {
	tmpKey := "@"
	tmpKeyID := ""
	_, err := encryptSecretWithPublicKey(&github.PublicKey{Key: &tmpKey, KeyID: &tmpKeyID}, "", "")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}
