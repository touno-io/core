package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
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
	Name      string `json:"name"`
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
		stx, err := pgx.Begin(db.LevelDefault)
		if db.IsRollback(err, stx) {
			db.Trace.Fatal(err)
		}

		// Decode the header contents
		payload, headers, err := jose.Decode(auth[7:], func(headers map[string]interface{}, payload string) interface{} {
			//log something
			var claims TokenClaims
			err := jsoniter.ConfigCompatibleWithStandardLibrary.UnmarshalFromString(payload, &claims)
			if err != nil {
				return err
			}

			publicBytes, err := store.Get(claims.ID)
			if err != nil {
				return err
			}

			publicKey, err := x509.ParsePKIXPublicKey(publicBytes)
			if err != nil {
				return err
			}

			return publicKey
		})

		db.Debugf("err = %v\n", err)
		//go use token
		db.Debugf("payload = %v\n", string(payload))

		//and/or use headers
		db.Debugf("headers = %v\n", headers)

		if err != nil {
			return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		}

		// recheck, err := jwt.ParseWithClaims(auth[7:], &TokenClaims{}, func(t *jwt.Token) (any, error) {
		// 	if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
		// 		return nil, fmt.Errorf("unexpected method: %s", t.Header["alg"])
		// 	}

		// 	if _, ok := t.Claims.(*TokenClaims); !ok {
		// 		return nil, fmt.Errorf("unexpected claims.")
		// 	}

		// 	return nil, nil
		// })

		// if err != nil {
		// 	return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		// }

		// claim, ok := recheck.Claims.(*TokenClaims)
		// if !ok {
		// 	return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		// }

		// db.Debugv(claim)

		// if err != nil {
		// 	return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		// }

		// usr, err := stx.QueryOne(`SELECT id FROM user_account
		// 	WHERE s_email = $1 AND (s_pwd is NOT NULL AND s_pwd = crypt($2, s_pwd));`, c.Locals("username"), c.Locals("password"))
		// if err != nil && err != db.ErrNoRows {
		// 	return c.Status(401).JSON(api.HTTP{Error: err.Error()})
		// } else if !usr.ToBoolean("id") {
		// 	return c.Status(401).JSON(api.HTTP{Error: "Unauthorized"})
		// }
		// db.Debug("ID:", usr.ToInt64("id"))

		// token, err := store.Get("session_id")
		// if err != nil {
		// 	return c.Status(500).JSON(api.HTTP{Error: fmt.Sprintf("Session Store: %s", err.Error())})
		// }

		// if err := stx.Commit(); err != nil {
		// 	db.Trace.Fatalf("stx: %s", err)
		// }

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

		usr, err := stx.QueryOne(`SELECT id, n_level, s_display_name, a_private_key, a_public_key FROM user_account 
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
			ID:        sessionId,
			Issuer:    c.Locals("username").(string),
			NotBefore: getTimeStamp(time.Now()),
			IssuedAt:  getTimeStamp(time.Now()),
			ExpiresAt: getTimeStamp(time.Now().Add(24 * time.Hour)),
		})

		tokenString, err := jose.SignBytes(payload, jose.RS256, privateKey)

		// tokenString, err := token.SignedString(privateKey)

		if err != nil {
			return api.ThrowInternalServerError(c, err)
		}

		if err := stx.Commit(); err != nil {
			db.Trace.Fatalf("stx: %s", err)
		}

		// payload, headers, err := jose.DecodeBytes(tokenString, &privateKey.PublicKey)
		// if err == nil {
		// 	//go use token
		// 	db.Debugf("payload = %v\n", payload)

		// 	//and/or use headers
		// 	db.Debugf("headers = %v\n", headers)
		// }

		// segments := strings.Split(tokenString, ".")
		// db.Debugv(segments)
		// err = rsa256Salt.Verify(strings.Join(segments[:2], "."), segments[2], publicKey)
		// if err != nil {
		// 	db.Debugf("Error while verifying key: %v", err)
		// }

		// if err != nil {
		// 	db.Debug(fmt.Errorf("validate: %w", err))
		// }

		// claims, ok := recheck.Claims.(*TokenClaims)
		// if !ok {
		// 	db.Debug(fmt.Errorf("validate: invalid"))
		// }
		// db.Debugv(claims)

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

func getTimeStamp(t time.Time) int64 {
	return t.UnixNano() / int64(time.Millisecond)
}
