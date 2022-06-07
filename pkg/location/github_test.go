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

func (m mockGitHubActionsService) CreateOrUpdateRepoSecret(context.Context, string, string, *github.EncryptedSecret) (*github.Response, error) {
	return m.updateSecretResponse.response, m.updateSecretResponse.error
}

func TestAddRepoSecretSuccess(t *testing.T) {
	actionsService := mockGitHubActionsService{
		// getPublicKeyResponse: struct {
		// 	publicKey *github.PublicKey
		// 	response  *github.Response
		// 	error     error
		// }{},
	}
	err := addRepoSecret(nil, actionsService, "", "", "", "")
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
	err := addRepoSecret(nil, actionsService, "", "", "", "")
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
	err := addRepoSecret(nil, actionsService, "", "", "", "")
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
