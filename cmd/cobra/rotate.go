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
	"github.com/ovotech/cloud-key-rotator/pkg/config"
	"github.com/ovotech/cloud-key-rotator/pkg/rotate"
	"github.com/spf13/cobra"
)

var (
	rotateCmd = &cobra.Command{
		Use:   "rotate",
		Short: "Rotate some cloud keys",
		Long:  `Rotate some cloud keys`,
		Run: func(cmd *cobra.Command, args []string) {
			var err error
			var c config.Config
			if c, err = config.GetConfig(configPath); err == nil {
				err = rotate.Rotate(account, provider, project, c)
			}
			if err != nil {
				logger.Fatal(err)
			}
		},
	}
)

func init() {
	rotateCmd.Flags().StringVarP(&account, "account", "a", defaultAccount,
		"Account to rotate")
	rotateCmd.Flags().StringVarP(&configPath, "config", "c", defaultConfigPath,
		"Absolute path of application config")
	rotateCmd.Flags().StringVarP(&provider, "provider", "p", defaultProvider,
		"Provider of account to rotate")
	rotateCmd.Flags().StringVarP(&project, "project", "j", defaultProject,
		"Project of account to rotate")
	rootCmd.AddCommand(rotateCmd)

}
