package cmd

import (
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"
	"fmt"
    "io"
	"io/ioutil"
    "math/rand"
)

var (
	globalHttpClient http.Client // one client fits all
	regexResponse    = regexp.MustCompile(`^https?://.*?(:\d{2,5})?/\w{5,6}/`)
	testDb	= make(map[string]string)
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


type transferShTest struct { // TODO
	transferSh	// struct is embedded into the test struct
	shouldFail bool // idk how to mark a failed test in go as a success (i.e the function under test should fail and return an error, but it is the expected outcome as it was given a malformed input); i am definitely structuring this wrong; this bool should help in marking failures as successful test cases
}

func uploadHandler(w http.ResponseWriter, req *http.Request){

	// If WriteHeader is not called explicitly, the first call to Write
	// will trigger an implicit WriteHeader(http.StatusOK).
	// w.WriteHeader(http.StatusOK)
	// io.WriteString(w, req.URL)
    uploadToken := encodeToToken(10000000 + int64(rand.Intn(1000000000)))
    deleteToken := encodeToToken(10000000+int64(rand.Intn(1000000000))) + encodeToToken(10000000+int64(rand.Intn(1000000000)))
    uploadUrl := fmt.Sprintf("http://%s/%s%s",  req.Host, uploadToken, req.URL)
    deleteUrl := fmt.Sprintf("%s/%s", uploadUrl, deleteToken)
    testDb[uploadUrl] = deleteUrl

	w.Header().Set("Server", "Transfer.sh HTTP Server 1.0"        )
	w.Header().Set("X-Made-With", "<3 by DutchCoders"             )
	w.Header().Set("X-Served-By", "Proudly served by DutchCoders" )
	w.Header().Set("X-Url-Delete", deleteUrl)
    io.WriteString(w, uploadUrl)

}

func SimulatePostRequest(transfer *transferShTest) error {

	// should honor the failure bit and act accordingly
	// post, then check body for url
	var (
		err  error
		body []byte
	)

	chanHttpResponses := make(chan *http.Response, len(transfer.filePaths))
	chanExtraStrings := make(chan []string, 0) // we won't be sending any extra information for this service
	if err = transfer.Post(chanHttpResponses, chanExtraStrings); err != nil {
		return err
	}
	// inspect the body
	for resp := range chanHttpResponses {
		defer resp.Body.Close()

		if body, err = ioutil.ReadAll(resp.Body); err != nil {
			fmt.Println("failed to read body") // body is new url returned by server
			return err
		}
		if matched := regexResponse.Match(body); matched == false {
			return fmt.Errorf("response string does not match the regex; response: %s", string(body))
		}
	}
	return nil

}
func TestPost(t *testing.T) {
	// start the mock server
	testServer := httptest.NewServer(http.HandlerFunc(uploadHandler))
	defer testServer.Close()
	serverTest := testServer.URL
	testSlice := []transferShTest{ // hostUrl, maxDOwnloads, maxDays, httpClient, filePaths, dbName,, dbBucketNaem, debug
		// normal test
		{
			transferSh: transferSh{serverTest, -1, 7, &globalHttpClient, []string{"/etc/hostname"}, validDbName, validBucketName, false},
			shouldFail: false,
		},

		// server that does not support the latest version
		{
			transferSh: transferSh{"https://transfer.sh", -1, 7, &globalHttpClient, []string{"/etc/hostname"}, validDbName, validBucketName, false},
			shouldFail: false,
		},

		// invalid db name (i.e new db)
		{
			transferSh: transferSh{serverTest, -1, 7, &globalHttpClient, []string{"/etc/hostname"}, "welp", validBucketName, false},
			shouldFail: false,
		},

		// invalid bucket name (i.e new bucket)
		{
			transferSh: transferSh{serverTest, -1, 7, &globalHttpClient, []string{"/etc/hostname"}, validDbName, "invalidBucket", false},
			shouldFail: true,
		},

		// ivalid db name and bucket name (i.e new db and bucket)
		{
			transferSh: transferSh{serverTest, -1, 7, &globalHttpClient, []string{"/etc/hostname"}, "welp", "invalidBucket", false},
			shouldFail: false,
		},

		// invalid file path (reminder: the []string{}string provided here should contain absolute paths]
		{
			transferSh: transferSh{serverTest, -1, 7, &globalHttpClient, []string{"/hostname"}, validDbName, validBucketName, false},
			shouldFail: true,
		},
		{
			transferSh: transferSh{serverTest, -1, 7, &globalHttpClient, []string{"/hostname", "/welp"}, validDbName, validBucketName, false},
			shouldFail: true,
		},

		// valid file paths
		{
			transferSh: transferSh{serverTest, -1, 7, &globalHttpClient, []string{"/etc/hostname", "/etc/passwd"}, validDbName, validBucketName, false},
			shouldFail: false,
		},
	}
	for _, testTransfer := range testSlice {
		if err := SimulatePostRequest(&testTransfer); err != nil && testTransfer.shouldFail == false {
			t.Error(err)
		}
	}
}

// func TestSaveUrl(t *testing.T){
// 
// 	http.Res
// }
