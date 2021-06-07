package location

import (
	"errors"
	"testing"

	circleci "github.com/jszwedko/go-circleci"
)

// Mock functions
type mockCircleCiClient struct {
	listEnvResponse struct {
		envVars []circleci.EnvVar
		error   error
	}
	deleteEnvResponse struct {
		error error
	}
	addEnvResponse struct {
		envVar *circleci.EnvVar
		error  error
	}
}

func (m mockCircleCiClient) ListEnvVars(account, repo string) ([]circleci.EnvVar, error) {
	return m.listEnvResponse.envVars, m.listEnvResponse.error
}

func (m mockCircleCiClient) DeleteEnvVar(account, repo, name string) error {
	return m.deleteEnvResponse.error
}

func (m mockCircleCiClient) AddEnvVar(account, repo, name, value string) (*circleci.EnvVar, error) {
	return m.addEnvResponse.envVar, m.addEnvResponse.error
}

// Test functions

func TestVerifyEnvVarsSuccess(t *testing.T) {
	client := mockCircleCiClient{listEnvResponse: struct {
		envVars []circleci.EnvVar
		error   error
	}{envVars: []circleci.EnvVar{{Name: "foo"}}, error: nil}}
	err := verifyCircleCiEnvVar("", "", "foo", client)
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}
}

func TestVerifyEnvVarsFail(t *testing.T) {
	client := mockCircleCiClient{listEnvResponse: struct {
		envVars []circleci.EnvVar
		error   error
	}{envVars: []circleci.EnvVar{}, error: nil}}
	err := verifyCircleCiEnvVar("", "", "", client)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestVerifyEnvVarsNotFound(t *testing.T) {
	client := mockCircleCiClient{listEnvResponse: struct {
		envVars []circleci.EnvVar
		error   error
	}{envVars: []circleci.EnvVar{{Name: "foo"}}, error: nil}}
	err := verifyCircleCiEnvVar("", "", "bar", client)
	if err == nil {
		t.Error("Expected error after env var not found, got nil")
	}
}

func TestUpdateEnvVarSuccess(t *testing.T) {
	client := mockCircleCiClient{
		listEnvResponse: struct {
			envVars []circleci.EnvVar
			error   error
		}{envVars: []circleci.EnvVar{{Name: "foo"}}, error: nil},
		deleteEnvResponse: struct{ error error }{error: nil},
		addEnvResponse: struct {
			envVar *circleci.EnvVar
			error  error
		}{envVar: &circleci.EnvVar{Name: "foo", Value: ""}, error: nil},
	}
	err := updateCircleCIEnvVar("", "", "foo", "", client)
	if err != nil {
		t.Errorf("Expected nil, got %s", err)
	}
}

func TestUpdateEnvVarNotFound(t *testing.T) {
	client := mockCircleCiClient{
		listEnvResponse: struct {
			envVars []circleci.EnvVar
			error   error
		}{envVars: []circleci.EnvVar{{Name: "foo"}}, error: nil},
	}
	err := updateCircleCIEnvVar("", "", "bar", "", client)
	if err == nil {
		t.Error("Expected error after env var not found, got nil")
	}
}

func TestUpdateEnvVarsListFail(t *testing.T) {
	client := mockCircleCiClient{
		listEnvResponse: struct {
			envVars []circleci.EnvVar
			error   error
		}{envVars: []circleci.EnvVar{}, error: errors.New("could not list env vars")},
	}
	err := updateCircleCIEnvVar("", "", "", "", client)
	if err == nil {
		t.Error("Expected error from listEnvVars, got nil")
	}
}

func TestUpdateEnvVarsDeleteFail(t *testing.T) {
	client := mockCircleCiClient{
		listEnvResponse: struct {
			envVars []circleci.EnvVar
			error   error
		}{envVars: []circleci.EnvVar{{Name: "foo"}}, error: nil},
		deleteEnvResponse: struct{ error error }{error: errors.New("could not delete env var")},
	}
	err := updateCircleCIEnvVar("", "", "foo", "", client)
	if err == nil {
		t.Error("Expected error from deleteEnvVar, got nil")
	}
}

func TestUpdateEnvVarsAddFail(t *testing.T) {
	client := mockCircleCiClient{
		listEnvResponse: struct {
			envVars []circleci.EnvVar
			error   error
		}{envVars: []circleci.EnvVar{{Name: "foo"}}, error: nil},
		deleteEnvResponse: struct{ error error }{error: nil},
		addEnvResponse: struct {
			envVar *circleci.EnvVar
			error  error
		}{envVar: nil, error: errors.New("could not add env var")},
	}
	err := updateCircleCIEnvVar("", "", "", "", client)
	if err == nil {
		t.Error("Expected error from addEnvVar, got nil")
	}
}
