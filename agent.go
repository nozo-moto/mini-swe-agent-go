package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Message struct {
	Role    string
	Content string
}

const completionMarker = "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT"

func NewRunner(model Model) *Runner {
	return &Runner{
		model: model,
	}
}

type Runner struct {
	model Model
}

func (r *Runner) Run(ctx context.Context, task, dir string, model func([]Message) (string, error)) ([]Message, error) {
	messages := []Message{{Role: "user", Content: task}}
	for step := 0; step < 10; step++ {
		command, err := model(messages)
		if err != nil {
			return messages, err
		}
		messages = append(messages, Message{Role: "assistant", Content: command})

		output, code, err := r.model.Execute(ctx, dir, command)
		if err != nil {
			return messages, err
		}

		observation := fmt.Sprintf("exit code: %d\n%s", code, output)
		messages = append(messages, Message{Role: "tool", Content: observation})

		first, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
		if code == 0 && strings.TrimSpace(first) == completionMarker {
			return messages, nil
		}
	}
	return messages, errors.New("10ステップに達しました")
}
