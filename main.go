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
	storeSession := db.CacheNew(pgx, "session")

	stxAuth, err := pgx.Begin(db.LevelDefault)
	if db.IsRollback(err, stxAuth) {
		db.Trace.Fatal(err)
	}
	appAuth := appV1.Group("/auth")

	appAuth.Post("/", basicauth.New(basicauth.Config{
		Authorizer:   auth.HandlerV1BasicAuthorizer,
		Unauthorized: auth.HandlerV1BasicUnauthorized,
	}), auth.HandlerV1BasicSignIn(pgx, stxAuth, storeSession))

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

	if err := storeSession.Close(); err != nil {
		db.Trace.Fatalf("session: %s", err)
	}
	if !stxAuth.Closed {
		if err := stxAuth.Commit(); err != nil {
			db.Trace.Fatalf("stx: %s", err)
		}
	}

	db.Debug(" - Close DB Connection")
	if err := pgx.Close(); err != nil {
		db.Trace.Fatalf("DB: %s", err)
	}
}
