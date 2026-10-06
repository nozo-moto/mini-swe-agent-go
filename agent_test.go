package main

import (
	"context"
	"strings"
	"testing"
)

// 固定応答はテストだけで使う。本番と同じModelの契約を満たす。
type modelFunc func(context.Context, []Message) (Message, error)

func (f modelFunc) Query(ctx context.Context, messages []Message) (Message, error) {
	return f(ctx, messages)
}

func bashReply(command string) Message {
	return Message{Role: "assistant", Action: &Action{ID: "test-call", Command: command}}
}

func TestRunPassesFailedCommandToModel(t *testing.T) {
	calls := 0
	model := modelFunc(func(_ context.Context, messages []Message) (Message, error) {
		calls++
		if calls == 1 {
			return bashReply("printf 'failure'; exit 2"), nil
		}
		if len(messages) != 3 || messages[1].Role != "assistant" || messages[2].Role != "tool" || messages[2].Content != "exit code: 2\nfailure" {
			t.Fatalf("実行結果が次のモデル呼び出しに渡っていない: %+v", messages)
		}
		if messages[2].ToolCallID != "test-call" {
			t.Fatal("tool result lost its call ID")
		}
		return bashReply("printf 'COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\\ndone\\n'"), nil
	})
	messages, err := NewRunner(model, BashEnvironment{}).Run(context.Background(), "task", t.TempDir())
	if err != nil || calls != 2 || len(messages) != 5 {
		t.Fatalf("calls=%d messages=%d err=%v", calls, len(messages), err)
	}
}

func TestCompletionRequiresSuccessfulFirstLine(t *testing.T) {
	for _, command := range []string{
		"printf 'COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\\n'; exit 1",
		"printf 'prefix\\nCOMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\\n'",
	} {
		t.Run(command, func(t *testing.T) {
			model := modelFunc(func(context.Context, []Message) (Message, error) { return bashReply(command), nil })
			runner := NewRunner(model, BashEnvironment{})
			runner.MaxSteps = 2
			messages, err := runner.Run(context.Background(), "task", t.TempDir())
			if err == nil || !strings.Contains(err.Error(), "2ステップ") || len(messages) != 5 {
				t.Fatalf("messages=%d err=%v", len(messages), err)
			}
		})
	}
}
