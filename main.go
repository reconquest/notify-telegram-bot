package main

import (
	"context"
	"time"

	"github.com/reconquest/notify-telegram-bot/internal/transport"

	"github.com/docopt/docopt-go"
	karma "github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	tb "gopkg.in/tucnak/telebot.v2"
)

var version = "[manual build]"

var usage = `notify-telegram-bot

Get json data and send notifications by json-key.

Usage:
  notify-telegram-bot [options]

Options:
  -c --config <path>  Read specified config file. [default: config.toml]
  --debug             Enable debug messages.
  -v --version        Print version.
  -h --help           Show this help.
`

func main() {
	args, err := docopt.ParseArgs(
		usage,
		nil,
		"notify-telegram-bot "+version,
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof(
		karma.Describe("version", version),
		"notify-telegram-bot started",
	)

	if args["--debug"].(bool) {
		log.SetLevel(log.LevelDebug)
	}

	log.Infof(nil, "loading configuration file: %q", args["--config"].(string))

	config, err := LoadConfig(args["--config"].(string))
	if err != nil {
		log.Fatal(err)
	}

	log.Infof(nil, "creating telegram bot")

	bot, err := tb.NewBot(
		tb.Settings{
			Token:  config.TelegramBotToken,
			Poller: &tb.LongPoller{Timeout: 10 * time.Second},
		},
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Infof(nil, "connecting to the database")
	database, err := NewDatabase(
		config.DatabaseURI,
		config.DatabaseName,
		context.Background(),
	)
	if err != nil {
		log.Fatal(err)
	}

	telegramBot := transport.NewBot(bot)

	coordinator := NewCoordinator(telegramBot, database, config)
	coordinator.cache = nil

	go func() {
		log.Info("start cycle with updating endpoints")
		for {
			err := coordinator.routineUpdateEndpoints()
			if err != nil {
				log.Error(err)
			}

			time.Sleep(1 * time.Second)
		}
	}()

	go func() {
		log.Info("start cycle with sending data to subscriber")
		for {
			err := coordinator.routineSendDataToSubscribers()
			if err != nil {
				log.Error(err)
			}

			time.Sleep(2 * time.Second)
		}
	}()

	go func() {
		log.Info("start cycle with cleaning unused endpoints")
		for {
			err := coordinator.routineCleanEndpoints()
			if err != nil {
				log.Error(err)
			}

			time.Sleep(60 * time.Second)
		}
	}()

	telegramBot.Handle("/start", coordinator.start)
	telegramBot.Handle("/help", coordinator.start)
	telegramBot.Handle("/subscribe", coordinator.subscribe)
	telegramBot.Handle("/stop", coordinator.stop)
	telegramBot.Handle("/list", coordinator.list)
	telegramBot.Handle("/unsubscribe", coordinator.unsubscribe)

	log.Infof(nil, "starting to listen and serve telegram bot")
	bot.Start()
}
