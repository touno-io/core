package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/tmilewski/goenv"
)

const (
	_ENV     = "ENV"
	_VERSION = "VERSION"
)

var (
	appName         string = "ouno-api"
	appVersion      string = ""
	appTitle        string = ""
	appIsProduction bool
)

func init() {
	appIsProduction = os.Getenv(_ENV) == "production"
	if !appIsProduction {
		goenv.Load()

	}
	rand.Seed(time.Now().UnixNano())

	content, err := ioutil.ReadFile(_VERSION)
	if err != nil {
		content, _ = ioutil.ReadFile(fmt.Sprintf("../%s", _VERSION))
	}
	appVersion = strings.TrimSpace(string(content))
	appTitle = fmt.Sprintf("%s@%s", appName, appVersion)

	log.SetFlags(log.Lshortfile | log.Ltime)
}

func generateSlug() string {
	var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	s := make([]rune, 6)

	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}

	return string(s)
}

var (
	pgx *PGClient = &PGClient{}
)

func main() {
	gracefulStop := make(chan os.Signal, 1)
	ctx := context.Background()
	pgx.Connect(&ctx, appTitle)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		CaseSensitive:         true,
		ErrorHandler: func(c *fiber.Ctx, e error) error {
			sentry.CaptureException(e)
			return nil
		},
	})

	app.Use(handerMiddlewareSecurity)

	app.Get("/health", handlerHealth)
	app.Get("/s/:hash", handlerShortURL)

	api := app.Group("/api", basicauth.New(basicauth.Config{
		Users: map[string]string{},
		Realm: "Forbidden",
		Authorizer: func(user string, pass string) bool {
			log.Println(user, ":", pass)
			return true
		},
		Unauthorized: func(c *fiber.Ctx) error {
			return fiber.ErrUnauthorized
		},
		ContextUsername: "username",
		ContextPassword: "password",
	}))

	api.Get("/", func(c *fiber.Ctx) error {
		c.SendString("{ OK: true }")
		return nil
	})

	app.Use(func(c *fiber.Ctx) error {
		return fiber.ErrNotFound
	})

	go appFiberListen(app, ":3000")
	log.Println(generateSlug())

	signal.Notify(gracefulStop, os.Interrupt, syscall.SIGTERM)
	<-gracefulStop
	log.Println("Graceful Exiting...")

	// if appIsProduction {
	// 	err = app.Shutdown()
	// 	if err != nil {
	// 		log.Warnf("Fiber: %s", err)
	// 	}
	// }

	// err = pgx.Close()
	// if err != nil {
	// 	log.Warnf("DB: %s", err)
	// }
}

func appFiberListen(app *fiber.App, port string) {
	log.Printf("Fiber Started at '%s'\n", port)
	if err := app.Listen(port); err != nil {
		log.Fatalf("listen: %+v", err)
	}
}

func handerMiddlewareSecurity(c *fiber.Ctx) error {
	// Set some security headers:
	c.Set("X-XSS-Protection", "1; mode=block")
	c.Set("X-Content-Type-Options", "nosniff")
	c.Set("X-Download-Options", "noopen")
	c.Set("Strict-Transport-Security", "max-age=5184000")
	c.Set("X-Frame-Options", "SAMEORIGIN")
	c.Set("X-DNS-Prefetch-Control", "off")
	return c.Next()
}

func handlerHealth(c *fiber.Ctx) error {
	return c.SendString("☕")
}
