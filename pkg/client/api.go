package client

import (
	"bytes"
	"encoding/json"
	"net/http"
)

// AxeClient handles communication with the Axe agent service.
type AxeClient struct {
	BaseURL string
	APIKey  string
}

func (c *AxeClient) ExecuteAgent(agentID string, input map[string]interface{}) (map[string]interface{}, error) {
	url := c.BaseURL + "/v1/agents/" + agentID + "/execute"
	data, _ := json.Marshal(input)
	
	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer " + c.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&result)
	return result, nil
}
