package handler

import (
    "net/http"
)

func ImagesHandler(w http.ResponseWriter, r *http.Request) {
    // Извлекаем token из URL: /api/images/{token}.png
    token := r.URL.Path[len("/api/images/"):]
    token = token[:len(token)-4] // убираем .png
    
    mu.RLock()
    data, exists := images[token]
    mu.RUnlock()
    
    if !exists {
        http.Error(w, "Image not found", http.StatusNotFound)
        return
    }
    
    w.Header().Set("Content-Type", "image/png")
    w.Header().Set("Cache-Control", "public, max-age=31536000")
    w.Write(data)
}