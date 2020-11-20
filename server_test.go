package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	karma "github.com/reconquest/karma-go"
	"github.com/reconquest/pkg/log"
)

var pathToTransactions = firstTransactions

const (
	transactionsOnPage  = 5
	updatedTransactions = "tests/test-data/updatedTransacitons.json"
	firstTransactions   = "tests/test-data/transactions.json"
	timeFormat          = "03:04:05 PM"
)

type ServerTime struct {
	Time string `json:"time"`
}

type Handler struct {
	Err                bool
	PathToTransactions string
}

type ServerClient struct {
	PathToTransactions string
}

type Transaction struct {
	TransactionID   string                     `json:"transactionId"`
	PurchaseDetails TransactionPurchaseDetails `json:"purchaseDetails"`
	CustomerDetails TransactionCustomerDetails `json:"customerDetails"`
}

type TransactionPurchaseDetails struct {
	SaleDate      string  `json:"saleDate"`
	PurchasePrice float64 `json:"purchasePrice"`
	Tier          string  `json:"tier"`
}

type TransactionCustomerDetails struct {
	Company string `json:"company"`
}

type TransactionData struct {
	AddonName     string  `json:"addon"`
	Company       string  `json:"company"`
	Tier          string  `json:"tier"`
	PurchasePrice float64 `json:"price"`
}

type TransactionResponse struct {
	TransactionMertics struct {
		Total  int64             `json:"total"`
		Latest []TransactionData `json:"latest"`
	} `json:"metrics"`
}

func NewHandler() *Handler {
	return &Handler{}
}

func getTransactionsFromTestPath(path string) (
	[]Transaction,
	error,
) {
	var transactions []Transaction
	firstData, err := getTestData(path)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(firstData, &transactions)
	if err != nil {
		return nil, err
	}
	return transactions, nil
}

func getTestData(path string) ([]byte, error) {
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return []byte{}, karma.Format(
			err,
			"unable to get data from path: %s", path)
	}
	return data, nil
}

func (handler *Handler) HandleTime(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	var timeData ServerTime
	timeData.Time = time.Now().Format(timeFormat)

	timeJson, err := json.Marshal(timeData)
	if err != nil {
		log.Errorf(
			err,
			"unable to marshal metrics data to json format",
		)

		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(responseWriter, string(timeJson))

}

func (handler *Handler) HandleMetricsTransactions(
	responseWriter http.ResponseWriter,
	request *http.Request,
) {
	transactions, err := getTransactionsFromTestPath(pathToTransactions)
	if err != nil {
		log.Errorf(
			err,
			"unable to get data",
		)

		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	if transactions == nil {
		log.Error(
			errors.New("transactions is empty"),
		)

		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json; charset=utf-8")
	var total int64
	total = 350

	lastUpdatedTransactions := getLatestTransactions(transactions)

	var response TransactionResponse
	response.TransactionMertics.Total = total
	response.TransactionMertics.Latest = lastUpdatedTransactions

	transactionsMetrics, err := json.Marshal(response)
	if err != nil {
		log.Errorf(
			err,
			"unable to marshal metrics data to json format",
		)

		responseWriter.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Fprintln(responseWriter, string(transactionsMetrics))
}

func getLatestTransactions(
	transactions []Transaction,
) []TransactionData {
	if len(transactions) < transactionsOnPage {
		return nil
	}
	var set []TransactionData
	var transactionData TransactionData

	for i := 0; i < transactionsOnPage; i++ {
		transactionData = TransactionData{
			Company:       transactions[i].CustomerDetails.Company,
			Tier:          transactions[i].PurchaseDetails.Tier,
			PurchasePrice: transactions[i].PurchaseDetails.PurchasePrice,
		}

		set = append(set, transactionData)
	}

	return set
}

func (client ServerClient) createTestServerWithTransactions() *httptest.Server {
	testServer := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter,
				r *http.Request) {
				handler := Handler{PathToTransactions: pathToTransactions}
				handler.HandleMetricsTransactions(w, r)
			}))
	return testServer
}

func (client *ServerClient) createTestServerWithTimeUpdating() *httptest.Server {
	testServer := httptest.NewServer(
		http.HandlerFunc(
			func(w http.ResponseWriter,
				r *http.Request) {
				handler := Handler{}
				handler.HandleTime(w, r)
			},
		),
	)
	return testServer

}

func (client *ServerClient) createUnstartedTestServer() *httptest.Server {
	testServer := httptest.NewUnstartedServer(
		http.HandlerFunc(
			func(w http.ResponseWriter,
				r *http.Request) {
				handler := Handler{}
				handler.HandleTime(w, r)
			},
		),
	)
	return testServer
}

func TestMain_FunctionalityOfServer(
	t *testing.T,
) {
	pathToTransactions = "tests/test-data/transactions.json"
	client := ServerClient{PathToTransactions: pathToTransactions}
	testServer := client.createTestServerWithTransactions()
	log.Info("testServer ", testServer)
	time.Sleep(10 * time.Minute)
	pathToTransactions = updatedTransactions
	client = ServerClient{PathToTransactions: pathToTransactions}

	time.Sleep(100 * time.Second)
	defer testServer.Close()

}

func TestMain_FunctionalityServerTime(
	t *testing.T,
) {
	client := ServerClient{PathToTransactions: pathToTransactions}
	testServer := client.createTestServerWithTimeUpdating()
	log.Info("testServer ", testServer)
	time.Sleep(100 * time.Second)

	defer testServer.Close()

}
