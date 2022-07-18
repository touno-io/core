package api

import (
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
	"strings"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	ua "github.com/mileusna/useragent"
	"github.com/touno-io/core/db"
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
	Code  int    `json:"code,omitempty"`
	Error string `json:"error,omitempty"`
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

func GetConnectingIP(c *fiber.Ctx) string {
	var ipAddr string = c.IP()
	raw := string(c.Request().Header.Header())
	regCfIP, _ := regexp.Compile("(?i)cf-connecting-ip:(.*?)\n")
	connectingIp := regCfIP.FindStringSubmatch(raw)

	if len(connectingIp) > 0 {
		ipAddr = strings.TrimSpace(connectingIp[1])
	}

	if ipAddr == "127.0.0.1" || ipAddr == "::1" {
		ipAddr = "www.touno.io"
	}
	return ipAddr
}

func GetUserAgent(c *fiber.Ctx) ua.UserAgent {
	raw := string(c.Request().Header.Header())
	regUserAgent, _ := regexp.Compile("(?i)user-agent:(.*?)\n")
	hAgent := regUserAgent.FindStringSubmatch(raw)
	return ua.Parse(strings.TrimSpace(hAgent[1]))
}

func ThrowInternalServerError(c *fiber.Ctx, err error) error {
	return ErrorHandlerThrow(c, fiber.StatusInternalServerError, err)
}

func ErrorHandlerThrow(c *fiber.Ctx, code int, err error) error {
	if code != fiber.StatusOK && code != fiber.StatusCreated {
		// Error(err)
		sentry.CaptureException(err)
		// } else if err != nil {
		// 	Warn(err)
	}
	return c.Status(code).JSON((HTTP{Code: code, Error: err.Error()}))
}

func HttpErrorf(code int, err error) *HTTP {
	return &HTTP{Code: code, Error: err.Error()}
}
func HttpErrorPrint(code int, format string, v ...any) *HTTP {
	return &HTTP{Code: code, Error: fmt.Sprintf(format, v...)}
}

func HttpErrorPrintf(code int, v ...any) *HTTP {
	return &HTTP{Code: code, Error: fmt.Sprintf("%s", v...)}
}

func FiberListen(app *fiber.App, port string) {
	db.Infof("Fiber Started at '%s'\n", port)
	if err := app.Listen(port); err != nil {
		db.Trace.Fatalf("Fiber listen: %+v", err)
	}
}

func HanderMiddlewareSecurity(c *fiber.Ctx) error {
	// Set some security headers:
	c.Set("X-XSS-Protection", "1; mode=block")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-Download-Options", "noopen")
	c.Set("X-Frame-Options", "SAMEORIGIN")
	c.Set("X-DNS-Prefetch-Control", "off")
	return c.Next()
}

func HandlerHealth(c *fiber.Ctx) error {
	return c.SendString("☕")
}
