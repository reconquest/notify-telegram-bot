package main

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/reconquest/pkg/log"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"gopkg.in/tucnak/telebot.v2"
	tb "gopkg.in/tucnak/telebot.v2"
)

// all tests should run with command: 'go test -run W -v -timeout 1h'
// for test on real data, uses http://time.jsontest.com/, if this site will not available,
// tests will not work, but these tests contain tests with test server data
// use it, if site will not available
// every test should run manually, because tests use time.Sleep functions for imitation
// of real time work of the bot

type Recipient struct {
	recipient string
}

type TestTransport interface {
	SendMessage(Recipient, string) error
}

type TestTelegram struct {
	bot             *tb.Bot
	lastSentMessage string
	allSentMessages []string
	recipient       tb.Recipient
	recipientData   []map[string][]string //key=recipient, []string = messages
}

func NewTestBot() *TestTelegram {
	return &TestTelegram{}

}

func (telegram *TestTelegram) SendMessage(recipient tb.Recipient, message string) error {
	newItem := make(map[string][]string)
	var messages []string

	isExists := false
	for _, item := range telegram.recipientData {
		_, ok := item[recipient.Recipient()]
		if ok {
			isExists = true
		} else {
			continue
		}
	}

	if isExists {
		for _, item := range telegram.recipientData {
			_, ok := item[recipient.Recipient()]
			if ok {
				item[recipient.Recipient()] = append(item[recipient.Recipient()], message)
			} else {
				continue
			}
		}

	} else {
		messages = append(messages, message)
		newItem[recipient.Recipient()] = messages
		telegram.recipientData = append(telegram.recipientData, newItem)
	}

	telegram.lastSentMessage = message
	telegram.allSentMessages = append(telegram.allSentMessages, message)
	telegram.recipient = recipient
	log.Info("telegram.sentMessage: ", telegram.lastSentMessage, "\n")
	return nil
}

func createTestDatabase() *Database {
	database, err := NewDatabase(
		os.Getenv("TEST_DATABASE_URI"),
		"licenses_test_"+fmt.Sprint(time.Now().UnixNano()),
		context.Background(),
	)
	if err != nil {
		panic(err)
	}

	return database
}

func createMessage(url,
	duration,
	JSONKey string,
	subscriberID int,
	chatID int64,
) *telebot.Message {
	if url == "" {
		url = "default.url"
	}

	chat := &telebot.Chat{chatID, "", "", "", "", ""}
	sender := &telebot.User{subscriberID, "", "", "", "", false}
	payload := url + " " + duration + " " + JSONKey

	if chatID == 0 {
		chat = &telebot.Chat{}

	}

	message := &telebot.Message{
		4444, sender, 0, chat, nil, nil, 0, nil, 0, "", "", "", payload, nil, "",
		nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, "", nil,
		nil, false, false, false, false, 0, 0, nil,
	}

	return message
}

func Test_Coordinator_routineCleanEndpoints_RemoveEdnpointWithoutSubscribers(
	t *testing.T,
) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	client := ServerClient{}
	serverWithTime := client.createTestServerWithTimeUpdating()
	defer serverWithTime.Close()

	urlServerTime, err := url.Parse(serverWithTime.URL)
	assert.NoError(t, err)

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	message := createMessage(urlServerTime.String(), "5s", "time", 1, 2)

	err = coordinator.subscribe(message)
	assert.NoError(t, err)

	subscription, err := testDatabase.FindInSubscriptions(bson.M{})
	assert.NoError(t, err)
	assert.Equal(t, urlServerTime.String(), subscription[0].URL)

	err = coordinator.routineCleanEndpoints()
	assert.NoError(t, err)
	endpoints, err := testDatabase.FindInEndpoints(bson.M{})
	assert.NoError(t, err)
	assert.Equal(t, urlServerTime.String(), endpoints[0].URL)

	err = testDatabase.RemoveSubscription(subscription[0].ID)
	assert.NoError(t, err)

	err = coordinator.routineCleanEndpoints()
	assert.NoError(t, err)

	endpointsAfterRemovingSubscriber, err := testDatabase.FindInEndpoints(bson.M{})
	assert.NoError(t, err)

	assert.Empty(t, endpointsAfterRemovingSubscriber)

}
func Test_Coordinator_SendMessageToUserAndWriteSubscriberToDatabase(t *testing.T) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	client := ServerClient{}
	serverWithTime := client.createTestServerWithTimeUpdating()
	defer serverWithTime.Close()

	urlServerTime, err := url.Parse(serverWithTime.URL)
	assert.NoError(t, err)

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	message := createMessage(urlServerTime.String(), "5s", "time", 1, 2)

	err = coordinator.subscribe(message)
	assert.NoError(t, err)

	dataFromDatabase, err := testDatabase.FindInSubscriptions(bson.M{})
	assert.NoError(t, err)

	assert.NotEmpty(t, dataFromDatabase)

	// first message should contain: "You successfully subscribed"
	assert.Contains(t, telegramBot.lastSentMessage, "You successfully subscribed!")
}

func Test_Coordinator_StopSendingMessagesAndDeletingSubscriptionInDatabase(t *testing.T) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	clientWithTimeUpdating := ServerClient{}
	serverWithTime := clientWithTimeUpdating.createTestServerWithTimeUpdating()
	defer serverWithTime.Close()

	urlServerTime, err := url.Parse(serverWithTime.URL)
	assert.NoError(t, err)

	clientServerTransactions := ServerClient{}
	serverWithTransactions := clientServerTransactions.createTestServerWithTransactions()
	defer serverWithTransactions.Close()

	urlServerTransactions, err := url.Parse(serverWithTransactions.URL)
	assert.NoError(t, err)

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	subscribeMessageForTime := createMessage(urlServerTime.String(), "5s", "time", 1, 2)

	err = coordinator.subscribe(subscribeMessageForTime)
	assert.NoError(t, err)

	dataAfterSubscribe, err := testDatabase.FindInSubscriptions(bson.M{})
	assert.NoError(t, err)

	assert.Contains(t, telegramBot.lastSentMessage, "You successfully subscribed!")
	assert.NotEmpty(t, dataAfterSubscribe)

	subscribeMessageForTransactions := createMessage(
		urlServerTransactions.String(),
		"3s", "metrics",
		1, 2,
	)

	//second subscription

	err = coordinator.subscribe(subscribeMessageForTransactions)
	assert.NoError(t, err)

	dataAfterSubscribe, err = testDatabase.FindInSubscriptions(bson.M{})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(dataAfterSubscribe))

	endpointsData, err := testDatabase.FindInEndpoints(bson.M{})
	assert.NoError(t, err)
	assert.Equal(t, 2, len(endpointsData))

	assert.Contains(t, telegramBot.lastSentMessage, "You successfully subscribed!")

	stopMessage := createMessage("", "", "", 1, 2)

	err = coordinator.stop(stopMessage)
	subscriptionDataAfterStopCommand, err := testDatabase.FindInSubscriptions(bson.M{})
	assert.NoError(t, err)

	endpointDataAfterStopCommand, err := testDatabase.FindInEndpoints(bson.M{})
	assert.NoError(t, err)

	assert.Empty(t, subscriptionDataAfterStopCommand, "subscritptions should be empty")
	assert.Empty(t, endpointDataAfterStopCommand, "endpoints should be empty")

}

func Test_Coordinator_NotContainDuplicateMessageOnTestServerData(
	t *testing.T,
) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	pathToTransactions = firstTransactions

	client := ServerClient{}
	testServer := client.createTestServerWithTimeUpdating()
	defer testServer.Close()

	url, err := url.Parse(testServer.URL)
	assert.NoError(t, err)

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	message := createMessage(url.String(), "10s", "time", 1, 2)

	err = coordinator.subscribe(message)
	assert.NoError(t, err)

	time.Sleep(200 * time.Second)

	var duplicate bool
	var duplicateRecords []string
	for index, record := range telegramBot.allSentMessages {
		for _, target := range telegramBot.allSentMessages[index+1:] {
			if target == record {
				duplicate = true
				duplicateRecords = append(duplicateRecords, record)
				break
			}
		}
	}

	assert.False(t, duplicate, "should not contain duplicates", "duplicateRecords: ", duplicateRecords)
}

func Test_Coordinator_NotContainDuplicateMessageOnRealData(
	t *testing.T,
) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()
	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	pathToTransactions = firstTransactions

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	message := createMessage("http://time.jsontest.com/", "10s", "time", 1, 2)

	err = coordinator.subscribe(message)
	assert.NoError(t, err)

	time.Sleep(200 * time.Second)

	var duplicate bool
	var duplicateRecords []string
	for index, record := range telegramBot.allSentMessages {
		for _, target := range telegramBot.allSentMessages[index+1:] {
			if target == record {
				duplicate = true
				duplicateRecords = append(duplicateRecords, record)
				break
			}
		}
	}

	assert.False(
		t,
		duplicate,
		"should not contain duplicates",
		"duplicate record: ",
		duplicateRecords,
	)

}

func Test_Coordinator_SendOnlyOneMessageIfJsonNotUpdatedOnServer(
	t *testing.T,
) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	clientServerTransactions := ServerClient{}
	serverWithTransactions := clientServerTransactions.createTestServerWithTransactions()
	defer serverWithTransactions.Close()

	urlServerTransactions, err := url.Parse(serverWithTransactions.URL)
	assert.NoError(t, err)

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	message := createMessage(urlServerTransactions.String(), "5s", "metrics", 1, 2)

	err = coordinator.subscribe(message)
	assert.NoError(t, err)

	time.Sleep(20 * time.Second)

	var duplicate bool
	var duplicateRecords []string
	for index, record := range telegramBot.allSentMessages {
		for _, target := range telegramBot.allSentMessages[index+1:] {
			if target == record {
				duplicate = true
				duplicateRecords = append(duplicateRecords, record)
				break
			}
		}
	}

	assert.False(
		t,
		duplicate,
		"should not contain duplicates",
		"duplicate record: ",
		duplicateRecords,
	)
}
func Test_Coordinator_NotContainDuplicateMessageOnRealDataWithMinuteInterval(t *testing.T) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	pathToTransactions = firstTransactions

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	message := createMessage("http://time.jsontest.com/", "1m", "time", 1, 2)

	err = coordinator.subscribe(message)
	assert.NoError(t, err)

	time.Sleep(10 * time.Minute)

	var duplicate bool
	var duplicateRecords []string
	for index, record := range telegramBot.allSentMessages {
		for _, target := range telegramBot.allSentMessages[index+1:] {
			if target == record {
				duplicate = true
				duplicateRecords = append(duplicateRecords, record)
				break
			}
		}
	}

	assert.False(
		t,
		duplicate,
		"should not contain duplicates",
		"duplicate record: ",
		duplicateRecords,
	)
}

func Test_Coordinator_NotContainDuplicateMessageOnTestServerDataWithMinuteInterval(t *testing.T) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()
	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	client := ServerClient{}
	serverWithTime := client.createTestServerWithTimeUpdating()
	defer serverWithTime.Close()

	urlServerTime, err := url.Parse(serverWithTime.URL)
	assert.NoError(t, err)

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	message := createMessage(urlServerTime.String(), "1m", "time", 1, 2)

	err = coordinator.subscribe(message)
	assert.NoError(t, err)

	time.Sleep(10 * time.Minute)

	var duplicate bool
	var duplicateRecords []string
	for index, record := range telegramBot.allSentMessages {
		for _, target := range telegramBot.allSentMessages[index+1:] {
			if target == record {
				duplicate = true
				duplicateRecords = append(duplicateRecords, record)
				break
			}
		}
	}

	assert.False(
		t,
		duplicate,
		"should not contain duplicates",
		"duplicate record: ",
		duplicateRecords,
	)

}

func Test_Coordinator_NotContainDuplicateMessageOnTestServerDataWithSeveralSubscribers(
	t *testing.T,
) {

	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	pathToTransactions = firstTransactions

	client := ServerClient{}
	testServer := client.createTestServerWithTimeUpdating()
	defer testServer.Close()

	url, err := url.Parse(testServer.URL)
	assert.NoError(t, err)

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	//subscriber 1
	message1 := createMessage(url.String(), "10s", "time", 1, 2)
	err = coordinator.subscribe(message1)
	assert.NoError(t, err)

	//subscriber 2
	time.Sleep(1 * time.Second)
	message2 := createMessage(url.String(), "5s", "time", 3, 4)
	err = coordinator.subscribe(message2)
	assert.NoError(t, err)

	//subscriber 3
	time.Sleep(10 * time.Second)
	message3 := createMessage(url.String(), "10s", "time", 5, 6)
	err = coordinator.subscribe(message3)
	assert.NoError(t, err)

	time.Sleep(100 * time.Second)

	assert.NotNil(t, telegramBot.recipientData)

	var duplicate bool
	var duplicateRecords []string
	for _, data := range telegramBot.recipientData {
		for _, item := range data {
			for index, message := range item {
				for _, target := range item[index+1:] {
					if target == message {
						duplicate = true
						duplicateRecords = append(duplicateRecords, message)
						break
					}
				}
			}
		}
	}

	assert.False(
		t,
		duplicate,
		"should not contain duplicates",
		"duplicate record: ",
		duplicateRecords,
	)
}

// with same url:
func Test_Coordinator_NotContainDuplicateMessageOnRealDataWithSeveralSubscribers(
	t *testing.T,
) {

	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	url := "http://time.jsontest.com/"
	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	// subscriber 1
	message1 := createMessage(url, "15s", "time", 1, 2)
	err = coordinator.subscribe(message1)
	assert.NoError(t, err)

	// subscriber 2
	time.Sleep(3 * time.Second)
	message2 := createMessage(url, "10s", "time", 4, 3)
	err = coordinator.subscribe(message2)
	assert.NoError(t, err)

	// subscriber 3
	time.Sleep(15 * time.Second)
	message3 := createMessage(url, "10s", "time", 5, 6)
	err = coordinator.subscribe(message3)
	assert.NoError(t, err)

	// subscriber 4
	time.Sleep(3 * time.Second)
	message4 := createMessage(url, "5s", "time", 7, 8)
	err = coordinator.subscribe(message4)
	assert.NoError(t, err)

	time.Sleep(100 * time.Second)

	assert.NotNil(t, telegramBot.recipientData)
	var duplicate bool
	var duplicateRecords []string
	for _, data := range telegramBot.recipientData {
		for _, item := range data {
			for index, message := range item {
				for _, target := range item[index+1:] {
					if target == message {
						duplicate = true
						duplicateRecords = append(duplicateRecords, message)
						break
					}
				}
			}
		}
	}

	assert.False(
		t,
		duplicate,
		"should not contain duplicates",
		"duplicate record: ",
		duplicateRecords,
		"total duplicates: ",
		len(duplicateRecords),
	)
}

func Test_Coordinator_NotContainDuplicateMessageOnRealDataWithSeveralSubscribersWithMinuteDuration(
	t *testing.T,
) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	url := "http://time.jsontest.com/"
	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	// subscriber 1
	message1 := createMessage(url, "2m", "time", 1, 2)
	err = coordinator.subscribe(message1)
	assert.NoError(t, err)

	// subscriber 2
	time.Sleep(3 * time.Second)
	message2 := createMessage(url, "1m", "time", 4, 3)
	err = coordinator.subscribe(message2)
	assert.NoError(t, err)

	// subscriber 3
	time.Sleep(15 * time.Second)
	message3 := createMessage(url, "1m", "time", 5, 6)
	err = coordinator.subscribe(message3)
	assert.NoError(t, err)

	// subscriber 4
	time.Sleep(3 * time.Second)
	message4 := createMessage(url, "30s", "time", 7, 8)
	err = coordinator.subscribe(message4)
	assert.NoError(t, err)

	time.Sleep(10 * time.Minute)

	assert.NotNil(t, telegramBot.recipientData)
	var duplicate bool
	var duplicateRecords []string
	for _, data := range telegramBot.recipientData {
		for _, item := range data {
			for index, message := range item {
				for _, target := range item[index+1:] {
					if target == message {
						duplicate = true
						duplicateRecords = append(duplicateRecords, message)
						break
					}
				}
			}
		}
	}

	assert.False(
		t,
		duplicate,
		"should not contain duplicates",
		"duplicate record: ",
		duplicateRecords,
		"total duplicates: ",
		len(duplicateRecords),
	)
}

func Test_Coordinator_NotContainDuplicateMessageOnRealDataWithSeveralSubscribersWithDifferentSubscriptions(
	t *testing.T,
) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	clientServerTransactions := ServerClient{}
	serverWithTransactions := clientServerTransactions.createTestServerWithTransactions()
	defer serverWithTransactions.Close()

	urlServerTransactions, err := url.Parse(serverWithTransactions.URL)
	assert.NoError(t, err)

	urlTime := "http://time.jsontest.com/"
	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	//subscriber 1
	message1 := createMessage(urlTime, "10s", "time", 1, 2)
	err = coordinator.subscribe(message1)
	assert.NoError(t, err)

	//subscriber 2
	time.Sleep(15 * time.Second)
	message2 := createMessage(urlServerTransactions.String(), "10s", "metrics", 3, 4)
	err = coordinator.subscribe(message2)
	assert.NoError(t, err)

	//subscriber 3
	time.Sleep(3 * time.Second)
	message3 := createMessage(urlServerTransactions.String(), "10s", "metrics", 5, 6)
	err = coordinator.subscribe(message3)
	assert.NoError(t, err)

	time.Sleep(100 * time.Second)

	assert.NotNil(t, telegramBot.recipientData)
	var duplicate bool
	var duplicateRecords []string
	for _, data := range telegramBot.recipientData {
		for _, item := range data {
			for index, message := range item {
				for _, target := range item[index+1:] {
					if target == message {
						duplicate = true
						duplicateRecords = append(duplicateRecords, message)
						break
					}
				}
			}
		}
	}

	assert.False(
		t,
		duplicate,
		"should not contain duplicates",
		"duplicate record: ",
		duplicateRecords,
	)
}

func Test_Coordinator_SendMessageAboutFailedConnectionIfConnectionToServerIsLost(
	t *testing.T,
) {
	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	pathToTransactions = firstTransactions

	client := ServerClient{}
	testServer := client.createUnstartedTestServer()
	// defer testServer.Close()

	listener, err := net.Listen("tcp", "127.0.0.1:8082")
	if err != nil {
		log.Fatal(err)
	}

	testServer.Listener.Close()
	testServer.Listener = listener
	testServer.Start()
	// defer testServer.Close()

	url, err := url.Parse(testServer.URL)
	assert.NoError(t, err)

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	message := createMessage(url.String(), "3s", "time", 1, 2)

	err = coordinator.subscribe(message)
	assert.NoError(t, err)

	time.Sleep(8 * time.Second)
	assert.Contains(t, telegramBot.lastSentMessage, "PM")
	testServer.Close()

	time.Sleep(8 * time.Second)
	assert.Contains(t, telegramBot.lastSentMessage, "URL - "+url.String()+"\n\nURL is unavailable!")

	newListener, err := net.Listen("tcp", "127.0.0.1:8082")
	if err != nil {
		log.Fatal(err)
	}

	testServer = client.createUnstartedTestServer()
	testServer.Listener.Close()
	testServer.Listener = newListener
	testServer.Start()

	time.Sleep(8 * time.Second)
	assert.Contains(t, telegramBot.lastSentMessage, "PM")

	testServer.Close()
}

func Test_Coordinator_NotDeleteEndpointIfThisEndpointHasTwoSubscribers(t *testing.T) {

	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	clientWithTimeUpdating := ServerClient{}
	serverWithTime := clientWithTimeUpdating.createTestServerWithTimeUpdating()
	defer serverWithTime.Close()

	urlServerTime, err := url.Parse(serverWithTime.URL)
	assert.NoError(t, err)

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)

	subscribeMessageForTimeWithFirstID := createMessage(
		urlServerTime.String(),
		"5s",
		"time",
		1,
		2,
	)

	subscribeMessageForTimeWithSecondID := createMessage(
		urlServerTime.String(),
		"5s",
		"time",
		2,
		3,
	)

	err = coordinator.subscribe(subscribeMessageForTimeWithFirstID)
	assert.NoError(t, err)

	err = coordinator.subscribe(subscribeMessageForTimeWithSecondID)
	assert.NoError(t, err)

	stopMessage := createMessage("", "", "", 1, 2)

	err = coordinator.stop(stopMessage)
	subscriptionDataAfterStopCommand, err := testDatabase.FindInSubscriptions(bson.M{})
	assert.NoError(t, err)

	endpointDataAfterStopCommand, err := testDatabase.FindInEndpoints(bson.M{})
	assert.NoError(t, err)

	assert.Equal(
		t,
		1,
		len(subscriptionDataAfterStopCommand),
		"should contain only one subscription",
	)

	assert.Equal(
		t,
		int64(3),
		subscriptionDataAfterStopCommand[0].Chat.ID,
		"should contain subscription with id: 3",
	)

	assert.Equal(
		t,
		1,
		len(endpointDataAfterStopCommand),
		"should contain only one endpoint",
	)
}

func Test_Coordinator_IfSubscribeCommandFromChatMessageSendsOnlyToChat(t *testing.T) {

	config, err := LoadConfig("./config.dev.toml")
	if err != nil {
		log.Fatal(err)
	}

	testDatabase := createTestDatabase()
	defer testDatabase.Disconnect()
	defer testDatabase.Drop()

	telegramBot := NewTestBot()

	coordinator := NewCoordinator(telegramBot, testDatabase, config)

	clientWithTimeUpdating := ServerClient{}
	serverWithTime := clientWithTimeUpdating.createTestServerWithTimeUpdating()
	defer serverWithTime.Close()

	urlServerTime, err := url.Parse(serverWithTime.URL)
	assert.NoError(t, err)

	go runRoutineUpdateEndpoints(coordinator)
	go runRoutineSendDataToSubscribers(coordinator)
	time.Sleep(time.Second)

	personID := 111
	var chatID int64
	chatID = 2222

	firstSubscribeMessageFromChat := createMessage(
		urlServerTime.String(),
		"1s",
		"time",
		personID,
		chatID,
	)

	chatID = 0
	secondSubscribeMessageFromUser := createMessage(
		urlServerTime.String(),
		"1s",
		"time",
		personID,
		chatID,
	)

	err = coordinator.subscribe(firstSubscribeMessageFromChat)
	assert.NoError(t, err)

	assert.NoError(t, err)
	time.Sleep(2 * time.Second)

	isChatRecipient := telegramBot.recipient.Recipient()
	assert.Equal(t, "2222", isChatRecipient)
	err = coordinator.subscribe(secondSubscribeMessageFromUser)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

	err = coordinator.subscribe(secondSubscribeMessageFromUser)
	assert.NoError(t, err)
	time.Sleep(2 * time.Second)
	isPersonRecipient := telegramBot.recipient.Recipient()

	assert.Equal(t, "111", isPersonRecipient)
	err = coordinator.subscribe(secondSubscribeMessageFromUser)
	assert.NoError(t, err)

	time.Sleep(2 * time.Second)

}

func updateJSONEveryOneSecondOnPage() {
	for {
		time.Sleep(1 * time.Second)
		pathToTransactions = firstTransactions
		time.Sleep(1 * time.Second)
		pathToTransactions = updatedTransactions
	}
}

func runRoutineUpdateEndpoints(coordinator *Coordinator) {
	for {
		err := coordinator.routineUpdateEndpoints()
		if err != nil {
			log.Error(err)
		}

		time.Sleep(1 * time.Second)
	}
}

func runRoutineSendDataToSubscribers(coordinator *Coordinator) {
	for {
		err := coordinator.routineSendDataToSubscribers()
		if err != nil {
			log.Error(err)
		}

		time.Sleep(2 * time.Second)
	}
}
