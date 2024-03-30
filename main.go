package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func proxyToClaude(c *gin.Context) {
	var openAIReq OpenAIRequest

	if err := c.BindJSON(&openAIReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	fmt.Println(openAIReq)
	var newMessages []struct {
		Role    string `json:"role"`
		Content string `json:"content"`
	}
	for i := 0; i < len(openAIReq.Messages); i++ {
		if openAIReq.Messages[i].Role == "system" && i+1 < len(openAIReq.Messages) {
			openAIReq.Messages[i+1].Content = openAIReq.Messages[i].Content + " " + openAIReq.Messages[i+1].Content
		} else if openAIReq.Messages[i].Role != "system" {
			newMessages = append(newMessages, openAIReq.Messages[i])
		}
	}

	openAIReq.Messages = newMessages

	claudeReqBody, err := json.Marshal(map[string]interface{}{
		"model":      openAIReq.Model,
		"max_tokens": 4096,
		"messages":   openAIReq.Messages,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal request for Claude API"})
		return
	}
	authorizationHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authorizationHeader, "Bearer ") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid Authorization header format"})
		return
	}

	apiKey := strings.TrimPrefix(authorizationHeader, "Bearer ")
	req, _ := http.NewRequest("POST", "https://api.anthropic.com/v1/messages", bytes.NewBuffer(claudeReqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to call Claude API"})
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from Claude API"})
		return
	}

	var claudeResp ClaudeAPIResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response from Claude API"})
		return
	}

	openAIResp := OpenAIResponse{
		ID:      claudeResp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   claudeResp.Model,
		Choices: []struct {
			Index   int `json:"index"`
			Message struct {
				Role    string `json:"role"`
				Content string `json:"content"`
			} `json:"message"`
			Logprobs     interface{} `json:"logprobs"`
			FinishReason string      `json:"finish_reason"`
		}{
			{
				Index: 0,
				Message: struct {
					Role    string `json:"role"`
					Content string `json:"content"`
				}{
					Role:    "assistant",
					Content: claudeResp.Content[0].Text,
				},
				Logprobs:     nil,
				FinishReason: "stop",
			},
		},
		Usage: struct {
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
			TotalTokens      int `json:"total_tokens"`
		}{
			PromptTokens:     len(openAIReq.Messages[0].Content),
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}
	c.JSON(http.StatusOK, openAIResp)
}

func main() {
	router := gin.Default()
	router.POST("/v1/completions", proxyToClaude)
	router.Run(":8080")
}
