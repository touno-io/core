package auth

import (
	"github.com/gofiber/fiber/v2"
	"github.com/touno-io/core/db"
)

type AuthToken struct {
	Token string `json:"token"`
}

func HandlerAuthMiddleware(pgx *db.PGClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		return c.Next()
	}
}

func HandlerV1BasicAuthorizer(pgx *db.PGClient) func(user, pass string) bool {
	return func(user, pass string) bool {
		return false
	}
}

func HandlerV1UserInfo(pgx *db.PGClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		return c.SendString("{}")
	}
}

func HandlerV1SignOut(pgx *db.PGClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		return c.SendString("{}")
	}
}
