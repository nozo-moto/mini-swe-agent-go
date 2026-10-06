package main

import (
	"context"
	"errors"
	"fmt"
	"io"
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
	Trace    io.Writer
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
		if r.Trace != nil {
			fmt.Fprintf(r.Trace, "\n[step %d/%d]\n", step+1, r.MaxSteps)
		}
		message, err := r.model.Query(ctx, messages)
		if err != nil {
			return messages, err
		}
		messages = append(messages, message)
		if message.Action == nil {
			if strings.TrimSpace(message.Content) == "" {
				return messages, errors.New("最終回答が空です")
			}
			if r.Trace != nil {
				fmt.Fprintf(r.Trace, "[final]\n%s\n", message.Content)
			}
			return messages, nil
		}

		if r.Trace != nil {
			fmt.Fprintf(r.Trace, "$ %s\n", message.Action.Command)
		}
		output, code, err := r.env.Execute(ctx, dir, message.Action.Command)
		if err != nil {
			return messages, err
		}

		observation := fmt.Sprintf("exit code: %d\n%s", code, output)
		messages = append(messages, Message{Role: "tool", Content: observation, ToolCallID: message.Action.ID})
		if r.Trace != nil {
			fmt.Fprintf(r.Trace, "[tool]\n%s\n", observation)
		}
	}
	return messages, fmt.Errorf("%dステップに達しました", r.MaxSteps)
}
