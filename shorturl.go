package main

import (
	"encoding/json"
	"fmt"
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

func handlerShortURL(c *fiber.Ctx) error {
	ipAddr := c.IP()
	IsLocalhost := ipAddr == "127.0.0.1" || ipAddr == "::1"
	if len(c.Params("hash")) != 4 || (IsLocalhost && appIsProduction) {
		return c.Status(fiber.StatusInternalServerError).SendString("Invalid URL redirect")
	}

	regHash, _ := regexp.Compile("^[a-zA-Z0-9]+")
	hashData := regHash.FindStringSubmatch(c.Params("hash"))
	stx, err := pgx.Begin()
	if err != nil {
		return err
	}
	short, err := stx.QueryOne("SELECT id,url FROM shorturl WHERE hash = $1", hashData[0])
	if stx.IsError(err) != nil {
		return c.SendString(err.Error())
	}

	raw := string(c.Request().Header.Header())
	regUserAgent, _ := regexp.Compile("User-Agent:.*?\n")
	header := regUserAgent.FindStringSubmatch(raw)
	agent := ua.Parse(strings.ReplaceAll(header[0], "User-Agent: ", ""))

	if IsLocalhost {
		ipAddr = ""
	}
	var res map[string]interface{}
	body, err := createClientHTTP("GET", fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,isp,proxy,hosting", ipAddr))
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

	if err := stx.Commit(); err != nil {
		return c.SendString(err.Error())
	}
	if appIsProduction {
		return c.Redirect(short["url"])
	} else {
		return c.SendString(
			fmt.Sprintf("url: %s\naddr: %s\nisp: %s\ncountry: %s\nproxy: %t\nhosting: %t\nos: %s\nbrowser: %s\nagent: %s",
				short["url"], c.IP(), res["isp"], res["country"], res["proxy"], res["hosting"], agent.OS, agent.Name, agent.String))
	}
}
