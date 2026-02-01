package handler

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "os"
    // УБРАТЬ: "strings" - не используется
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
    
    // Get Uploadcare public key from environment
    publicKey := os.Getenv("UPLOADCARE_PUBLIC_KEY")
    if publicKey == "" {
        sendJSONError(w, "Uploadcare configuration error", http.StatusInternalServerError)
        return
    }
    
    // Parse multipart form
    err := r.ParseMultipartForm(10 << 20) // 10MB
    if err != nil {
        sendJSONError(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
        return
    }
    
    // Get file
    file, _, err := r.FormFile("image")
    if err != nil {
        sendJSONError(w, "No image file: "+err.Error(), http.StatusBadRequest)
        return
    }
    defer file.Close()
    
    // Read file
    imageData, err := io.ReadAll(file)
    if err != nil {
        sendJSONError(w, "Failed to read image: "+err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Upload to Uploadcare
    fileURL, fileID, err := uploadToUploadcare(imageData, publicKey)
    if err != nil {
        sendJSONError(w, "Failed to upload to storage: "+err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Send success response
    response := ServerResponse{
        Success: true,
        ID:      fileID,
        URL:     fileURL,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func uploadToUploadcare(imageBytes []byte, publicKey string) (string, string, error) {
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    
    // Add file
    part, err := writer.CreateFormFile("file", "valentine.png")
    if err != nil {
        return "", "", err
    }
    
    _, err = io.Copy(part, bytes.NewReader(imageBytes))
    if err != nil {
        return "", "", err
    }
    
    // Add Uploadcare parameters
    writer.WriteField("UPLOADCARE_PUB_KEY", publicKey)
    writer.WriteField("UPLOADCARE_STORE", "1") // Auto-store file
    
    writer.Close()
    
    // Send request
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
    
    // Read response
    respBody, err := io.ReadAll(resp.Body)
    if err != nil {
        return "", "", err
    }
    
    // Parse JSON
    var uploadResp UploadcareResponse
    err = json.Unmarshal(respBody, &uploadResp)
    if err != nil {
        return "", "", err
    }
    
    // Extract file ID from response (UUID format)
    fileID := uploadResp.File
    
    // Construct CDN URL
    // Format: https://ucarecdn.com/{file_id}/-/format/png/
    fileURL := fmt.Sprintf("https://ucarecdn.com/%s/-/format/png/", fileID)
    
    return fileURL, fileID, nil
}

func sendJSONError(w http.ResponseWriter, message string, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{
        "error": message,
    })
}