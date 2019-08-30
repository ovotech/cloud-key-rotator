package rotate

import (
	"testing"

	keys "github.com/ovotech/cloud-key-client"
	"github.com/ovotech/cloud-key-rotator/pkg/config"
)

// MockProvider configuration, which keys library uses instead of AWS, GCP, etc. when accessing service account keys
type MockProvider struct {
	created bool
	deleted bool
}

func (m *MockProvider) Keys(project string, includeInactiveKeys bool) (keysArr []keys.Key, err error) {
	k := keys.Key{Account: "account1", ID: "1234", Age: keyAge, Provider: keys.Provider{Provider: "mockProvider"}}
	keysArr = append(keysArr, k)

	return
}

func (m *MockProvider) CreateKey(project, account string) (keyID, newKey string, err error) {
	m.created = true
	return
}

func (m *MockProvider) DeleteKey(project, account, keyID string) (err error) {
	m.deleted = true
	return
}

const keyAge = 1000
const shortRotationPeriod = keyAge - 100
const longRotationPeriod = keyAge + 100

func TestMetricsOnly(t *testing.T) {

	var m MockProvider
	keys.RegisterProvider("mockProvider", &m)

	var locations config.KeyLocations = config.KeyLocations{RotationAgeThresholdMins: shortRotationPeriod, ServiceAccountName: "account1"}
	err := Rotate("account1", "mockProvider", "project1", config.Config{RotationMode: false, EnableKeyAgeLogging: true,
		AccountKeyLocations: []config.KeyLocations{locations}})

	if err != nil {
		t.Error(err)
	}

	if m.created || m.deleted {
		t.Error("Key should not have been created or deleted, as not in rotation mode")
	}
}

func TestRotateWithinThreshold(t *testing.T) {

	var m MockProvider
	keys.RegisterProvider("mockProvider", &m)

	var locations config.KeyLocations = config.KeyLocations{RotationAgeThresholdMins: longRotationPeriod, ServiceAccountName: "account1"}
	err := Rotate("account1", "mockProvider", "project1", config.Config{RotationMode: true,
		AccountKeyLocations: []config.KeyLocations{locations}})

	if err != nil {
		t.Error(err)
	}

	if m.created || m.deleted {
		t.Error("Key should not have been created or deleted, as age of within threshold")
	}
}

func TestRotateOutsideThreshold(t *testing.T) {

	var m MockProvider
	keys.RegisterProvider("mockProvider", &m)

	var locations config.KeyLocations = config.KeyLocations{RotationAgeThresholdMins: shortRotationPeriod, ServiceAccountName: "account1"}
	err := Rotate("account1", "mockProvider", "project1", config.Config{RotationMode: true,
		AccountKeyLocations: []config.KeyLocations{locations}})

	if err != nil {
		t.Error(err)
	}

	if !m.created || !m.deleted {
		t.Error("Key should have been created and deleted, as age outside threshold")
	}
}
