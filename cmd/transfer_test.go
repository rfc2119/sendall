package cmd

import (
	"net/http"
	"regexp"
	"testing"
	"fmt"
	"io/ioutil"
)

var (
	globalHttpClient http.Client // one client fits all
	regexResponse    = regexp.MustCompile(`^https?://.*?(:\d{2,4})?/\w{5,6}/`)
)

const (
	serverTest      = "http://127.0.0.1:8090/"
	validDbName     = "sendall_test.db"
	validBucketName = "transfer.sh"
)

type transferShTest struct { // TODO
	transferSh	// struct is embedded into the test struct (
	shouldFail bool // idk how to mark a failed test in go as a success (i.e the function under test should fail and return an error, but it is the expected outcome as it was given a malformed input); i might be structuring this wrong; this bool should help in marking failures as successful test cases
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
		fmt.Println(err)
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
func TestSuite1(t *testing.T) {
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
