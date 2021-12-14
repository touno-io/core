package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
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
	Key     string `json:"key"`
	Name    string `json:"name"`
	Content string `json:"content"`
}

type ShortURL struct {
	URL     string    `json:"url"`
	Hash    string    `json:"hash"`
	Hit     int64     `json:"hit"`
	Created time.Time `json:"created"`
}

func generateSlug() string {
	var chars = []rune("0123456789abcdefghijklmnopqrstuvwxyz")
	s := make([]rune, 6)

	for i := range s {
		s[i] = chars[rand.Intn(len(chars))]
	}

	return string(s)
}

func handlerGetURL(c *fiber.Ctx) error {
	stx, err := pgx.Begin()
	if err != nil {
		return err
	}
	rows, err := stx.Query("SELECT url, hash, hit, created FROM shorturl")
	if stx.IsError(err) != nil {
		return c.SendString(err.Error())
	}
	url := []*ShortURL{}
	for rows.Next() {
		row, err := stx.FetchRow(rows)
		if stx.IsError(err) != nil {
			return c.SendString(err.Error())
		}

		url = append(url, &ShortURL{
			URL:     row["url"],
			Hash:    fmt.Sprintf("https://touno.io/s/%s", row["hash"]),
			Hit:     row.ToInt64("hit"),
			Created: row.ToTime("created"),
		})
	}

	if err := stx.Commit(); err != nil {
		return c.SendString(err.Error())
	}

	return c.JSON(url)
}

func handlerRedirectURL(c *fiber.Ctx) error {
	ipAddr := c.IP()
	IsLocalhost := ipAddr == "127.0.0.1" || ipAddr == "::1"
	if len(c.Params("hash")) != 4 || (IsLocalhost && appIsProduction) {
		return c.Status(fiber.StatusInternalServerError).SendString("Invalid URL redirect")
	}

	regHash, _ := regexp.Compile("^[a-z0-9]+")
	hashData := regHash.FindStringSubmatch(c.Params("hash"))
	stx, err := pgx.Begin()
	if err != nil {
		return err
	}
	short, err := stx.QueryOne("SELECT id,url FROM shorturl WHERE hash = $1", hashData[0])

	if stx.IsError(err) != nil {
		return c.SendString(err.Error())
	}

	// metaBody, err := fetch("GET", short["url"])
	// if stx.IsError(err) != nil {
	// 	return c.SendString(err.Error())
	// }

	// log.Printf("%s", string(metaBody))

	raw := string(c.Request().Header.Header())
	regUserAgent, _ := regexp.Compile("User-Agent:.*?\n")
	header := regUserAgent.FindStringSubmatch(raw)
	agent := ua.Parse(strings.ReplaceAll(header[0], "User-Agent: ", ""))

	if IsLocalhost {
		ipAddr = ""
	}
	var res map[string]interface{}
	body, err := fetch("GET", fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,isp,proxy,hosting", ipAddr))
	if stx.IsError(err) != nil {
		return c.SendString(err.Error())
	}

	if err := json.Unmarshal(body, &res); stx.IsError(err) != nil {
		return c.SendString(err.Error())
	}

	if res["status"] == "fail" {
		return c.SendString("IP can't is " + res["message"].(string))
	}
	if !agent.Bot {
		sAgent, err := json.Marshal(Agent{
			Name:    agent.Name,
			Version: agent.Version,
			IP:      c.IP(),
			Country: res["country"].(string),
			ISP:     res["isp"].(string),
			Proxy:   res["proxy"].(bool),
			Hosting: res["hosting"].(bool),
		})
		if stx.IsError(err) != nil {
			return c.SendString(err.Error())
		}

		sDevice, err := json.Marshal(Device{
			OS:        agent.OS,
			OSVersion: agent.Version,
			Mobile:    agent.Mobile,
			Tablet:    agent.Tablet,
			Desktop:   agent.Desktop,
			Bot:       agent.Bot,
		})
		if stx.IsError(err) != nil {
			return c.SendString(err.Error())
		}

		timeNow := time.Now()
		track, err := stx.QueryOne(`
			INSERT INTO shorturl_tracking (ip_addr,isp,country,proxy,hosting,visited)
			VALUES ($1,$2,$3,$4,$5,$6)
			ON CONFLICT ON CONSTRAINT uq_shorturl_tracking
			DO UPDATE SET
				isp = excluded.isp, country = excluded.country, proxy = excluded.proxy, hosting = excluded.hosting
			RETURNING visited
		;`, c.IP(), res["isp"].(string), res["country"].(string), res["proxy"].(bool), res["hosting"].(bool), timeNow.Format(time.RFC1123Z))
		if stx.IsError(err) != nil {
			return c.SendString(err.Error())
		}

		visited := track.ToTime("visited")

		if time.Since(visited).Minutes() > 30 || int16(timeNow.Sub(visited).Seconds()) < 1 {
			err = stx.Execute("UPDATE shorturl SET hit = hit + 1 WHERE hash = $1", hashData[0])
			if stx.IsError(err) != nil {
				return c.SendString(err.Error())
			}
			err := stx.Execute(`INSERT INTO shorturl_history (id, agent, device) VALUES ($1,$2,$3);`, short["id"], string(sAgent), string(sDevice))
			if stx.IsError(err) != nil {
				return c.SendString(err.Error())
			}
		}
	}

	nSeconds := 3
	if err := stx.Commit(); err != nil {
		return c.SendString(err.Error())
	}
	if appIsProduction {
		c.Response().Header.Add("Refresh", fmt.Sprintf("%d; url=%s", nSeconds, short["url"]))
	}

	return c.Render("index", fiber.Map{
		"Title":   "Hello, World!",
		"URL":     short["url"],
		"Seconds": nSeconds,
	})
}
