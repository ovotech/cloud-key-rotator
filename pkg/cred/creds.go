package cred

// Credentials type
type Credentials struct {
	CircleCIAPIToken string
	GitHubAccount    GitHubAccount
	AkrPass          string
	KmsKey           string
}

// GitHubAccount type
type GitHubAccount struct {
	GitHubAccessToken string
	GitName           string
	GitEmail          string
}
