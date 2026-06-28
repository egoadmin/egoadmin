package upload

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"

	platformi18n "github.com/egoadmin/egoadmin/internal/platform/i18n"
	"github.com/egoadmin/elib/pkg/util/xfile"
	"github.com/egoadmin/elib/pkg/util/xflake"
	"github.com/gin-gonic/gin"
	"github.com/gotomicro/ego/core/eerrors"
	"github.com/gotomicro/ego/core/elog"
	"github.com/gotomicro/ego/server/egin"
	"github.com/samber/lo"
)

// UploadOptions 上传options
type UploadOptions struct {
	RelativePath string
	HandleFunc   gin.HandlerFunc
}

// Start 初始化上传
func Start(cc *egin.Component, opts ...UploadOptions) {
	for _, opt := range opts {
		cc.POST(opt.RelativePath, opt.HandleFunc)
	}
}

// UploadInfo 文件上传信息
type UploadInfo struct {
	Name string `json:"name"` // 文件名
	Size string `json:"size"` // 文件大小
}

// UploadResp 上传响应信息
type UploadResp struct {
	Filename  string `json:"filename"`  // 文件存储名称，包含路径
	Size      string `json:"size"`      // 文件大小，单位字节
	Originame string `json:"originame"` // 文件原名称
}

// s3UploadOptions 上传选项
type s3UploadOptions func(*S3UploadOption)

// S3UploadOption S3上传配置
type S3UploadOption struct {
	maxDescSize   int64                                                 // 描述文件最大大小
	maxSingleSize int64                                                 // 单文件最大大小
	beforeHandle  func(*gin.Context) error                              // 文件上传开始处理前钩子
	beforeUpload  func(*gin.Context, *multipart.Part, UploadInfo) error // 文件上传前钩子
}

// WithS3UploadMaxSingleSize 单文件最大大小,单位字节
func WithS3UploadMaxSingleSize(size int64) s3UploadOptions {
	return func(o *S3UploadOption) {
		o.maxSingleSize = size
	}
}

// WithS3UploadBeforeHandle 文件上传开始处理前钩子
//
// 可在此生命周期做文件上传权限验证等处理
func WithS3UploadBeforeHandle(fn func(*gin.Context) error) s3UploadOptions {
	return func(o *S3UploadOption) {
		o.beforeHandle = fn
	}
}

// WithS3UploadBeforeUpload 文件上传前钩子
//
// 可在此生命周期做文件上传大小自定义限制等操作
//
// 请注意使用此钩子函数后,单文件上传大小限制将不再有效.
func WithS3UploadBeforeUpload(fn func(*gin.Context, *multipart.Part, UploadInfo) error) s3UploadOptions {
	return func(o *S3UploadOption) {
		o.beforeUpload = fn
	}
}

// WithS3Upload s3文件上传
//
// json: [{"name": "xx.txt", "size": "124324234"}]
// file: xx.txt
// file: yy.txt
//
// 文件下载时需要通过url中query参数指定下载文件名
// 具体可参考https://docs.aws.amazon.com/AmazonS3/latest/API/API_GetObject.html
//
// response-content-disposition|response-content-type
// dde-introduction.mp4?response-content-disposition=attachment; filename=testing.txt&response-content-type=application/octet-stream
//
// 返回信息
//
//	{
//		"files": [
//			{
//				"filename": "md/2023-5-21/294e8088c5cd3e564ccc1beca16a7b4817610fbfd4c3c55a.md",
//				"size": "1376",
//				"originame": "1.md"
//			}
//		]
//	}
func WithS3Upload(s3 *xfile.S3, flake xflake.Geter, opts ...s3UploadOptions) UploadOptions {
	opt := S3UploadOption{
		maxDescSize:   4 * 1024 * 1024,        // 4MB
		maxSingleSize: 1 * 1024 * 1024 * 1024, // 1GB
	}
	for _, o := range opts {
		o(&opt)
	}
	return UploadOptions{
		RelativePath: "/upload",
		HandleFunc: func(c *gin.Context) {
			var (
				err   error
				infos []UploadInfo
				resp  []UploadResp
			)
			defer func() {
				if err != nil {
					if se := new(eerrors.EgoError); errors.As(err, &se) {
						c.JSON(http.StatusOK, se)

						return
					}
				} else {
					c.JSON(http.StatusOK, gin.H{
						"files": resp,
					})
				}
			}()

			// gin回调,可在回调中做鉴权等逻辑
			if opt.beforeHandle != nil {
				if err = opt.beforeHandle(c); err != nil {
					return
				}
			}

			r, err := c.Request.MultipartReader()
			if err != nil {
				elog.Error("文件上传失败", elog.FieldErr(err))
				err = platformi18n.ErrorFailed(requestContext(c), "ReadFileInfoFailed", nil)

				return
			}

			// 读取json格式的文件上传描述信息
			p, err := r.NextPart()
			if errors.Is(err, io.EOF) {
				return
			}

			if err != nil {
				elog.Error("文件上传失败", elog.FieldErr(err))
				err = platformi18n.ErrorFailed(requestContext(c), "FileUploadFailed", nil)

				return
			}

			if name := p.FormName(); name != "json" {
				err = platformi18n.ErrorFailed(requestContext(c), "FileDescriptionMissing", nil)

				return
			}

			var b bytes.Buffer
			n, err := io.CopyN(&b, p, opt.maxDescSize+1)
			if err != nil && !errors.Is(err, io.EOF) {
				elog.Error("文件上传失败", elog.FieldErr(err))
				err = platformi18n.ErrorFailed(requestContext(c), "FileUploadFailed", nil)

				return
			}
			if n > opt.maxDescSize {
				err = platformi18n.ErrorFailed(requestContext(c), "UploadDescriptionTooLarge", nil)

				return
			}

			if err = json.Unmarshal(b.Bytes(), &infos); err != nil {
				elog.Error("文件上传失败", elog.FieldErr(err))
				err = platformi18n.ErrorFailed(requestContext(c), "FileUploadFailed", nil)

				return
			}

			// 循环读取multipart文件上传的每一part
			for {
				part, er := r.NextPart()
				if er == io.EOF {
					break
				}
				if er != nil {
					elog.Error("文件上传失败", elog.FieldErr(er))
					err = platformi18n.ErrorFailed(requestContext(c), "FileUploadFailed", nil)

					return
				}

				if name := part.FormName(); name != "file" {
					err = platformi18n.ErrorFailed(requestContext(c), "UploadedFileMissing", nil)

					return

				}

				// 获取文件名
				filename := part.FileName()
				if filename == "" {
					err = platformi18n.ErrorFailed(requestContext(c), "FileNameRequired", nil)

					return
				}

				info, ok := lo.Find(infos, func(item UploadInfo) bool {
					return item.Name == filename
				})
				if !ok {
					err = platformi18n.ErrorFailed(requestContext(c), "FileDescriptionMissing", nil)

					return
				}

				// 文件上传前钩子处理
				if opt.beforeUpload != nil {
					if er = opt.beforeUpload(c, part, info); er != nil {
						err = er

						return
					}
				}
				id, er := flake.Get()
				if er != nil {
					elog.Error("文件上传失败", elog.FieldErr(er))
					err = platformi18n.ErrorFailed(requestContext(c), "FileUploadFailed", nil)

					return
				}

				fname := genpath(id, path.Ext(filename))
				fsize, _ := strconv.ParseInt(info.Size, 10, 64)

				if opt.beforeUpload == nil && fsize > opt.maxSingleSize {
					err = platformi18n.ErrorFailed(requestContext(c), "FileTooLarge", nil)

					return
				}

				if er = s3.Upload(c.Request.Context(), fname, part, fsize); er != nil {
					elog.Error("文件上传失败", elog.FieldErr(er))
					err = platformi18n.ErrorFailed(requestContext(c), "FileUploadFailed", nil)

					return
				}

				resp = append(resp, UploadResp{
					Filename:  fname,
					Size:      strconv.FormatInt(fsize, 10),
					Originame: filename,
				})
			}

			return
		},
	}
}

func requestContext(c *gin.Context) context.Context {
	return platformi18n.WithAcceptLanguage(c.Request.Context(), c.GetHeader(platformi18n.HeaderAcceptLanguage))
}

// 扩展名/日期/hash,md5.ext
func genpath(id uint64, ext string) string {
	// 扩展名不存在就用other当目录
	var parentPath string
	if ext == "" {
		parentPath = "other"
	} else {
		ext = strings.ToLower(ext)
		parentPath = strings.Replace(ext, ".", "", -1)
	}

	tm := time.Now()
	year, month, day := tm.Date()
	dataStr := fmt.Sprintf("%d-%d-%d", year, month, day)
	// 文件名 hex(id) + hex(timestamp)
	name := fmt.Sprintf("%x%x", id, tm.UnixNano())
	filename := name + ext

	return path.Join(parentPath, dataStr, filename)
}
