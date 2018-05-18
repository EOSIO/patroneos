package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// Middleware returns a handler that can perform various operations
// and calls the next HTTP handler as the final action.
type middleware func(next http.HandlerFunc) http.HandlerFunc

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
func logFailure(message string, r *http.Request) {
	remoteHost := getHost(r)
	for _, logAgent := range appConfig.LogEndpoints {
		logEvent := Log{
			Host:    remoteHost,
			Success: false,
			Message: message,
		}
		body, err := json.Marshal(logEvent)
		if err != nil {
			log.Printf("Error marshalling failure message %s", err)
		}
		client.Post(logAgent, "application/json", bytes.NewBuffer(body))
		if err != nil {
			log.Print(err)
		}
	}
	log.Printf("Failure: %s %s", remoteHost, message)
}

// logSuccess logs a success to the Fail2Ban server
func logSuccess(message string, r *http.Request) {
	remoteHost := getHost(r)
	for _, logAgent := range appConfig.LogEndpoints {
		logEvent := Log{
			Host:    remoteHost,
			Success: true,
			Message: message,
		}
		body, err := json.Marshal(logEvent)
		if err != nil {
			log.Printf("Error marshalling success message %s", err)
		}
		client.Post(logAgent, "application/json", bytes.NewBuffer(body))
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
				logFailure("INVALID_JSON", r)
				http.Error(w, "INVALID_JSON", 400)
				return
			}
		}

		next.ServeHTTP(w, r)
	}
}

// validateSignatures checks that the transaction does not have more signatures than the max allowed.
func validateSignatures(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var transaction Transaction

		jsonBytes, _ := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))
		if len(jsonBytes) > 0 {
			err := json.Unmarshal(jsonBytes, &transaction)
			if err != nil {
				logFailure("PARSING_ERROR", r)
				return
			}
			if len(transaction.Signatures) > appConfig.MaxSignatures {
				logFailure("INVALID_NUMBER_SIGNATURES", r)
				http.Error(w, "INVALID_NUMBER_SIGNATURES", 400)
				return
			}
		}
		next.ServeHTTP(w, r)
	}
}

// validateContract checks that the transaction does not act on a blacklisted contract.
func validateContract(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var transaction Transaction

		jsonBytes, _ := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))
		if len(jsonBytes) > 0 {
			err := json.Unmarshal(jsonBytes, &transaction)
			if err != nil {
				logFailure("PARSING_ERROR", r)
				return
			}

			for _, action := range transaction.Actions {
				_, exists := appConfig.ContractBlackList[action.Code]
				if exists {
					logFailure("BLACKLISTED_CONTRACT", r)
					http.Error(w, "BLACKLISTED_CONTRACT", 400)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	}
}

// validateTransactionSize checks that the transaction data does not exceed the max allowed size.
func validateTransactionSize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonBytes, _ := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))
		if len(jsonBytes) > 0 {
			var transaction Transaction
			err := json.Unmarshal(jsonBytes, &transaction)
			if err != nil {
				logFailure("PARSING_ERROR", r)
				return
			}
			for _, action := range transaction.Actions {
				if len(action.Data) > appConfig.MaxTransactionSize {
					logFailure("INVALID_TRANSACTION_SIZE", r)
					http.Error(w, "INVALID_TRANSACTION_SIZE", 400)
					return
				}
			}
		}
		next.ServeHTTP(w, r)
	}
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

// If the request passes all middleware validations
// we forward it to the node to be processed.
func forwardCallToNodeos(w http.ResponseWriter, r *http.Request) {
	log.Println("forward calls to nodeos")

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

	body, _ = ioutil.ReadAll(res.Body)
	// TODO: this is just for debugging
	log.Printf("Nodeos response: %s - %s", res.Status, body)

	if res.StatusCode == 200 {
		logSuccess("SUCCESS", r)
	} else {
		logFailure("TRANSACTION_FAILED", r)
	}

	_, err = w.Write(body)
	if err != nil {
		log.Printf("Error writing response body %s", err)
		return
	}
}

func addFilterHandlers(mux *http.ServeMux) {
	// Middleware are executed in the order that they are passed to chainMiddleware.
	middlewareChain := chainMiddleware(
		validateJSON,
		validateTransactionSize,
		validateSignatures,
		validateContract,
	)

	mux.HandleFunc("/", middlewareChain(forwardCallToNodeos))
}
