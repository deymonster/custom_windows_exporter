package api

import (
	"encoding/base64"
	"net/http"
	"node_exporter_custom/internal/auth"
	"node_exporter_custom/metrics"
	"node_exporter_custom/registryutil"
	"strings"
)

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// 1. Проверка Basic Auth
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="Restricted"`)
			http.Error(w, "Authorization required", http.StatusUnauthorized)
			return
		}

		payload, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(authHeader, "Basic "))
		if err != nil {
			http.Error(w, "Invalid authorization header", http.StatusBadRequest)
			return
		}

		pair := strings.SplitN(string(payload), ":", 2)
		if len(pair) != 2 {
			http.Error(w, "Invalid credentials format", http.StatusBadRequest)
			return
		}

		// 2. Получаем текущий UUID из реестра (или генерируем если нет)
		currentUUID, err := GetCurrentUUID()
		if err != nil {
			http.Error(w, "System error", http.StatusInternalServerError)
			return
		}

		// 3. Проверяем логин (должен совпадать с UUID)
		if pair[0] != currentUUID {
			http.Error(w, "Invalid UUID credentials", http.StatusForbidden)
			return
		}

		// 4. Проверяем пароль
		if !auth.IsValid(pair[1]) {
			http.Error(w, "Invalid credentials", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func GetCurrentUUID() (string, error) {
	// Пытаемся прочитать из реестра
	uuid, err := registryutil.ReadUUIDFromRegistry()
	if err == nil {
		return uuid, nil
	}

	// Если нет в реестре - генерируем новый (но не сохраняем!)
	newUUID, err := metrics.GenerateHardwareUUID()
	if err != nil {
		return "", err
	}

	return newUUID, nil
}
