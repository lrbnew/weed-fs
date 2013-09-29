package operation

import (
	"bytes"
	"code.google.com/p/weed-fs/go/glog"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"path/filepath"
	"strings"
)

type UploadResult struct {
	Size  int
	Error string
}

var fileNameEscaper = strings.NewReplacer("\\", "\\\\", "\"", "\\\"")

func Upload(uploadUrl string, filename string, reader io.Reader, isGzipped bool, mtype string) (*UploadResult, error) {
	return upload_content(uploadUrl, func(w io.Writer) (err error) {
		_, err = io.Copy(w, reader)
		return
	}, filename, isGzipped, mtype)
}
func upload_content(uploadUrl string, fillBufferFunction func(w io.Writer) error, filename string, isGzipped bool, mtype string) (*UploadResult, error) {
	body_buf := bytes.NewBufferString("")
	body_writer := multipart.NewWriter(body_buf)
	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="file"; filename="%s"`, fileNameEscaper.Replace(filename)))
	if mtype == "" {
		mtype = mime.TypeByExtension(strings.ToLower(filepath.Ext(filename)))
	}
	if mtype != "" {
		h.Set("Content-Type", mtype)
	}
	if isGzipped {
		h.Set("Content-Encoding", "gzip")
	}
	file_writer, err := body_writer.CreatePart(h)
	if err != nil {
		glog.V(0).Infoln("error creating form file", err)
		return nil, err
	}
	if err = fillBufferFunction(file_writer); err != nil {
		glog.V(0).Infoln("error copying data", err)
		return nil, err
	}
	content_type := body_writer.FormDataContentType()
	if err = body_writer.Close(); err != nil {
		glog.V(0).Infoln("error closing body", err)
		return nil, err
	}
	resp, err := http.Post(uploadUrl, content_type, body_buf)
	if err != nil {
		glog.V(0).Infoln("failing to upload to", uploadUrl)
		return nil, err
	}
	defer resp.Body.Close()
	resp_body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	var ret UploadResult
	err = json.Unmarshal(resp_body, &ret)
	if err != nil {
		glog.V(0).Infoln("failing to read upload resonse", uploadUrl, string(resp_body))
		return nil, err
	}
	if ret.Error != "" {
		return nil, errors.New(ret.Error)
	}
	return &ret, nil
}
