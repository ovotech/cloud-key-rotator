package location

//keyWriter defines the function signature for writing key to a location, e.g. CircleCI, K8S cluster or GitHub.
import "github.com/ovotech/cloud-key-rotator/pkg/cred"

type KeyWriter interface {
	Write(serviceAccountName, keyID, key string, creds cred.Credentials) (UpdatedLocation, error)
}
