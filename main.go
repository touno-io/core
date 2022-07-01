package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
	"github.com/gofiber/template/html"
	"github.com/pressly/goose/v3"
	"github.com/tmilewski/goenv"
	"github.com/touno-io/core/api"
	"github.com/touno-io/core/api/auth"
	"github.com/touno-io/core/api/shorturl"
	"github.com/touno-io/core/db"
)

const (
	ENV     = "ENV"
	VERSION = "VERSION"
)

var (
	appName  string       = "touno-io/core"
	appTitle string       = ""
	pgx      *db.PGClient = &db.PGClient{}
)

func init() {
	if !api.IsProduction {
		goenv.Load()
	}
	rand.Seed(time.Now().UnixNano())

	appTitle = fmt.Sprintf("%s@%s", appName, api.Version)

	goose.SetTableName("db_version")
}

func main() {
	gracefulStop := make(chan os.Signal, 1)
	ctx := context.Background()
	pgx.Connect(&ctx, appTitle)

	if _, err := os.Stat("./db/schema"); !os.IsNotExist(err) {
		if dbVersion, err := goose.EnsureDBVersion(pgx.DB); dbVersion == 0 {
			if err != nil {
				db.Trace.Fatal(err)
			}

			if err = goose.Run("up", pgx.DB, "./db/schema"); err != nil {
				db.Trace.Fatal(err)
			}
		}
	}

	engine := html.New("./views", ".html")
	app := fiber.New(fiber.Config{
		Views:                 engine,
		DisableStartupMessage: true,
		CaseSensitive:         true,
		ErrorHandler: func(c *fiber.Ctx, e error) error {
			sentry.CaptureException(e)
			return nil
		},
	})
	// app.Use(etag.New())
	// app.Use(csrf.New(csrf.Config{
	// 	KeyLookup:      "header:X-Csrf-Token",
	// 	CookieName:     "csrf_",
	// 	CookieSameSite: "Strict",
	// 	Expiration:     3 * time.Hour,
	// 	KeyGenerator:   utils.UUID,
	// }))

	app.Static("/", "./assets")
	app.Use(api.HanderMiddlewareSecurity)
	app.Get("/health", api.HandlerHealth)
	app.Get("/s/:hash", shorturl.HandlerRedirectURL(pgx))

	appV1 := app.Group("/v1")

	// Initialize custom config
	storeSession := db.Cache("store_session")
	// storeBlocked := db.Cache("store_blockip")

	appAuth := appV1.Group("/auth")
	appAuth.Post("/", basicauth.New(basicauth.Config{
		Authorizer: func(user, pass string) bool {
			db.Debug(user, pass)
			return true
		},
		Unauthorized: func(c *fiber.Ctx) error {
			db.Debug("Unauthorized")
			// if c.IP() != "127.0.0.1" && c.IP() != "::1" && ipBlock[c.IP()] <= 5 {
			// 	ipBlock[c.IP()] += 1
			// }
			// if ipBlock[c.IP()] <= 5 {
			return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
			// } else {
			// 	return c.Status(403).JSON(api.HTTP{Error: "IP Blocked"})
			// }
		},
		ContextUsername: "_usr",
		ContextPassword: "_pwd",
	}), func(c *fiber.Ctx) error {
		token, err := storeSession.Get("session_id")
		if err != nil {
			return c.Status(500).JSON(api.HTTP{Error: fmt.Sprintf("Session Store: %s", err.Error())})
		}
		return c.JSON(auth.AuthToken{Token: string(token)})
	})
	appAuth.Get("/account", auth.HandlerV1UserInfo(pgx))
	appAuth.Delete("/", auth.HandlerV1SignOut(pgx))

	appApi := app.Group("/api", func(c *fiber.Ctx) error {
		return c.Next()
	})

	appApi.Get("/url", shorturl.HandlerGetURL(pgx))
	appApi.Post("/url", shorturl.HandlerAddURL(pgx))

	app.Use(func(c *fiber.Ctx) error {
		return c.Status(404).JSON(&api.HTTP{Error: "not implemented"})
	})

	go api.FiberListen(app, ":3000")

	signal.Notify(gracefulStop, os.Interrupt, syscall.SIGTERM)
	<-gracefulStop
	db.Info("Graceful Exiting...")

	if api.IsProduction {
		if err := app.Shutdown(); err != nil {
			db.Trace.Fatalf("Fiber: %s", err)
		}
	}

	if err := pgx.Close(); err != nil {
		db.Trace.Fatalf("DB: %s", err)
	}
}
