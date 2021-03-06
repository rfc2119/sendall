package cmd

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
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

type transferSh struct {
	// previously: a dummy empty struct{} to implement the interface

	// cmd options:
	hostUrl      string
	maxDownloads int
	maxDays      int

	// mandatory members
	httpClient   *http.Client
	filePaths    []string // ready and clean to be read from
	dbName       string
	dbBucketName string

	// other
	debug bool
}

func (receiver *transferSh) SaveUrl(receivedHttpResponses <-chan *http.Response, extra <-chan []string) error {

	var (
		body   []byte
		db     *bolt.DB
		bucket *bolt.Bucket
		err    error
	)
	allUrlsOk := true
	if db, err = bolt.Open(receiver.dbName, 0600, nil); err != nil {
		fmt.Println("could not open db")
		return err
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(receiver.dbBucketName))
		return err
	})

	if err != nil {
		fmt.Println("create bucket error")
		return err
	}
	for resp := range receivedHttpResponses { // channel extra is not used here

		defer resp.Body.Close()
		if body, err = ioutil.ReadAll(resp.Body); err != nil {
			fmt.Printf("failed to read body: %s", err)
			allUrlsOk = false
			continue
		}
		// fmt.Printf("%s\ndelete url: %s\n====\n", body, resp.Header.Get("X-Url-Delete"))
		fmt.Println(string(body)) // body is new url returned by the server

		err = db.Update(func(tx *bolt.Tx) error {
			bucket = tx.Bucket([]byte(receiver.dbBucketName))
			err := bucket.Put(body, []byte(resp.Header.Get("X-Url-Delete")))
			return err
		})
		if err != nil {
			fmt.Printf("error on writing %s: %s\n", body, err)
			allUrlsOk = false
			continue
		}
		fmt.Printf("wrote %s\n", body)

	}
	if allUrlsOk == false {
		return fmt.Errorf("one or more URLs were not saved properly")
	}

	return nil

}

func (receiver *transferSh) Post(receivedHttpResponses chan<- *http.Response, extra chan<- []string) error {

	var (
		file        *os.File
		url         string
		fileContent *bufio.Reader
		err         error
		newRequest  *http.Request
		holup       sync.WaitGroup
	)
	allRequestsOk := true
	for i := 0; i < len(receiver.filePaths); i++ {

		if file, err = os.Open(receiver.filePaths[i]); err != nil {
			fmt.Println(err)
			continue
		}
		// url = receiver.hostUrl + file.Name()      // TODO: url need to end in '/'
		url = receiver.hostUrl + "/" + sanitize(file.Name())       // TODO: imo we only need filepath.Clean(file.Name())
		fileContent = bufio.NewReader(file)                        // TODO: is this the appropriate way to read a file as an io.Reader ?
		newRequest, err = http.NewRequest("PUT", url, fileContent) // transfer.sh resolves file path and generates a folder with random name
		if err != nil {
			fmt.Println(err)
			allRequestsOk = false
			continue
		}
		// adding custom headers
		newRequest.Header.Add("Max-Downloads", strconv.Itoa(receiver.maxDownloads)) // TODO: Itoa() all the fields ?
		newRequest.Header.Add("Max-Days", strconv.Itoa(receiver.maxDays))

		holup.Add(1)
		go func(req *http.Request, c chan<- *http.Response, reqOk *bool) {
			defer holup.Done()
			if resp, err := receiver.httpClient.Do(req); err != nil {
				fmt.Printf("issuing request failed: %s\n", err)
				*reqOk = false
			} else {
				c <- resp
				// should pass here any extra strings to channel extra, but there is nothing to pass
			}
		}(newRequest, receivedHttpResponses, &allRequestsOk)
	}

	holup.Wait()
	if allRequestsOk == false {
		return fmt.Errorf("one or more request failed")
	}
	close(receivedHttpResponses)
	close(extra)
	return nil
}

func (receiver *transferSh) Delete() error {

	var (
		db        *bolt.DB
		err       error
		deleteUrl []byte
		req       *http.Request
		resp      *http.Response
		bucket    *bolt.Bucket
	)
	allRequestsOk := true

	// check if the db exists or not, also check the bucket
	if _, err = os.Stat(receiver.dbName); os.IsNotExist(err) {
		fmt.Println("delete: db file does not exist")
		return err
	}
	if err != nil {
		fmt.Println("create bucket error")
		return err
	}

	// open db, check if bucket exists; fetch delete link and request deletion
	if db, err = bolt.Open(receiver.dbName, 0600, nil); err != nil {
		fmt.Println("could not open db")
		return err
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(receiver.dbBucketName))
		return err
	})
	for _, file := range receiver.filePaths { // files provided should be the exact received url
		db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(receiver.dbBucketName))
			answer := bucket.Get([]byte(file))
			deleteUrl = make([]byte, len(answer))
			copy(deleteUrl, answer)

			return nil
		})
		if len(deleteUrl) == 0 {
			fmt.Printf("link %s does not have an entry in db\n", file)
			allRequestsOk = false
			continue
		}
		req, _ = http.NewRequest("DELETE", string(deleteUrl), nil)
		if resp, err = receiver.httpClient.Do(req); err != nil { // TODO: find out if Client.Do() does it in a goroutine
			fmt.Printf("issuing request failed: %s\n", err)
			allRequestsOk = false
			continue
		}
		// if receiver.debug {
		// 	resp.Write(os.Stdout)
		// }
		defer resp.Body.Close()
		// body, _ := ioutil.ReadAll(resp.Body)
		// if receiver.debug{
		// 	fmt.Println(string(body))
		// }
		// TODO: assume here we got a 200 response code (what is 200 for transfer ?)
		if resp.Status != "200 OK" {
			fmt.Println("method not allowed (invalid url or file was deleted)")
			allRequestsOk = false
			continue
		}
		err = db.Update(func(tx *bolt.Tx) error {

			bucket = tx.Bucket([]byte(receiver.dbBucketName))
			err = bucket.Delete([]byte(file)) // we're confident that the key does exist, right ?

			return err

		})
		if err != nil {
			fmt.Printf("error deleting link %s from db\n", file)
			allRequestsOk = false
			continue
		}
	}
	if allRequestsOk == false {
		return fmt.Errorf("one or more files were not deleted")
	}
	fmt.Println("done")
	return nil
}

var (
	// ====== default values for options
	transfer = transferSh{ // service transfer.sh
		hostUrl:      serverProd,
		maxDownloads: -1,
		maxDays:      7,
		httpClient:   &http.Client{},
		dbName:       "sendall.db",  // bolt db name
		dbBucketName: "transfer.sh", // bucket used within bolt; contains the posted urls -> deleted urls
		debug:        true,
	}
	// ======
	transferReqHeaders = []string{ // any custom headers used in issuing the request; for reference only
		"Max-Downloads",
		"Max-Days",
	}
	transferRespHeaders = []string{ // any custom headers received on response; for reference only
		"X-Url-Delete",
	}
	transferShCmd = &cobra.Command{
		Use:   "transfer",
		Short: "use transfer.sh service (credits go to DutchCoders)",
		Args:  cobra.MinimumNArgs(1),
		Run: func(command *cobra.Command, args []string) {
			// flags have populated cmd memebrs of the service struct
			transfer.filePaths = prepareFiles(args) // files are provided straight from the cmd interface; tidy them
			chanHttpResponses := make(chan *http.Response, len(transfer.filePaths))
			chanExtraStrings := make(chan []string, 0) // we won't be sending any extra information for this service
			go func() {
				if err := transfer.Post(chanHttpResponses, chanExtraStrings); err != nil {
					fmt.Println(err)
				}
			}()
			if err := transfer.SaveUrl(chanHttpResponses, chanExtraStrings); err != nil {
				fmt.Println(err)
			}
		},
	}

	transferShDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "delete a link posted before",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			// flags have populated cmd memebrs of the "transfer" struct
			transfer.filePaths = args // it is expected that provided arguments are the exact links you received from the service
			if err := transfer.Delete(); err != nil {
				fmt.Println(err)
			}
		},
	}
)

func init() {
	transferShCmd.Flags().IntVarP(&transfer.maxDownloads, "downloads", "e", transfer.maxDownloads, "Maximum number of downloads after which the link will expire")
	transferShCmd.Flags().IntVarP(&transfer.maxDays, "days", "d", transfer.maxDays, "Maximum number of days after which the file will be removed from the server")
	transferShCmd.Flags().StringVarP(&transfer.hostUrl, "host", "u", transfer.hostUrl, "service URL, for example if you host your own instance")
	transferShCmd.AddCommand(transferShDeleteCmd)
	rootCmd.AddCommand(transferShCmd)
}

// PUT: /put/$filename, /upload/$filename, /$filename
// POST: /
// DELETE: /$token/$filename/$deletiontoken		// provided by default by the server
// GET: /$token/$filename
// HEAD: /$token/$filename
