package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func testClient(url string) openai.Client {
	return openai.NewClient(option.WithBaseURL(url+"/v1/"), option.WithAPIKey("test-key"), option.WithMaxRetries(0))
}

func TestOpenAIModelRoundTrip(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request struct {
			Model    string `json:"model"`
			Messages []struct {
				Role       string          `json:"role"`
				Content    string          `json:"content"`
				ToolCallID string          `json:"tool_call_id"`
				ToolCalls  json.RawMessage `json:"tool_calls"`
			} `json:"messages"`
			Tools []struct {
				Function struct {
					Name string `json:"name"`
				} `json:"function"`
			} `json:"tools"`
		}
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			w.WriteHeader(400)
			return
		}
		if r.URL.Path != "/v1/chat/completions" || r.Header.Get("Authorization") != "Bearer test-key" || request.Model != "test-model" || len(request.Tools) != 1 || request.Tools[0].Function.Name != "bash" {
			t.Error("incorrect SDK request")
		}
		calls++
		w.Header().Set("Content-Type", "application/json")
		if calls == 1 {
			if len(request.Messages) != 2 || request.Messages[0].Content != "test instructions" || request.Messages[1].Content != "task" {
				t.Error("missing instructions or task")
			}
			fmt.Fprint(w, `{"choices":[{"finish_reason":"tool_calls","message":{"role":"assistant","content":"Let me check.","tool_calls":[{"id":"call-1","type":"function","function":{"name":"bash","arguments":"{\"command\":\"printf observed\"}"}}]}}]}`)
			return
		}
		if len(request.Messages) != 4 {
			t.Error("history was not preserved")
		} else {
			assistant, observation := request.Messages[2], request.Messages[3]
			var toolCalls []openai.ChatCompletionMessageToolCall
			if json.Unmarshal(assistant.ToolCalls, &toolCalls) != nil || len(toolCalls) != 1 || toolCalls[0].ID != "call-1" || toolCalls[0].Function.Name != "bash" || toolCalls[0].Function.Arguments != `{"command":"printf observed"}` || assistant.Content != "Let me check." {
				t.Error("assistant tool call was not preserved")
			}
			if observation.Role != "tool" || observation.ToolCallID != "call-1" || observation.Content != "exit code: 0\nobserved" {
				t.Error("shell observation was not passed back to the model")
			}
		}
		fmt.Fprint(w, `{"choices":[{"finish_reason":"stop","message":{"role":"assistant","content":"done"}}]}`)
	}))
	defer server.Close()
	model := OpenAIModel{Client: testClient(server.URL), Name: "test-model", Instructions: "test instructions"}
	messages, err := NewRunner(model, BashEnvironment{}).Run(context.Background(), "task", t.TempDir())
	if err != nil || calls != 2 || len(messages) != 4 || messages[3].Content != "done" {
		t.Fatalf("calls=%d messages=%+v err=%v", calls, messages, err)
	}
}

func TestOpenAIModelRejectsInvalidResponses(t *testing.T) {
	for _, body := range []string{
		`{"choices":[]}`,
		`{"choices":[{"finish_reason":"length","message":{"content":"partial"}}]}`,
		`{"choices":[{"finish_reason":"stop","message":{"content":""}}]}`,
		`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"a","type":"function","function":{"name":"unknown","arguments":"{}"}}]}}]}`,
		`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"a","type":"function","function":{"name":"bash","arguments":"bad json"}}]}}]}`,
		`{"choices":[{"finish_reason":"tool_calls","message":{"tool_calls":[{"id":"a"},{"id":"b"}]}}]}`,
	} {
		t.Run(body, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				fmt.Fprint(w, body)
			}))
			defer server.Close()
			model := OpenAIModel{Client: testClient(server.URL), Name: "test-model"}
			if _, err := model.Query(context.Background(), nil); err == nil {
				t.Fatal("expected response error")
			}
		})
	}
}

func TestOpenAIModelAPIFailureAndCancellation(t *testing.T) {
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"error":{"message":"invalid key"}}`)
	}))
	defer server.Close()
	model := OpenAIModel{Client: testClient(server.URL), Name: "test-model"}
	if _, err := model.Query(context.Background(), nil); err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected API error, got %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := model.Query(ctx, nil); err == nil {
		t.Fatal("expected cancellation")
	}
	if calls != 1 {
		t.Fatalf("unexpected retries or request after cancellation: %d", calls)
	}
}
