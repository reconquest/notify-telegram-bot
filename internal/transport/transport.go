package transport

import tb "gopkg.in/tucnak/telebot.v2"

type Transport interface {
	SendMessage(tb.Recipient, string) error
}
