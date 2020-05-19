package cmd

import (
	"net/http"
	"testing"
)

var (
	globalHttpClient http.Client // one client fits all
)

func TestPutMaxDownloads1(t *testing.T) {
	req := generalRequest{
		method: "PUT",
		files: []string{"/etc/passwd", "/usr/share/backgrounds/xfce/xfce-blue.jpg"},
		reqHeaders: map[string]string{
			"Max-Downloads": "1",
		},
	}
	sendRequestPrintResponse(&globalHttpClient, &req)
}
func TestPutMaxDownloads2(t *testing.T) {
	req := generalRequest{
		method: "PUT",
		// host:   "/put/", // TODO: ineffictive, as path is rewitten in dogeneralRequest
		files:  []string{"/etc/passwd"},
		reqHeaders: map[string]string{
			"Max-Downloads": "1",
		},
	}
	sendRequestPrintResponse(&globalHttpClient, &req)
}

