package alerts

import (
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"strings"

	domain "marketmind/api/internal/domain/alerts"
	service "marketmind/api/internal/service/alerts"
	"marketmind/api/internal/utils/httpresponse"
	"marketmind/api/pkg/alerts"
	"marketmind/utils/priceformat"
)

type alertResponse struct {
	ID             int64            `json:"alert_id,string"`
	Type           alerts.Kind      `json:"type"`
	Value          string           `json:"value"`
	Direction      alerts.Direction `json:"direction"`
	ReferencePrice string           `json:"reference_price"`
	TargetPrice    string           `json:"target_price"`
	IsActive       bool             `json:"is_active"`
	QuoteAsset     string           `json:"quote_asset"`
}

func toResponse(result service.Result) alertResponse {
	return alertResponse{
		ID: result.ID, Type: result.Type, Value: result.Value,
		Direction:      result.Direction,
		ReferencePrice: priceformat.TwoDecimals(result.ReferencePrice),
		TargetPrice:    priceformat.TwoDecimals(result.TargetPrice),
		IsActive:       result.IsActive, QuoteAsset: result.QuoteAsset,
	}
}

func readChange(w http.ResponseWriter, r *http.Request, allowActive bool) (service.Patch, bool) {
	mediaType, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" || (params["charset"] != "" && !strings.EqualFold(params["charset"], "utf-8")) {
		httpresponse.WriteError(w, http.StatusUnsupportedMediaType, "UNSUPPORTED_MEDIA_TYPE", "Expected application/json")
		return service.Patch{}, false
	}

	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8*1024))
	change, ok := decodeChange(decoder, allowActive)
	if !ok {
		httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "Invalid alert body")
		return service.Patch{}, false
	}
	if change.Condition != nil {
		if err := change.Condition.Validate(); err != nil {
			httpresponse.WriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
			return service.Patch{}, false
		}
	}
	return change, true
}

func decodeChange(decoder *json.Decoder, allowActive bool) (service.Patch, bool) {
	var fields map[string]json.RawMessage
	if err := decoder.Decode(&fields); err != nil || len(fields) == 0 || decoder.Decode(new(json.RawMessage)) != io.EOF {
		return service.Patch{}, false
	}

	var kind *alerts.Kind
	var value *string
	var direction *alerts.Direction
	var active *bool
	for key, raw := range fields {
		switch key {
		case "type":
			if err := json.Unmarshal(raw, &kind); err != nil || kind == nil {
				return service.Patch{}, false
			}
		case "value":
			if err := json.Unmarshal(raw, &value); err != nil || value == nil {
				return service.Patch{}, false
			}
		case "direction":
			if err := json.Unmarshal(raw, &direction); err != nil || direction == nil {
				return service.Patch{}, false
			}
		case "is_active":
			if err := json.Unmarshal(raw, &active); !allowActive || err != nil || active == nil {
				return service.Patch{}, false
			}
		default:
			return service.Patch{}, false
		}
	}
	change := service.Patch{IsActive: active}
	if kind == nil && value == nil && direction == nil && active != nil {
		return change, true
	}
	if kind == nil || value == nil || direction == nil {
		return service.Patch{}, false
	}
	change.Condition = &domain.Condition{Type: *kind, Value: *value, Direction: *direction}
	return change, true
}
