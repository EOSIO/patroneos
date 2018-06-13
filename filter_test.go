package main

// Middleware testing strategy adapted from https://medium.com/@PurdonKyle/unit-testing-golang-http-middleware-c7727ca896ea

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"testing"
)

type TestStruct struct {
	description  string
	url          string
	body         []byte
	expectedBody string
	expectedCode int
}

func setConfig() {
	appConfig = Config{}
	appConfig.ContractBlackList = map[string]bool{"currency": true}
	appConfig.MaxSignatures = 1
	appConfig.MaxTransactionSize = 50
	appConfig.MaxTransactions = 2
}

func getTestHandler() http.HandlerFunc {
	fn := func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("SUCCESS\n"))
		w.WriteHeader(200)
	}

	return http.HandlerFunc(fn)
}

func verifyMiddleware(t *testing.T, ts *httptest.Server, tc TestStruct) {
	url := string(ts.URL) + tc.url

	res, err := http.Post(url, "application/json", bytes.NewBuffer(tc.body))
	if err != nil {
		t.Errorf("There should not be a server error.")
	}

	if res != nil {
		defer res.Body.Close()
	}

	b, err := ioutil.ReadAll(res.Body)
	body := string(b)

	if err != nil {
		t.Errorf("There should not be a server error.")
	}

	if res.StatusCode != tc.expectedCode {
		t.Errorf("Expected status code to be %d and got %d.", tc.expectedCode, res.StatusCode)
	}

	if body != tc.expectedBody {
		t.Errorf("Expected body to be %s and got %s.", tc.expectedBody, body)
	}
}

func TestGetHostHeader(t *testing.T) {
	host := "testing"
	req, _ := http.NewRequest("GET", "localhost", nil)
	req.Header.Set("X-Forwarded-For", host)

	if getHost(req) != host {
		t.Errorf("Expected host to be %s", host)
	}
}

func TestGetHostRemoteAddr(t *testing.T) {
	host := "192.168.0.1"
	req, _ := http.NewRequest("GET", "localhost", nil)
	req.RemoteAddr = host
	if getHost(req) != host {
		t.Errorf("Expected host to be %s", host)
	}
}

func TestValidateJSON(t *testing.T) {
	tests := []TestStruct{
		{
			description:  "invalid",
			url:          "/",
			body:         []byte(`{"name"}`),
			expectedBody: "{\"message\":\"INVALID_JSON\",\"code\":400}",
			expectedCode: 400,
		},
		{
			description:  "valid",
			url:          "/",
			body:         []byte(`{"name": "Tony Stark"}`),
			expectedBody: "SUCCESS\n",
			expectedCode: 200,
		},
	}

	ts := httptest.NewServer(validateJSON(getTestHandler()))
	defer ts.Close()

	for _, tc := range tests {
		verifyMiddleware(t, ts, tc)
	}
}

func TestValidateMaxTransactions(t *testing.T) {
	tests := []TestStruct{
		{
			description:  "invalid",
			url:          "/",
			body:         []byte(`[{"name": "Tony Stark"}, {"name": "Steve Rogers"},{"name": "Bruce Banner"}]`),
			expectedBody: "{\"message\":\"TOO_MANY_TRANSACTIONS\",\"code\":400}",
			expectedCode: 400,
		},
		{
			description:  "valid",
			url:          "/",
			body:         []byte(`[{"name": "Tony Stark"}, {"name": "Steve Rogers"}]`),
			expectedBody: "SUCCESS\n",
			expectedCode: 200,
		},
	}

	ts := httptest.NewServer(validateMaxTransactions(getTestHandler()))
	defer ts.Close()

	setConfig()

	for _, tc := range tests {
		verifyMiddleware(t, ts, tc)
	}
}

func TestValidateContract(t *testing.T) {
	invalidAction := Action{
		Code: "currency",
		Data: "1234567890",
	}
	invalidTransaction := Transaction{
		Actions:    []Action{invalidAction},
		Signatures: []string{"12345"},
	}
	validTransaction := invalidTransaction
	validAction := invalidAction
	validAction.Code = "tokens"
	validTransaction.Actions = make([]Action, 1)
	validTransaction.Actions[0] = validAction

	invalidBody, _ := json.Marshal(invalidTransaction)
	validBody, _ := json.Marshal(validTransaction)
	tests := []TestStruct{
		{
			description:  "invalid",
			url:          "/",
			body:         invalidBody,
			expectedBody: "{\"message\":\"BLACKLISTED_CONTRACT\",\"code\":400}",
			expectedCode: 400,
		},
		{
			description:  "valid",
			url:          "/",
			body:         validBody,
			expectedBody: "SUCCESS\n",
			expectedCode: 200,
		},
	}

	ts := httptest.NewServer(validateContract(getTestHandler()))
	defer ts.Close()

	setConfig()

	for _, tc := range tests {
		verifyMiddleware(t, ts, tc)
	}
}

func TestValidateSignatures(t *testing.T) {
	invalidTransaction := Transaction{
		Actions: []Action{
			{
				Code: "tokens",
				Data: "1234567890",
			},
		},
		Signatures: []string{"12345", "54321"},
	}

	validTransaction := invalidTransaction
	validTransaction.Signatures = make([]string, 1)
	validTransaction.Signatures[0] = "12345"

	invalidBody, _ := json.Marshal(invalidTransaction)
	validBody, _ := json.Marshal(validTransaction)
	tests := []TestStruct{
		{
			description:  "invalid",
			url:          "/",
			body:         invalidBody,
			expectedBody: "{\"message\":\"INVALID_NUMBER_SIGNATURES\",\"code\":400}",
			expectedCode: 400,
		},
		{
			description:  "valid",
			url:          "/",
			body:         validBody,
			expectedBody: "SUCCESS\n",
			expectedCode: 200,
		},
	}

	ts := httptest.NewServer(validateMaxSignatures(getTestHandler()))
	defer ts.Close()

	setConfig()

	for _, tc := range tests {
		verifyMiddleware(t, ts, tc)
	}
}

func TestValidateTransactionSize(t *testing.T) {
	invalidAction := Action{
		Code: "tokens",
		Data: string(bytes.Repeat([]byte("a"), 100)),
	}

	invalidTransaction := Transaction{
		Actions:    []Action{invalidAction},
		Signatures: []string{"12345"},
	}

	validTransaction := invalidTransaction
	validAction := invalidAction
	validAction.Data = string([]byte("abcd"))
	validTransaction.Actions = make([]Action, 1)
	validTransaction.Actions[0] = validAction

	invalidBody, _ := json.Marshal(invalidTransaction)
	validBody, _ := json.Marshal(validTransaction)
	tests := []TestStruct{
		{
			description:  "invalid",
			url:          "/",
			body:         invalidBody,
			expectedBody: "{\"message\":\"INVALID_TRANSACTION_SIZE\",\"code\":400}",
			expectedCode: 400,
		},
		{
			description:  "valid",
			url:          "/",
			body:         validBody,
			expectedBody: "SUCCESS\n",
			expectedCode: 200,
		},
	}

	ts := httptest.NewServer(validateTransactionSize(getTestHandler()))
	defer ts.Close()

	setConfig()

	for _, tc := range tests {
		verifyMiddleware(t, ts, tc)
	}

}
