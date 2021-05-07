package location

import (
	"fmt"
	"testing"

	circleci "github.com/jszwedko/go-circleci"
)

func mockListEnvVarsSuccess(username, project string, client *circleci.Client) ([]circleci.EnvVar, error) {
	envVar := circleci.EnvVar{Name: "foo", Value: "bar"}
	return []circleci.EnvVar{envVar}, nil
}

func TestListEnvVarsSuccess(t *testing.T) {
	verifyCircleCiEnvVar("", "", "foo", nil, mockListEnvVarsSuccess)
}

func mockListEnvVarsFail(username, project string, client *circleci.Client) ([]circleci.EnvVar, error) {
	envVar := circleci.EnvVar{Name: "bar", Value: "bar"}
	return []circleci.EnvVar{envVar}, nil
}

func TestListEnvVarsFail(t *testing.T) {
	err := verifyCircleCiEnvVar("", "", "foo", nil, mockListEnvVarsFail)
	if err == nil {
		fmt.Println("err")
	}
}
