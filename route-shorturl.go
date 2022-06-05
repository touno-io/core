package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	ua "github.com/mileusna/useragent"
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

func hashRandomSlug() string {
	var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
	s := make([]rune, 4)

	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}

	return string(s)
}

func handlerGetURL(c *fiber.Ctx) error {
	stx, err := pgx.Begin()
	if err != nil {
		return c.SendString(err.Error())
	}
	rows, err := stx.Query("SELECT url, hash, hit, created FROM shorturl")
	if IsRollback(err, stx) {
		return c.SendString(err.Error())
	}
	url := []*ShortURL{}
	for rows.Next() {
		row, err := stx.FetchRow(rows)
		if IsRollback(err, stx) {
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

type NewURL struct {
	URL     string    `json:"url"`
	Hash    string    `json:"hash"`
	Created time.Time `json:"created"`
}

func handlerAddURL(c *fiber.Ctx) error {
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
	if IsRollback(err, stx) {
		return c.SendString(err.Error())
	}

	if short.ToInt64("item") > 0 {
		return c.SendString("URL exists.")
	}
	hashKey := hashRandomSlug()

	err = stx.Execute("INSERT INTO shorturl (hash,url) VALUES ($1,$2)", hashKey, body.URL)
	if IsRollback(err, stx) {
		return c.SendString(err.Error())
	}

	if err := stx.Commit(); err != nil {
		return c.SendString(err.Error())
	}
	body.Hash = fmt.Sprintf("/s/%s", hashKey)
	body.Created = time.Now()
	return c.JSON(body)
}

func fiberError(message string) fiber.Map {
	return fiber.Map{
		"Title": "Redirected",
		"URL":   "",
		"Meta":  "[]",
		"Error": message,
	}
}

func handlerRedirectURL(c *fiber.Ctx) error {
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
	if IsRollback(err, stx) {
		return c.Render("short-url", fiberError("Invalid URL redirect"))
	}

	// metaBody, err := fetch("GET", short["url"])
	// if asa.IsRollback(err, stx) {
	// 	return c.Render("short-url", fiberError(err.Error()))
	// }

	raw := string(c.Request().Header.Header())
	regUserAgent, _ := regexp.Compile("(?i)user-agent:(.*?)\n")
	hAgent := regUserAgent.FindStringSubmatch(raw)
	agent := ua.Parse(strings.TrimSpace(hAgent[1]))

	regCfIP, _ := regexp.Compile("(?i)cf-connecting-ip:(.*?)\n")
	connectingIp := regCfIP.FindStringSubmatch(raw)

	if len(connectingIp) > 0 {
		ipAddr = strings.TrimSpace(connectingIp[1])
	} else if ipAddr == "127.0.0.1" || ipAddr == "::1" {
		ipAddr = os.Getenv("IP_LOCALHOST")
	}

	var res map[string]interface{}
	body, err := fetch("GET", fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,isp,proxy,hosting", ipAddr))
	if IsRollback(err, stx) {
		return c.Render("short-url", fiberError(err.Error()))
	}

	if err := json.Unmarshal(body, &res); IsRollback(err, stx) {
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
	if IsRollback(err, stx) {
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
	if IsRollback(err, stx) {
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
	if IsRollback(err, stx) {
		return c.Render("short-url", fiberError(err.Error()))
	}

	visited := track.ToTime("visited")

	if time.Since(visited).Minutes() > 30 || int16(timeNow.Sub(visited).Seconds()) < 1 {
		err = stx.Execute("UPDATE shorturl SET hit = hit + 1 WHERE hash = $1", hashKey)
		if IsRollback(err, stx) {
			return c.Render("short-url", fiberError(err.Error()))
		}
		err = stx.Execute("UPDATE shorturl_tracking SET visited = $2 WHERE hash = $1", hashKey, timeNow.Format(time.RFC1123Z))
		if IsRollback(err, stx) {
			return c.Render("short-url", fiberError(err.Error()))
		}

		err := stx.Execute(`INSERT INTO shorturl_history (hash, agent, device) VALUES ($1,$2,$3);`, hashKey, string(sAgent), string(sDevice))
		if IsRollback(err, stx) {
			return c.Render("short-url", fiberError(err.Error()))
		}
	}

	nSeconds := 3
	if err := stx.Commit(); err != nil {
		return c.Render("short-url", fiberError(err.Error()))
	}
	if appIsProduction {
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