package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/getsentry/sentry-go"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/template/html"
	"github.com/pressly/goose/v3"
	"github.com/tmilewski/goenv"
	"github.com/touno-io/core/api"
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

	log.SetFlags(log.Lshortfile | log.Ltime)

	goose.SetTableName("db_version")
}

func main() {
	gracefulStop := make(chan os.Signal, 1)
	ctx := context.Background()
	pgx.Connect(&ctx, appTitle)

	if _, err := os.Stat("./db/schema"); !os.IsNotExist(err) {
		if dbVersion, err := goose.EnsureDBVersion(pgx.DB); dbVersion == 0 {
			if err != nil {
				log.Panic(err)
			}

			if err = goose.Run("up", pgx.DB, "./db/schema"); err != nil {
				log.Panic(err)
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
	app.Static("/assets", "./assets")

	app.Use(handerMiddlewareSecurity)

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("index", fiber.Map{})
	})

	app.Get("/health", handlerHealth)
	app.Get("/s/:hash", shorturl.HandlerRedirectURL(pgx))

	appApi := app.Group("/api", func(c *fiber.Ctx) error {
		return c.Next()
	})

	appApi.Get("/url", shorturl.HandlerGetURL(pgx))
	appApi.Post("/url", shorturl.HandlerAddURL(pgx))

	// app.Use(func(c *fiber.Ctx) error {
	// 	return c.Render("404", fiber.Map{})
	// })

	go appFiberListen(app, ":3000")

	signal.Notify(gracefulStop, os.Interrupt, syscall.SIGTERM)
	<-gracefulStop
	log.Println("Graceful Exiting...")

	if api.IsProduction {
		if err := app.Shutdown(); err != nil {
			log.Fatalf("Fiber: %s", err)
		}
	}

	if err := pgx.Close(); err != nil {
		log.Fatalf("DB: %s", err)
	}
}
