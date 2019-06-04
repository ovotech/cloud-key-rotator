package location

//UpdatedLocation type
type UpdatedLocation struct {
	LocationType string
	LocationURI  string
	LocationIDs  []string
}

//KeyWrapper type
type KeyWrapper struct {
	Key         string
	KeyID       string
	KeyProvider string
}
