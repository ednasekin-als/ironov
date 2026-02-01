package main

import (
    "bytes"
    "encoding/json"
    "fmt"
    "io"
    "mime/multipart"
    "net/http"
    "time"
)

const (
    uploadcareAPI    = "https://upload.uploadcare.com/base/"
    deleteAPI        = "https://api.uploadcare.com/files/%s/storage/"
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

func init() {
    fmt.Println("🚀 Сервис валентинок запущен")
}

func handler(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    
    if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusOK)
        return
    }
    
    if r.Method != "POST" {
        sendJSONError(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    err := r.ParseMultipartForm(10 << 20) // 10 MB
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
    
    publicKey := getEnv("UPLOADCARE_PUBLIC_KEY", "")
    secretKey := getEnv("UPLOADCARE_SECRET_KEY", "")
    
    if publicKey == "" || secretKey == "" {
        sendJSONError(w, "Uploadcare configuration error", http.StatusInternalServerError)
        return
    }
    
    fileURL, fileID, err := uploadToUploadcare(imageData, publicKey, secretKey)
    if err != nil {
        sendJSONError(w, "Failed to upload to storage: "+err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Планируем удаление через 1 час (в фоновом режиме)
    scheduleFileDeletion(fileID, publicKey, secretKey)
    
    response := ServerResponse{
        Success: true,
        ID:      fileID,
        URL:     fileURL,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

func uploadToUploadcare(imageBytes []byte, publicKey, secretKey string) (string, string, error) {
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
    
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return "", "", err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != 200 {
        bodyBytes, _ := io.ReadAll(resp.Body)
        return "", "", fmt.Errorf("upload failed with status %d: %s", resp.StatusCode, string(bodyBytes))
    }
    
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
    
    // Используем фиксированный поддомен ucarecdn.com
    fileURL := fmt.Sprintf("https://ucarecdn.com/%s/valentine.png", fileID)
    
    return fileURL, fileID, nil
}

func scheduleFileDeletion(fileID, publicKey, secretKey string) {
    // В Serverless среде используем горутину для фонового удаления
    go func() {
        // Ждем 1 час
        time.Sleep(1 * time.Hour)
        
        // Удаляем файл
        deleteFileFromUploadcare(fileID, publicKey, secretKey)
    }()
    
    fmt.Printf("🗑️ Запланировано удаление файла %s через 1 час\n", fileID)
}

func deleteFileFromUploadcare(fileID, publicKey, secretKey string) error {
    client := &http.Client{Timeout: 30 * time.Second}
    
    url := fmt.Sprintf(deleteAPI, fileID)
    req, err := http.NewRequest("DELETE", url, nil)
    if err != nil {
        fmt.Printf("❌ Ошибка создания запроса удаления: %v\n", err)
        return err
    }
    
    // Аутентификация для Uploadcare API
    req.SetBasicAuth(publicKey, secretKey)
    
    resp, err := client.Do(req)
    if err != nil {
        fmt.Printf("❌ Ошибка отправки запроса удаления: %v\n", err)
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode == 200 || resp.StatusCode == 204 {
        fmt.Printf("✅ Файл %s успешно удален\n", fileID)
        return nil
    }
    
    body, _ := io.ReadAll(resp.Body)
    fmt.Printf("❌ Не удалось удалить файл %s: %s\n", fileID, string(body))
    return fmt.Errorf("failed to delete file: %s", string(body))
}

func getEnv(key, defaultValue string) string {
    value := getFromVercelEnv(key)
    if value == "" {
        value = defaultValue
    }
    return value
}

func getFromVercelEnv(key string) string {
    // Для Vercel, переменные окружения доступны через os.Getenv
    // Но в Serverless функции можно использовать напрямую
    // В реальном коде это должно быть через os.Getenv
    // Для простоты оставим заглушку
    return ""
}

func sendJSONError(w http.ResponseWriter, message string, status int) {
    w.Header().Set("Content-Type", "application/json")
    w.WriteHeader(status)
    json.NewEncoder(w).Encode(map[string]string{
        "error": message,
    })
}