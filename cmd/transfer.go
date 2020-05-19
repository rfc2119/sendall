package cmd

import (
	"fmt"
	"strconv"
	// "strings"
	// "bytes"
	"bufio"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"sync"

	"github.com/boltdb/bolt"
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
	httpClient            = http.Client{}
	hostUrl, deleteUrl    string
	maxDownloads, maxDays int
	dbName                = "sendall.db"
	dbBucketName          = "deleteUrls" // bucket used with bolt
	transferReqHeaders    = []string{    // any custom headers used in issuing the request; for reference
		"Max-Downloads",
		"Max-Days",
	}
	transferRespHeaders = []string{ // any custom headers received on response
		"X-Url-Delete",
	}
	transferCmd = &cobra.Command{
		Use:   "transfer",
		Short: "use transfer.sh service (credits go to DutchCoders)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			fileList := make([]string, len(args)) // TODO: be careful if you changed command syntax
			for idx, file := range args {
				if absPath, err := filepath.Abs(file); err != nil { // calls filepath.Clean() on exit
					fmt.Println(err)
				} else {
					fileList[idx] = absPath
				}
			}
			req := generalRequest{
				method: "PUT", // default method is to upload
				host:   hostUrl,
				files:  fileList,
				reqHeaders: map[string]string{
					"Max-Downloads": strconv.Itoa(maxDownloads), // TODO: Itoa() all the fields ?
					"Max-Days":      strconv.Itoa(maxDays),
				},
			}
			sendRequestSaveResponse(&httpClient, &req)
		},
	}

	deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "delete a link posted before",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			var (
				db        *bolt.DB
				err       error
				deleteUrl []byte
				req       *http.Request
				resp      *http.Response
				bucket    *bolt.Bucket
			)

			if db, err = bolt.Open(dbName, 0600, nil); err != nil {
				fmt.Errorf("could not open db: %s", err)
				return
			}
			defer db.Close()
			for _, file := range args {
				db.View(func(tx *bolt.Tx) error {
					bucket = tx.Bucket([]byte(dbBucketName))
					answer := bucket.Get([]byte(file))
					deleteUrl = make([]byte, len(answer))
					copy(deleteUrl, answer)

					return nil
				})
				if len(deleteUrl) == 0 {
					fmt.Println("link %s does not have an entry in db", file)
					continue
				}
				req, _ = http.NewRequest("DELETE", string(deleteUrl), nil)
				if resp, err = httpClient.Do(req); err != nil { // TODO: find out if Client.Do() does it in a goroutine
					fmt.Printf("issuing request failed: %s", err)
					continue
				}
				defer resp.Body.Close()
				body, _ := ioutil.ReadAll(resp.Body)
				fmt.Println(string(body))
				// assume here we got a 200 response code
				err = db.Update(func(tx *bolt.Tx) error {

					bucket = tx.Bucket([]byte(dbBucketName))
					err = bucket.Delete([]byte(file)) // we're sure that the key does exist, right ?

					return err

				})
				if err != nil {
					fmt.Println("error deleting %s from db", err)
				}
			}
			fmt.Println("done")

		},
	}
)

func init() {
	transferCmd.Flags().IntVarP(&maxDownloads, "downloads", "e", -1, "Maximum number of downloads after which the link will expire")
	transferCmd.Flags().IntVarP(&maxDays, "days", "d", 1, "Maximum number of days after which the file will be removed from the server")
	transferCmd.Flags().StringVarP(&hostUrl, "host", "u", serverProd, "service URL, for example if you host your own instance")
	transferCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(transferCmd)
}

func saveResponse(c <-chan *http.Response) {
	var (
		body []byte
		db   *bolt.DB
		// bucket *bolt.Bucket
		err error
	)
	if db, err = bolt.Open("sendall.db", 0600, nil); err != nil {
		fmt.Errorf("could not open db: %s", err)
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(dbBucketName))
		return err
	})

	if err != nil {
		fmt.Printf("create bucket error: %s\n", err)
		return
	}
	for resp := range c {

		defer resp.Body.Close()
		if body, err = ioutil.ReadAll(resp.Body); err != nil {
			fmt.Printf("failed to read body: %s", err) // body is new url returned by server
			continue
		}

		err = db.Update(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(dbBucketName))
			err := bucket.Put(body, []byte(resp.Header.Get("X-Url-Delete")))
			return err
		})
		if err != nil {
			fmt.Printf("error on writing %s: %s", body, err)
			continue
		}
		fmt.Printf("wrote %s\n", body)

	}

}
func printResponse(c <-chan *http.Response) {
	for resp := range c {
		// fmt.Printf("got response from file %d\n", nfile)
		defer resp.Body.Close()
		if body, err := ioutil.ReadAll(resp.Body); err != nil {
			fmt.Println(err)
		} else {
			fmt.Printf("%s\ndelete url: %s\n====\n", body, resp.Header.Get("X-Url-Delete"))
		}
	}
}

func sendRequestSaveResponse(client *http.Client, req *generalRequest) {

	c := make(chan *http.Response, len(req.files))
	go doGeneralRequest(client, req, c)
	saveResponse(c)

}
func sendRequestPrintResponse(client *http.Client, req *generalRequest) {

	c := make(chan *http.Response, len(req.files))
	go doGeneralRequest(client, req, c)
	printResponse(c)

}

func doGeneralRequest(client *http.Client, req *generalRequest, chanResponse chan<- *http.Response) {
	var (
		file        *os.File
		url         string
		fileContent *bufio.Reader
		err         error
		newRequest  *http.Request
		holup       sync.WaitGroup
	)
	for i := 0; i < len(req.files); i++ {

		if file, err = os.Open(req.files[i]); err != nil {
			panic(err)
		}
		if req.host == serverProd { // comment this out; for local developing
			req.host = fmt.Sprintf("%s:%s/", serverDev, portDev)
		}
		// url = req.host + file.Name()      // TODO: url need to end in '/'
		url = req.host + sanitize(file.Name())
		fileContent = bufio.NewReader(file) // TODO: is this the appropriate way to read a file as an io.Reader ?

		newRequest, err = http.NewRequest(req.method, url, fileContent) // transfer.sh resolves filePath and generates a folder with random name
		if err != nil {
			fmt.Println(err)
		}
		for hdr, val := range req.reqHeaders {
			newRequest.Header.Add(hdr, val)
		}
		holup.Add(1)
		go func(req *http.Request, c chan<- *http.Response) { // TODO: not sure if client.Do() already dispatches a goroutine; if so, this is a waste
			if resp, err := client.Do(req); err != nil {
				fmt.Printf("issuing request failed: %s", err)
			} else {
				c <- resp
			}
			holup.Done()
		}(newRequest, chanResponse)
	}

	holup.Wait()
	close(chanResponse)
}

// PUT: /put/$filename, /upload/$filename, /$filename
// POST: /
// DELETE: /$token/$filename/$deletiontoken		// provided by default by the server
// GET: /$token/$filename
// HEAD: /$token/$filename
