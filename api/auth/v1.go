package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"

	jose "github.com/dvsekhvalnov/jose2go"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/touno-io/core/api"
	"github.com/touno-io/core/db"
)

type AuthToken struct {
	Token string `json:"token"`
}

type TokenClaims struct {
	Name      string `json:"nae"`
	UUID      string `json:"usr"`
	ID        string `json:"jti"`
	Issuer    string `json:"iss"`
	NotBefore int64  `json:"nbf"`
	IssuedAt  int64  `json:"iat"`
	ExpiresAt int64  `json:"exp"`
}

func HandlerAuthMiddleware(pgx *db.PGClient, store *db.Storage) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		auth := c.Get(fiber.HeaderAuthorization)
		if len(auth) <= 7 || strings.ToLower(auth[:6]) != "bearer" {
			return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		}

		_, _, err := jose.Decode(auth[7:], func(headers map[string]interface{}, payload string) interface{} {
			var claims TokenClaims
			err := jsoniter.ConfigCompatibleWithStandardLibrary.UnmarshalFromString(payload, &claims)
			if err != nil {
				return err
			}

			c.Locals("claims", claims)

			if time.Now().Sub(time.Unix(claims.NotBefore, 0)) < 0 && time.Now().Sub(time.Unix(claims.ExpiresAt, 0)) > 0 {
				return fmt.Errorf("Session Expired")
			}

			publicBytes, err := store.Get(claims.ID)
			if publicBytes == nil && err == nil {
				return fmt.Errorf("Session Deny")
			}

			if err != nil {
				return err
			}

			publicKey, err := x509.ParsePKIXPublicKey(publicBytes)
			if err != nil {
				return err
			}

			// pemPublic, err := x509.MarshalPKIXPublicKey(publicKey)
			// if err != nil {
			// 	return err
			// }
			// pemPublic = pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: pemPublic})
			// db.Debug(string(pemPublic))

			return publicKey
		})

		if err != nil {
			return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		}

		return c.Next()
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

		db.Info("Header:", c.Request().Header)

		usr, err := stx.QueryOne(`SELECT id, n_level, n_object, s_display_name, a_private_key, a_public_key FROM user_account 
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
		var sessionId string
		check, err := stx.QueryOne(`
			SELECT n_session FROM user_session
			WHERE user_id = $1 AND s_ipaddr = $2 AND t_created >= NOW() - INTERVAL '1 DAY'
		`, usr.ToInt64("id"), c.IP())

		if err != db.ErrNoRows && err != nil {
			return api.ThrowInternalServerError(c, err)
		} else if err != db.ErrNoRows {
			sessionId = check["n_session"]
		} else {
			sess, err := stx.QueryOne(`
			INSERT INTO user_session (user_id, s_ipaddr) VALUES ($1, $2)
			ON CONFLICT ON CONSTRAINT uq_user_ip
			DO UPDATE SET n_session = uuid_generate_v4(), t_created = NOW()
			RETURNING n_session;
		`, usr.ToInt64("id"), c.IP())

			if db.IsRollbackThrow(err, stx) {
				return api.ThrowInternalServerError(c, err)
			}
			sessionId = sess["n_session"]
			err = store.Set(sessionId, usr.ToByte("a_public_key"), time.Hour*24)
			if db.IsRollbackThrow(err, stx) {
				return api.ThrowInternalServerError(c, err)
			}
		}

		privateKey, _, err := ParsePKCS1PrivateKey(usr.ToByte("a_private_key"))
		if err != nil {
			return api.ThrowInternalServerError(c, err)
		}

		payload, err := jsoniter.ConfigCompatibleWithStandardLibrary.Marshal(TokenClaims{
			Name:      usr["s_display_name"],
			UUID:      usr["n_object"],
			ID:        sessionId,
			Issuer:    c.Locals("username").(string),
			NotBefore: getTimeStamp(time.Now()),
			IssuedAt:  getTimeStamp(time.Now()),
			ExpiresAt: getTimeStamp(time.Now().Add(24 * time.Hour)),
		})

		tokenString, err := jose.SignBytes(payload, jose.RS256, privateKey)

		if err != nil {
			return api.ThrowInternalServerError(c, err)
		}

		if err := stx.Commit(); err != nil {
			db.Trace.Fatalf("stx: %s", err)
		}

		return c.JSON(AuthToken{Token: tokenString})
	}
}

type UserPermission struct {
	Name string `json:"name"`
}
type UserAccount struct {
	Name       string           `json:"name"`
	Email      string           `json:"mail"`
	Level      string           `json:"level"`
	Permission []UserPermission `json:"permission"`
}

func HandlerV1UserInfo(pgx *db.PGClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		claims := c.Locals("claims").(TokenClaims)

		stx, err := pgx.Begin(db.LevelDefault)
		if db.IsRollback(err, stx) {
			db.Trace.Fatal(err)
		}
		account, err := stx.QueryOne(`
			SELECT s_display_name, s_email, n_level
			FROM user_account
			WHERE n_object = $1;
		`, claims.UUID)

		if err := stx.Commit(); err != nil {
			db.Trace.Fatalf("stx: %s", err)
		}

		return c.JSON(&UserAccount{
			Name:       account["s_display_name"],
			Email:      account["s_email"],
			Level:      account["n_level"],
			Permission: []UserPermission{},
		})
	}
}

func HandlerV1SignOut(pgx *db.PGClient, store *db.Storage) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		claims := c.Locals("claims").(TokenClaims)

		stx, err := pgx.Begin(db.LevelDefault)
		if db.IsRollback(err, stx) {
			db.Trace.Fatal(err)
		}

		err = stx.Execute(`DELETE FROM user_session WHERE n_session = $1;`, claims.ID)
		if db.IsRollbackThrow(err, stx) {
			return api.ThrowInternalServerError(c, err)
		}

		err = store.Delete(claims.ID)
		if db.IsRollbackThrow(err, stx) {
			return api.ThrowInternalServerError(c, err)
		}

		if err := stx.Commit(); err != nil {
			db.Trace.Fatalf("stx: %s", err)
		}

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

func getTimeStamp(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
