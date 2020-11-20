package main

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/globalsign/mgo/bson"
	karma "github.com/reconquest/karma-go"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/x/bsonx"
	tb "gopkg.in/tucnak/telebot.v2"
)

var ErrNoDocuments = errors.New("no documents")

type Endpoint struct {
	ID           primitive.ObjectID     `bson:"_id,omitempty"`
	URL          string                 `bson:"url"`
	Duration     time.Duration          `bson:"duration"`
	Data         map[string]interface{} `bson:"data"`
	PreviousData map[string]interface{} `bson:"previous_data"`
	RefreshAt    time.Time              `bson:"refresh_at"`
	Response     bool                   `bson:"response"`
	UpdatedAt    time.Time              `bson:"updated_at"`
}

type Database struct {
	URI  string
	name string

	Endpoints     *mongo.Collection
	Subscriptions *mongo.Collection

	client *mongo.Client

	context context.Context
}

type Subscriber struct {
	ID          primitive.ObjectID     `bson:"_id,omitempty"`
	URL         string                 `bson:"url"`
	Duration    time.Duration          `bson:"duration"`
	Sender      *tb.User               `bson:"sender"`
	Chat        *tb.Chat               `bson:"chat"`
	UserID      int                    `bson:"userid"`
	Keys        string                 `bson:"keys"`
	UpdatedAt   time.Time              `bson:"updated_at"`
	SendAt      time.Time              `bson:"send_at"`
	Data        map[string]interface{} `bson:"data"`
	Recipient   tb.Recipient
	RecipientID int
}

func (database *Database) connect() error {
	var err error
	opts := options.Client().ApplyURI(database.URI)

	database.client, err = mongo.Connect(database.context, opts)
	if err != nil {
		return karma.Format(
			err,
			"unable to connect to database %s",
			database.URI)
	}

	database.Subscriptions = database.client.Database(
		database.name,
	).Collection("subscriptions")

	database.Endpoints = database.client.Database(
		database.name,
	).Collection("endpoints")

	err = database.ensureEndpointsIndexes()
	if err != nil {
		return karma.Format(
			err,
			"can't create index for %s collection",
			database.Endpoints.Name())
	}

	err = database.ensureSubscriptionsIndexes()
	if err != nil {
		return karma.Format(
			err,
			"can't create index for %s collection",
			database.Subscriptions.Name())
	}

	return nil
}

func (database *Database) IsDup(err error) bool {
	return strings.Contains(err.Error(), "E11000")
}

func (database *Database) ensureEndpointsIndexes() error {
	_, err := database.Endpoints.Indexes().CreateOne(
		database.context,
		mongo.IndexModel{
			Keys: bsonx.Doc{
				{"url", bsonx.Int32(1)},
				{"duration", bsonx.Int32(1)},
			},
			Options: options.Index().SetUnique(true),
		},
	)

	if err != nil {
		return err
	}

	return nil
}

func (database *Database) ensureSubscriptionsIndexes() error {
	_, err := database.Subscriptions.Indexes().CreateOne(
		database.context,
		mongo.IndexModel{
			Keys: bsonx.Doc{
				{"userid", bsonx.Int32(1)},
				{"url", bsonx.Int32(1)},
			},
			Options: options.Index().SetUnique(true),
		},
	)

	if err != nil {
		return err
	}

	return nil
}

func (database *Database) ensureCollections() {
	database.Endpoints = database.client.Database(
		database.name,
	).Collection("endpoints")

	database.Subscriptions = database.client.Database(
		database.name,
	).Collection("subscriptions")
}

func (database *Database) RemoveEndpoint(id primitive.ObjectID) error {
	_, err := database.Endpoints.DeleteOne(
		context.Background(),
		bson.M{"_id": id},
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to remove record in endpoints collection",
		)
	}

	return nil
}

func (database *Database) RemoveSubscription(id primitive.ObjectID) error {
	_, err := database.Subscriptions.DeleteOne(
		context.Background(),
		bson.M{"_id": id},
	)
	if err != nil {
		return karma.Format(
			err,
			"unable to remove record in subscriptions collection",
		)
	}

	return nil
}

func (database *Database) deleteEndpointAndSubscription(url string) error {
	filter := bson.M{"url": url}
	_, err := database.Endpoints.DeleteOne(context.Background(), filter)
	if err != nil {
		return karma.Format(
			err,
			"unable to delete data in %s collection",
			database.Endpoints.Name(),
		)
	}

	_, err = database.Subscriptions.DeleteOne(context.Background(), filter)
	if err != nil {
		return karma.Format(
			err,
			"unable to delete data in %s collection",
			database.Subscriptions.Name(),
		)
	}

	return nil
}

func NewDatabase(uri string, name string, context context.Context) (
	*Database,
	error,
) {
	database := &Database{
		URI:     uri,
		name:    name,
		context: context,
	}

	err := database.connect()
	if err != nil {
		return nil, karma.Format(
			err,
			"unable to connect to database",
		)
	}

	return database, nil
}

func (database *Database) Disconnect() error {
	err := database.client.Disconnect(context.Background())
	if err != nil {
		return karma.Format(
			err, "unable to disconnect from database %s",
			database.name)
	}

	return nil
}

func (database *Database) Drop() error {
	err := database.client.Database(database.name).Drop(context.Background())
	if err != nil {
		return karma.Format(
			err,
			"unable to drop database %s",
			database.name)
	}

	return nil
}

func (database *Database) updateSubscriberStatus(
	userID int,
	url string,
	updatedAt time.Time,
) error {
	upsert := true
	_, err := database.Subscriptions.UpdateOne(
		database.context,
		bson.M{
			"url":    url,
			"userid": userID,
		},
		bson.M{"$set": bson.M{
			"updated_at": updatedAt,
		}},
		&options.UpdateOptions{
			Upsert: &upsert,
		},
	)

	if err != nil {
		return karma.Format(err, "unable to update subscriber status in database")
	}

	return nil
}

func (database *Database) updateSubscriber(
	subscriber Subscriber,
) error {
	duration := subscriber.Duration
	filter := bson.M{
		"_id": bson.M{
			"$eq": subscriber.ID,
		},
	}
	update := bson.M{"$set": bson.M{
		"send_at": time.Now().Add(duration),
	}}
	_, err := database.Subscriptions.UpdateOne(
		database.context,
		filter,
		update,
	)
	if err != nil {
		return err
	}

	return nil
}

func (database *Database) writeEndpoint(endpoint *Endpoint) error {
	_, err := database.Endpoints.InsertOne(
		context.Background(), endpoint,
	)
	if err != nil {
		if database.IsDup(err) {
			return nil
		}

		return karma.Format(
			err,
			"subscription with url = %s and duration = %v already exists in endpoint collection",
			endpoint.URL,
			endpoint.Duration,
		)
	}

	return nil
}

func (database *Database) upsertSubscriber(subscriber Subscriber) error {
	upsert := true
	_, err := database.Subscriptions.UpdateOne(
		context.Background(),
		bson.M{
			"url":    subscriber.URL,
			"userid": subscriber.UserID,
		},
		bson.M{"$set": bson.M{
			"duration": subscriber.Duration,
			"sender":   subscriber.Sender,
			"chat":     subscriber.Chat,
			"keys":     subscriber.Keys,
			"send_at":  time.Now().Add(subscriber.Duration),
		}},
		&options.UpdateOptions{
			Upsert: &upsert,
		},
	)

	if err != nil {
		return karma.Format(err, "unable to write to database")
	}

	return nil
}

func (database *Database) findSubscriber(
	subscriberID int,
	url string,
) (*Subscriber, error) {
	var subscriber Subscriber
	cursor := database.Subscriptions.FindOne(
		context.Background(),
		bson.M{
			"url":    url,
			"userid": subscriberID,
		},
	)

	err := cursor.Decode(&subscriber)
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, nil
		}

		return nil, karma.Format(
			err,
			"can't decode data from %s collection, subscriber_id %d",
			database.Subscriptions.Name(),
			subscriberID,
		)
	}

	return &subscriber, nil
}

func (database *Database) FindInSubscriptions(filter primitive.M) (
	[]Subscriber,
	error,
) {
	var data []Subscriber
	cursor, err := database.Subscriptions.Find(context.Background(),
		filter)
	if err != nil {
		return nil, karma.Format(
			err,
			"can't find data in %s collection",
			database.Subscriptions.Name())
	}

	err = cursor.All(context.Background(), &data)
	if err != nil {
		return nil, karma.Format(
			err,
			"can't decode data from %s collection",
			database.Subscriptions.Name())
	}

	return data, nil
}

func (database *Database) FindInEndpoints(filter primitive.M) (
	[]Endpoint,
	error,
) {
	var data []Endpoint
	cursor, err := database.Endpoints.Find(context.Background(),
		filter)
	if err != nil {
		return nil, karma.Format(
			err,
			"can't find data in %s collection",
			database.Endpoints.Name())
	}

	err = cursor.All(context.Background(), &data)
	if err != nil {
		return nil, karma.Format(
			err,
			"can't decode data from %s collection",
			database.Endpoints.Name())
	}

	return data, nil
}
