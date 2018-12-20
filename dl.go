package megadl

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strconv"
	"strings"
)

var API_ENDPOINT = "https://g.api.mega.co.nz"

var errInvalidURL = fmt.Errorf("invalid url")
var errAPI = fmt.Errorf("api error")

var seqno = 0

type handleReq struct {
	A      string `json:"a"`
	G      int    `json:"g"`
	Handle string `json:"p"`
}

type handleResp struct {
	Size       int    `json:"s"`
	Attributes string `json:"at"`
	Msd        int    `json:"msd"`
	URL        string `json:"g"`
}

// Info contains the necessary information to download a file from mega.
type Info struct {
	Name     string
	Size     int
	URL      string
	AesKey   []byte
	AesIV    []byte
	Progress chan int // Track the download progess (total downloaded bytes)
}

func base64urldecode(data string) ([]byte, error) {
	return base64.URLEncoding.WithPadding(base64.NoPadding).DecodeString(data)
}

func apiReq(request interface{}, response interface{}) error {
	// put request into an array
	var commands []interface{}
	commands = append(commands, request)

	// convert to json
	cj, err := json.Marshal(commands)
	if err != nil {
		return err
	}

	// make a post request
	url := fmt.Sprintf("%s/cs?id=%d", API_ENDPOINT, seqno)
	seqno++

	resp, err := http.Post(url, "application/json", bytes.NewBuffer(cj))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// read response
	bodyBytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// remove "[" and "]"
	bodyBytes = bodyBytes[1 : len(bodyBytes)-1]

	// convert json to struct
	err = json.Unmarshal(bodyBytes, response)
	if err != nil {
		if _, err := strconv.ParseInt(string(bodyBytes), 10, 64); err == nil {
			return errAPI
		}
		return err
	}

	return nil
}

func parseURL(url string) (handle, key string, err error) {
	split := strings.SplitN(url, "!", 3)
	if len(split) != 3 {
		err = errInvalidURL
		return
	}

	if split[0] != "https://mega.nz/#" {
		err = errInvalidURL
		return
	}

	handle, key = split[1], split[2]
	if len(handle) != 8 || len(key) != 43 {
		err = errInvalidURL
		return
	}

	return
}

func unpackKey(key string) (aesKey, aesIV, mac []byte, err error) {
	decodedKey, err := base64urldecode(key)
	if err != nil {
		return
	}

	aesKey = []byte{}
	for i := 0; i < 16; i++ {
		aesKey = append(aesKey, decodedKey[i]^decodedKey[i+16])
	}
	aesIV = append(decodedKey[16:24][:], 0, 0, 0, 0, 0, 0, 0, 0)
	mac = decodedKey[24:32][:]

	return
}

func getInfo(url string) (*Info, error) {
	handle, key, err := parseURL(url)
	if err != nil {
		return nil, err
	}

	// request file information
	resp := handleResp{}
	err = apiReq(&handleReq{
		A:      "g",
		G:      1,
		Handle: handle,
	}, &resp)
	if err != nil {
		return nil, err
	}

	attr, err := base64urldecode(resp.Attributes)
	if err != nil {
		return nil, err
	}

	aesKey, aesIV, mac, err := unpackKey(key)
	if err != nil {
		return nil, err
	}
	_ = mac

	// decrypt Attributes
	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, err
	}
	mode := cipher.NewCBCDecrypter(block, []byte{0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})

	dst := make([]byte, len(attr))
	mode.CryptBlocks(dst, attr)

	mega := []byte("MEGA")
	if !bytes.HasPrefix(dst, mega) {
		return nil, errAPI
	}
	dst = bytes.TrimPrefix(dst, mega)
	dst = bytes.Trim(dst, "\x00")

	// convert json Attributes to struct
	n := struct {
		FileName string `json:"n"`
	}{}
	err = json.Unmarshal(dst, &n)
	if err != nil {
		return nil, err
	}

	// return with the collected information
	return &Info{
		Name:     n.FileName,
		Size:     resp.Size,
		URL:      resp.URL,
		AesKey:   aesKey,
		AesIV:    aesIV,
		Progress: make(chan int, 10),
	}, nil
}

type fileReadCloser struct {
	Reader   *cipher.StreamReader
	Response *http.Response
	FileInfo *Info
	Total    int
}

func (f *fileReadCloser) Close() error {
	return f.Response.Body.Close()
}

func (f *fileReadCloser) Read(p []byte) (n int, err error) {
	n, err = f.Reader.Read(p)
	f.Total += n
	select {
	case f.FileInfo.Progress <- f.Total:
	default:
	}
	return
}

// Download file from mega and report file information
func Download(url string) (io.ReadCloser, *Info, error) {
	info, err := getInfo(url)
	if err != nil {
		return nil, nil, err
	}

	resp, err := http.Get(info.URL)
	if err != nil {
		return nil, nil, err
	}

	block, err := aes.NewCipher(info.AesKey)
	if err != nil {
		return nil, nil, err
	}

	stream := cipher.NewCTR(block, info.AesIV)
	reader := &cipher.StreamReader{S: stream, R: resp.Body}

	return &fileReadCloser{
		Reader:   reader,
		Response: resp,
		FileInfo: info,
	}, info, nil
}
