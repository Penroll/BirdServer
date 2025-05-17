package models

import (
	"BirdServer/db"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gorm.io/gorm"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

func AddBirdsToDb(data map[string]interface{}, feederToken string, database *gorm.DB) {
	var birds []string
	re := regexp.MustCompile(`\([^()]*\)`)

	if detections, ok := data["detections"].([]interface{}); ok {
		for _, d := range detections {
			if detMap, ok := d.(map[string]interface{}); ok {
				class, _ := detMap["class"].(string)
				rawClass := fmt.Sprintf("%s", class)
				processedClass := strings.TrimSpace(strings.ToLower(re.ReplaceAllString(rawClass, "")))
				birds = append(birds, processedClass)
			}
		}
	}
	hour := time.Now().Hour()
	for _, bird := range birds {
		var foundBird db.BirdData
		err := database.Where("name = ? AND feeder_token = ?", bird, feederToken).First(&foundBird).Error
		if err == nil {
			if foundBird.HourlyObservations == nil {
				foundBird.HourlyObservations = make(db.HourlyObservations)
			}
			foundBird.HourlyObservations[hour]++
			database.Save(&foundBird)
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			newBird := db.BirdData{
				Name:        bird,
				FeederToken: feederToken,
				CreatedAt:   time.Now(),
				HourlyObservations: db.HourlyObservations{
					hour: 1,
				},
			}
			database.Create(&newBird)
		} else {
			log.Println("Database error:", err)
		}
	}
	return
}

func GetBirdRelativeData(birdName string, feederToken string, database *gorm.DB) (data map[int]int) {
	var birdData []db.BirdData
	database.Where("name = ? AND feeder_token = ?", birdName, feederToken).Find(&birdData)
	return birdData[0].HourlyObservations
}

func SendImageAndReceiveJSON(inputPath string) (map[string]interface{}, error) {
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

	req, err := http.NewRequest("POST", "http://18.216.209.5:5000/process-image", body)
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

func ConvertDetectionsToString(data map[string]interface{}) string {
	var lines []string

	// Extract and format
	if detections, ok := data["detections"].([]interface{}); ok {
		for _, d := range detections {
			if detMap, ok := d.(map[string]interface{}); ok {
				class, _ := detMap["class"].(string)
				confidence, _ := detMap["confidence"].(float64)
				lines = append(lines, fmt.Sprintf("%s,%.2f", class, confidence))
			}
		}
	}

	// Join into one string with \n
	output := strings.Join(lines, "\n")
	return output
}
