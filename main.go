package main

import (
    "crypto/rand"
    "encoding/base64"
    "encoding/json"
    "fmt"
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

func uploadHandler(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    imgData := r.FormValue("image")
    if imgData == "" {
        http.Error(w, "No image data", http.StatusBadRequest)
        return
    }

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
        ViewURL: "/view/" + token,
    })
}

func viewHandler(w http.ResponseWriter, r *http.Request) {
    pathParts := strings.Split(r.URL.Path, "/")
    if len(pathParts) < 3 {
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }
    
    token := pathParts[2]
    
    mu.RLock()
    data, exists := images[token]
    mu.RUnlock()
    
    if !exists {
        http.Error(w, "Not found", http.StatusNotFound)
        return
    }
    
    w.Header().Set("Content-Type", "image/png")
    w.Write(data)
}

func Handler(w http.ResponseWriter, r *http.Request) {
    switch r.URL.Path {
    case "/upload":
        uploadHandler(w, r)
    case "/":
        http.ServeFile(w, r, "index.html")
    default:
        if strings.HasPrefix(r.URL.Path, "/view/") {
            viewHandler(w, r)
        } else {
            http.ServeFile(w, r, r.URL.Path[1:])
        }
    }
}

func main() {
    http.HandleFunc("/", Handler)
    port := "8080"
    http.ListenAndServe(":"+port, nil)
}