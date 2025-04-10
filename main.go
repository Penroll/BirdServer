package main

//RUN THIS ONE A t4g.small AWS INSTANCE, INCLUDING THE MODEL
//TRAIN THE YOLOv8s MODEL WITH THE DATASET DOWNLOADED FROM CORNELL

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
)

func requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		expectedToken := "Bearer my-secret-token" // Ideally from env/config

		if authHeader != expectedToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, "Hello World")
}

func getBirdsRecorded(w http.ResponseWriter, r *http.Request) {
	_, _ = fmt.Fprintf(w, "IN PROGRESS")
}

func getBirdFreqGraph(w http.ResponseWriter, r *http.Request) {
	bird := r.URL.Query().Get("bird")
	newString := "Bird is " + bird + "\n"
	_, _ = fmt.Fprintf(w, newString)

}

func getBirdsInImage(w http.ResponseWriter, r *http.Request) {
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
	err = os.MkdirAll("uploads", os.ModePerm)
	if err != nil {
		http.Error(w, "Error creating directory: "+err.Error(), http.StatusInternalServerError)
		return
	}
	out, err := os.Create(savePath)
	if err != nil {
		http.Error(w, "Error saving image: "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			http.Error(w, "Error closing file: "+err.Error(), http.StatusInternalServerError)
		}
	}(out)
	_, _ = io.Copy(out, file)

	// Optional: Send to Python microservice and get JSON result
	result, err := sendImageAndReceiveJSON(savePath)
	if err != nil {
		http.Error(w, "Microservice error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Return the result as JSON
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(result)
	if err != nil {
		http.Error(w, "Error encoding JSON: "+err.Error(), http.StatusInternalServerError)
		return
	}
}

func sendImageAndReceiveJSON(inputPath string) (map[string]interface{}, error) {
	file, err := os.Open(inputPath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(inputPath))
	if err != nil {
		return nil, err
	}
	_, err = io.Copy(part, file)
	if err != nil {
		return nil, err
	}
	_ = writer.Close()

	req, err := http.NewRequest("POST", "http://localhost:8000/process-image", body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Check response status
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("microservice returned status: %s", resp.Status)
	}

	// Parse JSON response
	var result map[string]interface{}
	decoder := json.NewDecoder(resp.Body)
	if err := decoder.Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode JSON: %v", err)
	}

	return result, nil
}

func main() {

	http.HandleFunc("/", handler)
	http.HandleFunc("/birds/", getBirdsRecorded)
	http.HandleFunc("/bird/", getBirdFreqGraph)
	http.HandleFunc("/identify-bird", getBirdsInImage)
	fmt.Println("Server started on port 8080")

	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		return
	}

}
