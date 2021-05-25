package location

import (
	"errors"
	"testing"

	circleci "github.com/jszwedko/go-circleci"
)

// Mock functions

type circleCICallListSuccess struct {
	client   *circleci.Client
	project  string
	username string
}
type circleCICallDeleteSuccess struct {
	client     *circleci.Client
	envVarName string
	project    string
	username   string
}
type circleCICallAddSuccess struct {
	client      *circleci.Client
	envVarName  string
	envVarValue string
	project     string
	username    string
}
type circleCICallListError struct {
	client   *circleci.Client
	project  string
	username string
}
type circleCICallDeleteError struct {
	client     *circleci.Client
	envVarName string
	project    string
	username   string
}
type circleCICallAddError struct {
	client      *circleci.Client
	envVarName  string
	envVarValue string
	project     string
	username    string
}

func (c circleCICallListSuccess) list() ([]circleci.EnvVar, error) {
	envVar := circleci.EnvVar{Name: "foo", Value: "bar"}
	return []circleci.EnvVar{envVar}, nil
}

func (c circleCICallListSuccess) getUsername() string {
	return c.username
}

func (c circleCICallListSuccess) getProject() string {
	return c.project
}

func (c circleCICallDeleteSuccess) delete() error {
	return nil
}

func (c circleCICallDeleteSuccess) getUsername() string {
	return c.username
}

func (c circleCICallDeleteSuccess) getProject() string {
	return c.project
}

func (c circleCICallDeleteSuccess) getEnvVarName() string {
	return c.envVarName
}

func (c circleCICallAddSuccess) add() (*circleci.EnvVar, error) {
	envVar := circleci.EnvVar{Name: "foo", Value: "bar"}
	return &envVar, nil
}

func (c circleCICallAddSuccess) getUsername() string {
	return c.username
}

func (c circleCICallAddSuccess) getProject() string {
	return c.project
}

func (c circleCICallAddSuccess) getEnvVarName() string {
	return c.envVarName
}

func (c circleCICallListError) list() ([]circleci.EnvVar, error) {
	return nil, errors.New("Mock listEnvVars error from CircleCI")
}

func (c circleCICallListError) getUsername() string {
	return c.username
}

func (c circleCICallListError) getProject() string {
	return c.project
}

func (c circleCICallDeleteError) delete() error {
	return errors.New("Mock deleteEnvVar error from CircleCI")
}

func (c circleCICallDeleteError) getUsername() string {
	return c.username
}

func (c circleCICallDeleteError) getProject() string {
	return c.project
}

func (c circleCICallDeleteError) getEnvVarName() string {
	return c.envVarName
}

func (c circleCICallAddError) add() (*circleci.EnvVar, error) {
	return nil, errors.New("Mock addEnvVar error from CircleCI")
}

func (c circleCICallAddError) getUsername() string {
	return c.username
}

func (c circleCICallAddError) getProject() string {
	return c.project
}

func (c circleCICallAddError) getEnvVarName() string {
	return c.envVarName
}

// Test functions

func TestVerifyEnvVarsSuccess(t *testing.T) {
	verifyCircleCiEnvVar(circleCICallListSuccess{}, "foo")
}

func TestVerifyEnvVarsFail(t *testing.T) {
	err := verifyCircleCiEnvVar(circleCICallListError{}, "")
	if err == nil {
		t.Error("Expected error, got nil")
	}
}

func TestVerifyEnvVarsNotFound(t *testing.T) {
	// verifying an env var of name "bar" should error, as our mock will only
	// return a "foo" env var
	err := verifyCircleCiEnvVar(circleCICallListSuccess{}, "bar")
	if err == nil {
		t.Error("Expected error after env var not found, got nil")
	}
}

func TestUpdateEnvVarSuccess(t *testing.T) {
	updateCircleCIEnvVar(circleCICallListSuccess{}, circleCICallDeleteSuccess{}, circleCICallAddSuccess{})
}

func TestUpdateEnvVarsListFail(t *testing.T) {
	err := updateCircleCIEnvVar(circleCICallListError{}, circleCICallDeleteSuccess{}, circleCICallAddSuccess{})
	if err == nil {
		t.Error("Expected error from listEnvVars, got nil")
	}
}

func TestUpdateEnvVarsDeleteFail(t *testing.T) {
	err := updateCircleCIEnvVar(circleCICallListSuccess{}, circleCICallDeleteError{}, circleCICallAddSuccess{})
	if err == nil {
		t.Error("Expected error from deleteEnvVar, got nil")
	}
}

func TestUpdateEnvVarsAddFail(t *testing.T) {
	err := updateCircleCIEnvVar(circleCICallListSuccess{}, circleCICallDeleteSuccess{}, circleCICallAddError{})
	if err == nil {
		t.Error("Expected error from addEnvVar, got nil")
	}
}
