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

// Config defines the application configuration
type Config struct {
	ListenPort         string          `json:"listenPort"`
	NodeosProtocol     string          `json:"nodeosProtocol"`
	NodeosURL          string          `json:"nodeosUrl"`
	NodeosPort         string          `json:"nodeosPort"`
	ContractBlackList  map[string]bool `json:"contractBlackList"`
	MaxSignatures      int             `json:"maxSignatures"`
	MaxTransactionSize int             `json:"maxTransactionSize"`
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

const configFile string = "./config.json"

var appConfig Config
var client http.Client

func getHost(r *http.Request) string {
	var remoteIP string

	if header := r.Header.Get("X-Forwarded-For"); header != "" {
		remoteIP = header
	} else {
		remoteIP = r.RemoteAddr
	}

	return remoteIP
}

func logFailure(message string, r *http.Request) {
	remoteHost := getHost(r)
	log.Printf("Failure: %s %s", remoteHost, message)
}

func logSuccess(message string, r *http.Request) {
	remoteHost := getHost(r)
	log.Printf("Success: %s %s", remoteHost, message)
}

func validateJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonBytes, err := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))
		if !json.Valid(jsonBytes) || err != nil {
			logFailure("Invalid JSON provided", r)
			http.Error(w, "INVALID_JSON", 400)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func validateSignatures(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonBytes, _ := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))
		var transaction Transaction
		err := json.Unmarshal(jsonBytes, &transaction)
		if err != nil {
			logFailure("Error parsing transaction format", r)
			return
		}
		if len(transaction.Signatures) > appConfig.MaxSignatures {
			// TODO: should this fail or allow through?
			logFailure("Too many signatures on the transaction", r)
			http.Error(w, "INVALID_NUMBER_SIGNATURES", 400)
			return
		}
		next.ServeHTTP(w, r)
	}
}

func validateContract(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonBytes, _ := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))
		var transaction Transaction
		err := json.Unmarshal(jsonBytes, &transaction)
		if err != nil {
			logFailure("Error parsing transaction format", r)
			return
		}

		for _, action := range transaction.Actions {
			_, exists := appConfig.ContractBlackList[action.Code]
			if exists {
				logFailure("This contract is blacklisted", r)
				http.Error(w, "BLACKLISTED_CONTRACT", 400)
				return
			}
		}
		next.ServeHTTP(w, r)
	}
}

func validateTransactionSize(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		jsonBytes, _ := ioutil.ReadAll(r.Body)
		r.Body = ioutil.NopCloser(bytes.NewBuffer(jsonBytes))
		var transaction Transaction
		err := json.Unmarshal(jsonBytes, &transaction)
		if err != nil {
			logFailure("Error parsing transaction format", r)
			return
		}
		for _, action := range transaction.Actions {
			if len(action.Data) > appConfig.MaxTransactionSize {
				logFailure("Transaction size exceed maximum", r)
				http.Error(w, "INVALID_TRANSACTION_SIZE", 400)
				return
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
	log.Printf("Nodeos response: %s - %s", res.Status, body)
	logSuccess("", r)
	w.Write(body)
}

func updateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		responseBody, err := json.MarshalIndent(appConfig, "", "    ")
		if err != nil {
			log.Printf("Failed to marshal config %s", err)
		}
		w.Write(responseBody)
	} else if r.Method == "POST" {
		body, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(body, &appConfig)
		ioutil.WriteFile(configFile, body, 0644)
	}
}

func parseConfigFile() {
	fileBody, err := ioutil.ReadFile(configFile)

	if err != nil {
		log.Fatalf("Error reading configuration file.")
	}

	err = json.Unmarshal(fileBody, &appConfig)

	if err != nil {
		log.Fatalf("Error unmarshaling configuration file.")
	}
}

func main() {
	client = http.Client{}

	middlewareChain := chainMiddleware(
		validateJSON,
		validateTransactionSize,
		validateSignatures,
		validateContract,
	)

	parseConfigFile()

	log.Println("Proxying and filtering nodeos requests...")
	http.HandleFunc("/", middlewareChain(forwardCallToNodeos))
	http.HandleFunc("/config", updateConfig)
	log.Fatal(http.ListenAndServe(":"+appConfig.ListenPort, nil))
}
