package cmd

import (
	"fmt"
	// "strings"
	// "bytes"
	"bufio"
	"net/http"
	"os"

	"github.com/spf13/cobra"
)

const (
	serverDev  = "http://127.0.0.1"
	portDev    = "8090"
	serverProd = "https://transfer.sh"
)

type generalRequest struct {
	method        string
	path          string
	files         []string
	customHeaders map[string]string
}

var (
	host            = fmt.Sprintf("%s:%s/", serverDev, portDev)
	transferHeaders = []string{
		"Max-Downloads",
		"Max-Days",
		"X-Url-Delete",
	}
	transferCmd = &cobra.Command{
		Use:   "transfer.sh",
		Short: "transfer.sh service",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("executed transfer.sh!")
		},
	}
)

func init() {
	rootCmd.AddCommand(transferCmd)
}

func doGeneralRequest(client *http.Client, req *generalRequest, chanResponse chan *http.Response) {
	var (
		file *os.File

		fileContent bufio.Reader
		err         error
	)
	for i := 0; i < len(req.files); i++ {

		if file, err = os.Open(req.files[i]); err != nil {
			panic(err)
		}
		path := host + sanitize(req.files[i])
		body := bufio.NewReader(file)
	
		newRequest, err := http.NewRequest(req.method, path, body)
		if err != nil {
			fmt.Println(err)
		}
		for hdr, val := range req.customHeaders {
			newRequest.Header.Add(hdr, val)
		}
		if resp, err := client.Do(newRequest); err != nil {
			fmt.Printf("issuing request failed: %s", err)
		} else {
			chanResponse <- resp
		}

	}
	close(chanResponse)
}

// PUT: /put/$filename, /upload/$filename, /$filename
// POST: /
// DELETE: /$token/$filename/$deletiontoken
// GET: /$token/$filename
// HEAD: /$token/$filename
