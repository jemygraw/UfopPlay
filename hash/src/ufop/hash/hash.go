package hash

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"hash"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"ufop"
)

type Hasher struct {
	outputFormat string
}

// OutputFormat supports json, string, xml, default is string
// Support PersistentOps and Pipeline
type HasherConfig struct {
	OutputFormat string `json:"output_format"`
}

func (this *Hasher) Name() string {
	return "hash"
}

func (this *Hasher) InitConfig(jobConf string) (err error) {
	confFp, openErr := os.Open(jobConf)
	if openErr != nil {
		err = fmt.Errorf("open hash config failed, %s", openErr.Error())
		return
	}

	config := HasherConfig{}
	decoder := json.NewDecoder(confFp)
	decodeErr := decoder.Decode(&config)
	if decodeErr != nil {
		err = fmt.Errorf("parse hash config failed, %s", decodeErr.Error())
		return
	}

	this.outputFormat = config.OutputFormat

	return
}

func (this *Hasher) parse(cmd string) (hashType string, err error) {
	pattern := `hash/(md5|sha1)`
	matched, _ := regexp.MatchString(pattern, cmd)
	if !matched {
		err = errors.New("invalid hash command format")
		return
	}

	items := strings.Split(cmd, "/")
	hashType = items[1]

	return

}

func (this *Hasher) Do(req ufop.UfopRequest, reqBody io.ReadCloser) (result interface{}, resultType int, contentType string, err error) {
	defer reqBody.Close()
	hashType, pErr := this.parse(req.Cmd)
	if pErr != nil {
		err = pErr
		return
	}

	var h hash.Hash
	if hashType == "md5" {
		h = md5.New()
	} else {
		h = sha1.New()
	}

	var hashResult string

	if req.Url != "" {
		//check url
		respBody, respErr := http.Get(req.Url)
		if respErr != nil {
			err = fmt.Errorf("get source content error, %s", respErr.Error())
			return
		}
		defer respBody.Body.Close()

		_, cpErr := io.Copy(h, respBody.Body)
		if cpErr != nil {
			err = fmt.Errorf("read source content error, %s", cpErr)
		}

		hashResult = hex.EncodeToString(h.Sum(nil))
	} else {
		//check reqBody
		_, cpErr := io.Copy(h, reqBody)
		if cpErr != nil {
			err = fmt.Errorf("read source content error, %s", cpErr)
		}

		hashResult = hex.EncodeToString(h.Sum(nil))
	}

	if this.outputFormat == "json" {
		if hashType == "md5" {
			result = struct {
				Md5 string `json:"md5"`
			}{
				Md5: hashResult,
			}
		} else {
			result = struct {
				Sha1 string `json:"sha1"`
			}{
				Sha1: hashResult,
			}
		}

		resultType = ufop.RESULT_TYPE_JSON
		contentType = ufop.CONTENT_TYPE_JSON
	} else if this.outputFormat == "xml" {
		if hashType == "md5" {
			result = struct {
				XMLName xml.Name `xml:"hash"`
				Md5     string   `xml:"md5"`
			}{
				Md5: hashResult,
			}
		} else {
			result = struct {
				XMLName xml.Name `xml:"hash"`
				Sha1    string   `xml:"sha1"`
			}{
				Sha1: hashResult,
			}
		}

		resultType = ufop.RESULT_TYPE_XML
		contentType = ufop.CONTENT_TYPE_XML
	} else {
		if hashType == "md5" {
			hashResult = "md5=" + hashResult
		} else {
			hashResult = "sha1=" + hashResult
		}

		result = []byte(hashResult)
		resultType = ufop.RESULT_TYPE_OCTET_BYTES
		contentType = ufop.CONTENT_TYPE_STRING
	}

	return
}
