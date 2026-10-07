package provider

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/F5Networks/terraform-provider-f5ads/internal/provider/clients/naas"
)

func formatAPIError(statusCode int, body []byte) string {
	status := fmt.Sprintf("status: %d", statusCode)
	if statusText := http.StatusText(statusCode); statusText != "" {
		status += " " + statusText
	}

	var apiError naas.Error
	if err := json.Unmarshal(body, &apiError); err != nil {
		return status
	}

	details := []string{status}
	if apiError.Detail != nil && *apiError.Detail != "" {
		details = append(details, "detail: "+*apiError.Detail)
	}

	return strings.Join(details, ", ")
}
