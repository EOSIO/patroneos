package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

// Middleware returns a handler that can perform various operations
// and calls the next HTTP handler as the final action.
type middleware func(next http.HandlerFunc) http.HandlerFunc

// ErrorMessage defines the structure of an error response
type ErrorMessage struct {
	Message string `json:"message"`
	Code    int    `json:"code"`
}

// Action represents the structure of an action rpc payload
type Action struct {
	Code          string        `json:"code"`
	Type          string        `json:"type"`
	Recipients    []string      `json:"recipients"`
	Authorization []interface{} `json:"authorization"`
	Data          string        `json:"data"`
}

// Transaction describes the structure of a transaction rpc payload
type Transaction struct {
	RefBlockNum    string        `json:"ref_block_num"`
	RefBlockPrefix string        `json:"ref_block_prefix"`
	Expiration     string        `json:"expiration"`
	Scope          []string      `json:"scope"`
	Actions        []Action      `json:"actions"`
	Signatures     []string      `json:"signatures"`
	Authorizations []interface{} `json:"authorizations"`
}

// Define Context Keys
type contextKey string

var (
	transactionsKey = contextKey("transactions")
)

var client = http.Client{}

// getHost returns the host based on the existence of the X-Forwarded-For header.
func getHost(r *http.Request) string {
	var remoteHost string

	if header := r.Header.Get("X-Forwarded-For"); header != "" {
		remoteHost = header
	} else {
		remoteHost = r.RemoteAddr
	}

	return remoteHost
}

// logFailure logs a failure to the Fail2Ban server
func logFailure(message string, w http.ResponseWriter, r *http.Request) {
	remoteHost := getHost(r)
	for _, logAgent := range appConfig.LogEndpoints {
		if !strings.Contains(logAgent, "/patroneos/fail2ban-relay") {
			logAgent += "/patroneos/fail2ban-relay"
		}
		logEvent := Log{
			Host:    remoteHost,
			Success: false,
			Message: message,
		}
		body, err := json.Marshal(logEvent)
		if err != nil {
			log.Printf("Error marshalling failure message %s", err)
		}
		_, err = client.Post(logAgent, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Print(err)
		}
	}
	log.Printf("Failure: %s %s", remoteHost, message)
	if w != nil {
		errorBody, _ := json.Marshal(ErrorMessage{Message: message, Code: 400})
		w.Header().Add("X-REJECTED-BY", "patroneos")
		w.Header().Add("CONTENT-TYPE", "application/json")
		w.WriteHeader(400)
		_, err := w.Write(errorBody)
		if err != nil {
			log.Printf("Error writing response body %s", err)
		}
	}
}

// logSuccess logs a success to the Fail2Ban server
func logSuccess(message string, r *http.Request) {
	remoteHost := getHost(r)
	for _, logAgent := range appConfig.LogEndpoints {
		if !strings.Contains(logAgent, "/patroneos/fail2ban-relay") {
			logAgent += "/patroneos/fail2ban-relay"
		}
		logEvent := Log{
			Host:    remoteHost,
			Success: true,
			Message: message,
		}
		body, err := json.Marshal(logEvent)
		if err != nil {
			log.Printf("Error marshalling success message %s", err)
		}
		_, err = client.Post(logAgent, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Print(err)
		}
	}
	log.Printf("Success: %s %s", remoteHost, message)
}

// validateJSON checks that the POST body contains a valid JSON object.
func validateJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonBytes, err := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))
		if len(jsonBytes) > 0 {
			if !json.Valid(jsonBytes) || err != nil {
				logFailure("INVALID_JSON", w, r)
				return
			}
		}

		next.ServeHTTP(w, r)
	}
}

// validateMaxSignatures checks that the transaction does not have more signatures than the max allowed.
func validateMaxSignatures(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		transactions, ctx, err := getTransactions(r)
		if err != nil {
			logFailure(err.Error(), w, r)
			return
		}

		for _, transaction := range transactions {
			if len(transaction.Signatures) > appConfig.MaxSignatures {
				logFailure("INVALID_NUMBER_SIGNATURES", w, r)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// validateContract checks that the transaction does not act on a blacklisted contract.
func validateContract(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		transactions, ctx, err := getTransactions(r)
		if err != nil {
			logFailure(err.Error(), w, r)
			return
		}

		for _, transaction := range transactions {
			for _, action := range transaction.Actions {
				_, exists := appConfig.ContractBlackList[action.Code]
				if exists {
					logFailure("BLACKLISTED_CONTRACT", w, r)
					return
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// validateMaxTransactions checks that the number of transactions in the request does not exceed the defined maximum.
func validateMaxTransactions(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		transactions, ctx, err := getTransactions(r)
		if err != nil {
			logFailure(err.Error(), w, r)
			return
		}

		if len(transactions) > appConfig.MaxTransactions {
			logFailure("TOO_MANY_TRANSACTIONS", w, r)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// validateTransactionSize checks that the transaction data does not exceed the max allowed size.
func validateTransactionSize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		transactions, ctx, err := getTransactions(r)
		if err != nil {
			logFailure(err.Error(), w, r)
			return
		}

		for _, transaction := range transactions {
			for _, action := range transaction.Actions {
				if len(action.Data) > appConfig.MaxTransactionSize {
					logFailure("INVALID_TRANSACTION_SIZE", w, r)
					return
				}
			}
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	}
}

// getTransactions parses json and returns a slice containing the transactions
func getTransactions(r *http.Request) ([]Transaction, context.Context, error) {
	var transactions []Transaction
	var transaction Transaction

	// Context has not been set
	if r.Context().Value(transactionsKey) == nil {
		// Read request body
		jsonBytes, _ := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))

		// Determine if JSON is a single object or an array of objects
		body := strings.TrimSpace(string(jsonBytes))

		if strings.HasPrefix(body, "{") {
			// Single Object
			err := json.Unmarshal(jsonBytes, &transaction)

			if err != nil {
				return nil, nil, errors.New("PARSE_ERROR")
			}

			transactions = append(transactions, transaction)
		} else if strings.HasPrefix(body, "[") {
			// Array of Objects
			err := json.Unmarshal(jsonBytes, &transactions)

			if err != nil {
				return nil, nil, errors.New("PARSE_ERROR")
			}
		}

		// Add transactions to request context so subsequent middleware does not have to parse the transactions again
		ctx := context.WithValue(r.Context(), transactionsKey, transactions)
		return transactions, ctx, nil
	}

	// Context already exists
	transactions = r.Context().Value(transactionsKey).([]Transaction)
	return transactions, r.Context(), nil
}

// Walks through the middleware list in reverse order and
// pass the return value into the function before it so they are called
// in the correct order.
// Middleware pattern inspired by https://hackernoon.com/simple-http-middleware-with-go-79a4ad62889b
func chainMiddleware(mw ...middleware) middleware {
	return func(final http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			last := final
			for i := len(mw) - 1; i >= 0; i-- {
				last = mw[i](last)
			}
			last(w, r)
		}
	}
}

func copyHeaders(response http.Header, request http.Header) {
	for key, value := range request {
		// Let our server set the Content-Length
		if key == "Content-Length" {
			continue
		}
		for _, header := range value {
			response.Add(key, header)
		}
	}
}

// If the request passes all middleware validations
// we forward it to the node to be processed.
func forwardCallToNodeos(w http.ResponseWriter, r *http.Request) {
	nodeosHost := fmt.Sprintf("%s://%s:%s", appConfig.NodeosProtocol, appConfig.NodeosURL, appConfig.NodeosPort)
	url := nodeosHost + r.URL.String()
	method := r.Method
	body, _ := ioutil.ReadAll(r.Body)

	request, err := http.NewRequest(method, url, bytes.NewBuffer(body))

	if err != nil {
		log.Printf("Error in creating request %s", err)
		return
	}

	res, err := client.Do(request)

	if err != nil {
		log.Printf("Error in executing request %s", err)
		return
	}

	defer res.Body.Close()

	body, _ = ioutil.ReadAll(res.Body)

	if res.StatusCode == 200 {
		logSuccess("SUCCESS", r)
	} else {
		logFailure("TRANSACTION_FAILED", nil, r)
	}

	copyHeaders(w.Header(), res.Header)
	w.WriteHeader(res.StatusCode)
	_, err = w.Write(body)
	if err != nil {
		log.Printf("Error writing response body %s", err)
		return
	}
}

func relay(w http.ResponseWriter, r *http.Request) {
	message := "Patroneos cannot receive fail2ban relay requests when running in filter mode. Please check your config."
	log.Printf("%s", message)

	errorBody, _ := json.Marshal(ErrorMessage{Message: message, Code: 403})

	w.WriteHeader(http.StatusForbidden)
	_, err := w.Write(errorBody)

	if err != nil {
		log.Printf("Error writing response body %s", err)
		return
	}
}

func addFilterHandlers(mux *http.ServeMux) {
	// Middleware are executed in the order that they are passed to chainMiddleware.
	middlewareChain := chainMiddleware(
		validateJSON,
		validateMaxTransactions,
		validateTransactionSize,
		validateMaxSignatures,
		validateContract,
	)

	mux.HandleFunc("/", middlewareChain(forwardCallToNodeos))
	mux.HandleFunc("/patroneos/fail2ban-relay", relay)
}
