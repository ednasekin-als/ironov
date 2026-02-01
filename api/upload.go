package handler

import (
    "crypto/rand"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "os"
    "path/filepath"
    "strings"
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
    w.Header().Set("Access-Control-Allow-Origin", "*")
    
    if r.Method == "OPTIONS" {
        w.WriteHeader(http.StatusOK)
        return
    }
    
    // Обработка статических картинок
    if strings.HasPrefix(r.URL.Path, "/api/images/") && strings.HasSuffix(r.URL.Path, ".png") {
        // Извлекаем token из URL
        token := r.URL.Path[len("/api/images/"):]
        token = token[:len(token)-4]
        
        // Читаем файл из /tmp (единственное место на Vercel с write access)
        filePath := filepath.Join("/tmp", token+".png")
        
        data, err := os.ReadFile(filePath)
        if err != nil {
            http.NotFound(w, r)
            return
        }
        
        w.Header().Set("Content-Type", "image/png")
        w.Header().Set("Cache-Control", "public, max-age=31536000")
        w.Write(data)
        return
    }
    
    // Обработка загрузки
    if r.URL.Path == "/api/upload" {
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }
        
        // Парсим multipart форму
        err := r.ParseMultipartForm(10 << 20)
        if err != nil {
            http.Error(w, "Failed to parse form", http.StatusBadRequest)
            return
        }
        
        // Получаем файл
        file, _, err := r.FormFile("image")
        if err != nil {
            http.Error(w, "No image file", http.StatusBadRequest)
            return
        }
        defer file.Close()
        
        // Читаем файл
        imageData, err := io.ReadAll(file)
        if err != nil {
            http.Error(w, "Failed to read image", http.StatusInternalServerError)
            return
        }
        
        // Генерируем ID
        token := generateToken()
        
        // Сохраняем в файл в /tmp
        filePath := filepath.Join("/tmp", token+".png")
        err = os.WriteFile(filePath, imageData, 0644)
        if err != nil {
            http.Error(w, "Failed to save image", http.StatusInternalServerError)
            return
        }
        
        // Отправляем JSON ответ
        response := UploadResponse{
            Success: true,
            ID:      token,
            URL:     "/api/images/" + token + ".png",
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(response)
        return
    }
    
    http.NotFound(w, r)
}