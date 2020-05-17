package cmd

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"testing"
)

var (
	globalHttpClient http.Client // one client fits all
)

func printResponse(c chan *http.Response) {
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

func sendRequestReceiveResponse(req *generalRequest) {

	c := make(chan *http.Response, len(req.files))
	go doGeneralRequest(&globalHttpClient, req, c)
	printResponse(c)

}
func TestPutMaxDownloads1(t *testing.T) {
	req := generalRequest{
		method: "PUT",
		// path:   "/",
		files: []string{"/etc/passwd", "/usr/share/backgrounds/xfce/xfce-blue.jpg"},
		customHeaders: map[string]string{
			"Max-Downloads": "1",
		},
	}
	sendRequestReceiveResponse(&req)
}
func TestPutMaxDownloads2(t *testing.T) {
	req := generalRequest{
		method: "PUT",
		path:   "/put/", // TODO: ineffictive, as path is rewitten in dogeneralRequest
		files:  []string{"/etc/passwd"},
		customHeaders: map[string]string{
			"Max-Downloads": "1",
		},
	}
	sendRequestReceiveResponse(&req)
}

