package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	messages, err := NewRunner(BashModel{}).Run(ctx, "挨拶を出力してください", ".", scriptedModel)
	for _, message := range messages {
		fmt.Printf("[%s]\n%s\n\n", message.Role, message.Content)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func scriptedModel(messages []Message) (string, error) {
	if len(messages) == 1 {
		return "printf 'hello\\n'", nil
	}
	return "printf 'COMPLETE_TASK_AND_SUBMIT_FINAL_OUTPUT\\n完了しました\\n'", nil
}
