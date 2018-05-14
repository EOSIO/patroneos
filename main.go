package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
)

// Config holds the configuration for the entire application
type Config struct {
	ListenPort     string `json:"listenPort"`
	NodeosProtocol string `json:"nodeosProtocol"`
	NodeosURL      string `json:"nodeosUrl"`
	NodeosPort     string `json:"nodeosPort"`
}

const configFile string = "./config.json"

var config Config
var client http.Client

func forwardCallToNodeos(w http.ResponseWriter, r *http.Request) {
	log.Println("forward calls to nodeos")

	nodeosHost := fmt.Sprintf("%s://%s:%s", config.NodeosProtocol, config.NodeosURL, config.NodeosPort)
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
	w.Write(body)
}

func updateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == "GET" {
		responseBody, err := json.MarshalIndent(config, "", "    ")
		if err != nil {
			log.Printf("Failed to marshal config %s", err)
		}
		w.Write(responseBody)
	} else if r.Method == "POST" {
		body, _ := ioutil.ReadAll(r.Body)
		json.Unmarshal(body, &config)
		ioutil.WriteFile(configFile, body, 0644)
	}
}

func parseConfigFile() {
	fileBody, err := ioutil.ReadFile(configFile)

	if err != nil {
		log.Fatalf("Error reading configuration file.")
	}

	err = json.Unmarshal(fileBody, &config)

	if err != nil {
		log.Fatalf("Error unmarshaling configuration file.")
	}
}

func main() {
	client = http.Client{}

	parseConfigFile()

	log.Println("Proxying and filtering nodeos requests...")
	http.HandleFunc("/", forwardCallToNodeos)
	http.HandleFunc("/config", updateConfig)
	log.Fatal(http.ListenAndServe(":"+config.ListenPort, nil))
}
