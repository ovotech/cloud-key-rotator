package main

import (
	"errors"
	"fmt"
	"testing"

	keys "github.com/eversc/cloud-key-client"
)

// var includeFilterTests = []struct {
// 	provider      string
// 	project       string
// 	saAccounts    []string
// 	keyAccount    string
// 	filteredCount int
// }{
// 	{"gcp", "test-project", []string{"test-sa"}, "should-not-be-returned", 0},
// 	{"gcp", "test-project", []string{"test-sa"}, "test-sa", 1},
// 	{"aws", "test-project", []string{"test-sa"}, "test.sa", 0}, //filtered out due to "." in keyAccount
// 	{"aws", "test-project", []string{"test-sa"}, "test-sa", 1},
// }
//
// func TestFilterKeysInclude(t *testing.T) {
// 	for _, filterTest := range includeFilterTests {
// 		psa := providerServiceAccounts{
// 			Provider: cloudProvider{Name: filterTest.provider,
// 				Project: filterTest.project},
// 			Accounts: filterTest.saAccounts,
// 		}
// 		appConfig := config{IncludeSAs: []providerServiceAccounts{psa}}
// 		key := keys.Key{
// 			Account: filterTest.keyAccount,
// 			Provider: keys.Provider{
// 				Provider: filterTest.provider, GcpProject: filterTest.project}}
// 		keys := []keys.Key{key}
// 		expected := filterTest.filteredCount
// 		actual := len(filterKeys(keys, appConfig))
// 		if actual != expected {
// 			t.Errorf("Incorrect number of keys after filtering, want: %d, got: %d",
// 				expected, actual)
// 		}
// 	}
// }
//
// var noFilterTests = []struct {
// 	provider      string
// 	project       string
// 	keyAccount    string
// 	rotationMode  bool
// 	filteredCount int
// }{
// 	{"gcp", "test-project", "test-sa", true, 0},
// 	{"gcp", "test-project", "test-sa", false, 1},
// }
//
// func TestFilterKeysNoIncludeOrExclude(t *testing.T) {
// 	for _, noFilterTest := range noFilterTests {
// 		appConfig := config{RotationMode: noFilterTest.rotationMode}
// 		key := keys.Key{
// 			Account: noFilterTest.keyAccount,
// 			Provider: keys.Provider{
// 				Provider: noFilterTest.provider, GcpProject: noFilterTest.project}}
// 		keys := []keys.Key{key}
// 		expected := noFilterTest.filteredCount
// 		actual := len(filterKeys(keys, appConfig))
// 		if actual != expected {
// 			t.Errorf("Incorrect number of keys after filtering, want: %d, got: %d",
// 				expected, actual)
// 		}
// 	}
// }

var excludeFilterTests = []struct {
	provider      string
	project       string
	saAccounts    []string
	keyAccount    string
	filteredCount int
}{
	{"gcp", "test-project", []string{"test-sa"}, "should-be-returned", 1},
	{"gcp", "test-project", []string{"test-sa"}, "test-sa", 0},
}

func TestFilterKeysExclude(t *testing.T) {
	for _, filterTest := range excludeFilterTests {
		psa := providerServiceAccounts{
			Provider: cloudProvider{Name: filterTest.provider,
				Project: filterTest.project},
			Accounts: filterTest.saAccounts,
		}
		appConfig := config{ExcludeSAs: []providerServiceAccounts{psa}}
		key := keys.Key{
			Account: filterTest.keyAccount,
			Provider: keys.Provider{
				Provider: filterTest.provider, GcpProject: filterTest.project}}
		keys := []keys.Key{key}
		expected := filterTest.filteredCount
		actual := len(filterKeys(keys, appConfig))
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

var validAwsKeyTests = []struct {
	includeUserKeys bool
	keyName         string
	valid           bool
}{
	{true, "first.last", true},
	{true, "firstlast", true},
	{false, "first.last", false},
	{false, "firstlast", true},
}

func TestValidAwsKey(t *testing.T) {
	for _, validAwsKeyTest := range validAwsKeyTests {
		appConfig := config{IncludeAwsUserKeys: validAwsKeyTest.includeUserKeys}
		key := keys.Key{Name: validAwsKeyTest.keyName}
		expected := validAwsKeyTest.valid
		actual := validAwsKey(key, appConfig)
		if actual != expected {
			t.Errorf("Incorrect bool returned, want: %t, got: %t", expected, actual)
		}
	}
}

var sliceContainsTests = []struct {
	slice        []string
	searchString string
	contains     bool
}{
	{[]string{"test"}, "test", true},
	{[]string{"test"}, "Test", false},
	{[]string{"testing"}, "test", false},
	{[]string{"test", "testing"}, "test", true},
	{[]string{"test", "testing"}, "Test", false},
}

func TestContains(t *testing.T) {
	for _, sliceContainsTest := range sliceContainsTests {
		expected := sliceContainsTest.contains
		actual := contains(sliceContainsTest.slice, sliceContainsTest.searchString)
		if actual != expected {
			t.Errorf("Incorrect bool returned, want: %t, got: %t", expected, actual)
		}
		fmt.Println()
	}
}
