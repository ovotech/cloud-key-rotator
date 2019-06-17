package rotate_test

import (
	"testing"
	"github.com/ovotech/cloud-key-rotator/pkg/rotate"
	"github.com/ovotech/cloud-key-rotator/pkg/config"
	keys "github.com/ovotech/cloud-key-client"
)

type MockProvider struct {
}

func (m MockProvider) Keys(project string, includeInactiveKeys bool) (keys []keys.Key, err error) {
	return
}

func (m MockProvider) CreateKey(project, account string) (keyID, newKey string, err error) {
	return
}

func (m MockProvider) DeleteKey(project, account, keyID string) (err error) {
	return
}

func TestRotate(t *testing.T) {

	var m MockProvider
	keys.RegisterProvider("mockProvider", m)

	err := rotate.Rotate("account1", "mockProvider", "project1", config.Config{})

	if err != nil {
		t.Error(err)
	}

	// TODO check outcomes on mocked provider
}
