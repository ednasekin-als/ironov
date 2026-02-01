package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"time"
)

const uploadcareAPI = "https://upload.uploadcare.com/base/"

type UploadcareResponse struct {
	File string `json:"file"`
}

type ServerResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	URL     string `json:"url"`
}

func Handler(w http.ResponseWriter, r *http.Request) {
	// CORS headers
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	publicKey := os.Getenv("UPLOADCARE_PUBLIC_KEY")
	secretKey := os.Getenv("UPLOADCARE_SECRET_KEY")

	if publicKey == "" {
		sendJSONError(w, "Uploadcare configuration error", http.StatusInternalServerError)
		return
	}

	err := r.ParseMultipartForm(10 << 20)
	if err != nil {
		sendJSONError(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		sendJSONError(w, "No image file: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer file.Close()

	imageData, err := io.ReadAll(file)
	if err != nil {
		sendJSONError(w, "Failed to read image: "+err.Error(), http.StatusInternalServerError)
		return
	}

	fileURL, fileID, err := uploadToUploadcare(imageData, publicKey, header.Filename)
	if err != nil {
		sendJSONError(w, "Failed to upload to storage: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Удаление через 5 минут
	if secretKey != "" {
		go func(id string) {
			time.Sleep(5 * time.Minute)
			_ = deleteFromUploadcare(id, secretKey)
		}(fileID)
	}

	response := ServerResponse{
		Success: true,
		ID:      fileID,
		URL:     fileURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func uploadToUploadcare(imageBytes []byte, publicKey string, filename string) (string, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", "", err
	}

	_, err = io.Copy(part, bytes.NewReader(imageBytes))
	if err != nil {
		return "", "", err
	}

	writer.WriteField("UPLOADCARE_PUB_KEY", publicKey)
	writer.WriteField("UPLOADCARE_STORE", "1")

	writer.Close()

	req, err := http.NewRequest("POST", uploadcareAPI, body)
	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var uploadResp UploadcareResponse
	err = json.Unmarshal(respBody, &uploadResp)
	if err != nil {
		return "", "", err
	}

	fileID := uploadResp.File
	fileURL := fmt.Sprintf("https://ucarecdn.com/%s/", fileID)

	return fileURL, fileID, nil
}

func deleteFromUploadcare(fileID, secretKey string) error {
	req, err := http.NewRequest(
		"DELETE",
		"https://api.uploadcare.com/files/"+fileID+"/",
		nil,
	)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Uploadcare.Simple "+secretKey+":")
	req.Header.Set("Accept", "application/vnd.uploadcare-v0.7+json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return nil
}

func sendJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
