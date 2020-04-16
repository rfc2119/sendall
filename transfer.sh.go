package main

import (
	"net/http"
	"bytes"

)
const (
	serverDev = "http://127.0.0.1"
	portDev = "8090"
	serverProd = "https://transfer.sh"
)

func main(){
	serverURL := fmt.Sprintf("%s:%s", serverDev, portDev)
	client := http.Client{}

	postRequest := http.NewRequest("PUT", serverURL, bytes.NewReader(arg1))
	postRequest.Header.Add("Max-Downloads", maxDownloads)	// ok to include header if value is empty
	postRequest.Header.Add("Max-Days", maxDays)

	if resp, err := client.Do(postRequest); err != nil {
		fmt.Println("posted? should check the response")
		return
	}

	deleteRequest := http.NewRequest("DELETE", serverURL, nil)
	deleteRequest.Header.Add("X-Url-Delete", deleteURL)
	if resp, err := client.Do(deleteRequest); err != nil {
		fmt.Println("deleted? should check the result")
		return
	}
}
