package auth

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/touno-io/core/api"
	"github.com/touno-io/core/db"
)

type AuthToken struct {
	Token string `json:"token"`
}

type Role struct {
	Type  string                `json:"type"`
	Token string                `json:"token"`
	Role  []map[string][]string `json:"role"`
}

func HandlerAuthMiddleware(pgx *db.PGClient, stx *db.PGTx, store *db.Storage) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		usr, err := stx.QueryOne(`SELECT id FROM user_account 
			WHERE s_email = $1 AND (s_pwd is NOT NULL AND s_pwd = crypt($2, s_pwd));`, c.Locals("username"), c.Locals("password"))
		if err != nil && err != db.ErrNoRows {
			return c.Status(401).JSON(api.HTTP{Error: err.Error()})
		} else if !usr.ToBoolean("id") {
			return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		}
		db.Debug("ID:", usr.ToInt64("id"))

		token, err := store.Get("session_id")
		if err != nil {
			return c.Status(500).JSON(api.HTTP{Error: fmt.Sprintf("Session Store: %s", err.Error())})
		}
		return c.JSON(AuthToken{Token: string(token)})
	}
}

func HandlerV1BasicAuthorizer(user, pass string) bool {
	return true
}
func HandlerV1BasicUnauthorized(c *fiber.Ctx) error {
	return c.Status(404).JSON(api.HTTP{Error: "Not Found"})
}
func HandlerV1BasicSignIn(pgx *db.PGClient, stx *db.PGTx, store *db.Storage) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		usr, err := stx.QueryOne(`SELECT id, n_level FROM user_account 
			WHERE s_email = $1 AND (s_pwd is NOT NULL AND s_pwd = crypt($2, s_pwd));`, c.Locals("username"), c.Locals("password"))
		if err != nil && err != db.ErrNoRows {
			return c.Status(401).JSON(api.HTTP{Error: err.Error()})
		} else if !usr.ToBoolean("id") {
			return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		} else if usr["n_level"] == "BANED" {
			return c.Status(401).JSON(api.HTTP{Error: "Baned"})
		}
		_, err = stx.QueryOne(`
			SELECT 
				t.e_role, t.s_token, r.s_name, array_to_json(array_agg(p.s_name)) permission
			FROM user_account a
			INNER JOIN user_token t ON t.user_id = a.id
			INNER JOIN user_role r on r.id = t.user_role_id
			INNER JOIN user_role_permission rp on rp.user_role_id = r.id
			INNER JOIN user_permission p on p.id = rp.user_permission_id
			WHERE a.id = $1
			GROUP BY t.e_role, t.s_token, r.s_name
		`, usr.ToInt64("id"))

		if err != db.ErrNoRows && err != nil {
			return c.Status(500).JSON(api.HTTP{Error: err.Error()})
		}
		var role *Role
		if err == db.ErrNoRows {
			role = &Role{
				Type:  "USER",
				Token: "",
			}
		}
		// {
		// 	"type": "USER",
		// 	"token": "37d0d4b351374c8e94bd054402166eaa",
		// 	"role": [
		// 		{
		// 			"viewer": {
		// 				"home": [ "view" ],
		// 				"blog": [ "view" ]
		// 			}
		// 		}
		// 	]
		// }
		token, err := store.Get("session_id")
		if err != nil {
			return c.Status(500).JSON(api.HTTP{Error: fmt.Sprintf("Session Store: %s", err.Error())})
		}
		return c.JSON(AuthToken{Token: string(token)})
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
