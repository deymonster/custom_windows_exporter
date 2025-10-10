package api

import (
	"encoding/base64"
	"net/http"
	"strings"

	"node_exporter_custom/internal/secrets"
	"node_exporter_custom/metrics"
	"node_exporter_custom/registryutil"
)

func AuthMiddleware(secretsMgr *secrets.Manager, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

		currentUUID, err := GetCurrentUUID()
		if err != nil {
			http.Error(w, "System error", http.StatusInternalServerError)
			return
		}

		if pair[0] != currentUUID {
			http.Error(w, "Invalid UUID credentials", http.StatusForbidden)
			return
		}

		if secretsMgr == nil || !secretsMgr.ValidatePassword(pair[1]) {
			http.Error(w, "Invalid credentials", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func GetCurrentUUID() (string, error) {
	uuid, err := registryutil.ReadUUIDFromRegistry()
	if err == nil {
		return uuid, nil
	}

	newUUID, err := metrics.GenerateHardwareUUID()
	if err != nil {
		return "", err
	}

	return newUUID, nil
}
