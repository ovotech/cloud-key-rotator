package cred

type Credentials struct {
	CircleCIAPIToken string
	GitHubAccount    GitHubAccount
	AkrPass          string
	KmsKey           string
}

// gitHubAccount type
type GitHubAccount struct {
	GitHubAccessToken string
	GitName           string
	GitEmail          string
}
