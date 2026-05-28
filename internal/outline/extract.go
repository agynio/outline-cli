package outline

import "fmt"

func ResponseData(response map[string]any) any {
	if data, ok := response["data"]; ok {
		return data
	}
	return response
}

func DocumentText(response map[string]any) (string, error) {
	data, ok := ResponseData(response).(map[string]any)
	if !ok {
		return "", fmt.Errorf("document data missing from response")
	}
	text, ok := data["text"].(string)
	if !ok {
		return "", fmt.Errorf("document text missing from response")
	}
	return text, nil
}
