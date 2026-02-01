package handler

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
)

// Базовый URL для загрузки файлов на Uploadcare
const uploadcareAPI = "https://upload.uploadcare.com/base/"

// Структура ответа Uploadcare после загрузки
type UploadcareResponse struct {
	File string `json:"file"` // UUID файла на Uploadcare
}

// Структура ответа нашего API
type ServerResponse struct {
	Success bool   `json:"success"`
	ID      string `json:"id"`
	URL     string `json:"url"`
}

// Основной хэндлер для Vercel / Netlify / Lambda
func Handler(w http.ResponseWriter, r *http.Request) {
	// --- CORS заголовки ---
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

	// --- Простой preflight для OPTIONS ---
	if r.Method == "OPTIONS" {
		w.WriteHeader(http.StatusOK)
		return
	}

	// --- Проверяем метод ---
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// --- Получаем публичный ключ из окружения ---
	publicKey := os.Getenv("UPLOADCARE_PUBLIC_KEY")
	if publicKey == "" {
		sendJSONError(w, "Uploadcare configuration error", http.StatusInternalServerError)
		return
	}

	// --- Парсим multipart форму (для файла) ---
	err := r.ParseMultipartForm(10 << 20) // макс. 10 МБ
	if err != nil {
		sendJSONError(w, "Failed to parse form", http.StatusBadRequest)
		return
	}

	// --- Получаем файл ---
	file, _, err := r.FormFile("image")
	if err != nil {
		sendJSONError(w, "No image file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// --- Читаем файл в память ---
	imageData, err := io.ReadAll(file)
	if err != nil {
		sendJSONError(w, "Failed to read image", http.StatusInternalServerError)
		return
	}

	// --- Отправляем файл на Uploadcare ---
	fileURL, fileID, err := uploadToUploadcare(imageData, publicKey)
	if err != nil {
		sendJSONError(w, "Failed to upload to storage: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// --- Формируем ответ API ---
	response := ServerResponse{
		Success: true,
		ID:      fileID,
		URL:     fileURL,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// Функция загрузки на Uploadcare
func uploadToUploadcare(imageBytes []byte, publicKey string) (string, string, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// --- Добавляем файл ---
	part, err := writer.CreateFormFile("file", "valentine.png")
	if err != nil {
		return "", "", err
	}
	_, err = io.Copy(part, bytes.NewReader(imageBytes))
	if err != nil {
		return "", "", err
	}

	// --- Добавляем ключи и параметры ---
	writer.WriteField("UPLOADCARE_PUB_KEY", publicKey)
	writer.WriteField("UPLOADCARE_STORE", "1") // сохраняем файл навсегда
	// Убрали "UPLOADCARE_EXPIRE", чтобы файлы не удалялись автоматически

	writer.Close()

	// --- Формируем POST запрос ---
	req, err := http.NewRequest("POST", uploadcareAPI, body)
	if err != nil {
		return "", "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	// --- Отправляем запрос ---
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()

	// --- Читаем ответ ---
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", "", err
	}

	var uploadResp UploadcareResponse
	err = json.Unmarshal(respBody, &uploadResp)
	if err != nil {
		return "", "", err
	}

	// --- Формируем прямой URL через поддомен ---
	fileID := uploadResp.File
	fileURL := fmt.Sprintf("https://1kqur3jhqh.ucarecd.net/%s/valentine.png", fileID)

	return fileURL, fileID, nil
}

// Отправка JSON ошибки
func sendJSONError(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{
		"error": message,
	})
}
