/*
 * @Author: Vincent Yang
 * @Date: 2024-03-18 01:12:14
 * @LastEditors: Vincent Yang
 * @LastEditTime: 2025-01-22 15:41:43
 * @FilePath: /claude2openai/main.go
 * @Telegram: https://t.me/missuo
 * @GitHub: https://github.com/missuo
 *
 * Copyright © 2024 by Vincent, All Rights Reserved.
 */

package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// Global debug flag
var debugMode bool

// debugLog prints a message only if debug mode is enabled
func debugLog(format string, args ...interface{}) {
	if debugMode {
		fmt.Printf(format+"\n", args...)
	}
}

func processMessages(openAIReq OpenAIRequest) ([]ClaudeMessage, *string) {
	var newMessages []ClaudeMessage
	var systemPrompt *string

	for i := 0; i < len(openAIReq.Messages); i++ {
		message := openAIReq.Messages[i]

		if message.Role == "system" {
			// Extract system message as string
			var systemContent string
			switch c := message.Content.(type) {
			case string:
				systemContent = c
			case []interface{}:
				// For array content, concatenate all text parts
				for _, part := range c {
					if contentMap, ok := part.(map[string]interface{}); ok {
						if contentMap["type"] == "text" {
							systemContent += contentMap["text"].(string)
						}
					}
				}
			}
			systemPrompt = &systemContent
		} else {
			// Process regular messages
			claudeMessage := ClaudeMessage{
				Role:    message.Role,
				Content: []ClaudeContent{},
			}

			switch content := message.Content.(type) {
			case string:
				// Simple string content
				claudeMessage.Content = append(claudeMessage.Content, ClaudeContent{
					Type: "text",
					Text: content,
				})
			case []interface{}:
				// Process array of content blocks
				for _, part := range content {
					if contentMap, ok := part.(map[string]interface{}); ok {
						contentType, _ := contentMap["type"].(string)

						if contentType == "text" {
							text, _ := contentMap["text"].(string)
							claudeMessage.Content = append(claudeMessage.Content, ClaudeContent{
								Type: "text",
								Text: text,
							})
						} else if contentType == "image_url" {
							// Handle image URLs
							if imageURL, ok := contentMap["image_url"].(map[string]interface{}); ok {
								url, _ := imageURL["url"].(string)

								// For base64 images
								if strings.HasPrefix(url, "data:image/") {
									// Extract data portion from data URL
									parts := strings.Split(url, ",")
									if len(parts) >= 2 {
										mediaType := strings.TrimSuffix(strings.TrimPrefix(parts[0], "data:"), ";base64")
										source := &ClaudeSource{
											Type:      "base64",
											MediaType: mediaType,
											Data:      parts[1],
										}
										imageContent := ClaudeContent{
											Type:   "image",
											Source: source,
										}
										claudeMessage.Content = append(claudeMessage.Content, imageContent)
									}
								} else {
									// For HTTP URLs
									source := &ClaudeSource{
										Type:      "url",
										MediaType: "image/jpeg", // Default, could be refined with content-type check
										Data:      url,
									}
									imageContent := ClaudeContent{
										Type:   "image",
										Source: source,
									}
									claudeMessage.Content = append(claudeMessage.Content, imageContent)
								}
							}
						}
					}
				}
			}

			// Only add message if it has content
			if len(claudeMessage.Content) > 0 {
				newMessages = append(newMessages, claudeMessage)
			}
		}
	}

	return newMessages, systemPrompt
}

func createClaudeRequest(openAIReq OpenAIRequest, stream bool) ([]byte, error) {
	var claudeReq ClaudeAPIRequest
	claudeReq.Model = openAIReq.Model
	claudeReq.MaxTokens = 4096
	claudeReq.Messages, claudeReq.System = processMessages(openAIReq)
	claudeReq.Stream = stream
	claudeReq.Temperature = openAIReq.Temperature
	claudeReq.TopP = openAIReq.TopP

	// Debug the Claude request structure
	debugLog("Claude messages structure:")
	for i, msg := range claudeReq.Messages {
		contentJson, _ := json.Marshal(msg.Content)
		debugLog("Message %d, Role: %s, Content: %s", i, msg.Role, string(contentJson))
	}

	return json.Marshal(claudeReq)
}

func parseAuthorizationHeader(c *gin.Context) (string, error) {
	authorizationHeader := c.GetHeader("Authorization")
	if !strings.HasPrefix(authorizationHeader, "Bearer ") {
		return "", fmt.Errorf("invalid Authorization header format")
	}
	return strings.TrimPrefix(authorizationHeader, "Bearer "), nil
}

func sendClaudeRequest(claudeReqBody []byte, apiKey string) (*http.Response, error) {
	// Get endpoint from environment variable or use default
	endpoint := os.Getenv("ANTHROPIC_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.anthropic.com/v1/messages"
	} else {
		endpoint = strings.TrimSuffix(endpoint, "/")
		if strings.HasSuffix(endpoint, "/v1/messages") || strings.HasSuffix(endpoint, "/v1") {
			return nil, fmt.Errorf("invalid ANTHROPIC_ENDPOINT format: %s. The endpoint should be the base URL only (e.g., https://api.anthropic.com) without '/v1/messages'", endpoint)
		}
		endpoint = endpoint + "/v1/messages"
	}

	debugLog("Using Claude API endpoint: %s", endpoint)

	req, _ := http.NewRequest("POST", endpoint, bytes.NewBuffer(claudeReqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-version", "2023-06-01") // Consider updating to a newer version if needed

	client := &http.Client{}
	return client.Do(req)
}

func proxyToClaude(c *gin.Context, openAIReq OpenAIRequest) {
	claudeReqBody, err := createClaudeRequest(openAIReq, false)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal request for Claude API"})
		return
	}

	apiKey, err := parseAuthorizationHeader(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := sendClaudeRequest(claudeReqBody, apiKey)
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

	// Log raw response for debugging
	debugLog("Raw Claude API response: %s", string(body))

	var claudeResp ClaudeAPIResponse
	if err := json.Unmarshal(body, &claudeResp); err != nil {
		debugLog("Error unmarshaling Claude response: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse response from Claude API"})
		return
	}

	// Debug the unmarshaled response
	responseBytes, _ := json.Marshal(claudeResp)
	debugLog("Unmarshaled Claude response: %s", string(responseBytes))
	if claudeResp.Error != nil {
		c.JSON(resp.StatusCode, gin.H{"error": OpenAIError{Type: claudeResp.Error.Type, Message: claudeResp.Error.Message}})
		return
	}

	// Extract text content from Claude response
	var responseText string

	// Debug the entire response content structure
	rawContent, _ := json.Marshal(claudeResp.Content)
	debugLog("Raw content array: %s", string(rawContent))

	for i, content := range claudeResp.Content {
		// Debug each content block
		debugLog("Content block %d, type: %s, text: %s", i, content.Type, content.Text)

		if content.Type == "text" {
			responseText += content.Text
		} else if content.Type == "" && content.Text != "" {
			// Handle older API response format or unexpected structure
			responseText += content.Text
		}
	}

	// If responseText is still empty, try different approach
	if responseText == "" && len(claudeResp.Content) > 0 {
		debugLog("No text found in content blocks, trying alternative extraction")

		// Try to extract text regardless of content type
		for _, content := range claudeResp.Content {
			if content.Text != "" {
				responseText += content.Text
				debugLog("Found text in content block: %s", content.Text)
			}
		}

		// Last resort: use the first block's text field no matter what
		if responseText == "" && len(claudeResp.Content) > 0 {
			responseText = claudeResp.Content[0].Text
			debugLog("Last resort text: %s", responseText)
		}
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
					Content: responseText,
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
			PromptTokens:     claudeResp.Usage.InputTokens,
			CompletionTokens: claudeResp.Usage.OutputTokens,
			TotalTokens:      claudeResp.Usage.InputTokens + claudeResp.Usage.OutputTokens,
		},
	}
	c.JSON(http.StatusOK, openAIResp)
}

func proxyToClaudeStream(c *gin.Context, openAIReq OpenAIRequest) {
	// Debug the request structure
	reqBytes, _ := json.Marshal(openAIReq)
	debugLog("Streaming request: %s", string(reqBytes))

	claudeReqBody, err := createClaudeRequest(openAIReq, true)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to marshal request for Claude API"})
		return
	}

	// Debug the Claude request being sent
	debugLog("Claude request: %s", string(claudeReqBody))

	apiKey, err := parseAuthorizationHeader(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	resp, err := sendClaudeRequest(claudeReqBody, apiKey)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to send request to Claude API"})
		return
	}
	defer resp.Body.Close()

	reader := bufio.NewReader(resp.Body)
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Streaming not supported"})
		return
	}

	// Generate a single ID for the entire conversation
	chatID := fmt.Sprintf("chatcmpl-%s", uuid.New().String())
	timestamp := time.Now().Unix()

	var content string
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				break
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to read response from Claude API"})
			return
		}

		lineStr := strings.TrimSpace(string(line))
		// Debug each line received
		debugLog("Stream line: %s", lineStr)

		if strings.HasPrefix(lineStr, "event: message_start") {
			data := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"refusal\":null,\"role\":\"assistant\"},\"finish_reason\":null,\"index\":0,\"logprobs\":null}],\"created\":%d,\"id\":\"%s\",\"model\":\"%s\",\"object\":\"chat.completion.chunk\",\"system_fingerprint\":\"fp_f3927aa00d\"}\n\n",
				timestamp, chatID, openAIReq.Model)
			c.Writer.WriteString(data)
			flusher.Flush()
		} else if strings.HasPrefix(lineStr, "data:") {
			dataStr := strings.TrimSpace(strings.TrimPrefix(lineStr, "data:"))

			// Debug the raw data
			debugLog("Stream data chunk: %s", dataStr)

			var data map[string]interface{}
			err := json.Unmarshal([]byte(dataStr), &data)
			if err != nil {
				debugLog("Error parsing stream data: %v", err)
				continue
			}

			debugLog("Data type: %v", data["type"])

			if data["type"] == "content_block_delta" {
				deltaRaw, ok := data["delta"]
				if !ok {
					debugLog("No delta field in content_block_delta: %v", data)
					continue
				}

				delta, ok := deltaRaw.(map[string]interface{})
				if !ok {
					debugLog("Delta is not a map: %v", deltaRaw)
					continue
				}

				debugLog("Delta type: %v", delta["type"])

				if delta["type"] == "text_delta" {
					textDeltaRaw, ok := delta["text"]
					if !ok {
						debugLog("No text field in text_delta: %v", delta)
						continue
					}

					textDelta, ok := textDeltaRaw.(string)
					if !ok {
						debugLog("Text delta is not a string: %v", textDeltaRaw)
						continue
					}

					content += textDelta
					responseData := fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":\"%s\"},\"finish_reason\":null,\"index\":0,\"logprobs\":null}],\"created\":%d,\"id\":\"%s\",\"model\":\"%s\",\"object\":\"chat.completion.chunk\",\"system_fingerprint\":\"fp_f3927aa00d\"}\n\n",
						escapeJSON(textDelta), timestamp, chatID, openAIReq.Model)
					c.Writer.WriteString(responseData)
					flusher.Flush()
				}
			}
		} else if strings.HasPrefix(lineStr, "event: message_stop") {
			// Debug message stop event
			debugLog("Received message_stop event. Complete content: %s", content)

			data := fmt.Sprintf("data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\",\"index\":0,\"logprobs\":null}],\"created\":%d,\"id\":\"%s\",\"model\":\"%s\",\"object\":\"chat.completion.chunk\",\"system_fingerprint\":\"fp_f3927aa00d\"}\n\n",
				timestamp, chatID, openAIReq.Model)
			c.Writer.WriteString(data)
			flusher.Flush()
			c.Writer.WriteString("data: [DONE]\n\n")
			break
		}
	}
}

func escapeJSON(str string) string {
	b, _ := json.Marshal(str)
	return string(b[1 : len(b)-1])
}

var defaultModels = []string{
	"claude-3-5-haiku-20241022", // 默认模型
	"claude-3-5-sonnet-20241022",
	"claude-3-5-sonnet-20240620",
	"claude-3-opus-20240229",
	"claude-3-sonnet-20240229",
	"claude-3-haiku-20240307",
	"claude-2.1",
	"claude-2.0",
}

// getAllowedModels gets the list of allowed models from environment variable
// or falls back to the default list
func getAllowedModels() []string {
	allowedModelsEnv := os.Getenv("ALLOWED_MODELS")
	if allowedModelsEnv != "" {
		// Split the comma-separated string into a slice
		allowedModels := strings.Split(allowedModelsEnv, ",")
		// Trim whitespace from each model name
		for i, model := range allowedModels {
			allowedModels[i] = strings.TrimSpace(model)
		}
		return allowedModels
	}
	return defaultModels
}

func handler(c *gin.Context) {
	var openAIReq OpenAIRequest

	if err := c.BindJSON(&openAIReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Get allowed models
	allowedModels := getAllowedModels()

	// Default model is the first in the allowed models list
	if !isInSlice(openAIReq.Model, allowedModels) {
		openAIReq.Model = allowedModels[0]
	}

	// If stream is true, proxy to Claude with stream
	if openAIReq.Stream {
		proxyToClaudeStream(c, openAIReq)
	} else {
		proxyToClaude(c, openAIReq)
	}
}

func modelsHandler(c *gin.Context) {
	// Get allowed models
	allowedModels := getAllowedModels()

	openAIResp := OpenAIModelsResponse{
		Object: "list",
	}
	for _, model := range allowedModels {
		openAIResp.Data = append(openAIResp.Data, OpenAIModel{
			ID:      model,
			Object:  "model",
			OwnedBy: "user",
		})
	}
	c.JSON(http.StatusOK, openAIResp)
}

func isInSlice(str string, list []string) bool {
	for _, item := range list {
		if item == str {
			return true
		}
	}
	return false
}

func main() {
	// Define command line flags
	var port string
	var debug bool
	flag.StringVar(&port, "p", "", "specify server port")
	flag.BoolVar(&debug, "debug", false, "enable debug mode")
	flag.Parse()

	// Initialize debug mode from flag or environment variable
	if debug {
		debugMode = true
	} else {
		debugEnv := os.Getenv("DEBUG")
		if debugEnv != "" {
			if debugVal, err := strconv.ParseBool(debugEnv); err == nil {
				debugMode = debugVal
			}
		}
	}

	// If command line flag is empty, try to get port from environment variable
	if port == "" {
		port = os.Getenv("PORT")
		// If environment variable is also empty, use default port
		if port == "" {
			port = "6600"
		}
	}
	gin.SetMode(gin.ReleaseMode)
	r := gin.Default()
	r.Use(cors.Default())
	r.GET("/", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"message": "Welcome to Claude2OpenAI, Made by Vincent Yang. https://github.com/missuo/claude2openai",
		})
	})
	r.POST("/v1/chat/completions", handler)
	r.GET("/v1/models", modelsHandler)
	r.NoRoute(func(c *gin.Context) {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    http.StatusNotFound,
			"message": "Path not found",
		})
	})
	// Start Claude2OpenAI with specified port
	fmt.Printf("Claude2OpenAI is running on port %s (debug mode: %v)\n", port, debugMode)
	r.Run(fmt.Sprintf(":%s", port))
}
