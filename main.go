package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

// Структура для ответа
type UploadResponse struct {
	ViewURL string `json:"viewUrl"`
}

// Генерация случайного имени файла
func generateFilename() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

// Обработчик загрузки картинки
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Получаем картинку из base64
	r.ParseForm()
	imgData := r.FormValue("image")
	if imgData == "" {
		http.Error(w, "No image data", http.StatusBadRequest)
		return
	}

	// Проверяем и убираем data:image/png;base64, если есть
	if strings.Contains(imgData, "base64,") {
		imgData = strings.Split(imgData, "base64,")[1]
	}

	// Декодируем base64
	data, err := base64.StdEncoding.DecodeString(imgData)
	if err != nil {
		http.Error(w, "Invalid image data: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Создаем директории если нет
	os.MkdirAll("uploads", 0755)
	os.MkdirAll("tokens", 0755)

	// Сохраняем файл
	filename := generateFilename() + ".png"
	filepathStr := filepath.Join("uploads", filename)
	
	err = os.WriteFile(filepathStr, data, 0644)
	if err != nil {
		http.Error(w, "Failed to save image: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Генерируем токен
	token := generateFilename()
	
	// Сохраняем информацию о файле
	tokenPath := filepath.Join("tokens", token)
	err = os.WriteFile(tokenPath, []byte(filename), 0644)
	if err != nil {
		http.Error(w, "Failed to save token", http.StatusInternalServerError)
		return
	}
	
	// Возвращаем JSON ответ
	response := UploadResponse{
		ViewURL: "/view/" + token,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Обработчик просмотра картинки
func viewHandler(w http.ResponseWriter, r *http.Request) {
	// Извлекаем токен из URL
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	token := pathParts[2]
	
	// Читаем имя файла по токену
	filenameBytes, err := os.ReadFile(filepath.Join("tokens", token))
	if err != nil {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	
	filename := string(filenameBytes)
	filepathStr := filepath.Join("uploads", filename)
	
	// Проверяем существование файла
	if _, err := os.Stat(filepathStr); os.IsNotExist(err) {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}
	
	// Отдаем картинку с правильными заголовками
	w.Header().Set("Content-Type", "image/png")
	http.ServeFile(w, r, filepathStr)
}

// Обработчик для viewer.html
func viewerHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "viewer.html")
}

// Файловый сервер для статики
func fileServerHandler(dir string) http.Handler {
	return http.StripPrefix("/static/", http.FileServer(http.Dir(dir)))
}

func main() {
	// Создаем директории если нет
	os.MkdirAll("uploads", 0755)
	os.MkdirAll("tokens", 0755)
	
	// Регистрируем обработчики
	http.HandleFunc("/upload", uploadHandler)
	http.HandleFunc("/view/", viewHandler)
	http.HandleFunc("/viewer", viewerHandler)
	http.Handle("/static/", fileServerHandler("."))
	
	// Главный роут
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			http.ServeFile(w, r, "index.html")
		} else {
			// Пробуем отдать статический файл
			http.ServeFile(w, r, r.URL.Path[1:])
		}
	})

	port := 8080
	log.Printf("Сервер запущен: http://localhost:%d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}