package cmd

import (
	"fmt"
	"os"
	"net/http"

	// homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
)


// all services should implement this
type service interface {
	// TODO: there should be some method that Post() uses on all files; this method should be a candidate as a goroutine 
	Post(receivedHttpResponses chan<- *http.Response, extra chan<- []string) error		// constructs a new http request with requested files and send it; the response is sent to the channel receivedHttpResponses in order for the save method to save deletion tokens; the channel "extra" is used to communicate any extra information  necessary for the save method to operation
	SaveUrl(receivedHttpResponses <-chan *http.Response, extra <-chan []string) error		// saves deletion tokens obtained from the service in local db
	Delete() error		// fetches urls:deletion_tokens from local db (saved by the save method) and issue a delete request to the service, then deletes the record from the db
}

var (
	// Used for flags.
	// cfgFile     string
	// userLicense string

	rootCmd = &cobra.Command{
		// Version: VERSION,
		Args: cobra.MinimumNArgs(1),
		Short: "sendall wraps several file sharing backends into one app",
		Long:  "sendall  eases the usage of anonymous file-sharing websites by wraping them under one interface. you can checkout a quick demo at <demo_website_hopefully>",

		//Run: func(cmd *cobra.Command, args []string) {
		//	fmt.Println("root")
		//	return
		//},
	}
)

func init() {
	// cobra.OnInitialize(initConfig)

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
