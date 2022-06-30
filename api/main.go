package api

import (
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
)

const (
	ENV     = "ENV"
	VERSION = "VERSION"
)

var (
	IsProduction bool
	Version      string = ""
)

func init() {
	IsProduction = os.Getenv(ENV) == "production"

	content, err := ioutil.ReadFile(VERSION)
	if err != nil {
		content, _ = ioutil.ReadFile(fmt.Sprintf("../%s", VERSION))
	}
	Version = strings.TrimSpace(string(content))
}

type HTTP struct {
	Code  int     `json:"code,omitempty"`
	Error *string `json:"error"`
}

// func (e *HTTP) ErrorHandlerThrow(c *fiber.Ctx) error {
// 	err := errors.New(*e.Error)
// 	if e.Code != fiber.StatusOK && e.Code != fiber.StatusCreated {
// 		Error(err)
// 		sentry.CaptureException(err)
// 	} else if err != nil {
// 		Warn(err)
// 	}
// 	return c.Status(e.Code).JSON((HTTP{Code: e.Code, Error: e.Error}))
// }

func ErrorHandlerThrow(c *fiber.Ctx, code int, err error) error {
	errorMessage := err.Error()
	if code != fiber.StatusOK && code != fiber.StatusCreated {
		// Error(err)
		sentry.CaptureException(err)
		// } else if err != nil {
		// 	Warn(err)
	}
	return c.Status(code).JSON((HTTP{Code: code, Error: &errorMessage}))
}

func HttpErrorf(code int, err error) *HTTP {
	errMsg := err.Error()
	return &HTTP{Code: code, Error: &errMsg}
}
func HttpErrorPrint(code int, format string, v ...interface{}) *HTTP {
	errMsg := fmt.Sprintf(format, v...)
	return &HTTP{Code: code, Error: &errMsg}
}

func HttpErrorPrintf(code int, v ...interface{}) *HTTP {
	errMsg := fmt.Sprintf("%s", v...)
	return &HTTP{Code: code, Error: &errMsg}
}
