package main

import (
	"encoding/json"
	"log"
)

func logMessage(userID int64, username, messageType, content string) {
	log.Printf("[User %d (@%s)] %s: %s", userID, username, messageType, content)
}

func logOpenRouterRequest(userID int64, username string, reqBody interface{}) {
	reqJSON, _ := json.Marshal(reqBody)
	log.Printf("[OpenRouter Request] User %d (@%s): %s", userID, username, string(reqJSON))
}

func logOpenRouterResponse(userID int64, username string, statusCode int, respBody []byte) {
	log.Printf("[OpenRouter Response] User %d (@%s) Status %d: %s", userID, username, statusCode, string(respBody))
}
