package handler

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "strings"
    "sync"
)

var (
    images = make(map[string][]byte)
    mu     sync.RWMutex
)

type UploadResponse struct {
    ViewURL string `json:"viewUrl"`
}

func generateToken() string {
    b := make([]byte, 16)
    rand.Read(b)
    return fmt.Sprintf("%x", b)
}

func Handler(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/api/upload":
        if r.Method != "POST" {
            http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
            return
        }

        // Читаем данные формы
        err := r.ParseMultipartForm(10 << 20) // 10MB
        if err != nil {
            // Пробуем как обычную форму
            r.ParseForm()
        }

        var imgData string
        
        // Пробуем получить файл
        if file, _, err := r.FormFile("image"); err == nil {
            defer file.Close()
            data, _ := io.ReadAll(file)
            imgData = base64.StdEncoding.EncodeToString(data)
        } else {
            // Пробуем получить base64
            imgData = r.FormValue("image")
        }

        if imgData == "" {
            http.Error(w, "No image data", http.StatusBadRequest)
            return
        }

        // Убираем data:image/png;base64, если есть
        if strings.Contains(imgData, "base64,") {
            imgData = strings.Split(imgData, "base64,")[1]
        }

        data, err := base64.StdEncoding.DecodeString(imgData)
        if err != nil {
            http.Error(w, "Invalid image data", http.StatusBadRequest)
            return
        }

        token := generateToken()
        
        mu.Lock()
        images[token] = data
        mu.Unlock()

        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(UploadResponse{
            ViewURL: "/api/view/" + token,
        })

    case "/api/view":
        token := r.URL.Query().Get("token")
        if token == "" {
            // Пробуем из пути
            pathParts := strings.Split(r.URL.Path, "/")
            if len(pathParts) >= 4 {
                token = pathParts[3]
            }
        }
        
        if token == "" {
            http.Error(w, "Token required", http.StatusBadRequest)
            return
        }
        
        mu.RLock()
        data, exists := images[token]
        mu.RUnlock()
        
        if !exists {
            http.Error(w, "Not found", http.StatusNotFound)
            return
        }
        
        w.Header().Set("Content-Type", "image/png")
        w.Write(data)

    default:
        http.NotFound(w, r)
    }
}