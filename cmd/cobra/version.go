package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var (
	defaultLdFlag  = "-"
	version        = defaultLdFlag
	commit         = defaultLdFlag
	date           = defaultLdFlag
	osArch         = defaultLdFlag
	versionField   = "Version"
	gitCommitField = "Git commit"
	builtField     = "Built"
	osArchField    = "OS/Arch"
	ldFlags        = map[string]string{
		versionField:   version,
		gitCommitField: commit,
		builtField:     date,
		osArchField:    osArch}
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
