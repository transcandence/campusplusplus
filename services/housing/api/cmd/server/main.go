package main

import (
	"log"
	"net/http"
)

func main() {
	log.Println("Housing service is running")
	log.Fatal(http.ListenAndServe(":8080", nil))
}