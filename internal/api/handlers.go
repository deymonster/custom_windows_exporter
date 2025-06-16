package api

import (
	"net/http"
	"node_exporter_custom/metrics"
	"node_exporter_custom/registryutil"
)

type UUIDHandler struct{}

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

	metrics.HardwareUUIDChanged.Set(0)
	w.Write([]byte("UUID updated"))
}

func (h *UUIDHandler) Restart(w http.ResponseWriter, r *http.Request) {
	// Реализация перезапуска
	w.Write([]byte("Restart initiated"))
}
