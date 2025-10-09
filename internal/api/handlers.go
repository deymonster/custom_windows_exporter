package api

import (
	"net/http"

	"node_exporter_custom/internal/secrets"
	"node_exporter_custom/metrics"
	"node_exporter_custom/registryutil"
)

type UUIDHandler struct{}

func NewRouter(secretsMgr *secrets.Manager) http.Handler {
	mux := http.NewServeMux()
	uuidHandler := &UUIDHandler{}
	mux.Handle("/api/update-uuid", AuthMiddleware(secretsMgr, http.HandlerFunc(uuidHandler.UpdateUUID)))
	return mux
}

func (h *UUIDHandler) UpdateUUID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	newUUID, err := metrics.GenerateHardwareUUID()
	if err != nil {
		http.Error(w, "Generation failed", http.StatusInternalServerError)
		return
	}

	if err := registryutil.WriteUUIDToRegistry(newUUID); err != nil {
		http.Error(w, "Registry update failed", http.StatusInternalServerError)
		return
	}

	if err := metrics.RefreshUUIDMetrics(); err != nil {
		http.Error(w, "Failed to refresh metrics", http.StatusInternalServerError)
		return
	}

	if _, err := w.Write([]byte("UUID updated")); err != nil {
		return
	}
}
