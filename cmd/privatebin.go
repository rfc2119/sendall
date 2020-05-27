package cmd

import (
	"bytes"
	"compress/zlib" // TODO: add zlib support
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"golang.org/x/crypto/pbkdf2"
	"io"
	"io/ioutil"
	"net/http"

	"github.com/boltdb/bolt"
	"github.com/btcsuite/btcutil/base58"
	"github.com/spf13/cobra"
)

const (
	nonceSizeBytes  = 16 // privatebin uses a nonce of 16 bytes by default
	aesKeySizeBytes = 32 // using aes-256-gcm; for reference only
	gcmTagSize      = 16 // for reference
	kdfSaltSize     = 8  // for reference
	kdfIterations   = 100000
)

// Array1 : not used directly in the paste request
type Array1 struct { // TODO: more descriptive name
	Nonce           []byte // base64(cipher_iv); getRandomBytes(16) default
	Kdfsalt         []byte // base64(kdf_salt); getRandomBytes(8) default
	KdfIterations   int    // pbkdf_iterations; default
	KdfKeySize      int    // pbkdf_keysize; default
	CipherTagSize   int    // cipher_tag_size; default
	CipherAlgo      string // cipher_algo; default
	CipherMode      string // cipher_mode; default
	CompressionType string // compression_type; default
}

// AuthData : Format is the paste's format.
type AuthData struct {
	EncryptionDetails []interface{}
	Format            string // format of the paste
	OpenDiscussion    int    // open-discussion flag
	BurnAfterReading  int    // burn-after-reading flag
}

// PasteData : !shrug (see https://github.com/PrivateBin/PrivateBin/wiki/Encryption-format#data-passed-in)
type PasteData struct {
	Paste           string        `json:"paste"` // ciphertext (encrypted zlib'd plaintext)
	Attachment      string        `json:"attachment"`
	AttachementName string        `json:"attachment_name"`
	Children        []interface{} `json:"children"`
}

// PasteMeta : https://raw.githubusercontent.com/PrivateBin/PrivateBin/master/js/types.jsonld
type PasteMeta struct {
	Expire string `json:"expire"` // ["5min", "10min", "1hour", "1day", "1week", "1month", "1year", "never"]
}

// PasteResponse : A request's response, parsed
type PasteResponse struct {
	Status      int    `json:"status"`
	Id          string `json:"id"`
	Url         string `json:"url"`
	Deletetoken string `json:"deletetoken"`
}

// PasteRequest : A paste request (TODO: keep struct local and apply NewRequest as a method)
type PasteRequest struct {
	AuthData   []interface{} `json:"adata"`
	Meta       PasteMeta     `json:"meta"`
	Version    int           `json:"v"`
	CipherText []byte        `json:"ct"`
}

// NewRequest : Forges a new request to be posted.
func NewRequest(aData []interface{}, cipherText []byte, expiryDate string) *PasteRequest {

	var (
		req PasteRequest
	)

	meta := PasteMeta{expiryDate}

	req.AuthData = aData
	req.Meta = meta
	req.Version = 2 // constant; defined by private bin API
	req.CipherText = cipherText
	return &req

}

var (
	// ====== default values for private bin
	pbinGlobal = privateBin{ // service private bin
		hostUrl:          "https://bin.fraq.io",
		maxDays:          "1week",
		format:           "plaintext",
		openDiscussion:   0,
		burnAfterReading: 0,
		httpClient:       &http.Client{},
		dbName:           "sendall.db", // bolt db name
		dbBucketName:     "privateBin", // bucket used within bolt; contains the posted urls -> deleted urls
		debug: true,
	}

	privateBinCmd = &cobra.Command{
		Use:   "privatebin",
		Short: "use privatebin to post your text files safely",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			chanHttpResponses := make(chan *http.Response, len(pbinGlobal.filePaths))
			chanExtraStrings := make(chan []string, 0) // we won't be sending anything for this service

	// files are provided straight from the cmd interface; tidy them up
	pbinGlobal.filePaths = prepareFiles(args)
			go func() {
				if err := pbinGlobal.Post(chanHttpResponses, chanExtraStrings); err != nil {
					fmt.Println(err)
				}
			}()
			if err := pbinGlobal.SaveUrl(chanHttpResponses, chanExtraStrings); err != nil {
				fmt.Println(err)
			}
		},
	}

	privateBinDeleteCmd = &cobra.Command{
		Use:   "delete",
		Short: "delete a link posted before",
		Args:  cobra.MinimumNArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
	pbinGlobal.filePaths = args

			if err := pbinGlobal.Delete(); err != nil {
				fmt.Println(err)
			}
		},
	}
)

func init() {
	// privateBinCmd.Flags().IntVarP(&maxDownloads, "downloads", "e", maxDownloads, "Maximum number of downloads after which the link will expire")
	privateBinCmd.Flags().StringVarP(&pbinGlobal.maxDays, "days", "d", pbinGlobal.maxDays, "Maximum number of days after which the file will be removed from the server"+
		"\nvalues:  [5min, 10min, 1hour, 1day, 1week, 1month, 1year, never]")
	privateBinCmd.Flags().StringVarP(&pbinGlobal.hostUrl, "host", "u", pbinGlobal.hostUrl, "service URL, for example if you host your own instance")
	privateBinCmd.Flags().StringVarP(&pbinGlobal.format, "format", "f", pbinGlobal.format, "format of the paste; values: [markdown, plaintext]")

	privateBinCmd.Flags().IntVarP(&pbinGlobal.openDiscussion, "open-discussion", "o", pbinGlobal.openDiscussion, "opens paste for discussion (paste comments are not supported atm)") // TODO: support paste comments
	privateBinCmd.Flags().IntVarP(&pbinGlobal.burnAfterReading, "burn-after-reading", "b", pbinGlobal.burnAfterReading, "invalidates paste after one access")
	privateBinCmd.AddCommand(privateBinDeleteCmd)
	rootCmd.AddCommand(privateBinCmd)
}

type privateBin struct {
	// cmd options
	hostUrl, maxDays, format         string
	openDiscussion, burnAfterReading int

	// mandatory struct memebers
	httpClient           *http.Client
	dbName, dbBucketName string
	filePaths            []string

	// other
	debug	bool
}

func (pbinReciever *privateBin) Delete() error {

	// TODO: refactor into a common method
	var (
		db        *bolt.DB
		err       error
		deleteUrl []byte
		// req       *http.Request
		resp      *http.Response
		bucket    *bolt.Bucket
	)

	if db, err = bolt.Open(pbinReciever.dbName, 0600, nil); err != nil {
		fmt.Println("could not open db")
		return err
	}
	defer db.Close()
	for _, file := range pbinReciever.filePaths { // files provided should be the exact received url
		db.View(func(tx *bolt.Tx) error {
			bucket := tx.Bucket([]byte(pbinReciever.dbBucketName))
			answer := bucket.Get([]byte(file))
			deleteUrl = make([]byte, len(answer))
			copy(deleteUrl, answer)

			return nil
		})
		if len(deleteUrl) == 0 {
			fmt.Printf("link %s does not have an entry in db", file)
			continue
		}
		if resp, err = http.Get(string(deleteUrl)); err != nil { // TODO: find out if Client.Do() does it in a goroutine
			fmt.Printf("issuing request failed: %s", err)
			continue
		}
		defer resp.Body.Close()
		body, _ := ioutil.ReadAll(resp.Body)
		fmt.Println(string(body))	// TODO: what is the response for a delete request ?
		// TODO: assume here we got a 200 response code
		err = db.Update(func(tx *bolt.Tx) error {

			bucket = tx.Bucket([]byte(pbinReciever.dbBucketName))
			err = bucket.Delete([]byte(file)) // we're sure that the key does exist, right ?
			return err

		})
		if err != nil {
			fmt.Printf("error deleting link %s from db", file)
			return err
		}
	}
	fmt.Println("done")
	return nil
}

func (pbinReciever *privateBin) SaveUrl(receivedHttpResponses <-chan *http.Response, extra <-chan []string) error {

	var (
		parsedResponse PasteResponse
		err            error
	)
	for resp := range receivedHttpResponses {
		defer resp.Body.Close()
		// body, err := ioutil.ReadAll(r.Body)
		// fmt.Printf("received body: %s, err %s\n", body, err)
		// err = json.Unmarshal(body, &parsedResponse)
		if err = json.NewDecoder(resp.Body).Decode(&parsedResponse); err != nil {
			fmt.Println("json decoding error") // TODO: use logging
		}
		//if pasteResp, err = recvPaste(receivedHttpResponses); err != nil {
		//	fmt.Println("could not receive paste")
		//	return err
		//}
		extraInfo := <-extra // [0] is paste's private key
		key := []byte(extraInfo[0])

		if parsedResponse.Status != 0 {
			fmt.Println("This is impossible to see unless the same key AND message were used")
			return err
		}
		url := fmt.Sprintf("%s%s#%s", pbinReciever.hostUrl, parsedResponse.Url, base58.Encode(key))
		deleteUrl := fmt.Sprintf("%s/?pasteid=%s&deletetoken=%s", pbinReciever.hostUrl, parsedResponse.Id, parsedResponse.Deletetoken)
		fmt.Printf("response: %v\n", parsedResponse)             // TODO: use logging
		fmt.Printf("url: %s\n delete url: %s\n", url, deleteUrl) // TODO: use logging

		// save into db (TODO: refactor)

		var (
			// body   []byte
			db     *bolt.DB
			bucket *bolt.Bucket
			err    error
		)
		if db, err = bolt.Open(pbinReciever.dbName, 0600, nil); err != nil {
			fmt.Println("could not open db")
			return err
		}
		defer db.Close()
		err = db.Update(func(tx *bolt.Tx) error {
			_, err = tx.CreateBucketIfNotExists([]byte(pbinReciever.dbBucketName))
			return err
		})

		if err != nil {
			fmt.Println("create bucket error")
			return err
		}
		// for resp := range c {

		//defer resp.Body.Close()
		//if body, err = ioutil.ReadAll(resp.Body); err != nil {
		//	fmt.Printf("failed to read body: %s", err) // body is new url returned by server
		//	continue
		//}
		//if printOnly {
		//	fmt.Printf("url: %s\n delete url: %s\n", url, deleteUrl) // TODO: use logging
		//	continue
		//}

		err = db.Update(func(tx *bolt.Tx) error {
			bucket = tx.Bucket([]byte(pbinReciever.dbBucketName))
			err := bucket.Put([]byte(url), []byte(deleteUrl))
			return err
		})
		if err != nil {
			fmt.Printf("error on writing %s: %s", deleteUrl, err)
			continue
		}
		fmt.Printf("wrote %s\n", deleteUrl)

	}
	return nil
}

func (pbinReciever *privateBin) Post(receivedHttpResponses chan *http.Response, extra chan []string) error {
	// TODO: code needs to be concurrent

	var (
		// pasteResp PasteResponse
		pasteReq  *PasteRequest
		err       error
		plaintext []byte
	)

	fmt.Println(pbinReciever.filePaths)
	for i := 0; i < len(pbinReciever.filePaths); i++ {
		if plaintext, err = ioutil.ReadFile(pbinReciever.filePaths[i]); err != nil {
			fmt.Println("read file error")
			return err
		}
		key, nonce, kdfsalt := generateEncryptionParameters()
		adata := generateAuthenticationData(nonce, kdfsalt, pbinReciever.format, pbinReciever.openDiscussion, pbinReciever.burnAfterReading)
		aesKey := pbkdf2.Key(key, kdfsalt, kdfIterations, aesKeySizeBytes, sha256.New)
		ciphertext := encrypt(plaintext, aesKey, nonce, adata) // auth tag is appended to ciphertext
		pasteReq = NewRequest(adata, ciphertext, pbinReciever.maxDays)
		go func() error { // TODO: how to return the error to main routine and check it?
			if err = recvPaste(receivedHttpResponses, pasteReq); err != nil { // will forge a new request, send it and forward the response back to channel
				fmt.Println(err)
				return err
			}
			return nil
		}()
		extra <- []string{string(key)} // we neeed the key to construct the url and save the url into db

	}
	close(receivedHttpResponses)
	close(extra)
	return nil
}

func generateAuthenticationData(iv []byte, dummyKDFsalt []byte, format string, openDiscussion int, burnAfterReading int) []interface{} {
	// encryptionInfo := Array1{iv, dummyKDFsalt, 10000, 265, 128, "aes", "gcm", "zlib"}
	// encryptionInfo := make([]interface{}, 0)
	// aData := make([]interface{}, 0); then append
	var (
		encryptionInfo, aData []interface{}
	)
	encryptionInfo = append(encryptionInfo, iv, dummyKDFsalt, kdfIterations, aesKeySizeBytes*8, nonceSizeBytes*8, "aes", "gcm", "none") // TODO: get back zlib support (replace none with zlib)
	aData = append(aData, encryptionInfo, format, openDiscussion, burnAfterReading)
	return aData
}

func generateEncryptionParameters() (key, iv, kdfSalt []byte) {

	// since we'll be using a different random key for each paste,
	// a fixed nonce should be OK (but we won't do it anyway)
	totalSize := aesKeySizeBytes + nonceSizeBytes + kdfSaltSize
	keyWithNonceAndKdfSalt := make([]byte, totalSize)
	if _, err := io.ReadFull(rand.Reader, keyWithNonceAndKdfSalt); err != nil {
		panic(err.Error())
	}

	key = keyWithNonceAndKdfSalt[:aesKeySizeBytes]
	iv = keyWithNonceAndKdfSalt[aesKeySizeBytes : aesKeySizeBytes+nonceSizeBytes]
	kdfSalt = keyWithNonceAndKdfSalt[totalSize-kdfSaltSize:]

	return key, iv, kdfSalt

}

func recvPaste(chanHttpResponses chan<- *http.Response, pasteReq *PasteRequest) error {
	// marshals data, sends a new request and then send the received response to channel
	var (
		pasteReqJson []byte
		req          *http.Request
		// resp        *http.Response
		err error
	)

	if pasteReqJson, err = json.Marshal(pasteReq); err != nil { // Marshal, not NewEncoder
		fmt.Println("unable to marshal req")
		return err
	}
	// fmt.Printf("marhsalled json: %s\n", pasteReqJson)
	// ==== cert ==== //
	//  self-signed certificates workaround (https://groups.google.com/d/msg/golang-nuts/v5ShM8R7Tdc/I2wyTy1o118J)
	// ==== end cert ///
	if req, err = http.NewRequest("POST", pbinGlobal.hostUrl, bytes.NewReader(pasteReqJson)); err != nil {
		fmt.Println("failed to generate a request")

		return err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Add("X-Requested-With", "JSONHttpRequest") // reason we used http.NewRequest w/ Client.Do()

	// debugging sent request as seen in a proxy; remmeber that the request is consumed if you used WriteProxy, so you can't re-use it later
	// url, err := http.ProxyFromEnvironment(req)
	// fmt.Printf("proxy: %s %s\n", url, err)
	// req.WriteProxy(os.Stdout)

	if resp, err := pbinGlobal.httpClient.Do(req); err != nil {
		fmt.Println("post to paste site error") // TODO: use log
		return err
	} else {
		chanHttpResponses <- resp
		return nil
	}

}

func encrypt(plaintext, key, iv []byte, authenticationData []interface{}) (ciphertext []byte) {
	// compresses the message with zlib and encrypts it with a random key

	block, err := aes.NewCipher(key) // will auto-pick aes-256 because of key size
	if err != nil {
		panic(err.Error())
	}

	aesgcm, err := cipher.NewGCMWithNonceSize(block, nonceSizeBytes) //TODO: should instruct privatebin to use standard nonce size instead
	if err != nil {
		panic(err.Error())
	}

	// compress and encrypt message, then encode key and return
	var (
		compressedCiphertext bytes.Buffer
		// pasteData       PasteData
		// encodedCompressedPlaintext bytes.Buffer
		cipherJson, authenticatedDataJson []byte
	)
	// pasteData = PasteData{Paste: string(plaintext)}      // TODO: support file attachement and paste linking
	pasteData := struct {
		Paste string `json:"paste"`
	}{
		string(plaintext),
	}
	if cipherJson, err = json.Marshal(pasteData); err != nil { // Marshal, not NewEncoder
		panic(err.Error())
	}
	if authenticatedDataJson, err = json.Marshal(authenticationData); err != nil { // Marshal, not NewEncoder
		panic(err.Error())
	}
	// fmt.Printf("marshalled cipher: %s\n", cipherJson) // TODO: output this on debug flag
	// fmt.Printf("marshalled adata: %s\n", authenticatedDataJson) // TODO: output this on debug flag

	// TODO: add check for compression support (for now, assuming defaults); get back zlib support!
	compressedCiphertextWriter := zlib.NewWriter(&compressedCiphertext)
	compressedCiphertextWriter.Write(cipherJson)
	compressedCiphertextWriter.Close()
	// encoder := base64.NewEncoder(base64.StdEncoding, &encodedCompressedPlaintext)
	// encoder.Write(compressedCiphertext.Bytes())
	// encoder.Close()

	// authData is authenticated as well(https://github.com/r4sas/PBinCLI/blob/682b47fbd3e24a8a53c3b484ba896a5dbc85cda2/pbincli/format.py#L122)
	// kudos to filo for hinting about the tag location (https://github.com/golang/go/issues/32742)
	// look for function " decryptOrPromptPassword" in privatebin.js; start debugging there
	// TODO: fully support the API (https://github.com/PrivateBin/PrivateBin/wiki/API)
	ciphertext = aesgcm.Seal(nil, iv, cipherJson, authenticatedDataJson) // TODO: zzzzlib
	// 	encodedNonce := base64.StdEncoding.EncodeToString(nonce)
	// 	encodedCipherText := base64.StdEncoding.EncodeToString(ciphertext)
	// fmt.Printf("pt: %s\n key: %s\n", plaintext, base58.Encode(key)) // TODO: output this on debug flag
	return ciphertext
}
