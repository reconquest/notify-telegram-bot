package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/reconquest/notify-telegram-bot/internal/printer"

	"github.com/reconquest/notify-telegram-bot/internal/transport"

	karma "github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"gopkg.in/tucnak/telebot.v2"
	tb "gopkg.in/tucnak/telebot.v2"
)

type UpdatedAndPreviousData struct {
	updatedData  interface{}
	previousData interface{}
}

type Coordinator struct {
	transport transport.Transport
	database  *Database
	config    *Config
	cache     map[int]UpdatedAndPreviousData
	channel   chan string
}

func NewCoordinator(
	transport transport.Transport,
	database *Database,
	config *Config,
) *Coordinator {
	return &Coordinator{
		transport: transport,
		database:  database,
		config:    config,
	}
}

func (coordinator *Coordinator) start(message *tb.Message) error {
	text := "Hi! I am a telegram bot and I can notify you about all changes" +
		" in any json data fields by url, if url unavailable " +
		"I'll let you know." +
		"All commands in bot:\n\n/start - start bot" +
		"\n\n/help - show commands\n\n/list - show list" +
		" with your subscriptions.\n\n" +
		"/subscribe url duration json-key.nested-key,second-key - " +
		"subscribe\n\nExample: / subscribe http://time.jsontest.com/ 1h date,time\n\n" +
		"/unsubscribe subscriptionID - unsubscribe from one selected " +
		"subscription\n\nExample: /unsubscribe 5e7891f34940ad7f3746e2dd\n\n" +
		"/stop - unsubscribe from all subscriptions"

	var recipient telebot.Recipient
	var recipientID int
	if message.Chat != nil {
		recipient = message.Chat
		recipientID = int(message.Chat.ID)
	} else {
		recipient = message.Sender
		recipientID = message.Sender.ID
	}

	err := coordinator.transport.SendMessage(recipient, text)
	if err != nil {
		return karma.Format(err, "unable to send message to user: %d ",
			recipientID)
	}

	return nil
}

func (coordinator *Coordinator) createFirstMessageAfterSubscribe(
	subscriber *Subscriber,
) ([]string, error) {
	url := subscriber.URL
	keys := strings.Split(subscriber.Keys, ",")
	data, err := getJSON(url)
	if err != nil {
		if err == errorResponse {
			return nil, errorResponse
		}

		return nil, err
	}

	var messageWithData []string
	var notification string
	isAddedID := false
	for _, key := range keys {
		nestedKey := strings.Split(key, ".")
		record, err := getValueByKey(data, nestedKey)
		if err != nil {
			log.Debugf(nil, "unable to get data by key, key = %s", nestedKey)
		}

		if record == nil {
			continue
		}

		preparedMessage := printer.String(record)
		if isAddedID == false {
			notification = fmt.Sprintf(
				"ID - %s\n\n%v",
				subscriber.ID.Hex(),
				preparedMessage,
			)
			isAddedID = true
		} else {
			notification = fmt.Sprintf(
				"%v",
				preparedMessage,
			)
		}

		messageWithData = append(messageWithData, notification)
	}

	return messageWithData, nil
}

func (coordinator *Coordinator) subscribe(message *tb.Message) error {
	payload := strings.Split(message.Payload, " ")
	senderID := message.Sender.ID
	sender := message.Sender
	var chat *tb.Chat

	chatID := validateIsChatID(message)
	if chatID != 0 {
		senderID = chatID
		chat = message.Chat
		sender = nil
	}

	var recipient telebot.Recipient
	if message.Chat != nil {
		recipient = message.Chat

	} else {
		recipient = message.Sender
	}

	if len(payload) != 3 {
		text := "Data required!\n" +
			"In format:  /subscribe url duration json-key.nested-key,second-key"
		err := coordinator.transport.SendMessage(recipient, text)
		if err != nil {
			return karma.Format(err, "unable to send message to user")
		}

		return nil
	}

	var (
		err         error
		endpointURL = payload[0]
		duration    = payload[1]
		keys        = payload[2]
	)

	if !isValidURL(endpointURL) {
		errMessage := "You wrote the wrong url"
		err = coordinator.transport.SendMessage(recipient, errMessage)
		if err != nil {
			return karma.Format(err, "unable to send message to user")
		}

		return nil
	}

	refreshDuration, err := time.ParseDuration(duration)
	if err != nil {
		errMessage := "Your write incorrect duration"
		err = coordinator.transport.SendMessage(recipient, errMessage)
		if err != nil {
			return karma.Format(err, "unable to send message to user")
		}
		return nil
	}

	subscriber := Subscriber{
		URL:      endpointURL,
		UserID:   senderID,
		Duration: refreshDuration,
		Sender:   sender,
		Chat:     chat,
		Keys:     keys,
	}

	endpoint := &Endpoint{
		URL:       endpointURL,
		Duration:  refreshDuration,
		RefreshAt: time.Now(),
		Response:  true,
		UpdatedAt: time.Now(),
	}

	foundSubscriber, err := coordinator.database.findSubscriber(
		subscriber.UserID,
		subscriber.URL,
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to find subscriber in the database",
		)
	}

	switch foundSubscriber {
	case nil:
		err = coordinator.database.upsertSubscriber(subscriber)
		if err != nil {
			return karma.Format(
				err,
				"unable to upsert subsrciber, subscriber_id: %d",
				subscriber.UserID,
			)
		}

		err = coordinator.database.writeEndpoint(endpoint)
		if err != nil {
			return karma.Format(
				err,
				"unable to upsert endpoint, endpoint_url: %s",
				endpoint.URL,
			)
		}

		foundSubscriber, err := coordinator.database.findSubscriber(
			subscriber.UserID,
			subscriber.URL,
		)
		if err != nil {
			return karma.Format(
				err,
				"unable to find subscriber in the database",
			)
		}

		var message []string
		message, err = coordinator.createFirstMessageAfterSubscribe(foundSubscriber)
		if err != nil && err != errorResponse {
			return karma.Format(
				err,
				"unable to getfirst data after subscribe: %s",
				endpoint.URL,
			)
		}

		if err == errorResponse {
			message = []string{"\nURL is unavailable!"}
		}

		err = coordinator.transport.SendMessage(
			recipient,
			"You successfully subscribed!\n"+strings.Join(
				message, "\n\n"),
		)

		if err != nil {
			return karma.Format(
				err,
				"unable to send message to user_id: %d",
				senderID,
			)
		}

	default:
		if foundSubscriber.Duration == refreshDuration {
			err = coordinator.transport.SendMessage(
				recipient,
				"You have already subscribed on this URL with same duration",
			)
			if err != nil {
				return karma.Format(
					err,
					"unable to send message to user_id: %d",
					senderID,
				)
			}
		} else {
			err = coordinator.database.upsertSubscriber(subscriber)
			if err != nil {
				return karma.Format(
					err,
					"unable to upsert subsrciber, subscriber_id: %d",
					subscriber.UserID,
				)
			}

			err = coordinator.database.writeEndpoint(endpoint)
			if err != nil {
				return karma.Format(
					err,
					"unable to upsert endpoint, endpoint_url: %s",
					endpoint.URL,
				)
			}

			err = coordinator.transport.SendMessage(
				recipient,
				"Duration was successfully updated",
			)
			if err != nil {
				return karma.Format(
					err,
					"unable to send message to user_id: %d",
					senderID,
				)
			}
		}
	}

	return nil
}

func (coordinator *Coordinator) stop(message *tb.Message) error {
	var err error
	var resultsOfUser []Subscriber
	var resultsDB []Subscriber

	var recipient telebot.Recipient
	var recipientID int
	if message.Chat != nil {
		recipient = message.Chat
		recipientID = int(message.Chat.ID)

	} else {
		recipient = message.Sender
		recipientID = message.Sender.ID

	}

	findURL := bson.M{"userid": recipientID}
	cursorURL, _ := coordinator.database.Subscriptions.Find(
		coordinator.database.context,
		findURL,
	)
	err = cursorURL.All(coordinator.database.context, &resultsOfUser)
	if err != nil {
		return karma.Format(err, "unable to decode data")
	}

	if len(resultsOfUser) == 0 {
		err = coordinator.transport.SendMessage(recipient, "You don't have any subscriptions")
		if err != nil {
			return karma.Format(err, "unable to send message to user: %d ",
				recipientID)
		}

		return nil
	}

	searchForMatches := bson.M{
		"url":    bson.M{"$exists": true, "$ne": nil},
		"userid": bson.M{"$ne": recipientID},
	}

	cursorSub, err := coordinator.database.Subscriptions.Find(
		coordinator.database.context,
		searchForMatches,
	)
	if err != nil {
		return karma.Format(err, "unable to find in database")
	}

	err = cursorSub.All(coordinator.database.context, &resultsDB)
	if err != nil {
		return karma.Format(err, "unable to decode data")
	}

	if len(resultsDB) == 0 {
		for _, result := range resultsOfUser {
			endpointsFilter := bson.M{"url": result.URL}
			_, err = coordinator.database.Endpoints.DeleteMany(
				coordinator.database.context,
				endpointsFilter,
			)
			if err != nil {
				return karma.Format(err, "unable to delete data in collection")
			}
		}
	}

	_, err = coordinator.database.Subscriptions.DeleteMany(
		coordinator.database.context,
		findURL,
	)
	if err != nil {
		return karma.Format(err, "unable to delete data in collection")
	}

	textmessage := "All notifications stopped"
	err = coordinator.transport.SendMessage(recipient, textmessage)
	if err != nil {
		return karma.Format(
			err,
			"unable to send message to user: %d ",
			recipientID)
	}

	log.Debugf(
		nil,
		"unsubscribed from all subscriptions, recipient_id: %s ",
		strconv.Itoa(recipientID),
	)
	return nil
}

func (coordinator *Coordinator) list(message *tb.Message) error {

	var recipient telebot.Recipient
	var recipientID int
	if message.Chat != nil {
		recipient = message.Chat
		recipientID = int(message.Chat.ID)

	} else {
		recipient = message.Sender
		recipientID = message.Sender.ID

	}

	filter := bson.M{"userid": recipientID}

	var results []Subscriber
	cursor, err := coordinator.database.Subscriptions.Find(coordinator.database.context, filter)
	if err != nil {
		return karma.Format(err, "unable to find data in Subscription collection ")
	}

	err = cursor.All(coordinator.database.context, &results)
	if err != nil {
		return karma.Format(err, "unable to decode data")
	}

	var text []string
	if len(results) == 0 {
		err = coordinator.transport.SendMessage(recipient, "You don't have any subscriptions")
		if err != nil {
			return karma.Format(
				err,
				"unable to send message to user: %d ",
				recipientID,
			)
		}

		return nil
	}

	for _, res := range results {
		text = append(text, fmt.Sprintf(
			"\nID - %s\nURL - %s\nDURATION - %s\nJSON KEY - %v",
			res.ID.Hex(),
			res.URL,
			res.Duration.String(),
			res.Keys,
		))
	}

	textmessage := strings.Join(text, "\n")
	err = coordinator.transport.SendMessage(recipient, "\nMy subscriptions:\n"+textmessage)
	if err != nil {
		return karma.Format(err, "unable to send message to user: %d ",
			recipientID,
		)
	}

	return nil
}

func (coordinator *Coordinator) unsubscribe(message *tb.Message) error {
	var err error
	var resultsDB []Subscriber
	var resultsOfUser []Subscriber

	var recipient telebot.Recipient
	var recipientID int
	if message.Chat != nil {
		recipient = message.Chat
		recipientID = int(message.Chat.ID)

	} else {
		recipient = message.Sender
		recipientID = message.Sender.ID

	}

	subscritptionID, _ := primitive.ObjectIDFromHex(string(message.Payload))
	findSub := bson.M{
		"_id":    subscritptionID,
		"userid": recipientID,
	}

	cursorURL, _ := coordinator.database.Subscriptions.Find(coordinator.database.context, findSub)
	err = cursorURL.All(coordinator.database.context, &resultsOfUser)
	if err != nil {
		return karma.Format(err, "unable to decode data")
	}

	if len(resultsOfUser) == 0 {
		err = coordinator.transport.SendMessage(
			recipient,
			"You don't have subscription with this id",
		)
		if err != nil {
			return karma.Format(err, "unable to send message to user: %d ",
				recipientID)
		}

		return karma.Format(err, "no this subscription in database")
	}

	searchForMatches := bson.M{
		"url":    bson.M{"$exists": true, "$ne": nil},
		"userid": bson.M{"$ne": recipientID},
	}

	cursorSub, _ := coordinator.database.Subscriptions.Find(coordinator.database.context,
		searchForMatches)
	err = cursorSub.All(coordinator.database.context, &resultsDB)
	if err != nil {
		return karma.Format(err, "unable to decode data")
	}

	deletingEndpoints := bson.M{"url": resultsOfUser[0].URL}
	if len(resultsDB) == 0 {
		_, err = coordinator.database.Endpoints.DeleteMany(coordinator.database.context,
			deletingEndpoints)
		if err != nil {
			return karma.Format(err, "unable to delete data in collection")
		}
	}

	_, err = coordinator.database.Subscriptions.DeleteOne(coordinator.database.context, findSub)
	if err != nil {
		return karma.Format(err, "unable to delete data in collection")
	}

	var messageWithData []string

	notification := fmt.Sprintf(
		"ID - %s\nURL - %s\nJSON KEY - %s\n\n",
		message.Payload,
		resultsOfUser[0].URL,
		resultsOfUser[0].Keys,
	)
	messageWithData = append(messageWithData, "Unsubscribed:\n"+notification)
	err = coordinator.transport.SendMessage(
		recipient,
		strings.Join(
			messageWithData, "\n\n"),
	)

	if err != nil {
		return karma.Format(err, "unable to send message to user: %d ",
			recipientID)
	}

	return nil
}
