package api

import (
	"BirdServer/db"
	"BirdServer/models"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

func GetBirdsHandler(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		feederToken := strings.TrimPrefix(authHeader, "Bearer ")
		var birdData []db.BirdData
		database.Where("feeder_token = ?", feederToken).Find(&birdData)
		finalStrings, err := models.ConvertBirdDataToDTO(birdData)
		if err != nil {
			fmt.Println(err)
		}
		_, _ = fmt.Fprintf(w, finalStrings)
	}
}

func GetBirdTimes(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		feederToken := strings.TrimPrefix(authHeader, "Bearer ")
		bird := r.URL.Query().Get("bird")
		birdData := models.GetBirdRelativeData(strings.ToLower(bird), feederToken, database)
		birdString := fmt.Sprint(birdData)
		_, _ = fmt.Fprintf(w, birdString)
	}
}

func IdentifyBirdsHandler(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		feederToken := strings.TrimPrefix(authHeader, "Bearer ")
		// Limit request size to prevent abuse (e.g., 10MB)
		r.ParseMultipartForm(10 << 20)

		// Get uploaded file
		file, header, err := r.FormFile("image")
		if err != nil {
			http.Error(w, "Failed to get uploaded file: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		// Ensure uploads folder exists
		os.MkdirAll("uploads", os.ModePerm)

		// Save the file locally
		savePath := filepath.Join("uploads", header.Filename)
		out, err := os.Create(savePath)
		if err != nil {
			http.Error(w, "Failed to create file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer out.Close()

		_, err = io.Copy(out, file)
		if err != nil {
			http.Error(w, "Failed to save file: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Call your Python microservice (replace this with your actual logic)
		result, err := models.SendImageAndReceiveJSON(savePath)
		if err != nil {
			os.Remove(savePath) // clean up even on error
			http.Error(w, "Microservice error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Clean up the uploaded file
		os.Remove(savePath)

		toPrint := models.ConvertDetectionsToString(result)
		println(toPrint)
		models.AddBirdsToDb(result, feederToken, database)

		// Return the result as JSON
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(result)
		if err != nil {
			http.Error(w, "Error encoding JSON: "+err.Error(), http.StatusInternalServerError)
			return
		}
	}
}

func Auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := os.Getenv("feeder_token") // replace with an environment variable or better secure method

		authHeader := r.Header.Get("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, "Unauthorized: Missing or invalid Authorization header", http.StatusUnauthorized)
			return
		}

		sentToken := strings.TrimPrefix(authHeader, "Bearer ")
		if sentToken != token {
			http.Error(w, "Unauthorized: Invalid token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}
