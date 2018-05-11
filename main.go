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

func parseConfigFile(filename string) {
	fileBody, err := ioutil.ReadFile(filename)

	if err != nil {
		log.Fatalf("Error reading configuration file.")
	}

	json.Unmarshal(fileBody, &config)
}

func main() {
	client = http.Client{}

	parseConfigFile("./config.json")

	log.Println("Proxying and filtering nodeos requests...")
	http.HandleFunc("/", forwardCallToNodeos)
	log.Fatal(http.ListenAndServe(":"+config.ListenPort, nil))
}
