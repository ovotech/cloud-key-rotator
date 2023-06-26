package location

import (
	"context"
	crypto_rand "crypto/rand"
	"encoding/base64"
	"fmt"
	"strconv"

	"github.com/ovotech/cloud-key-rotator/pkg/cred"

	"github.com/google/go-github/v45/github"
	"golang.org/x/crypto/nacl/box"
	"golang.org/x/oauth2"
)

// GitHubActionsService type
type GitHubActionsService interface {
	GetRepoPublicKey(context.Context, string, string) (*github.PublicKey, *github.Response, error)
	CreateOrUpdateEnvSecret(context.Context, int, string, *github.EncryptedSecret) (*github.Response, error)
	CreateOrUpdateRepoSecret(context.Context, string, string, *github.EncryptedSecret) (*github.Response, error)
}

// GitHub type
type GitHub struct {
	Base64Decode bool
	Env          string
	KeyIDEnvVar  string
	KeyEnvVar    string
	Owner        string
	Repo         string
}

func (github GitHub) Write(serviceAccountName string, keyWrapper KeyWrapper, creds cred.Credentials) (updated UpdatedLocation, err error) {

	logger.Infof("Starting GitHub env var updates, owner: %s, repo: %s", github.Owner, github.Repo)
	ctx, client, err := githubAuth(creds.GitHubAPIToken)
	provider := keyWrapper.KeyProvider
	key := keyWrapper.Key
	// if configured, base64 decode the key (GCP return encoded keys)
	if github.Base64Decode {
		var keyb []byte
		keyb, err = base64.StdEncoding.DecodeString(key)
		if err != nil {
			return
		}
		key = string(keyb)
	}

	var keyEnvVar string
	var idValue bool
	if keyEnvVar, err = getVarNameFromProvider(provider, github.KeyEnvVar, idValue); err != nil {
		return
	}

	var keyIDEnvVar string
	idValue = true
	if keyIDEnvVar, err = getVarNameFromProvider(provider, github.KeyIDEnvVar, idValue); err != nil {
		return
	}

	// create the ActionsService from the client so we can pass into addRepoSecret()
	actionsService := client.Actions

	if len(keyIDEnvVar) > 0 {
		if err = addEnvOrRepoSecret(ctx, actionsService, github.Owner, github.Repo, github.Env, keyIDEnvVar, keyWrapper.KeyID); err != nil {
			return
		}
	}

	if err = addEnvOrRepoSecret(ctx, actionsService, github.Owner, github.Repo, github.Env, keyEnvVar, key); err != nil {
		return
	}

	updated = UpdatedLocation{
		LocationType: "GitHub",
		LocationURI:  fmt.Sprintf("%s/%s", github.Owner, github.Repo),
		LocationIDs:  []string{keyIDEnvVar, keyEnvVar}}

	return updated, nil
}

// githubAuth returns a GitHub client and context.
func githubAuth(token string) (context.Context, *github.Client, error) {
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	return ctx, client, nil
}

// addRepoSecret will add a secret to a GitHub repo for use in GitHub Actions.
//
// Finally, the secretName and secretValue will determine the name of the secret added and it's corresponding value.
//
// The actual transmission of the secret value to GitHub using the api requires that the secret value is encrypted
// using the public key of the target repo. This encryption is done using x/crypto/nacl/box.
//
// First, the public key of the repo is retrieved. The public key comes base64
// encoded, so it must be decoded prior to use.
//
// Second, the decode key is converted into a fixed size byte array.
//
// Third, the secret value is converted into a slice of bytes.
//
// Fourth, the secret is encrypted with box.SealAnonymous using the repo's decoded public key.
//
// Fifth, the encrypted secret is encoded as a base64 string to be used in a github.EncodedSecret type.
//
// Sixt, The other two properties of the github.EncodedSecret type are determined. The name of the secret to be added
// (string not base64), and the KeyID of the public key used to encrypt the secret.
// This can be retrieved via the public key's GetKeyID method.
//
// Finally, the github.EncodedSecret is passed into the GitHub client.Actions.CreateOrUpdateRepoSecret method to
// populate the secret in the GitHub repo.
func addEnvOrRepoSecret(ctx context.Context, actionsService GitHubActionsService, owner, repo, env, secretName, secretValue string) error {
	publicKey, _, err := actionsService.GetRepoPublicKey(ctx, owner, repo)
	if err != nil {
		return err
	}

	encryptedSecret, err := encryptSecretWithPublicKey(publicKey, secretName, secretValue)
	if err != nil {
		return err
	}

	if env != "" {
		repoID, err := strconv.Atoi(repo)
		if err != nil {
			return fmt.Errorf("Error parsing repo string: %s to int: %w", repo, err)
		}
		_, err = actionsService.CreateOrUpdateEnvSecret(ctx, repoID, env, encryptedSecret)
		if err != nil {
			return fmt.Errorf("Actions.CreateOrUpdateEnvSecret returned error: %v", err)
		}
	} else {
		_, err = actionsService.CreateOrUpdateRepoSecret(ctx, owner, repo, encryptedSecret)
		if err != nil {
			return fmt.Errorf("Actions.CreateOrUpdateRepoSecret returned error: %v", err)
		}
	}

	logger.Infof("Added GitHub secret: %s to %s/%s", secretName, owner, repo)

	return nil
}

func encryptSecretWithPublicKey(publicKey *github.PublicKey, secretName string, secretValue string) (*github.EncryptedSecret, error) {
	decodedPublicKey, err := base64.StdEncoding.DecodeString(publicKey.GetKey())
	if err != nil {
		return nil, fmt.Errorf("base64.StdEncoding.DecodeString was unable to decode public key: %v", err)
	}

	var boxKey [32]byte
	copy(boxKey[:], decodedPublicKey)
	secretBytes := []byte(secretValue)
	encryptedBytes, err := box.SealAnonymous([]byte{}, secretBytes, &boxKey, crypto_rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("box.SealAnonymous failed with error %w", err)
	}

	encryptedString := base64.StdEncoding.EncodeToString(encryptedBytes)
	keyID := publicKey.GetKeyID()
	encryptedSecret := &github.EncryptedSecret{
		Name:           secretName,
		KeyID:          keyID,
		EncryptedValue: encryptedString,
	}
	return encryptedSecret, nil
}
