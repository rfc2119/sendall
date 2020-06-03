package cmd

import (
	"fmt"
	// "errors"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/boltdb/bolt"
	"github.com/gorilla/mux"
)

var (
	globalHttpClient http.Client // one client fits all
	regexResponse    = regexp.MustCompile(`^https?://.*?(:\d{2,5})?/\w{5,6}/`)
	testDb           = make(map[string]string)
)

const (
	// serverTest      = "http://127.0.0.1:8090/"
	validDbName     = "sendall_test.db"
	validBucketName = "transfer.sh"
	// characters used for short-urls
	SYMBOLS = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// someone set us up the bomb !!
	BASE = int64(len(SYMBOLS))
)

/*
https://github.com/fs111/kurz.go/blob/master/src/codec.go

Originally written and Copyright (c) 2011 André Kelpe
Modifications Copyright (c) 2015 John Ko

// encodeToTokens a number into our *base* representation
// TODO can this be made better with some bitshifting?
// TODO: this is VERY SLOW!
*/
func encodeToToken(number int64) string {
	rest := number % BASE
	// strings are a bit weird in go...
	result := string(SYMBOLS[rest])
	if number-rest != 0 {
		newnumber := (number - rest) / BASE
		result = encodeToToken(newnumber) + result
	}
	return result
}

type transferShTest struct {
	transferSh      // struct is embedded into the test struct
	shouldFail bool // when a test returns a valid err, it should not be marked as failure, because the input was already malformed; i am definitely structuring this wrong;
}

func uploadHandler(w http.ResponseWriter, req *http.Request) {
	// a mock handler that always returns a valid URL
	// this should be changed soon

	// If WriteHeader is not called explicitly, the first call to Write
	// will trigger an implicit WriteHeader(http.StatusOK).
	// w.WriteHeader(http.StatusOK)
	// io.WriteString(w, req.URL)
	uploadToken := encodeToToken(10000000 + int64(rand.Intn(1000000000)))
	deleteToken := encodeToToken(10000000+int64(rand.Intn(1000000000))) + encodeToToken(10000000+int64(rand.Intn(1000000000)))
	uploadUrl := fmt.Sprintf("http://%s/%s%s", req.Host, uploadToken, req.URL)
	deleteUrl := fmt.Sprintf("%s/%s", uploadUrl, deleteToken)

	// update db
	// testDb[uploadUrl] = deleteUrl
	db, err := bolt.Open(validDbName, 0600, nil)
	if err != nil {

		fmt.Println("could not open db")
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		bucket := tx.Bucket([]byte(validBucketName))
		err := bucket.Put([]byte(uploadUrl), []byte(deleteUrl))
		return err
	})
	if err != nil {
		fmt.Println("welp")
	}

	w.Header().Set("Server", "Transfer.sh HTTP Server 1.0")
	w.Header().Set("X-Made-With", "<3 by DutchCoders")
	w.Header().Set("X-Served-By", "Proudly served by DutchCoders")
	w.Header().Set("X-Url-Delete", deleteUrl)
	io.WriteString(w, uploadUrl)
}

func deleteHandler(w http.ResponseWriter, req *http.Request) {
	// if a client has dialed this url, then we're sure it's the delete url.
	// Delete() searches the database for the delete link, and issue a delete
	// request. since this handler is called after being matching with a regex,
	// it's likely his is a legit request
	w.WriteHeader(http.StatusOK)
}

func initDb() error {
	var (
		db  *bolt.DB
		err error
	)
	if db, err = bolt.Open(validDbName, 0600, nil); err != nil {
		fmt.Println("could not open db")
		return err
	}
	defer db.Close()
	err = db.Update(func(tx *bolt.Tx) error {
		_, err = tx.CreateBucketIfNotExists([]byte(validBucketName))
		return err
	})

	if err != nil {
		fmt.Println("create bucket error")
		return err
	}
	return nil

}
func SimulatePostRequest(transfer *transferShTest) (error, []string) {

	// should honor the failure bit and act accordingly
	// post, then check body for url
	var (
		err  error
		body []byte
	)

	postedUrls := []string{}
	chanHttpResponses := make(chan *http.Response, len(transfer.filePaths))
	chanExtraStrings := make(chan []string, 0) // we won't be sending any extra information for this service
	if err = transfer.Post(chanHttpResponses, chanExtraStrings); err != nil {
		return err, postedUrls
	}
	// inspect the body
	for resp := range chanHttpResponses {
		defer resp.Body.Close()

		if body, err = ioutil.ReadAll(resp.Body); err != nil {
			fmt.Println("failed to read body") // body is new url returned by server
			return err, postedUrls
		}
		if matched := regexResponse.Match(body); matched == false {
			return fmt.Errorf("response string does not match the regex; response is: %s", string(body)), postedUrls
		}

		postedUrls = append(postedUrls, string(body)) // to test Delete() as well

	}
	return nil, postedUrls

}

func TestPostAndDelete(t *testing.T) {
	var (
		err        error
		postedUrls []string
	)
	if err = initDb(); err != nil {
		fmt.Println(err)
	}

	// start the mock server
	router := mux.NewRouter()
	// router.Methods("PUT", "DELETE")     // allow only these methods for testing
	router.HandleFunc("/{token}/{filename}/{deletionToken:\\w{10,12}}", deleteHandler).Methods("DELETE")
	router.HandleFunc("/put/{filename}", uploadHandler).Methods("PUT")
	router.HandleFunc("/upload/{filename}", uploadHandler).Methods("PUT")
	router.HandleFunc("/{filename}", uploadHandler).Methods("PUT")

	testServer := httptest.NewServer(router)
	defer testServer.Close()
	hostUrl := testServer.URL

	// wannabe tests
	sliceTests := []transferShTest{
		// hostUrl, maxDOwnloads, maxDays, httpClient, filePaths, dbName,, dbBucketNaem, debug
		// normal settings
		{
			transferSh: transferSh{hostUrl, -1, 7, &globalHttpClient, []string{"/etc/hostname"}, validDbName, validBucketName, false},
			shouldFail: false,
		},

		// server that does not support the latest version with a valid file
		// {
		// 	transferSh: transferSh{"https://transfer.sh", -1, 7, &globalHttpClient, []string{"/etc/hostname"}, validDbName, validBucketName, false},
		// 	shouldFail: false,
		// },

		// invalid db name (i.e new db)
		{
			transferSh: transferSh{hostUrl, -1, 7, &globalHttpClient, []string{"/etc/hostname"}, "welp", validBucketName, false},
			shouldFail: true,
		},

		// invalid bucket name (i.e new bucket)
		{
			transferSh: transferSh{hostUrl, -1, 7, &globalHttpClient, []string{"/etc/hostname"}, validDbName, "invalidBucket", false},
			shouldFail: true,
		},

		// ivalid db name and bucket name (i.e new db and bucket)
		{
			transferSh: transferSh{hostUrl, -1, 7, &globalHttpClient, []string{"/etc/hostname"}, "welp", "invalidBucket", false},
			shouldFail: true,
		},

		// invalid file path (reminder: the []string provided here should contain absolute paths)
		{
			transferSh: transferSh{hostUrl, -1, 7, &globalHttpClient, []string{"/hostname"}, validDbName, validBucketName, false},
			shouldFail: true,
		},
		{
			transferSh: transferSh{hostUrl, -1, 7, &globalHttpClient, []string{"/hostname", "/welp"}, validDbName, validBucketName, false},
			shouldFail: true,
		},

		// valid file paths
		{
			transferSh: transferSh{hostUrl, -1, 7, &globalHttpClient, []string{"/etc/hostname", "/etc/passwd"}, validDbName, validBucketName, false},
			shouldFail: false,
		},
	}

	// using tests
	for _, test := range sliceTests {
		if err, postedUrls = SimulatePostRequest(&test); err != nil && test.shouldFail == false {
			t.Error(err)
		}
		test.filePaths = postedUrls
		if err = test.Delete(); err != nil && test.shouldFail == false {
			t.Error(err)
		}
	}
}

// TODO: it'll be a bit tiresome to test saveUrl, so i opted for another option: make the server save the url instead of the client. of course saveUrl is not tested in this way, but it's a temporary solution
// func TestSaveUrl(t *testing.T){
//
// }
