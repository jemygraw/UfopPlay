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

//return 0 on success, otherwise failure
int save_iptc_info_to_jpeg_file(IptcData *iptcData, const char *inputFilePath, const char *outputFilePath);

int save_iptc_info_to_jpeg_file(IptcData *iptcData, const char *inputFilePath, const char *outputFilePath) {
    unsigned char *iptcBuf = NULL;
    unsigned int iptcBufLen;
    unsigned char ps3Buf[256 * 256];
    int ps3Len;
    int wLen;

    unsigned char outBuf[255 * 255];
    FILE *inputFile, *outputFile;

    iptc_data_sort(iptcData);

    inputFile = fopen(inputFilePath, "r");
    if (!inputFile) {
        fprintf(stderr, "failed to open src image file\n");
        return -1;
    }

    ps3Len = iptc_jpeg_read_ps3(inputFile, ps3Buf, sizeof(ps3Buf));
    fclose(inputFile);
    if (ps3Len < 0) {
        fprintf(stderr, "parse jpeg image file error");
        return -1;
    }

    if (iptc_data_save(iptcData, &iptcBuf, &iptcBufLen) < 0) {
        fprintf(stderr, "failed to generate IPTC bytestream\n");
        return -1;
    }

    ps3Len = iptc_jpeg_ps3_save_iptc(ps3Buf, (unsigned int) ps3Len, iptcBuf, iptcBufLen, outBuf, sizeof(outBuf));
    iptc_data_free_buf(iptcData, iptcBuf);

    inputFile = fopen(inputFilePath, "r");
    if (!inputFile) {
        fprintf(stderr, "failed to open src image file\n");
        return -1;
    }

    outputFile = fopen(outputFilePath, "w");
    if (!outputFile) {
        fprintf(stderr, "failed to open output image file\n");
        return -1;
    }

    wLen = iptc_jpeg_save_with_ps3(inputFile, outputFile, outBuf, (unsigned int) ps3Len);
    fclose(outputFile);
    if (wLen < 0) {
        fprintf(stderr, "failed to wrtie iptc info into image file\n");
        return -1;
    }

    return 0;
}

*/
import "C"

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"ufop"
	"ufop/utils"
	"unsafe"

	"github.com/jemygraw/base/container/set"
	"github.com/qiniu/log"
)

type IptcManager struct {
}

type IptcConfig struct {
}

func (m *IptcManager) Name() string {
	return "iptc"
}

func (m *IptcManager) InitConfig(jobConf string) (err error) {
	return
}

/*

iptc/view
iptc/set/urlsafe_base64_encode(ip_info_json_str)

对于set命令的参数，采用JSON的方式来传递，这样方便未来的扩展，目前支持的就是 IptcInfo 里面的几个字段的修改。

*/
func (m *IptcManager) parse(cmd string) (iptcCmd string, iptcParam string, err error) {
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
func (m *IptcManager) Do(req ufop.UfopRequest, ufopBody io.ReadCloser) (result interface{}, resultType int, contentType string, err error) {
	reqId := req.ReqId
	iptcCmd, iptcParam, pErr := m.parse(req.Cmd)
	if pErr != nil {
		err = pErr
		return
	}

	log.Infof("[%s] image iptc cmd `%s` with param `%s`", reqId, iptcCmd, iptcParam)
	imageURL := req.Url
	jobID := utils.Md5Hex(fmt.Sprintf("%s%d", imageURL, time.Now().UnixNano()))
	imageFile := filepath.Join(os.TempDir(), "src_"+jobID)
	defer os.Remove(imageFile)

	//download image
	resp, respErr := http.Get(imageURL)
	if respErr != nil {
		err = fmt.Errorf("get image failed: %s", respErr.Error())
		return
	}
	//check mimetype
	//reqMime := resp.Header.Get("Content-Type")
	reqMime := req.MimeType
	log.Infof("[%s] Content-Type: %s", reqId, reqMime)
	if reqMime != "image/jpeg" && reqMime != "image/jpg" {
		err = fmt.Errorf("unsupported image file with mimetype %s", reqMime)
		//close boy
		resp.Body.Close()
		return
	}

	//write file to local disk
	writeFp, openErr := os.Create(imageFile)
	if openErr != nil {
		err = fmt.Errorf("open local image file error, %s", openErr.Error())
		return
	}
	_, cpErr := io.Copy(writeFp, resp.Body)
	resp.Body.Close()
	if cpErr != nil {
		err = fmt.Errorf("save local image file error, %s", cpErr.Error())
		writeFp.Close()
		return
	}
	writeFp.Close()

	if iptcCmd == "view" {
		return m.getIptcInfo(reqId, imageFile)
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

		//do not delete by defer
		outputFile := filepath.Join(os.TempDir(), "dest_"+jobID)
		return m.setIptcInfo(reqId, imageFile, iptcReq, outputFile)
	}
}

func (m *IptcManager) getIptcInfo(reqId, imageFile string) (result interface{}, resultType int, contentType string, err error) {
	//get image iptc attribute values
	cgoImageFile := C.CString(imageFile)
	cgoImageIptcData := C.iptc_data_new_from_jpeg(cgoImageFile)
	if cgoImageIptcData == nil {
		err = errors.New("no image iptc found")
		return
	}

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
	contentType = ufop.CONTENT_TYPE_JSON

	return
}

func (m *IptcManager) setIptcInfo(reqId, imageFile string, iptcReq IptcReq, outputFile string) (result interface{},
	resultType int, contentType string, err error) {
	//City, ObjectName, Keywords, OriginatingProgram
	//get image iptc attribute values
	cgoImageFile := C.CString(imageFile)
	defer C.free(unsafe.Pointer(cgoImageFile))
	cgoImageIptcData := C.iptc_data_new_from_jpeg(cgoImageFile)
	if cgoImageIptcData == nil {
		//new iptc info found image
		cgoImageIptcData = C.iptc_data_new()
	}
	log.Infof("[%s] image iptc data has %d attributes", reqId, int(cgoImageIptcData.count))
	//set encoding
	C.iptc_data_set_encoding_utf8(cgoImageIptcData)

	var success C.int
	//edit iptc image info
	//city
	if iptcReq.City != "" {
		cgoCityDataset := C.iptc_data_get_dataset(cgoImageIptcData, C.IPTC_RECORD_APP_2, C.IPTC_TAG_CITY)
		if cgoCityDataset == nil {
			cgoCityDataset := C.iptc_dataset_new()
			defer C.iptc_dataset_free(cgoCityDataset)

			C.iptc_dataset_set_tag(cgoCityDataset, C.IPTC_RECORD_APP_2, C.IPTC_TAG_CITY)
			success = C.iptc_dataset_set_data(cgoCityDataset, (*C.uchar)(unsafe.Pointer(C.CString(iptcReq.City))),
				C.uint(len(iptcReq.City)), C.IPTC_VALIDATE)
			if success <= 0 {
				err = errors.New("add attribute City failed")
				return
			}
			success = C.iptc_data_add_dataset(cgoImageIptcData, cgoCityDataset)
			if success != 0 {
				err = errors.New("add attribute City failed")
				return
			}
		} else {
			//Returns : -1 on error, 0 if validation failed, the number of bytes copied on success
			success = C.iptc_dataset_set_data(cgoCityDataset, (*C.uchar)(unsafe.Pointer(C.CString(iptcReq.City))),
				C.uint(len(iptcReq.City)), C.IPTC_VALIDATE)
			if success <= 0 {
				err = errors.New("edit attribute City failed")
				return
			}
		}
	}

	//object name
	if iptcReq.ObjectName != "" {
		cgoObjectNameDataset := C.iptc_data_get_dataset(cgoImageIptcData, C.IPTC_RECORD_APP_2, C.IPTC_TAG_OBJECT_NAME)
		if cgoObjectNameDataset == nil {
			cgoObjectNameDataset = C.iptc_dataset_new()
			defer C.iptc_dataset_free(cgoObjectNameDataset)

			C.iptc_dataset_set_tag(cgoObjectNameDataset, C.IPTC_RECORD_APP_2, C.IPTC_TAG_OBJECT_NAME)
			success = C.iptc_dataset_set_data(cgoObjectNameDataset, (*C.uchar)(unsafe.Pointer(C.CString(iptcReq.ObjectName))),
				C.uint(len(iptcReq.ObjectName)), C.IPTC_VALIDATE)
			if success <= 0 {
				err = errors.New("add attribute ObjectName failed")
				return
			}
			success = C.iptc_data_add_dataset(cgoImageIptcData, cgoObjectNameDataset)
			if success != 0 {
				err = errors.New("add attribute ObjectName failed")
				return
			}
		} else {
			//Returns : -1 on error, 0 if validation failed, the number of bytes copied on success
			success = C.iptc_dataset_set_data(cgoObjectNameDataset, (*C.uchar)(unsafe.Pointer(C.CString(iptcReq.ObjectName))),
				C.uint(len(iptcReq.ObjectName)), C.IPTC_VALIDATE)
			if success <= 0 {
				err = errors.New("edit attribute ObjectName failed")
				return
			}
		}
	}

	//program
	if iptcReq.OriginatingProgram != "" {
		cgoProgramDataset := C.iptc_data_get_dataset(cgoImageIptcData, C.IPTC_RECORD_APP_2, C.IPTC_TAG_ORIGINATING_PROGRAM)
		if cgoProgramDataset == nil {
			cgoProgramDataset := C.iptc_dataset_new()
			defer C.iptc_dataset_free(cgoProgramDataset)

			C.iptc_dataset_set_tag(cgoProgramDataset, C.IPTC_RECORD_APP_2, C.IPTC_TAG_ORIGINATING_PROGRAM)
			success = C.iptc_dataset_set_data(cgoProgramDataset, (*C.uchar)(unsafe.Pointer(C.CString(iptcReq.OriginatingProgram))),
				C.uint(len(iptcReq.OriginatingProgram)), C.IPTC_VALIDATE)
			if success <= 0 {
				err = errors.New("add attribute OriginatingProgram failed")
				return
			}
			success := C.iptc_data_add_dataset(cgoImageIptcData, cgoProgramDataset)
			if success != 0 {
				err = errors.New("add attribute OriginatingProgram failed")
				return
			}
		} else {
			//Returns : -1 on error, 0 if validation failed, the number of bytes copied on success
			success = C.iptc_dataset_set_data(cgoProgramDataset, (*C.uchar)(unsafe.Pointer(C.CString(iptcReq.OriginatingProgram))),
				C.uint(len(iptcReq.OriginatingProgram)), C.IPTC_VALIDATE)
			if success <= 0 {
				err = errors.New("edit attribute OriginatingProgram failed")
				return
			}
		}
	}

	oldKeywords := make([]string, 0, cgoImageIptcData.count)
	//keywords
	for i := C.uint(0); i < cgoImageIptcData.count; i++ {
		dataSet := C.get_iptc_dataset(cgoImageIptcData, i)
		//check name
		attrName := C.GoString(dataSet.info.name)
		attrValue := C.GoString((*C.char)(unsafe.Pointer(dataSet.data)))
		if attrName == "Keywords" {
			oldKeywords = append(oldKeywords, attrValue)
		}
	}

	oldKeywordsSet := set.NewStringSet(oldKeywords...)
	newKeywordsSet := set.NewStringSet(iptcReq.Keywords...)

	toAddKeywords := newKeywordsSet.Difference(oldKeywordsSet).Elems()
	toDelKeywords := oldKeywordsSet.Difference(newKeywordsSet).Elems()

	cgoToDelDatasets := make([]*C.struct__IptcDataSet, 0, len(toDelKeywords))
	for _, keywordToDel := range toDelKeywords {
		for i := C.uint(0); i < cgoImageIptcData.count; i++ {
			dataSet := C.get_iptc_dataset(cgoImageIptcData, i)
			//check name
			attrName := C.GoString(dataSet.info.name)
			attrValue := C.GoString((*C.char)(unsafe.Pointer(dataSet.data)))
			if attrName == "Keywords" && attrValue == keywordToDel {
				cgoToDelDatasets = append(cgoToDelDatasets, dataSet)
			}
		}
	}

	//delete old datasets
	for _, dataset := range cgoToDelDatasets {
		C.iptc_data_remove_dataset(cgoImageIptcData, dataset)
	}

	//add new datasets
	for _, keyword := range toAddKeywords {
		newDataSet := C.iptc_dataset_new()
		defer C.iptc_dataset_free(newDataSet)

		C.iptc_dataset_set_tag(newDataSet, C.IPTC_RECORD_APP_2, C.IPTC_TAG_KEYWORDS)
		success = C.iptc_dataset_set_data(newDataSet, (*C.uchar)(unsafe.Pointer(C.CString(keyword))),
			C.uint(len(keyword)), C.IPTC_VALIDATE)
		if success <= 0 {
			err = errors.New("add attribute Keywords failed")
			return
		}
		success = C.iptc_data_add_dataset(cgoImageIptcData, newDataSet)
		if success != 0 {
			err = errors.New("add attribute Keywords failed")
			return
		}
	}

	defer C.iptc_data_unref(cgoImageIptcData)

	//write iptc data into jpeg file
	cgoOutputFile := C.CString(outputFile)
	defer C.free(unsafe.Pointer(cgoOutputFile))
	success = C.save_iptc_info_to_jpeg_file(cgoImageIptcData, cgoImageFile, cgoOutputFile)
	if success != 0 {
		err = errors.New("write iptc info into image file failed")
		return
	}

	log.Infof("[%s] write iptc info to image success", reqId)
	result = outputFile
	resultType = ufop.RESULT_TYPE_OCTET_FILE
	contentType = "image/jpeg"
	return
}
