package cmd

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	method     string
	host       string
	files      []string
	reqHeaders map[string]string // for each c in transferReqHeaders: reqHeaders[c] = <val>
}

type generalResponse struct {
	respHeaders map[string]string // for each c in transferRespHeaders: reqHeaders[c] = <val>
}

type transferSh struct {
	// dummy type to implement the interface
}

func (*transferSh) Post(files []string) {

	// files are provided straight from the cmd interface
	fileList := make([]string, len(files)) // TODO: be careful if you changed command syntax
	for idx, file := range files {
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
}
func (*transferSh) Delete(files []string) {

	var (
		db        *bolt.DB
		err       error
		deleteUrl []byte
		req       *http.Request
		resp      *http.Response
		bucket    *bolt.Bucket
	)

	if db, err = bolt.Open(dbName, 0600, nil); err != nil {
		fmt.Println("could not open db: %s", err)
		return
	}
	defer db.Close()
	for _, file := range files { // files provided should be the exact received url
		db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(dbBucketName))
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
		// TODO: assume here we got a 200 response code
		err = db.Update(func(tx *bolt.Tx) error {

			bucket = tx.Bucket([]byte(dbBucketName))
			err = bucket.Delete([]byte(file)) // we're sure that the key does exist, right ?

			return err

		})
		if err != nil {
			fmt.Printf("error deleting %s from db", err)
		}
	}
	fmt.Println("done")
}

var (
	// ====== default values for options
	hostUrl      = serverProd
	maxDownloads = -1
	maxDays      = 7
	// ======
	transferReqHeaders = []string{ // any custom headers used in issuing the request; for reference only
		"Max-Downloads",
		"Max-Days",
	}
	transferRespHeaders = []string{ // any custom headers received on response; for reference only
		"X-Url-Delete",
	}
	// transferDefaults = map[string]interface{}{ // defaults for the cmd interface; for refernce
	// 	"downlaods": -1,
	// 	"days":      7,
	// 	"host":      serverProd,
	// }
	httpClient   = http.Client{}
	dbName       = "sendall.db" // bolt db name
	dbBucketName = "deleteUrls" // bucket used with bolt
	transfer     transferSh     // service transfer.sh
	transferCmd  = &cobra.Command{
		Use:   "transfer",
		Short: "use transfer.sh service (credits go to DutchCoders)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			transfer.Post(args)
		},
	}

	deleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "delete a link posted before",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			transfer.Delete(args)
		},
	}
)

func init() {
	transferCmd.Flags().IntVarP(&maxDownloads, "downloads", "e", maxDownloads, "Maximum number of downloads after which the link will expire")
	transferCmd.Flags().IntVarP(&maxDays, "days", "d", maxDays, "Maximum number of days after which the file will be removed from the server")
	transferCmd.Flags().StringVarP(&hostUrl, "host", "u", hostUrl, "service URL, for example if you host your own instance")
	transferCmd.AddCommand(deleteCmd)
	rootCmd.AddCommand(transferCmd)
}

func saveResponse(c <-chan *http.Response, printOnly bool) error {
	var (
		body   []byte
		db     *bolt.DB
		bucket *bolt.Bucket
		err    error
	)
	if db, err = bolt.Open("sendall.db", 0600, nil); err != nil {
		fmt.Println("could not open db")
		return err
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(dbBucketName))
		return err
	})

	if err != nil {
		fmt.Println("create bucket error")
		return err
	}
	for resp := range c {

		defer resp.Body.Close()
		if body, err = ioutil.ReadAll(resp.Body); err != nil {
			fmt.Printf("failed to read body: %s", err) // body is new url returned by server
			continue
		}
		if printOnly {
			fmt.Printf("%s\ndelete url: %s\n====\n", body, resp.Header.Get("X-Url-Delete"))
			continue
		}

		err = db.Update(func(tx *bolt.Tx) error {
			bucket = tx.Bucket([]byte(dbBucketName))
			err := bucket.Put(body, []byte(resp.Header.Get("X-Url-Delete")))
			return err
		})
		if err != nil {
			fmt.Printf("error on writing %s: %s", body, err)
			continue
		}
		fmt.Printf("wrote %s\n", body)

	}
	return nil

}

func sendRequestSaveResponse(client *http.Client, req *generalRequest) {

	c := make(chan *http.Response, len(req.files))
	go doGeneralRequest(client, req, c)
	if err := saveResponse(c, false); err != nil {
		fmt.Println(err)
	}

}
func sendRequestPrintResponse(client *http.Client, req *generalRequest) {

	c := make(chan *http.Response, len(req.files))
	go doGeneralRequest(client, req, c)
	if err := saveResponse(c, true); err != nil {
		fmt.Println(err)
	}

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
