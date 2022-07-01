package shorturl

import (
	"fmt"
	"math/rand"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	ua "github.com/mileusna/useragent"
	"github.com/touno-io/core/api"
	"github.com/touno-io/core/db"
)

type Agent struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	IP      string `json:"ip"`
	Country string `json:"country"`
	ISP     string `json:"isp"`
	Proxy   bool   `json:"proxy"`
	Hosting bool   `json:"hosting"`
}
type Device struct {
	OS        string `json:"os"`
	OSVersion string `json:"osversion"`
	Mobile    bool   `json:"mobile"`
	Tablet    bool   `json:"tablet"`
	Desktop   bool   `json:"desktop"`
	Bot       bool   `json:"bot"`
}

type Meta struct {
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ShortURL struct {
	URL     string    `json:"url"`
	Hash    string    `json:"hash"`
	Hit     int64     `json:"hit"`
	Created time.Time `json:"created"`
}

type NewURL struct {
	URL     string    `json:"url"`
	Hash    string    `json:"hash"`
	Created time.Time `json:"created"`
}

func hashRandomSlug() string {
	var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	s := make([]rune, 4)

	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}

	return string(s)
}

func HandlerGetURL(pgx *db.PGClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		stx, err := pgx.Begin()
		if err != nil {
			return c.SendString(err.Error())
		}
		rows, err := stx.Query("SELECT url, hash, hit, created FROM shorturl ORDER BY created DESC")
		if db.IsRollback(err, stx) {
			return c.SendString(err.Error())
		}
		url := []*ShortURL{}
		for rows.Next() {
			row, err := stx.FetchRow(rows)
			if db.IsRollback(err, stx) {
				return c.SendString(err.Error())
			}

			url = append(url, &ShortURL{
				URL:     row["url"],
				Hash:    fmt.Sprintf("/s/%s", row["hash"]),
				Hit:     row.ToInt64("hit"),
				Created: row.ToTime("created"),
			})
		}

		if err := stx.Commit(); err != nil {
			return c.SendString(err.Error())
		}

		return c.JSON(url)
	}
}

func HandlerAddURL(pgx *db.PGClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		body := NewURL{}
		err := c.BodyParser(&body)
		if err != nil {
			return c.SendString(err.Error())
		}

		if body.URL == "" {
			return c.SendString("URL empty.")
		}

		_, err = url.Parse(body.URL)
		if err != nil {
			return c.SendString(err.Error())
		}

		stx, err := pgx.Begin()
		if err != nil {
			return c.SendString(err.Error())
		}

		short, err := stx.QueryOne("SELECT COUNT(*) item FROM shorturl WHERE url = $1", body.URL)
		if db.IsRollback(err, stx) {
			return c.SendString(err.Error())
		}

		if short.ToInt64("item") > 0 {
			return c.SendString("URL exists.")
		}
		hashKey := hashRandomSlug()

		err = stx.Execute("INSERT INTO shorturl (hash,url) VALUES ($1,$2)", hashKey, body.URL)
		if db.IsRollback(err, stx) {
			return c.SendString(err.Error())
		}

		if err := stx.Commit(); err != nil {
			return c.SendString(err.Error())
		}
		body.Hash = fmt.Sprintf("/s/%s", hashKey)
		body.Created = time.Now()
		return c.JSON(body)
	}
}

func fiberError(message string) fiber.Map {
	return fiber.Map{
		"Title": "Redirected",
		"URL":   "",
		"Meta":  "[]",
		"Error": message,
	}
}

func HandlerRedirectURL(pgx *db.PGClient) func(c *fiber.Ctx) error {
	return func(c *fiber.Ctx) error {
		regHash, _ := regexp.Compile("^[a-zA-Z0-9]+")

		ipAddr := c.IP()
		if len(c.Params("hash")) != 4 {
			return c.Render("short-url", fiberError("Invalid URL redirect"))
		}

		hashKey := regHash.FindStringSubmatch(c.Params("hash"))[0]
		stx, err := pgx.Begin()
		if err != nil {
			return c.Render("short-url", fiberError(err.Error()))
		}

		short, err := stx.QueryOne("SELECT title, meta, url FROM shorturl WHERE hash = $1", hashKey)
		if db.IsRollback(err, stx) {
			return c.Render("short-url", fiberError("Invalid URL redirect"))
		}

		raw := string(c.Request().Header.Header())
		regUserAgent, _ := regexp.Compile("(?i)user-agent:(.*?)\n")
		hAgent := regUserAgent.FindStringSubmatch(raw)
		agent := ua.Parse(strings.TrimSpace(hAgent[1]))

		regCfIP, _ := regexp.Compile("(?i)cf-connecting-ip:(.*?)\n")
		connectingIp := regCfIP.FindStringSubmatch(raw)

		if len(connectingIp) > 0 {
			ipAddr = strings.TrimSpace(connectingIp[1])
		}

		if ipAddr == "127.0.0.1" || ipAddr == "::1" {
			ipAddr = "touno.io"
		}

		json := jsoniter.ConfigCompatibleWithStandardLibrary

		client := resty.New()
		client.JSONMarshal = json.Marshal
		client.JSONUnmarshal = json.Unmarshal

		var res map[string]interface{}
		_, err = client.R().
			SetHeader("Content-Type", "application/json").
			SetResult(&res).
			SetPathParams(map[string]string{"ipAddr": ipAddr}).
			SetError(&res).
			Post("http://ip-api.com/json/{ipAddr}?fields=status,country,isp,proxy,hosting")

		if db.IsRollback(err, stx) {
			return c.Render("short-url", fiberError(err.Error()))
		}

		if res["status"] == "fail" {
			return c.SendString(fmt.Sprintf("%s\n\nIP: %s can't is %+v", raw, ipAddr, res))
		}

		sAgent, err := json.Marshal(Agent{
			Name:    agent.Name,
			Version: agent.Version,
			IP:      ipAddr,
			Country: res["country"].(string),
			ISP:     res["isp"].(string),
			Proxy:   res["proxy"].(bool),
			Hosting: res["hosting"].(bool),
		})
		if db.IsRollback(err, stx) {
			return c.Render("short-url", fiberError(err.Error()))
		}

		sDevice, err := json.Marshal(Device{
			OS:        agent.OS,
			OSVersion: agent.Version,
			Mobile:    agent.Mobile,
			Tablet:    agent.Tablet,
			Desktop:   agent.Desktop,
			Bot:       agent.Bot,
		})
		if db.IsRollback(err, stx) {
			return c.Render("short-url", fiberError(err.Error()))
		}

		timeNow := time.Now()
		track, err := stx.QueryOne(`
			INSERT INTO shorturl_tracking AS st (ip_addr,hash,isp,country,proxy,hosting,visited)
			VALUES ($1,$2,$3,$4,$5,$6,$7)
			ON CONFLICT ON CONSTRAINT uq_shorturl_tracking
			DO UPDATE SET
				isp = excluded.isp, country = excluded.country, proxy = excluded.proxy, hosting = excluded.hosting, hit = st.hit + 1
			RETURNING visited
		;`, ipAddr, hashKey, res["isp"].(string), res["country"].(string), res["proxy"].(bool), res["hosting"].(bool), timeNow.Format(time.RFC1123Z))
		if db.IsRollback(err, stx) {
			return c.Render("short-url", fiberError(err.Error()))
		}

		visited := track.ToTime("visited")

		if time.Since(visited).Minutes() > 30 || int16(timeNow.Sub(visited).Seconds()) < 1 {
			err = stx.Execute("UPDATE shorturl SET hit = hit + 1 WHERE hash = $1", hashKey)
			if db.IsRollback(err, stx) {
				return c.Render("short-url", fiberError(err.Error()))
			}
			err = stx.Execute("UPDATE shorturl_tracking SET visited = $2 WHERE hash = $1", hashKey, timeNow.Format(time.RFC1123Z))
			if db.IsRollback(err, stx) {
				return c.Render("short-url", fiberError(err.Error()))
			}

			err := stx.Execute(`INSERT INTO shorturl_history (hash, agent, device) VALUES ($1,$2,$3);`, hashKey, string(sAgent), string(sDevice))
			if db.IsRollback(err, stx) {
				return c.Render("short-url", fiberError(err.Error()))
			}
		}

		nSeconds := 2
		if err := stx.Commit(); err != nil {
			return c.Render("short-url", fiberError(err.Error()))
		}
		if api.IsProduction {
			c.Response().Header.Add("Refresh", fmt.Sprintf("%d; url=%s", nSeconds, short["url"]))
		}

		// <meta name="title" content="asdasdasdasdasd">
		// <meta name="description" content="asdasdasdasdasd">
		// <meta name="keywords" content="asd">
		// <meta name="robots" content="noindex, nofollow">
		// <meta name="language" content="English">

		return c.Render("short-url", fiber.Map{
			"Title": short["title"],
			"URL":   short["url"],
			"Meta":  short["meta"],
			"Error": "",
		})
	}
}
