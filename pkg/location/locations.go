package location

//UpdatedLocation type
type UpdatedLocation struct {
	LocationType string
	LocationURI  string
	LocationIDs  []string
}

//keyWrapper type
type keyWrapper struct {
	key         string
	keyID       string
	keyProvider string
}
