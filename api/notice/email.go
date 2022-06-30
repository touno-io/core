package notice

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/gofiber/fiber/v2"
	"github.com/touno-io/core/db"
	gomail "gopkg.in/mail.v2"
)

type EmailProvider struct {
	Port int    `json:"port"`
	SMTP string `json:"smtp"`
	User string `json:"username"`
	Pass string `json:"password"`
}

type EmailRoom struct {
	To     string `json:"to"`
	From   string `json:"from"`
	Prefix string `json:"subject_prefix"`
}

// type TelegramRequest struct {
// 	Mode string `json:"parse_mode"`
// 	Text string `json:"text"`
// }

// type TelegramResponse struct {
// 	OK          bool        `json:"ok"`
// 	ErrorCode   int32       `json:"error_code,omitempty"`
// 	Description string      `json:"description,omitempty"`
// 	Result      interface{} `json:"result"`
// }

// const telegramAPI string = "https://api.telegram.org"

func ProviderEmail(c *fiber.Ctx, notice db.PGRow) (string, string, error) {
	email := gomail.NewMessage()

	empty := "{}"
	provider := new(EmailProvider)
	room := new(EmailRoom)
	if err := json.Unmarshal(notice.ToByte("provider"), provider); err != nil {
		return empty, empty, err
	}
	if err := json.Unmarshal(notice.ToByte("room"), room); err != nil {
		return empty, empty, err
	}

	reqHead := c.GetReqHeaders()

	subjectMail := ""
	if reqHead["Content-Type"] == "text/html" {
		body := string(c.Body())
		rxtitle := regexp.MustCompile((`<title>.+?<\/`))
		subjectMail = strings.ReplaceAll(strings.ReplaceAll(rxtitle.FindString(body), `</`, ""), "<title>", "")
		email.SetBody("text/html", body)
	} else {
		email.SetBody("text/plain", string(c.Body()))
	}

	if reqHead["Subject"] != "" {
		subjectMail = reqHead["Subject"]
	}

	email.SetHeader("From", room.From)
	email.SetHeader("To", room.To)
	email.SetHeader("Subject", strings.TrimSpace(fmt.Sprintf("%s %s", room.Prefix, subjectMail)))

	// Set E-Mail sender
	// email.SetHeader("X-Priority", "1 (Highest)")
	// email.SetHeader("X-MSMail-Priority", "High")
	// email.SetHeader("X-Message-Flag", "Follow up")
	// email.SetHeader("Importance", "High")

	deliver := gomail.NewDialer(provider.SMTP, provider.Port, provider.User, provider.Pass)
	deliver.TLSConfig = &tls.Config{InsecureSkipVerify: true}

	if err := deliver.DialAndSend(email); err != nil {
		return empty, `{"error":"` + err.Error() + `"}`, err
	}

	return empty, empty, nil
}
