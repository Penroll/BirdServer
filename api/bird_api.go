package api

import (
	"BirdServer/db"
	"BirdServer/models"
	"encoding/json"
	"fmt"
	"gorm.io/gorm"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// TODO: Get databse queries out of here
func GetBirdsHandler(database *gorm.DB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		feederToken := strings.TrimPrefix(authHeader, "Bearer ")
		var birdData []db.BirdData
		database.Select("name").Where("feeder_token = ?", feederToken).Find(&birdData)

		var birdNames []string
		for _, bird := range birdData {
			birdNames = append(birdNames, bird.Name+",")
		}
		slices.Sort(birdNames)
		finalStrings := fmt.Sprint(slices.Compact(birdNames))
		// Remove the final comma
		finalStrings = finalStrings[:len(finalStrings)-2] + "]"
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
		err := r.ParseMultipartForm(10 << 20)
		if err != nil {
			http.Error(w, "Error: File too large. Please limit to 10MB.", http.StatusBadRequest)
			return
		} // 10MB

		file, header, err := r.FormFile("image")
		if err != nil {
			http.Error(w, "Error reading image: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer func(file multipart.File) {
			err := file.Close()
			if err != nil {
				http.Error(w, "Error closing file: "+err.Error(), http.StatusInternalServerError)
			}
		}(file)

		// Optional: Save the image locally
		savePath := filepath.Join("uploads", header.Filename)

		// Optional: Send to Python microservice and get JSON result
		result, err := models.SendImageAndReceiveJSON(savePath)
		if err != nil {
			http.Error(w, "Microservice error: "+err.Error(), http.StatusInternalServerError)
			return
		}

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
