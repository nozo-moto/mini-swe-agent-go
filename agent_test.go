package main

import (
	"bytes"
	"context"
	"fmt"
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
		return Message{Role: "assistant", Content: "失敗を確認しました"}, nil
	})
	messages, err := NewRunner(model, BashEnvironment{}).Run(context.Background(), "task", t.TempDir())
	if err != nil || calls != 2 || len(messages) != 4 {
		t.Fatalf("calls=%d messages=%d err=%v", calls, len(messages), err)
	}
}

func TestCommandOutputDoesNotCompleteRun(t *testing.T) {
	for _, command := range []string{
		"printf 'COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\\n'; exit 1",
		"printf 'COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\\n'",
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

func TestRunMultipleStepsThenFinalAnswer(t *testing.T) {
	calls := 0
	var trace bytes.Buffer
	model := modelFunc(func(_ context.Context, messages []Message) (Message, error) {
		calls++
		if !strings.Contains(trace.String(), fmt.Sprintf("[step %d/3]", calls)) {
			t.Fatal("step was not displayed before querying the model")
		}
		switch calls {
		case 1:
			return bashReply("printf hello > greeting.txt"), nil
		case 2:
			if len(messages) != 3 || messages[2].Content != "exit code: 0\n" {
				t.Fatalf("missing first observation: %+v", messages)
			}
			return bashReply("cat greeting.txt"), nil
		case 3:
			if len(messages) != 5 || messages[4].Content != "exit code: 0\nhello" || !strings.Contains(trace.String(), "exit code: 0\nhello") {
				t.Fatalf("missing second observation: %+v", messages)
			}
			return Message{Role: "assistant", Content: "helloを確認しました"}, nil
		default:
			t.Fatal("model queried after final answer")
			return Message{}, nil
		}
	})
	runner := NewRunner(model, BashEnvironment{})
	runner.MaxSteps, runner.Trace = 3, &trace
	messages, err := runner.Run(context.Background(), "create and read", t.TempDir())
	if err != nil || calls != 3 || len(messages) != 6 || !strings.Contains(trace.String(), "[final]\nhelloを確認しました") {
		t.Fatalf("calls=%d messages=%+v trace=%q err=%v", calls, messages, trace.String(), err)
	}
}

func TestRunRejectsEmptyFinalAnswer(t *testing.T) {
	model := modelFunc(func(context.Context, []Message) (Message, error) {
		return Message{Role: "assistant", Content: " "}, nil
	})
	_, err := NewRunner(model, BashEnvironment{}).Run(context.Background(), "task", t.TempDir())
	if err == nil {
		t.Fatal("empty response was treated as completion")
	}
}
