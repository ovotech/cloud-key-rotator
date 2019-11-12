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

package cmd

import (
	"fmt"
	"strconv"

	"github.com/ovotech/cloud-key-rotator/pkg/build"
	"github.com/spf13/cobra"
)

var (
	versionField   = "Version"
	gitCommitField = "Git commit"
	builtField     = "Built"
	osArchField    = "OS/Arch"
	ldFlags        = map[string]string{
		versionField:   build.Version,
		gitCommitField: build.Commit,
		builtField:     build.Date,
		osArchField:    build.OsArch}
	//map insertion order isn't maintained, so use a slice for that
	ldFlagOrder  = []string{versionField, gitCommitField, builtField, osArchField}
	formatString = "%-" + strconv.Itoa(maxLength(ldFlagOrder)+2) + "v%s\n"
	versionCmd   = &cobra.Command{
		Use:   "version",
		Short: "Version number of cloud-key-rotator",
		Long:  "Version number of cloud-key-rotator",
		Run: func(cmd *cobra.Command, args []string) {
			for _, ldf := range ldFlagOrder {
				fmt.Printf(formatString, ldf+":", ldFlags[ldf])
			}
		},
	}
)

func maxLength(s []string) (maxLength int) {
	for _, i := range s {
		length := len(i)
		if len(i) > maxLength {
			maxLength = length
		}
	}
	return
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
