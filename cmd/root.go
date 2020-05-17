package cmd

import (
    "fmt"
    "os"

	// homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"

)


var (
	// Used for flags.
	// cfgFile     string
	// userLicense string

	rootCmd     = &cobra.Command{
		Version: VERSION,
		Use:     "sendall <host> [flags] [<cmd1> [cmd1_flags], ...]",
		Short:   "sendall wraps several file sharing backends into one app",
		Long: `sendall  eases the usage of anonymous file sharing websites
                by wraping them under one interface.
                you can checkout a quick demo at <demo_website_hopefully>`,

		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("root")
			return
		},
	}
)


func init() {
	// cobra.OnInitialize(initConfig)

	// rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.cobra.yaml)")
	rootCmd.PersistentFlags().StringP("author", "a", "YOUR NAME", "author name for copyright attribution")
	// rootCmd.PersistentFlags().StringVarP(&userLicense, "license", "l", "", "name of license for the project")

	// main commands are defined here (are they added by default ? verify)
}


func initConfig() {
	// if cfgFile != "" {
	// 	// Use config file from the flag.
	// } else {
	// 	// Find home directory.
	// 	home, err := homedir.Dir()
	// 	if err != nil {
	// 		er(err)
	// 	}
        // fmt.Println("found home %s", home)
	// 	// Search config in home directory with name ".cobra" (without extension).
	// }

}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
