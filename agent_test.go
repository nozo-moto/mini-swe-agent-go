package main

import (
	"context"
	"strings"
	"testing"
)

func TestRunPassesFailedCommandToModel(t *testing.T) {
	calls := 0
	model := func(messages []Message) (string, error) {
		calls++
		if calls == 1 {
			return "printf 'failure'; exit 2", nil
		}
		if len(messages) != 3 || messages[1].Role != "assistant" || messages[2].Role != "tool" || messages[2].Content != "exit code: 2\nfailure" {
			t.Fatalf("実行結果が次のモデル呼び出しに渡っていない: %+v", messages)
		}
		return "printf 'COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\\ndone\\n'", nil
	}
	messages, err := run(context.Background(), "task", t.TempDir(), model)
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
			model := func([]Message) (string, error) { return command, nil }
			messages, err := run(context.Background(), "task", t.TempDir(), model)
			if err == nil || !strings.Contains(err.Error(), "10ステップ") || len(messages) != 21 {
				t.Fatalf("messages=%d err=%v", len(messages), err)
			}
		})
	}
}
