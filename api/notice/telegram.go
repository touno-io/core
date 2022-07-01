package notice

import (
	"regexp"

	"github.com/go-resty/resty/v2"
	"github.com/gofiber/fiber/v2"
	jsoniter "github.com/json-iterator/go"
	"github.com/touno-io/core/db"
)

type TelegramProvider struct {
	Token string `json:"token"`
}
type TelegramRoom struct {
	Mode   string `json:"mode"`
	Name   string `json:"name"`
	ChatId int64  `json:"chatId"`
}

type TelegramRequest struct {
	Mode string `json:"parse_mode"`
	Text string `json:"text"`
}

type TelegramResponse struct {
	OK          bool        `json:"ok"`
	ErrorCode   int32       `json:"error_code,omitempty"`
	Description string      `json:"description,omitempty"`
	Result      interface{} `json:"result"`
}

const telegramAPI string = "https://api.telegram.org"
const jsonEmpty string = "{}"

func ProviderTelegram(c *fiber.Ctx, req *RequestNotice, notice db.PGRow) (string, string, error) {
	provider := new(TelegramProvider)
	room := new(TelegramRoom)
	json := jsoniter.ConfigCompatibleWithStandardLibrary

	if err := json.Unmarshal(notice.ToByte("provider"), provider); err != nil {
		return jsonEmpty, jsonEmpty, err
	}
	if err := json.Unmarshal(notice.ToByte("room"), room); err != nil {
		return jsonEmpty, jsonEmpty, err
	}

	client := resty.New()
	client.JSONMarshal = json.Marshal
	client.JSONUnmarshal = json.Unmarshal

	var fixText = regexp.MustCompile(`([-.])`)
	reqSender := &TelegramRequest{Mode: room.Mode, Text: fixText.ReplaceAllString(req.Message, `\$1`)}
	resSender := &TelegramResponse{}

	_, err := client.R().
		SetHeader("Content-Type", "application/json").
		SetBody(reqSender).SetResult(resSender).
		SetPathParams(map[string]string{
			"uri":     telegramAPI,
			"token":   provider.Token,
			"chat_id": string(rune(room.ChatId)),
		}).
		Post("{uri}/bot{token}/sendMessage?chat_id={chat_id}")

	var (
		reqBody []byte
		resBody []byte
	)

	if reqBody, err = json.Marshal(reqSender); err != nil {
		return jsonEmpty, jsonEmpty, err
	}

	if resBody, err = json.Marshal(resSender); err != nil {
		return jsonEmpty, jsonEmpty, err
	}

	return string(reqBody), string(resBody), err
}
