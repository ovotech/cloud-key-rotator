package rotate

import (
	keys "github.com/ovotech/cloud-key-client"
	"github.com/ovotech/cloud-key-rotator/pkg/config"
	"testing"
)

type KeyResults struct {
	created bool
	deleted bool
}

func mockCreateFn(res *KeyResults) func(key keys.Key) (keyID, newKey string, err error) {
	return func(key keys.Key) (keyID, newKey string, err error) {
		res.created = true
		return
	}
}

func mockDeleteFn(res *KeyResults) func(key keys.Key) (err error) {
	return func(key keys.Key) (err error) {
		res.deleted = true
		return
	}
}

func mockKeys(providers []keys.Provider, includeInactiveKeys bool) (keysArr []keys.Key, err error) {
	k := keys.Key{Account: "account1", ID: "1234", Age: keyAge, Provider: keys.Provider{Provider: "provider1"}}
	keysArr = append(keysArr, k)
	return
}

const keyAge = 1000
const shortRotationPeriod = keyAge - 100
const longRotationPeriod = keyAge + 100

func TestMetricsOnly(t *testing.T) {

	var res KeyResults

	createKeyFn = mockCreateFn(&res)
	delKeyFn = mockDeleteFn(&res)
	keysFn = mockKeys

	err := Rotate("account1", "mockProvider", "project1", config.Config{RotationMode: false})

	if err != nil {
		t.Error(err)
	}

	if res.created || res.deleted {
		t.Error("Key should not have been created or deleted, as not in rotation mode")
	}
}

func TestRotateWithinThreshold(t *testing.T) {

	var res KeyResults

	createKeyFn = mockCreateFn(&res)
	delKeyFn = mockDeleteFn(&res)
	keysFn = mockKeys

	var locations config.KeyLocations = config.KeyLocations{RotationAgeThresholdMins: longRotationPeriod, ServiceAccountName: "account1"}
	err := Rotate("account1", "mockProvider", "project1", config.Config{RotationMode: true,
		AccountKeyLocations: []config.KeyLocations{locations}})

	if err != nil {
		t.Error(err)
	}

	if res.created || res.deleted {
		t.Error("Key should not have been created or deleted, as age of within threshold")
	}
}

func TestRotateOutsideThreshold(t *testing.T) {

	var res KeyResults

	createKeyFn = mockCreateFn(&res)
	delKeyFn = mockDeleteFn(&res)
	keysFn = mockKeys

	var locations config.KeyLocations = config.KeyLocations{RotationAgeThresholdMins: shortRotationPeriod, ServiceAccountName: "account1"}
	err := Rotate("account1", "mockProvider", "project1", config.Config{RotationMode: true,
		AccountKeyLocations: []config.KeyLocations{locations}})

	if err != nil {
		t.Error(err)
	}

	if !res.created || !res.deleted {
		t.Error("Key should have been created and deleted, as age outside threshold")
	}
}