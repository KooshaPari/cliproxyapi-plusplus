package management

import (
	"encoding/json"
	"fmt"

	"github.com/fxamacker/cbor/v2"
)

// encodeJSONStringToCBOR converts a JSON string payload into CBOR bytes.
func encodeJSONStringToCBOR(jsonString string) ([]byte, error) {
	var payload any
	if errUnmarshal := json.Unmarshal([]byte(jsonString), &payload); errUnmarshal != nil {
		return nil, errUnmarshal
	}
	return cbor.Marshal(payload)
}

// decodeCBORBodyToTextOrJSON decodes CBOR bytes to plain text (for string payloads) or JSON string.
func decodeCBORBodyToTextOrJSON(raw []byte) (string, error) {
	if len(raw) == 0 {
		return "", nil
	}

	var payload any
	if errUnmarshal := cbor.Unmarshal(raw, &payload); errUnmarshal != nil {
		return "", errUnmarshal
	}

	jsonCompatible := cborValueToJSONCompatible(payload)
	switch typed := jsonCompatible.(type) {
	case string:
		return typed, nil
	case []byte:
		return string(typed), nil
	default:
		jsonBytes, errMarshal := json.Marshal(jsonCompatible)
		if errMarshal != nil {
			return "", errMarshal
		}
		return string(jsonBytes), nil
	}
}

// cborValueToJSONCompatible recursively converts CBOR-decoded values into JSON-marshalable values.
func cborValueToJSONCompatible(value any) any {
	switch typed := value.(type) {
	case map[any]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[fmt.Sprint(key)] = cborValueToJSONCompatible(item)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			out[key] = cborValueToJSONCompatible(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = cborValueToJSONCompatible(item)
		}
		return out
	default:
		return typed
	}
}
