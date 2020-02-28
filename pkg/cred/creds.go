// Copyright 2019 OVO Technology
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cred

// Credentials type
type Credentials struct {
	CircleCIAPIToken string
	GitAccount       GitAccount
	AkrPass          string
	AkrPath          string
	KmsKey           string
	GocdServer       GocdServer
	AtlasKeys		 AtlasKeys
}

// GitAccount type
type GitAccount struct {
	GitAccessToken string
	GitName        string
	GitEmail       string
}

// GocdServer type
type GocdServer struct {
	Server       string
	SkipSslCheck bool
	Username     string
	Password     string
}

// Atlas type
type AtlasKeys struct {
	PublicKey	string
	PrivateKey	string
}