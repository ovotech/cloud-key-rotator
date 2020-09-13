// Copyright 2019 OVO Technology
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package rotate

import (
	"testing"

	keys "github.com/ovotech/cloud-key-client"
	"github.com/ovotech/cloud-key-rotator/pkg/config"
)

var includeFilterTests = []struct {
	provider             string
	project              string
	saAccounts           []string
	keyAccount           string
	userSpecifiedAccount string
	filteredCount        int
}{
	{"gcp", "test-project", []string{"test-sa"}, "should-not-be-returned", "", 0},
	{"gcp", "test-project", []string{"test-sa"}, "test-sa", "", 1},
	{"aws", "test-project", []string{"test-sa"}, "test.sa", "", 0}, //filtered out due to "." in keyAccount
	{"aws", "test-project", []string{"test-sa"}, "test-sa", "", 1},
	{"gcp", "test-project", []string{"test-sa"}, "test-sa", "test-sa", 1},
	{"gcp", "test-project", []string{"test-sa"}, "test-sa", "should-not-be-returned", 0},
}

func TestFilterKeysInclude(t *testing.T) {
	for _, filterTest := range includeFilterTests {
		psa := config.ProviderServiceAccounts{
			Provider: config.CloudProvider{Name: filterTest.provider,
				Project: filterTest.project},
			ProviderAccounts: filterTest.saAccounts,
		}

		psas := []config.ProviderServiceAccounts{psa}
		includeFilter := config.Filter{Mode: "include", Accounts: psas}
		appConfig := config.Config{RotationMode: true, AccountFilter: includeFilter}

		key := keys.Key{
			Account: filterTest.keyAccount,
			Provider: keys.Provider{
				Provider: filterTest.provider, GcpProject: filterTest.project}}
		keys := []keys.Key{key}
		expected := filterTest.filteredCount
		filteredKeys, _ := filterKeys(keys, appConfig, filterTest.userSpecifiedAccount)
		actual := len(filteredKeys)
		if actual != expected {
			t.Errorf("Incorrect number of keys after filtering, want: %d, got: %d",
				expected, actual)
		}
	}
}

var noFilterTests = []struct {
	provider      string
	project       string
	keyAccount    string
	rotationMode  bool
	filteredCount int
}{
	{"gcp", "test-project", "test-sa", true, 0},
	{"gcp", "test-project", "test-sa", false, 1},
}

func TestFilterKeysNoIncludeOrExclude(t *testing.T) {
	for _, noFilterTest := range noFilterTests {
		appConfig := config.Config{RotationMode: noFilterTest.rotationMode}
		key := keys.Key{
			Account: noFilterTest.keyAccount,
			Provider: keys.Provider{
				Provider: noFilterTest.provider, GcpProject: noFilterTest.project}}
		keys := []keys.Key{key}
		expected := noFilterTest.filteredCount
		filteredKeys, _ := filterKeys(keys, appConfig, "")
		actual := len(filteredKeys)
		if actual != expected {
			t.Errorf("Incorrect number of keys after filtering, want: %d, got: %d",
				expected, actual)
		}
	}
}

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
		psa := config.ProviderServiceAccounts{
			Provider: config.CloudProvider{Name: filterTest.provider,
				Project: filterTest.project},
			ProviderAccounts: filterTest.saAccounts,
		}
		psas := []config.ProviderServiceAccounts{psa}
		excludeFilter := config.Filter{Mode: "exclude", Accounts: psas}
		appConfig := config.Config{RotationMode: true, AccountFilter: excludeFilter}
		key := keys.Key{
			Account: filterTest.keyAccount,
			Provider: keys.Provider{
				Provider: filterTest.provider, GcpProject: filterTest.project}}
		keys := []keys.Key{key}
		expected := filterTest.filteredCount
		filteredKeys, _ := filterKeys(keys, appConfig, "")
		actual := len(filteredKeys)
		if actual != expected {
			t.Errorf("Incorrect number of keys after filtering, want: %d, got: %d",
				expected, actual)
		}
	}
}

var validKeyTests = []struct {
	provider        string
	project         string
	includeUserKeys bool
	keyName         string
	valid           bool
}{
	{"aws", "", true, "first.last", true},
	{"aws", "", true, "firstlast", true},
	{"aws", "", false, "first.last", false},
	{"aws", "", false, "firstlast", true},
}

func TestValidKey(t *testing.T) {
	for _, validKeyTest := range validKeyTests {
		appConfig := config.Config{IncludeAwsUserKeys: validKeyTest.includeUserKeys}
		key := keys.Key{Provider: keys.Provider{
			Provider: validKeyTest.provider, GcpProject: validKeyTest.project}, Name: validKeyTest.keyName}
		expected := validKeyTest.valid
		actual := validKey(key, appConfig)
		if actual != expected {
			t.Errorf("Incorrect bool returned, want: %t, got: %t", expected, actual)
		}
	}
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
		appConfig := config.Config{IncludeAwsUserKeys: validAwsKeyTest.includeUserKeys}
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
	}
}

var filterTests = []struct {
	provider             string
	project              string
	saAccounts           []string
	keyAccount           string
	keyExistsInFiltering bool
}{
	{"gcp", "test-project", []string{"test-sa"}, "doesnt-exist-in-filtering", false},
	{"gcp", "test-project", []string{"test-sa"}, "test-sa", true},
}

func TestKeyDefinedInFiltering(t *testing.T) {
	for _, filterTest := range filterTests {
		psa := config.ProviderServiceAccounts{
			Provider: config.CloudProvider{Name: filterTest.provider,
				Project: filterTest.project},
			ProviderAccounts: filterTest.saAccounts,
		}
		psas := []config.ProviderServiceAccounts{psa}
		key := keys.Key{
			Account: filterTest.keyAccount,
			Provider: keys.Provider{
				Provider: filterTest.provider, GcpProject: filterTest.project}}
		expected := filterTest.keyExistsInFiltering
		actual := keyDefinedInFiltering(psas, key)
		if actual != expected {
			t.Errorf("Incorrect bool returned, want: %t, got: %t", expected, actual)
		}
	}

}

// func TestNewInterfaceApproach(t *testing.T) {
// 	newInterfaceApproach()
// }

var flagValidationTests = []struct {
	account     string
	provider    string
	project     string
	shouldError bool
}{
	{"sa-account", "", "", true},
	{"sa-account", "gcp", "", true},
	{"sa-account", "gcp", "gcp-project", false},
	{"sa-account", "aws", "", false},
}

func TestValidateFlags(t *testing.T) {
	for _, flagValidationTest := range flagValidationTests {
		err := validateFlags(flagValidationTest.account, flagValidationTest.provider, flagValidationTest.project)
		actual := err != nil
		expected := flagValidationTest.shouldError
		if actual != expected {
			t.Errorf("Incorrect error behaviour encountered")
		}
	}
}

var obfuscationTests = []struct {
	keyID    string
	expected string
}{
	{"", ""},
	{"abcd", "abcd"},
	{"abcd12345", "*****2345"},
}

func TestObfuscate(t *testing.T) {
	for _, obfuscationTest := range obfuscationTests {
		actual := obfuscate(obfuscationTest.keyID)
		if actual != obfuscationTest.expected {
			t.Errorf("Incorrect obfuscation result, want %s, got %s", obfuscationTest.expected, actual)
		}
	}
}
