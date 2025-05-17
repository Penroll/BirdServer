package main

//RUN THIS ONE A t4g.small AWS INSTANCE, INCLUDING THE MODEL
//TRAIN THE YOLOv8s MODEL WITH THE DATASET DOWNLOADED FROM CORNELL

import (
	"BirdServer/api"
	"BirdServer/db"
	"fmt"
	"log"
	"net/http"
)

func defaultHandler(w http.ResponseWriter, _ *http.Request) {
	_, _ = fmt.Fprintf(w, "Hello World")
}

func main() {

	database := db.ConnectDB()

	http.HandleFunc("/", defaultHandler)

	http.HandleFunc("/api/birds/", api.Auth(api.GetBirdsHandler(database)))
	http.HandleFunc("/api/bird/", api.Auth(api.GetBirdTimes(database)))
	http.HandleFunc("/api/identify-bird", api.Auth(api.IdentifyBirdsHandler(database)))
	fmt.Println("Server started on port 8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
