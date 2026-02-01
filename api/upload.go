package handler

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

const (
    uploadcareAPI    = "https://upload.uploadcare.com/base/"
    deleteAPI        = "https://api.uploadcare.com/files/%s/storage/"
    publicKey        = "YOUR_PUBLIC_KEY" // Замени на свой
    secretKey        = "YOUR_SECRET_KEY" // Замени на свой
)

type UploadcareResponse struct {
    File string `json:"file"`
}

type ServerResponse struct {
    Success bool   `json:"success"`
    ID      string `json:"id"`
    URL     string `json:"url"`
}

type DeleteScheduler struct {
    fileID string
    timer  *time.Timer
}

var deleteQueue = make(map[string]*DeleteScheduler)

func Handler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusOK)
        return
    }
    
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    err := r.ParseMultipartForm(10 << 20)
    if err != nil {
        sendJSONError(w, "Failed to parse form", http.StatusBadRequest)
        return
    }
    
    file, _, err := r.FormFile("image")
    if err != nil {
        sendJSONError(w, "No image file", http.StatusBadRequest)
        return
    }
    defer file.Close()
    
    imageData, err := io.ReadAll(file)
    if err != nil {
        sendJSONError(w, "Failed to read image", http.StatusInternalServerError)
        return
    }
    
    fileURL, fileID, err := uploadToUploadcare(imageData)
    if err != nil {
        sendJSONError(w, "Failed to upload to storage: "+err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Планируем удаление через 1 час
    scheduleFileDeletion(fileID)
    
    response := ServerResponse{
        Success: true,
        ID:      fileID,
        URL:     fileURL,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func uploadToUploadcare(imageBytes []byte) (string, string, error) {
    body := &bytes.Buffer{}
    writer := multipart.NewWriter(body)
    
    part, err := writer.CreateFormFile("file", "valentine.png")
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
    
    // Используем фиксированный поддомен
    fileURL := fmt.Sprintf("https://ucarecdn.com/%s/valentine.png", fileID)
    
    return fileURL, fileID, nil
}

func scheduleFileDeletion(fileID string) {
    // Отменяем предыдущий таймер, если он существует
    if scheduler, exists := deleteQueue[fileID]; exists {
        scheduler.timer.Stop()
        delete(deleteQueue, fileID)
    }
    
    // Создаем новый таймер на 1 час
    timer := time.AfterFunc(1*time.Hour, func() {
        deleteFileFromUploadcare(fileID)
        delete(deleteQueue, fileID)
    })
    
    deleteQueue[fileID] = &DeleteScheduler{
        fileID: fileID,
        timer:  timer,
    }
    
    fmt.Printf("🗑️ Запланировано удаление файла %s через 1 час\n", fileID)
}

func deleteFileFromUploadcare(fileID string) error {
    client := &http.Client{}
    
    url := fmt.Sprintf(deleteAPI, fileID)
    req, err := http.NewRequest("DELETE", url, nil)
    if err != nil {
        return err
    }
    
    // Аутентификация для Uploadcare API
    req.SetBasicAuth(publicKey, secretKey)
    
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == 200 || resp.StatusCode == 204 {
        fmt.Printf("✅ Файл %s успешно удален\n", fileID)
        return nil
    }
    
    body, _ := io.ReadAll(resp.Body)
    return fmt.Errorf("failed to delete file: %s", string(body))
}

func sendJSONError(w http.ResponseWriter, message string, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{
        "error": message,
    })
}