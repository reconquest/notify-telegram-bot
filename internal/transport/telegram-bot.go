package transport

import (
	"github.com/reconquest/pkg/log"
	tb "gopkg.in/tucnak/telebot.v2"
)

type Telegram struct {
	bot *tb.Bot
}

type Recipient struct {
	recipient tb.Recipient
}

func NewBot(bot *tb.Bot) *Telegram {
	return &Telegram{
		bot: bot,
	}
}

func (telegram *Telegram) SendMessage(recipient tb.Recipient, message string) error {
	_, err := telegram.bot.Send(recipient, message)
	if err != nil {
		return err
	}

	return nil
}

func (telegram *Telegram) Handle(
	cmd string,
	fn func(*tb.Message) error,
) {
	telegram.bot.Handle(cmd, func(message *tb.Message) {
		err := fn(message)
		if err != nil {
			log.Infof(nil, "error while processing %s: %s", cmd, err)
		}
	})
}
