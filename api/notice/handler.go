package notice

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/touno-io/core/api"
	"github.com/touno-io/core/db"
)

const (
	TELEGRAM   = "telegram"
	SLACK      = "slack"
	MSTEAM     = "msteam"
	LINE       = "line"
	LINENOTIFY = "line-notify"
	EMAIL      = "email"
	WEBHOOK    = "webhook"
	NATIVE     = "native"
)

type RequestNotice struct {
	Message string `json:"message"`
}

func HandlerNoticeMessage(pgx *db.PGClient) func(*fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		req := new(RequestNotice)
		if c.GetReqHeaders()["Content-Type"] == "application/json" {
			if err := c.BodyParser(req); err != nil {
				return api.ErrorHandlerThrow(c, fiber.StatusBadRequest, err)
			}
		}

		stx, err := pgx.Begin(db.LevelDefault)
		if db.IsRollback(err, stx) {
			return api.ErrorHandlerThrow(c, fiber.StatusInternalServerError, err)
		}

		pgNotice, err := stx.Query(`
			SELECT
				ss.notice_room_id, pv.e_type, st.n_uuid, pv.o_param provider, sr.o_param room
			FROM app.notice_section st
			INNER JOIN app.notice_subscriber ss ON ss.notice_section_id = st.id
			INNER JOIN app.notice_room sr ON sr.id = ss.notice_room_id
			INNER JOIN app.notice_provider pv ON pv.id = sr.notice_provider_id
			WHERE st.cf_courier_app_id = $1::int4 AND st.s_name = $2
				AND st.t_deleted IS NULL AND NOT pv.b_deleted
		`, c.Locals("cf_courier_app_id"), c.Params("roomName"))

		if db.IsRollback(err, stx) {
			return api.ErrorHandlerThrow(c, fiber.StatusInternalServerError, err)
		}
		defer pgNotice.Close()

		var sqlParam = regexp.MustCompile("(['])")
		res := new(api.HTTP)

		i := 0
		errJob := 0
		var historyInserted []string
		for pgNotice.Next() {
			notice, err := stx.FetchRow(pgNotice)
			if db.IsRollback(err, stx) {
				return api.ErrorHandlerThrow(c, fiber.StatusInternalServerError, err)
			}

			go func() {
				defer func() {
					if err := recover(); err != nil {
						db.Error("panic occurred:", err)
						errJob++
					}
				}()
				var (
					reqBody string
					resBody string
					errMsg  string
				)
				switch notice["e_type"] {
				case TELEGRAM:
					if reqBody, resBody, err = ProviderTelegram(c, req, notice); db.IsRollbackThrow(err, nil) {
						errMsg = fmt.Sprintf("Provider '%s'::%s", notice["e_type"], err)
						res.Error = strings.TrimSpace(fmt.Sprintf("%s\n%s", res.Error, errMsg))
						db.Error(errMsg)
					}
				case EMAIL:
					if reqBody, resBody, err = ProviderEmail(c, notice); db.IsRollbackThrow(err, nil) {
						errMsg = fmt.Sprintf("Provider '%s'::%s", notice["e_type"], err)
						res.Error = strings.TrimSpace(fmt.Sprintf("%s\n%s", res.Error, errMsg))
						db.Error(errMsg)
					}
				default:
					errMsg = fmt.Sprintf("Provider '%s'::Not Implemented", notice["e_type"])
					res.Error = strings.TrimSpace(fmt.Sprintf("%s\n%s", res.Error, errMsg))
					db.Error(errMsg)
				}

				historyInserted = append(historyInserted, fmt.Sprintf("(%s, '[%s]'::jsonb, %t)",
					notice["notice_room_id"],
					strings.Join([]string{sqlParam.ReplaceAllString(reqBody, "'$1"), sqlParam.ReplaceAllString(resBody, "'$1")}, ","), errMsg == ""),
				)
				errJob++
			}()
			i++
		}

		if i == 0 {
			stx.Commit()
			return api.ErrorHandlerThrow(c, fiber.StatusBadRequest, fmt.Errorf("%s not found", c.Params("roomName")))
		}

		for {
			time.Sleep(10 * time.Millisecond)
			if errJob >= i {
				break
			}
		}

		if len(historyInserted) != 0 {
			err = stx.Execute(fmt.Sprintf(`
			INSERT INTO "app"."notice_history" ("notice_room_id", "o_sender", "b_sended")
			VALUES %s;
		`, strings.Join(historyInserted, ",")))

			if db.IsRollback(err, stx) {
				return api.ErrorHandlerThrow(c, fiber.StatusInternalServerError, err)
			}
		}

		if err = stx.Commit(); db.IsRollback(err, stx) {
			return api.ErrorHandlerThrow(c, fiber.StatusInternalServerError, err)
		}

		return c.JSON(res)
	}
}
