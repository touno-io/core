package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v4"
	"github.com/touno-io/core/api"
	"github.com/touno-io/core/db"
)

type AuthToken struct {
	Token string `json:"token"`
}

type TokenClaims struct {
	Name string `json:"name"`
	jwt.RegisteredClaims
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
func HandlerV1BasicSignIn(pgx *db.PGClient, store *db.Storage) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		stx, err := pgx.Begin(db.LevelDefault)
		if db.IsRollback(err, stx) {
			db.Trace.Fatal(err)
		}

		usr, err := stx.QueryOne(`SELECT id, n_level, s_display_name, a_private_key FROM user_account 
			WHERE s_email = $1 AND (s_pwd is NOT NULL AND s_pwd = crypt($2, s_pwd));`, c.Locals("username"), c.Locals("password"))

		if db.IsRollbackThrow(err, stx) {
			return api.ErrorHandlerThrow(c, fiber.StatusUnauthorized, err)
		} else if !usr.ToBoolean("id") {
			if err := stx.Rollback(); err != nil {
				db.Trace.Fatalf("stx: %s", err)
			}
			return api.ErrorHandlerThrow(c, fiber.StatusUnauthorized, errors.New("Unauthorized"))
		} else if usr["n_level"] == "BANED" {
			if err := stx.Rollback(); err != nil {
				db.Trace.Fatalf("stx: %s", err)
			}
			return api.ErrorHandlerThrow(c, fiber.StatusUnauthorized, errors.New("Baned"))
		}
		// _, err = stx.QueryOne(`
		// 	SELECT
		// 		t.e_role, t.s_token, r.s_name, array_to_json(array_agg(p.s_name)) permission
		// 	FROM user_account a
		// 	INNER JOIN user_token t ON t.user_id = a.id
		// 	INNER JOIN user_role r on r.id = t.user_role_id
		// 	INNER JOIN user_role_permission rp on rp.user_role_id = r.id
		// 	INNER JOIN user_permission p on p.id = rp.user_permission_id
		// 	WHERE a.id = $1
		// 	GROUP BY t.e_role, t.s_token, r.s_name
		// `, usr.ToInt64("id"))

		// if db.IsRollbackThrow(err, stx) && err != db.ErrNoRows {
		// 	return api.ThrowInternalServerError(c, err)
		// }

		// if err == db.ErrNoRows {
		// 	// publicBlock, privateBlock, err := generateRSAKey()
		// 	// if db.IsRollbackThrow(err, stx) {
		// 	// 	return api.ThrowInternalServerError(c, err)
		// 	// }

		// 	// err = stx.Execute(`
		// 	// 	UPDATE user_account SET a_public_key=$2, a_private_key=$3 WHERE id = $1;
		// 	// `, usr.ToInt64("id"), publicBlock.Bytes, privateBlock.Bytes)
		// 	// if db.IsRollbackThrow(err, stx) {
		// 	// 	return api.ThrowInternalServerError(c, err)
		// 	// }

		// }

		sess, err := stx.QueryOne(`
			INSERT INTO user_session (user_id, s_ipaddr) VALUES ($1, $2)
			ON CONFLICT ON CONSTRAINT uq_user_ip
			DO UPDATE SET n_session = uuid_generate_v4()
			RETURNING n_session;
		`, usr.ToInt64("id"), c.IP())
		if db.IsRollbackThrow(err, stx) {
			return api.ThrowInternalServerError(c, err)
		}

		// err = createNewRSAKey(stx, usr["id"])
		// if err != nil {
		// 	return api.ThrowInternalServerError(c, err)
		// }
		token := jwt.NewWithClaims(jwt.SigningMethodRS256, &TokenClaims{
			usr["s_display_name"],
			jwt.RegisteredClaims{
				IssuedAt:  jwt.NewNumericDate(time.Now()),
				ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
				Issuer:    c.Locals("username").(string),
				ID:        sess["n_session"],
			},
		})

		privateKey, _, err := ParsePKCS1PrivateKey(usr.ToByte("a_private_key"))
		if err != nil {
			return api.ThrowInternalServerError(c, err)
		}

		// Sign and get the complete encoded token as a string using the secret
		tokenString, err := token.SignedString(privateKey)
		if err != nil {
			return api.ThrowInternalServerError(c, err)
		}

		if err := stx.Commit(); err != nil {
			db.Trace.Fatalf("stx: %s", err)
		}

		// privateKey2, _, err := generateRSAKey()
		// if err != nil {
		// 	return api.ThrowInternalServerError(c, err)
		// }

		tokenCheck, err := jwt.ParseWithClaims(tokenString, &TokenClaims{}, func(t *jwt.Token) (interface{}, error) {
			return privateKey, nil
		})
		if tokenCheck.Valid {
			fmt.Println("You look nice today")
		} else if ve, ok := err.(*jwt.ValidationError); ok {
			if ve.Errors&jwt.ValidationErrorMalformed != 0 {
				fmt.Println("That's not even a token")
			} else if ve.Errors&(jwt.ValidationErrorExpired|jwt.ValidationErrorNotValidYet) != 0 {
				// Token is either expired or not active yet
				fmt.Println("Timing is everything")
			} else {
				fmt.Println("Couldn't handle this token:", err)
			}
		} else {
			fmt.Println("Couldn't handle this token:", err)
		}

		// db.Debug(string(pemPublic))
		// db.Debug(string(pemPrivate))
		// role = &Role{
		// 	Type: "USER",
		// 	Role: []map[string][]string{},
		// }

		return c.JSON(AuthToken{Token: tokenString})
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

func generateRSAKey() (*rsa.PrivateKey, *rsa.PublicKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, nil, err
	}

	return privateKey, &privateKey.PublicKey, nil
}

func createNewRSAKey(stx *db.PGTx, userId string) error {
	privateKey, publicKey, err := generateRSAKey()
	pemPublic, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return err
	}

	err = stx.Execute(`
		UPDATE user_account SET a_private_key=$2, a_public_key=$3 WHERE id = $1;
	`, userId, x509.MarshalPKCS1PrivateKey(privateKey), pemPublic)
	if db.IsRollbackThrow(err, stx) {
		return err
	}

	return nil
}

func ParsePKCS1PrivateKey(privateKey []byte) (*rsa.PrivateKey, *rsa.PublicKey, error) {
	priv, err := x509.ParsePKCS1PrivateKey(privateKey)
	if err != nil {
		return nil, nil, err
	}

	return priv, &priv.PublicKey, nil
}

func PEMEncodeToMemory(privateKey *rsa.PrivateKey) ([]byte, []byte, error) {

	pemPrivate := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})
	pemPublic, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, nil, err
	}

	pemPublic = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pemPublic})
	return pemPrivate, pemPublic, nil
}
