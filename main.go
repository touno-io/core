package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/basicauth"
)

func init() {
	// appTitle, _, appIsProduction = daas.Initialize(appName)
	// cfgApp = daas.PrepareDataStore(&pgx, &ctx, appTitle, appName)
}

func main() {
	gracefulStop := make(chan os.Signal, 1)

	app := fiber.New(fiber.Config{
		DisableStartupMessage: true,
		CaseSensitive:         true,
		ErrorHandler: func(c *fiber.Ctx, e error) error {

			return nil
		},
	})

	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("☕")
	})

	app.Group("/api", basicauth.New(basicauth.Config{
		Realm: "Forbidden",
		Authorizer: func(user string, pass string) bool {
			return true
		},
		Unauthorized: func(c *fiber.Ctx) error {
			c.Status(fiber.ErrUnauthorized.Code)
			return fmt.Errorf("Unauthorized")
		},
		ContextUsername: "username",
		ContextPassword: "password",
	}))

	app.Use(func(c *fiber.Ctx) error {
		return fiber.ErrBadGateway
	})

	go appFiberListen(app, ":3000")

	signal.Notify(gracefulStop, os.Interrupt, syscall.SIGTERM)
	<-gracefulStop
	fmt.Println("Graceful Exiting...")

	// if appIsProduction {
	// 	err = app.Shutdown()
	// 	if err != nil {
	// 		fmt.Warnf("Fiber: %s", err)
	// 	}
	// }

	// err = pgx.Close()
	// if err != nil {
	// 	fmt.Warnf("DB: %s", err)
	// }
}

func appFiberListen(app *fiber.App, port string) {
	fmt.Printf("Fiber Started at '%s'\n", port)
	if err := app.Listen(port); err != nil {
		fmt.Errorf("Listen: %s", err)
	}
}
