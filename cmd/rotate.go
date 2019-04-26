package cmd

import (
	"github.com/ovotech/cloud-key-rotator/pkg/config"
	"github.com/ovotech/cloud-key-rotator/pkg/log"
	"github.com/ovotech/cloud-key-rotator/pkg/rotate"
	"github.com/spf13/cobra"
)

var (
	rotateCmd = &cobra.Command{
		Use:   "rotate",
		Short: "Rotate some cloud keys",
		Long:  `Rotate some cloud keys`,
		Run: func(cmd *cobra.Command, args []string) {
			logger.Info("cloud-key-rotator rotate called")
			var err error
			var c config.Config
			if c, err = config.GetConfig(defaultConfigPath); err == nil {
				err = rotate.Rotate(account, provider, project, c)
			}
			if err != nil {
				logger.Error(err)
			}
		},
	}
	account           string
	configPath        string
	defaultConfigPath = "/etc/cloud-key-rotator/"
	provider          string
	project           string
	defaultAccount    string
	defaultProvider   string
	defaultProject    string
	logger            = log.StdoutLogger().Sugar()
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
