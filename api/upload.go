package handler

import (
    "crypto/rand"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "sync"
)

var (
    images = make(map[string][]byte)
    mu     sync.RWMutex
)

// Ответ для фронтенда
type UploadResponse struct {
    Success bool   `json:"success"`
    ID      string `json:"id"`
    URL     string `json:"url"`
}

func generateToken() string {
    b := make([]byte, 16)
    rand.Read(b)
    return fmt.Sprintf("%x", b)
}

func Handler(w http.ResponseWriter, r *http.Request) {
    // Устанавливаем CORS заголовки
    w.Header().Set("Access-Control-Allow-Origin", "*")
    w.Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS")
    w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
    
    if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusOK)
        return
    }
    
    // Только POST для /api/upload
    if r.URL.Path != "/api/upload" {
        http.NotFound(w, r)
        return
    }
    
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }
    
    // Парсим multipart форму
    err := r.ParseMultipartForm(10 << 20) // 10MB
    if err != nil {
        http.Error(w, "Failed to parse form: "+err.Error(), http.StatusBadRequest)
        return
    }
    
    // Получаем файл
    file, _, err := r.FormFile("image")
    if err != nil {
        http.Error(w, "No image file: "+err.Error(), http.StatusBadRequest)
        return
    }
    defer file.Close()
    
    // Читаем файл
    imageData, err := io.ReadAll(file)
    if err != nil {
        http.Error(w, "Failed to read image: "+err.Error(), http.StatusInternalServerError)
        return
    }
    
    // Генерируем ID
    token := generateToken()
    
    // Сохраняем в память
    mu.Lock()
    images[token] = imageData
    mu.Unlock()
    
    // Отправляем JSON ответ
    response := UploadResponse{
        Success: true,
        ID:      token,
        URL:     "/api/images/" + token + ".png",  // исправленный путь
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}