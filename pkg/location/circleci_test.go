package location

import (
	"errors"
	"testing"

	circleci "github.com/jszwedko/go-circleci"
)

// Mock functions

func mockListEnvVars(username, project string, client *circleci.Client) ([]circleci.EnvVar, error) {
	envVar := circleci.EnvVar{Name: "foo", Value: "bar"}
	return []circleci.EnvVar{envVar}, nil
}

func mockListEnvVarsError(username, project string, client *circleci.Client) ([]circleci.EnvVar, error) {
	return nil, errors.New("Mock listEnvVars error from CircleCI")
}

func mockDeleteEnvVar(username, project, envVarName string, client *circleci.Client) error {
	return nil
}

func mockDeleteEnvVarError(username, project, envVarName string, client *circleci.Client) error {
	return errors.New("Mock deleteEnvVar error from CircleCI")
}

func mockAddEnvVar(username, project, envVarName, envVarValue string, client *circleci.Client) (*circleci.EnvVar, error) {
	envVar := circleci.EnvVar{Name: "foo", Value: "bar"}
	return &envVar, nil
}

func mockAddEnvVarError(username, project, envVarName, envVarValue string, client *circleci.Client) (*circleci.EnvVar, error) {
	return nil, errors.New("Mock addEnvVar error from CircleCI")
}

// Test functions

func TestVerifyEnvVarsSuccess(t *testing.T) {
	verifyCircleCiEnvVar("", "", "foo", nil, mockListEnvVars)
}

func TestVerifyEnvVarsFail(t *testing.T) {
	err := verifyCircleCiEnvVar("", "", "", nil, mockListEnvVarsError)
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestVerifyEnvVarsNotFound(t *testing.T) {
	// verifying an env var of name "bar" should error, as our mock will only
	// return a "foo" env var
	err := verifyCircleCiEnvVar("", "", "bar", nil, mockListEnvVars)
	if err == nil {
		t.Error("Expected error after env var not found, got nil")
	}
}

func TestUpdateEnvVarSuccess(t *testing.T) {
	updateCircleCIEnvVar("", "", "foo", "", nil, mockListEnvVars, mockDeleteEnvVar, mockAddEnvVar)
}

func TestUpdateEnvVarNotFound(t *testing.T) {
	err := updateCircleCIEnvVar("", "", "bar", "", nil, mockListEnvVars, mockDeleteEnvVar, mockAddEnvVar)
	if err == nil {
		t.Error("Expected error after env var not found, got nil")
	}
}

func TestUpdateEnvVarsListFail(t *testing.T) {
	err := updateCircleCIEnvVar("", "", "", "", nil, mockListEnvVarsError, mockDeleteEnvVar, mockAddEnvVar)
	if err == nil {
		t.Error("Expected error from listEnvVars, got nil")
	}
}

func TestUpdateEnvVarsDeleteFail(t *testing.T) {
	err := updateCircleCIEnvVar("", "", "", "", nil, mockListEnvVars, mockDeleteEnvVarError, mockAddEnvVar)
	if err == nil {
		t.Error("Expected error from deleteEnvVar, got nil")
	}
}

func TestUpdateEnvVarsAddFail(t *testing.T) {
	err := updateCircleCIEnvVar("", "", "", "", nil, mockListEnvVars, mockDeleteEnvVar, mockAddEnvVarError)
	if err == nil {
		t.Error("Expected error from addEnvVar, got nil")
	}
}
