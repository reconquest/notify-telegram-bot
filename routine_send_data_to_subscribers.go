package main

import (
	"daniil/notify-telegram-bot/internal/printer"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/globalsign/mgo/bson"
	karma "github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

func (coordinator *Coordinator) routineSendDataToSubscribers() error {
	var subscribers []Subscriber

	cursor, err := coordinator.database.Subscriptions.Find(
		coordinator.database.context,
		bson.M{"send_at": bson.M{"$lt": time.Now()}},
	)
	if err != nil {
		return karma.Format(err, "unable to find subscription data")
	}

	err = cursor.All(coordinator.database.context, &subscribers)
	if err != nil {
		return karma.Format(err, "unable to decode mongodb data")
	}

	for _, subscriber := range subscribers {
		err := coordinator.sendDataToSubscriber(subscriber)
		if err != nil {
			log.Errorf(
				nil,
				"unable to send data from endpoint to user: %s, %v",
				err, subscriber.UserID,
			)
		}
	}

	return nil
}

func (coordinator *Coordinator) sendDataToSubscriber(subscriber Subscriber) error {
	defer func() {
		if err := recover(); err != nil {
			log.Error(
				err,
			)
		}
	}()

	if subscriber.Chat != nil {
		subscriber.Recipient = subscriber.Chat
		subscriber.RecipientID = int(subscriber.Chat.ID)
	} else {
		subscriber.Recipient = subscriber.Sender
		subscriber.RecipientID = subscriber.Sender.ID
	}

	endpointFilter := bson.M{
		"url":      subscriber.URL,
		"duration": subscriber.Duration,
	}
	cursor, err := coordinator.database.Endpoints.Find(
		coordinator.database.context,
		endpointFilter,
	)
	if err != nil {
		return karma.Format(err, "unable to find subscription data")
	}

	var endpoints []Endpoint
	err = cursor.All(coordinator.database.context, &endpoints)
	if err != nil {
		return karma.Format(err, "unable to decode data from database")
	}

	if len(endpoints) == 0 {
		return fmt.Errorf(
			"endpoints is empty, "+
				"url = %s; duration = %s",
			subscriber.URL, subscriber.Duration)
	}

	// send message if not response by url
	if endpoints[0].Response == false {
		err := coordinator.sendMessageAboutUnavailableURL(
			subscriber,
		)
		if err != nil {
			return karma.Format(err, "unable to send message")
		}

		err = coordinator.database.updateSubscriber(subscriber)
		if err != nil {
			return karma.Format(err, "unable to update subscriber data in database")
		}

		log.Debugf(
			nil,
			"waiting response from url: %s",
			subscriber.URL,
		)

		return nil
	}

	keys := strings.Split(subscriber.Keys, ",")

	// default case
	messageWithData := coordinator.prepareMessageForSubscriber(
		keys,
		endpoints,
		subscriber,
	)

	if messageWithData == nil {
		return nil
	}

	err = coordinator.transport.SendMessage(
		subscriber.Recipient,
		strings.Join(
			messageWithData, "\n\n"),
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to send message to user: %d",
			subscriber.Recipient,
		)
	}

	err = coordinator.database.updateSubscriber(subscriber)
	if err != nil {
		return karma.Format(err, "unable to update subscriber data in the database")
	}

	err = coordinator.database.updateSubscriberStatus(
		subscriber.UserID,
		subscriber.URL,
		endpoints[0].UpdatedAt,
	)
	if err != nil {
		return karma.Format(err, "unable to update subscriber status in database")
	}

	return nil
}

func (coordinator *Coordinator) prepareMessageForSubscriber(
	keys []string,
	endpoints []Endpoint,
	subscriber Subscriber,
) []string {
	var messageWithData []string
	var notification string
	isAddedID := false
	for _, key := range keys {
		nestedKey := strings.Split(key, ".")
		updatedData, err := getValueByKey(endpoints[0].Data, nestedKey)
		if err != nil {
			log.Errorf(nil, "unable to get data by key, key = %s", nestedKey)
		}

		previousData, err := getValueByKey(endpoints[0].PreviousData, nestedKey)
		if err != nil {
			log.Errorf(nil, "unable to get data by key, key = %s", nestedKey)
		}

		if previousData == nil || endpoints[0].UpdatedAt == subscriber.UpdatedAt {
			return nil
		}

		if reflect.DeepEqual(updatedData, previousData) ||
			updatedData == nil {
			continue
		}

		preparedMessage := printer.String(updatedData)

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

	return messageWithData
}

func (coordinator *Coordinator) sendMessageAboutUnavailableURL(
	subscriber Subscriber,
) error {
	var text []string
	message := "URL is unavailable!"
	text = append(text, fmt.Sprintf(
		"\nID - %s\nURL - %s\n\n%s",
		subscriber.ID.Hex(),
		subscriber.URL,
		message,
	))

	err := coordinator.transport.SendMessage(
		subscriber.Recipient,
		strings.Join(
			text, "\n\n"),
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to send message to user: %d",
			subscriber.Recipient,
		)
	}

	return nil
}
