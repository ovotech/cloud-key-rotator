package cmd

import (
	"testing"

	keys "github.com/ovotech/cloud-key-client"
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
		psa := providerServiceAccounts{
			Provider: cloudProvider{Name: filterTest.provider,
				Project: filterTest.project},
			ProviderAccounts: filterTest.saAccounts,
		}

		psas := []providerServiceAccounts{psa}
		includeFilter := filter{Mode: "include", Accounts: psas}
		appConfig := config{RotationMode: true, AccountFilter: includeFilter}

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
		appConfig := config{RotationMode: noFilterTest.rotationMode}
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
		psa := providerServiceAccounts{
			Provider: cloudProvider{Name: filterTest.provider,
				Project: filterTest.project},
			ProviderAccounts: filterTest.saAccounts,
		}
		psas := []providerServiceAccounts{psa}
		excludeFilter := filter{Mode: "exclude", Accounts: psas}
		appConfig := config{RotationMode: true, AccountFilter: excludeFilter}
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
		appConfig := config{IncludeAwsUserKeys: validKeyTest.includeUserKeys}
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
		psa := providerServiceAccounts{
			Provider: cloudProvider{Name: filterTest.provider,
				Project: filterTest.project},
			ProviderAccounts: filterTest.saAccounts,
		}
		psas := []providerServiceAccounts{psa}
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
