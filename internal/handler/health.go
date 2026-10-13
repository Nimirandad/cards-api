package handler

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"time"
)

type healthResponse struct {
	Status     string `json:"status"`
	InstanceID string `json:"instance_id"`
}

func Health(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(healthResponse{
		Status:     "ok",
		InstanceID: instanceID(),
	})
}

func instanceID() string {
	metadataURI := os.Getenv("ECS_CONTAINER_METADATA_URI_V4")
	if metadataURI == "" {
		return ""
	}

	client := http.Client{Timeout: time.Second}
	response, err := client.Get(strings.TrimRight(metadataURI, "/") + "/task")
	if err != nil {
		return ""
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return ""
	}

	var metadata struct {
		TaskARN string `json:"TaskARN"`
	}
	if json.NewDecoder(response.Body).Decode(&metadata) != nil {
		return ""
	}

	parts := strings.Split(metadata.TaskARN, "/")
	return parts[len(parts)-1]
}
