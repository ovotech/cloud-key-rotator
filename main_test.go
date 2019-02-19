package main

import (
	"errors"
	"testing"

	keys "github.com/eversc/cloud-key-client"
)

var includeFilterTests = []struct {
	provider      string
	project       string
	saAccount     string
	keyAccount    string
	filteredCount int
}{
	{"gcp", "test-project", "test-sa", "should-not-be-returned", 0},
	{"gcp", "test-project", "test-sa", "test-sa", 1},
	{"aws", "test-project", "test-sa", "test.sa", 0},
	{"aws", "test-project", "test-sa", "test-sa", 1},
}

func TestFilterKeysInclude(t *testing.T) {
	for _, filterTest := range includeFilterTests {
		sa := serviceAccount{
			Provider: cloudProvider{
				Name:    filterTest.provider,
				Project: filterTest.project},
			Account: filterTest.saAccount}
		appConfig := config{IncludeSAs: []serviceAccount{sa}}
		key := keys.Key{
			Account: filterTest.keyAccount,
			Provider: keys.Provider{
				Provider: filterTest.provider, GcpProject: filterTest.project}}
		keys := []keys.Key{key}
		actual := len(filterKeys(keys, appConfig))
		expected := filterTest.filteredCount
		if actual != expected {
			t.Errorf("Incorrect number of keys after filtering, want: %d, got: %d",
				expected, actual)
		}
	}
}

func TestFilterKeysNoIncludeOrExclude(t *testing.T) {
	appConfig := config{}
	key := keys.Key{
		Account: "test-sa",
		Provider: keys.Provider{
			Provider: "gcp", GcpProject: "test-project"}}
	keys := []keys.Key{key}
	actual := len(filterKeys(keys, appConfig))
	expected := 0
	if actual != expected {
		t.Errorf("Incorrect number of keys after filtering, want: %d, got: %d",
			expected, actual)
	}
}

var excludeFilterTests = []struct {
	provider      string
	project       string
	saAccount     string
	keyAccount    string
	filteredCount int
}{
	{"gcp", "test-project", "test-sa", "should-be-returned", 1},
	{"gcp", "test-project", "test-sa", "test-sa", 0},
}

func TestFilterKeysExclude(t *testing.T) {
	for _, filterTest := range excludeFilterTests {
		sa := serviceAccount{
			Provider: cloudProvider{
				Name:    filterTest.provider,
				Project: filterTest.project},
			Account: filterTest.saAccount}
		appConfig := config{ExcludeSAs: []serviceAccount{sa}}
		key := keys.Key{
			Account: filterTest.keyAccount,
			Provider: keys.Provider{
				Provider: filterTest.provider, GcpProject: filterTest.project}}
		keys := []keys.Key{key}
		actual := len(filterKeys(keys, appConfig))
		expected := filterTest.filteredCount
		if actual != expected {
			t.Errorf("Incorrect number of keys after filtering, want: %d, got: %d",
				expected, actual)
		}
	}
}

func TestCheck(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()
	check(errors.New("this should cause panic"))
}
