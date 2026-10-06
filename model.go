package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/openai/openai-go"
)

type OpenAIModel struct {
	Client       openai.Client
	Name         string
	Instructions string
}

func (m OpenAIModel) Query(ctx context.Context, history []Message) (Message, error) {
	messages := []openai.ChatCompletionMessageParamUnion{}
	if m.Instructions != "" {
		messages = append(messages, openai.SystemMessage(m.Instructions))
	}
	for _, message := range history {
		switch message.Role {
		case "user":
			messages = append(messages, openai.UserMessage(message.Content))
		case "assistant":
			assistant := openai.AssistantMessage(message.Content)
			if message.Action != nil {
				args, _ := json.Marshal(map[string]string{"command": message.Action.Command})
				assistant.OfAssistant.ToolCalls = []openai.ChatCompletionMessageToolCallParam{{
					ID:       message.Action.ID,
					Function: openai.ChatCompletionMessageToolCallFunctionParam{Name: "bash", Arguments: string(args)},
				}}
			}
			messages = append(messages, assistant)
		case "tool":
			messages = append(messages, openai.ToolMessage(message.Content, message.ToolCallID))
		default:
			return Message{}, fmt.Errorf("unknown message role: %s", message.Role)
		}
	}
	response, err := m.Client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: m.Name, Messages: messages, ParallelToolCalls: openai.Bool(false),
		Tools: []openai.ChatCompletionToolParam{{Function: openai.FunctionDefinitionParam{
			Name: "bash", Description: openai.String("Run a bash command and return its output and exit code."),
			Parameters: openai.FunctionParameters{
				"type": "object", "required": []string{"command"},
				"properties":           map[string]any{"command": map[string]any{"type": "string"}},
				"additionalProperties": false,
			},
		}}},
	})
	if err != nil {
		return Message{}, fmt.Errorf("query model: %w", err)
	}
	if len(response.Choices) == 0 {
		return Message{}, errors.New("model returned no choices")
	}
	choice := response.Choices[0]
	if choice.FinishReason != "stop" && choice.FinishReason != "tool_calls" {
		return Message{}, fmt.Errorf("model response did not finish normally: %s", choice.FinishReason)
	}
	reply := choice.Message
	if reply.Refusal != "" {
		return Message{}, fmt.Errorf("model refused: %s", reply.Refusal)
	}
	message := Message{Role: "assistant", Content: reply.Content}
	if len(reply.ToolCalls) == 0 {
		if strings.TrimSpace(reply.Content) == "" {
			return Message{}, errors.New("model returned no text or tool call")
		}
		return message, nil
	}
	if len(reply.ToolCalls) != 1 {
		return Message{}, errors.New("expected one bash call; no commands executed")
	}
	call := reply.ToolCalls[0]
	var args struct {
		Command string `json:"command"`
	}
	if call.Type != "function" || call.Function.Name != "bash" || strings.TrimSpace(call.ID) == "" ||
		json.Unmarshal([]byte(call.Function.Arguments), &args) != nil || strings.TrimSpace(args.Command) == "" {
		return Message{}, errors.New("invalid bash call; no command executed")
	}
	message.Action = &Action{ID: call.ID, Command: args.Command}
	return message, nil
}
