package main

import (
	"github.com/gorilla/mux"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
	"strconv"
	"log"
)

var (
	bills = make(map[string]*Bill)
)

type Bill struct {
	Released bool
	Expiry   time.Time
}

func handler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	billId := vars["billId"]
	sessionId, _ := r.Cookie("sessionId")

	if !isAuthenticated(sessionId.Value) {
		return
	}

	billIdInt, _ := strconv.ParseInt(billId, 10, 32)
	billIdString := strconv.FormatInt(billIdInt, 10)
	if isAuthorized(billId, sessionId.Value) {
		if billIdString != "0" {
			releaseBill(billId, 10*time.Second)
		}
	}

	if IsBillReleased(billId) {
		internalBillingURL := strings.Replace("http://127.0.0.1:8081/internal/billing/:billId", ":billId", billIdString, 1)
		
		req, _ := http.NewRequest("GET", internalBillingURL, nil)
		req.Header.Set("Authorization", getBearerToken())
		client := &http.Client{}
		resp, _ := client.Do(req)
	
		defer resp.Body.Close()

		billingResponseBody, _ := ioutil.ReadAll(resp.Body)
		w.Write(billingResponseBody)
	}

}

func releaseBill(billId string, expiryTime time.Duration) {
	bill, exists := bills[billId]
	if !exists {
		bill = &Bill{}
		bills[billId] = bill
	}

	bill.Released = true
	bill.Expiry = time.Now().Add(expiryTime)

	go func() {
		<-time.After(expiryTime)
		bill, exists := bills[billId]
		if exists && time.Now().After(bill.Expiry) {
			bill.Released = false
			delete(bills, billId)
		}
	}()
}

func IsBillReleased(billId string) bool {
	bill, exists := bills[billId]
	return exists && bill.Released
}

func isAuthorized(billId string, sessionId string) bool {
	sessionBillMap := map[string]string{
		"bob": "1111",
		"alice": "2222",
		"john": "3333",
	}

	if mappedBillId, exists := sessionBillMap[sessionId]; exists {
		return mappedBillId == billId
	}

	return false
}

func handleInternalBillingRequest(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	billId := vars["billId"]

	billingData, err := ioutil.ReadFile("bills/" + billId)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	w.Write(billingData)
}

func isAuthenticated(sessionId string) bool {
	validSessionIds := []string{"bob", "alice", "john"}

	for _, validSessionId := range validSessionIds {
		if sessionId == validSessionId {
			return true
		}
	}

	return false
}


func getBearerToken() string {
	return "bearer_token"
}


func main() {
	router := mux.NewRouter()
	router.HandleFunc("/billing/{billId}", handler)

	internalRouter := mux.NewRouter()
	internalRouter.HandleFunc("/internal/billing/{billId}", handleInternalBillingRequest)

	go func() {
		if err := http.ListenAndServe(":8081", internalRouter); err != nil {
			log.Fatal(err)
	        }
	}()

	if err := http.ListenAndServe(":8080", router); err != nil {
		log.Fatal(err)
        }
}
