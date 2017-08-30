package iptc

/*
#cgo CFLAGS: -I /usr/local/Cellar/libiptcdata/1.0.4/include
#cgo LDFLAGS: -L /usr/local/Cellar/libiptcdata/1.0.4/lib -liptcdata
#include <libiptcdata/iptc-data.h>
#include <libiptcdata/iptc-jpeg.h>
#include <stdlib.h>

IptcDataSet *get_iptc_dataset(IptcData *iptcData, unsigned int);

IptcDataSet *get_iptc_dataset(IptcData *iptcData, unsigned int i) {
	return iptcData->datasets[i];
}
*/
import "C"

import (
	"errors"
	"fmt"
	"github.com/qiniu/log"
	"io"
	//"net/http"
	"encoding/base64"
	"encoding/json"
	"image/jpeg"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"ufop"
	"ufop/utils"
	"unsafe"
)

type IptcManager struct {
}

type IptcConfig struct {
}

func (this *IptcManager) Name() string {
	return "iptc"
}

func (this *IptcManager) InitConfig(jobConf string) (err error) {
	return
}

/*

iptc/view
iptc/set/urlsafe_base64_encode(ip_info_json_str)

对于set命令的参数，采用JSON的方式来传递，这样方便未来的扩展，目前支持的就是 IptcInfo 里面的几个字段的修改。

*/
func (this *IptcManager) parse(cmd string) (iptcCmd string, iptcParam string, err error) {
	pattern := `^iptc/(view|set/[0-9a-zA-Z-_=]+)$`
	matched, _ := regexp.MatchString(pattern, cmd)
	if !matched {
		err = errors.New("invalid iptc command format")
		return
	}

	cmdItems := strings.Split(cmd, "/")
	iptcCmd = cmdItems[1]
	switch iptcCmd {
	case "set":
		iptcParam = cmdItems[2]
	}
	return
}

/**
使用CGO的方式调用libiptcdata库的方法
*/
func (this *IptcManager) Do(req ufop.UfopRequest, ufopBody io.ReadCloser) (result interface{}, resultType int, contentType string, err error) {
	reqId := req.ReqId
	iptcCmd, iptcParam, pErr := this.parse(req.Cmd)
	if pErr != nil {
		err = pErr
		return
	}

	log.Infof("[%s] image iptc cmd `%s` with param `%s`", reqId, iptcCmd, iptcParam)
	imageURL := req.Url
	imageFile := filepath.Join(os.TempDir(), "src_"+utils.Md5Hex(fmt.Sprintf("%s%d", imageURL, time.Now().UnixNano())))
	defer os.Remove(imageFile)

	//download image
	//
	// resp, respErr := http.Get(imageURL)
	// if respErr != nil {
	// 	err = fmt.Errorf("get image failed: %s", respErr.Error())
	// 	return
	// }
	// //read file into memory
	// imageData, readErr := ioutil.ReadAll(resp.Body)
	// resp.Body.Close()
	// if readErr != nil {
	// 	err = fmt.Errorf("get image failed: %s", readErr.Error())
	// 	return
	// }

	//check mimetype

	imageFile = "/Users/jemy/XLab/iptc/iptc-base.jpg"
	if iptcCmd == "view" {
		return this.getIptcInfo(reqId, imageFile)
	} else {
		var iptcReq IptcReq
		iptcReqJson, decodeErr := base64.URLEncoding.DecodeString(iptcParam)
		if decodeErr != nil {
			err = fmt.Errorf("invalid iptc set param, %s", decodeErr)
			return
		}

		decodeErr = json.Unmarshal(iptcReqJson, &iptcReq)
		if decodeErr != nil {
			err = fmt.Errorf("invalid iptc set param, %s", decodeErr)
			return
		}
		return this.setIptcInfo(reqId, imageFile, iptcReq)
	}

	return
}

func (this *IptcManager) getIptcInfo(reqId, imageFile string) (result interface{}, resultType int, contentType string, err error) {
	//get image iptc attribute values
	cgoImageFile := C.CString(imageFile)
	cgoImageIptcData := C.iptc_data_new_from_jpeg(cgoImageFile)
	defer C.iptc_data_unref(cgoImageIptcData)

	log.Infof("[%s] image iptc data has %d attributes", reqId, int(cgoImageIptcData.count))
	//City, ObjectName, Keywords, OriginatingProgram, DateCreated, TimeCreated
	var iptcInfo IptcInfo
	keywords := make([]string, 0, 100)

	for i := C.uint(0); i < cgoImageIptcData.count; i++ {
		dataSet := C.get_iptc_dataset(cgoImageIptcData, i)
		//check name
		attrName := C.GoString(dataSet.info.name)
		attrValue := C.GoString((*C.char)(unsafe.Pointer(dataSet.data)))

		switch attrName {
		case "City":
			iptcInfo.City = attrValue
		case "ObjectName":
			iptcInfo.ObjectName = attrValue
		case "Keywords":
			keywords = append(keywords, attrValue)
		case "OriginatingProgram":
			iptcInfo.OriginatingProgram = attrValue
		case "DateCreated":
			iptcInfo.DateCreated = attrValue
		case "TimeCreated":
			iptcInfo.TimeCreated = attrValue
		}
	}

	iptcInfo.Keywords = keywords

	//get image width & height
	imageReader, readErr := os.Open(imageFile)
	if readErr != nil {
		err = fmt.Errorf("read image local file error, %s", readErr.Error())
		return
	}
	defer imageReader.Close()

	imgObj, decodeErr := jpeg.Decode(imageReader)
	if decodeErr != nil {
		err = fmt.Errorf("src image not valid jpeg error, %s", decodeErr)
		return
	}

	imgWidth := imgObj.Bounds().Max.X - imgObj.Bounds().Min.X
	imgHeight := imgObj.Bounds().Max.Y - imgObj.Bounds().Min.Y

	//set response
	iptcResp := IptcResp{
		Width:  imgWidth,
		Height: imgHeight,
		IPTC:   iptcInfo,
	}

	log.Infof("[%s] image iptc resp: %s", reqId, iptcResp.ToJsonString())
	result = iptcResp
	resultType = ufop.RESULT_TYPE_JSON
	contentType = "application/json"

	return
}

func (this *IptcManager) setIptcInfo(reqId, imageFile string, iptcReq IptcReq) (result interface{}, resultType int, contentType string, err error) {
	//City, ObjectName, Keywords, OriginatingProgram

	return
}
