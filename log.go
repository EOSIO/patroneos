package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
)

// Log defines the fields needed for the Fail2Ban logs
type Log struct {
	Host    string `json:"host"`
	Success bool   `json:"success"`
	Message string `json:"message"`
}

var logFile *os.File
var logger *log.Logger

// listenForLogs listens to the middleware for success/failure logs
// and logs them to the correct file for Fail2Ban
func listenForLogs(w http.ResponseWriter, r *http.Request) {
	var logEntry Log

	body, _ := ioutil.ReadAll(r.Body)

	err := json.Unmarshal(body, &logEntry)
	if err != nil {
		log.Printf("Error unmarshalling logs %s", err)
		return
	}

	// Print to file and stderr for now
	logger.Printf("%s %t %s", logEntry.Host, logEntry.Success, logEntry.Message)
	log.Printf("%s %t %s", logEntry.Host, logEntry.Success, logEntry.Message)
}

func addLogHandlers(mux *http.ServeMux) {
	var err error
	logFile, err = os.OpenFile(appConfig.LogFileLocation, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalf("Error opening log file %s", err)
	}

	logger = log.New(logFile, "", log.LstdFlags)
	mux.HandleFunc("/", listenForLogs)
}
