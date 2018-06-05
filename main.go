package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// Config defines the application configuration
type Config struct {
	ListenPort         string          `json:"listenPort"`
	NodeosProtocol     string          `json:"nodeosProtocol"`
	NodeosURL          string          `json:"nodeosUrl"`
	NodeosPort         string          `json:"nodeosPort"`
	ContractBlackList  map[string]bool `json:"contractBlackList"`
	MaxSignatures      int             `json:"maxSignatures"`
	MaxTransactionSize int             `json:"maxTransactionSize"`
	MaxTransactions    int             `json:"maxTransactions"`
	LogEndpoints       []string        `json:"logEndpoints"`
	FilterEndpoints    []string        `json:"filterEndpoints"`
	LogFileLocation    string          `json:"logFileLocation"`
}

var configFile string
var operatingMode string

var appConfig Config

// updateConfig allows the configuration to be updated via POST requests.
func updateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		responseBody, err := json.MarshalIndent(appConfig, "", "    ")
		if err != nil {
			log.Printf("Failed to marshal config %s", err)
			return
		}

		_, err = w.Write(responseBody)
		if err != nil {
			log.Printf("Error writing response body %s", err)
			return
		}
	} else if r.Method == "POST" {
		body, _ := ioutil.ReadAll(r.Body)

		err := json.Unmarshal(body, &appConfig)
		if err != nil {
			log.Printf("Error unmarshalling updated config %s", err)
			return
		}

		err = ioutil.WriteFile(configFile, body, 0644)
		if err != nil {
			log.Printf("Error writing new configuration to file %s", err)
			return
		}
	}
}

func parseArgs() {
	const (
		defaultConfigLocation = "./config.json"
		defaultOperatingMode  = "filter"
		defaultShowHelp       = false
	)
	var showHelp bool
	flag.BoolVar(&showHelp, "h", defaultShowHelp, "shows application help")
	flag.StringVar(&configFile, "configFile", defaultConfigLocation, "location of the file used for application configuration")
	flag.StringVar(&operatingMode, "mode", defaultOperatingMode, "mode in which the application will run")

	flag.Parse()

	if showHelp {
		flag.Usage()
		os.Exit(1)
	}
}

func parseConfigFile() {
	fileBody, err := ioutil.ReadFile(configFile)

	if err != nil {
		log.Fatalf("Error reading configuration file.")
	}

	err = json.Unmarshal(fileBody, &appConfig)

	if err != nil {
		log.Fatalf("Error unmarshalling configuration file.")
	}
}

func main() {
	parseArgs()
	parseConfigFile()

	mux := http.NewServeMux()
	mux.HandleFunc("/patroneos/config", updateConfig)

	if operatingMode == "filter" {
		addFilterHandlers(mux)
		fmt.Println("Filtering node requests...")
	} else if operatingMode == "fail2ban-relay" {
		addLogHandlers(mux)
		fmt.Println("Relaying log events to fail2ban...")
	} else {
		fmt.Printf("This mode is not supported.")
		os.Exit(1)
	}

	log.Fatal(http.ListenAndServe(":"+appConfig.ListenPort, mux))
}
