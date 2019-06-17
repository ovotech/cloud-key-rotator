package rotate_test

import (
	"testing"
	"fmt"
	"github.com/ovotech/cloud-key-rotator/pkg/rotate"
	"github.com/ovotech/cloud-key-rotator/pkg/config"
	keys "github.com/ovotech/cloud-key-client"
)

type MockProvider struct {
}

func (m MockProvider) Keys(project string, includeInactiveKeys bool) (keysArr []keys.Key, err error) {
	k := keys.Key{Account: "account1", ID: "1234", Provider: keys.Provider{Provider: "provider1"}}
	keysArr = append(keysArr, k)

	return
}

func (m MockProvider) CreateKey(project, account string) (keyID, newKey string, err error) {
	fmt.Println("CreateKey...")
	return
}

func (m MockProvider) DeleteKey(project, account, keyID string) (err error) {
	fmt.Println("DeleteKey...")
	return
}

func TestRotate(t *testing.T) {
	
	var m MockProvider

	keys.RegisterProvider("mockProvider", m)

	err := rotate.Rotate("account1", "mockProvider", "project1", config.Config{RotationMode: /*true*/ false})

	if err != nil {
		t.Error(err)
	}

	// TODO check outcomes on mocked provider
}