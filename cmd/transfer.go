package cmd

import (
	"fmt"
	// "strings"
	// "bytes"
	"bufio"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

const (
	serverDev  = "http://127.0.0.1"
	portDev    = "8090"
	serverProd = "https://transfer.sh"
)

type generalRequest struct {
	method      string
	host        string
	files       []string
	reqHeaders  map[string]string // TODO: string->string
	respHeaders map[string]string
}

var (
	hostUrl, deleteUrl    string
	maxDownloads, maxDays int
	transferReqHeaders    = []string{ // any custom headers used in issuing the request; for reference
		"Max-Downloads",
		"Max-Days",
		"X-Url-Delete",
	}
	transferRespHeaders = []string{ // any custom headers received on response
		// "X-Url-Delete",
	}
	transferCmd = &cobra.Command{
		Use:   "transfer.sh",
		Short: "use transfer.sh service",
		Run: func(cmd *cobra.Command, args []string) {
			fileList := make([]string, len(args)) // TODO: be careful if you changed command syntax
			for idx, file := range args {
				if absPath, err := filepath.Abs(file); err != nil{ // calls filepath.Clean() on exit
					fmt.Println(err)
				} else {
				fileList[idx] = absPath
			}
			}
			req := generalRequest{
				method: "PUT",		// default method is to upload
				host:   hostUrl,
				files:  fileList,
				reqHeaders: map[string]string{
					"Max-Downloads": string(maxDownloads),	// TODO: string() all the fields ?
					"Max-Days":      string(maxDays),
					// "X-Url-Delete": deleteUrl
				},
			}
			fmt.Println(req.reqHeaders)
		},
	}
)

func init() {
	transferCmd.Flags().IntVarP(&maxDownloads, "downloads", "e", -1, "Maximum number of downloads after which the link will expire")
	transferCmd.Flags().IntVarP(&maxDays, "days", "d", 7, "Maximum number of days after which the file will be removed from the server")
	transferCmd.Flags().StringVarP(&hostUrl, "host", "u", serverProd, "service URL, for example if you host your own instance")
	rootCmd.AddCommand(transferCmd)
}

func doGeneralRequest(client *http.Client, req *generalRequest, chanResponse chan *http.Response) {
	var (
		file        *os.File
		url    string
		fileContent *bufio.Reader
		err         error
		newRequest  *http.Request
	)
	for i := 0; i < len(req.files); i++ {

		if file, err = os.Open(req.files[i]); err != nil {
			panic(err)
		}
		if req.host == ""{		// for local developing
			req.host = fmt.Sprintf("%s:%s/", serverDev, portDev)
		}
		url = req.host + file.Name()      // TODO: url need to end in '/'
		fileContent = bufio.NewReader(file) // TODO: is this the appropriate way to read a file as an io.Reader ?

		newRequest, err = http.NewRequest(req.method, url, fileContent) // transfer.sh resolves filePath and generates a folder with random name
		if err != nil {
			fmt.Println(err)
		}
		for hdr, val := range req.reqHeaders {
			newRequest.Header.Add(hdr, val)
		}
		go func(req *http.Request, c chan *http.Response) { // TODO: not sure if client.Do() already dispatches a goroutine; if so, this is a waste
			if resp, err := client.Do(req); err != nil {
				fmt.Printf("issuing request failed: %s", err)
			} else {
				c <- resp
			}
		}(newRequest, chanResponse)
	}

	close(chanResponse)
}

// PUT: /put/$filename, /upload/$filename, /$filename
// POST: /
// DELETE: /$token/$filename/$deletiontoken		// provided by default by the server
// GET: /$token/$filename
// HEAD: /$token/$filename
