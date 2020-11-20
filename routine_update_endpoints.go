package main

import (
	"errors"
	"time"

	karma "github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
	"go.mongodb.org/mongo-driver/bson"
	tb "gopkg.in/tucnak/telebot.v2"
)

func (coordinator *Coordinator) routineUpdateEndpoints() error {
	var endpoints []Endpoint
	filter := bson.M{"refresh_at": bson.M{"$lt": time.Now()}}

	cursor, err := coordinator.database.Endpoints.Find(coordinator.database.context, filter)
	if err != nil {
		return karma.Format(err, "unable to find in endpoints collection")
	}

	err = cursor.All(coordinator.database.context, &endpoints)
	if err != nil {
		return karma.Format(err, "unable to decode mongodb data")
	}

	for _, endpoint := range endpoints {
		err := coordinator.updateEndpoint(endpoint)

		if err != nil {
			log.Errorf(
				err,
				"unable to update endpoint data endpoint_id: %s", endpoint.ID.Hex(),
			)
		}
	}

	return nil
}

func (coordinator *Coordinator) updateEndpoint(endpoint Endpoint) error {
	log.Debugf(
		nil,
		"start endpoint %v data refresh\n",
		endpoint.ID,
	)
	data, err := getJSON(endpoint.URL)
	if err != nil {
		if err == errorResponse && endpoint.Response == true {
			err = coordinator.updateEndpointResponseFiled(endpoint)
			if err != nil {
				return karma.Format(err, "unable to update endpoint in the database")
			}

			return nil
		}

		if err == errorResponse && endpoint.Response == false {
			err = coordinator.updateEndpointRefreshAtField(endpoint)
			if err != nil {
				return karma.Format(err, "unable to update endpoint in the database")
			}

			return nil
		}

		return karma.Format(
			err,
			"unable to get json from url = %s ",
			endpoint.URL,
		)
	}

	if data == nil {
		return errors.New("json data is empty")
	}

	duration := endpoint.Duration
	filter := bson.M{"_id": bson.M{"$eq": endpoint.ID}}
	update := bson.M{"$set": bson.M{
		"refresh_at":    time.Now().Add(duration),
		"data":          data,
		"previous_data": endpoint.Data,
		"response":      true,
		"updated_at":    time.Now(),
	}}
	_, err = coordinator.database.Endpoints.UpdateOne(
		coordinator.database.context,
		filter,
		update,
	)
	if err != nil {
		return karma.Format(err, "unable to update mongo")
	}

	log.Debugf(
		nil,
		"endpoint %v data was successfully refreshed",
		endpoint.ID,
	)

	return nil
}

func validateIsChatID(message *tb.Message) int {
	if message.Chat.ID != 0 {
		return int(message.Chat.ID)
	}

	return 0
}

func (coordinator *Coordinator) updateEndpointResponseFiled(
	endpoint Endpoint,
) error {
	filter := bson.M{"_id": bson.M{"$eq": endpoint.ID}}
	update := bson.M{"$set": bson.M{
		"refresh_at": time.Now().Add(endpoint.Duration),
		"response":   false,
	}}

	_, err := coordinator.database.Endpoints.UpdateOne(
		coordinator.database.context,
		filter,
		update,
	)
	if err != nil {
		return karma.Format(err, "unable to update mongo")
	}

	log.Debugf(
		nil,
		"url is unavailable, url: %s, endpoint: %v ",
		endpoint.URL,
		endpoint.ID,
	)
	return nil
}

func (coordinator *Coordinator) updateEndpointRefreshAtField(
	endpoint Endpoint,
) error {
	filter := bson.M{"_id": bson.M{"$eq": endpoint.ID}}
	update := bson.M{"$set": bson.M{
		"refresh_at": time.Now().Add(endpoint.Duration),
		"response":   false,
	}}

	_, err := coordinator.database.Endpoints.UpdateOne(
		coordinator.database.context,
		filter,
		update,
	)
	if err != nil {
		return karma.Format(err, "unable to update mongo")
	}

	log.Debugf(
		nil,
		"url is unavailable, url: %s, endpoint: %v ",
		endpoint.URL,
		endpoint.ID,
	)
	return nil
}
