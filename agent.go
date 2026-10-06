package main

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

type Message struct {
	Role       string
	Content    string
	Action     *Action
	ToolCallID string
}

type Action struct {
	ID      string
	Command string
}

type Model interface {
	Query(context.Context, []Message) (Message, error)
}

const completionMarker = "COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT"

func NewRunner(model Model, env Environment) *Runner {
	return &Runner{
		model:    model,
		env:      env,
		MaxSteps: 10,
	}
}

type Runner struct {
	model    Model
	env      Environment
	MaxSteps int
}

func (r *Runner) Run(ctx context.Context, task, dir string) ([]Message, error) {
	if r.MaxSteps <= 0 {
		return nil, errors.New("MaxSteps は1以上にしてください")
	}
	messages := []Message{{Role: "user", Content: task}}
	for step := 0; step < r.MaxSteps; step++ {
		if err := ctx.Err(); err != nil {
			return messages, err
		}
		message, err := r.model.Query(ctx, messages)
		if err != nil {
			return messages, err
		}
		messages = append(messages, message)
		if message.Action == nil {
			return messages, nil
		}

		output, code, err := r.env.Execute(ctx, dir, message.Action.Command)
		if err != nil {
			return messages, err
		}

		observation := fmt.Sprintf("exit code: %d\n%s", code, output)
		messages = append(messages, Message{Role: "tool", Content: observation, ToolCallID: message.Action.ID})

		first, _, _ := strings.Cut(strings.TrimSpace(output), "\n")
		if code == 0 && strings.TrimSpace(first) == completionMarker {
			return messages, nil
		}
	}
	return messages, fmt.Errorf("%dステップに達しました", r.MaxSteps)
}
