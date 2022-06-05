package main

import (
	"log"

	"github.com/gofiber/fiber/v2"
)

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
