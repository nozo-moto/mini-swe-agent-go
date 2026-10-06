package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func main() {
	task := flag.String("task", "挨拶を出力してください", "エージェントへの依頼")
	dir := flag.String("dir", ".", "bashの作業ディレクトリ")
	maxSteps := flag.Int("max-steps", 10, "LLMへの問い合わせ回数の上限（1以上）")
	flag.Parse()
	if err := godotenv.Load(); err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintln(os.Stderr, ".env を読み込めません。ファイルの形式と権限を確認してください。")
		os.Exit(1)
	}
	if strings.TrimSpace(os.Getenv("OPENAI_API_KEY")) == "" || strings.TrimSpace(os.Getenv("OPENAI_MODEL")) == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY と OPENAI_MODEL を設定してください。互換APIには OPENAI_BASE_URL も指定します。")
		os.Exit(1)
	}
	if flag.NArg() != 0 || strings.TrimSpace(*task) == "" {
		fmt.Fprintln(os.Stderr, "依頼は -task に指定してください。空の依頼は使えません。")
		os.Exit(1)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	model := OpenAIModel{
		Client: openai.NewClient(option.WithMaxRetries(0), option.WithRequestTimeout(2*time.Minute)),
		Name:   os.Getenv("OPENAI_MODEL"),
		Instructions: "Use bash to work on the user's task. Call at most one tool per response. " +
			"Each command starts in a fresh shell; files persist but cd and environment changes do not. " +
			"When finished, explain the result without calling a tool.",
	}
	runner := NewRunner(model, BashEnvironment{})
	runner.MaxSteps = *maxSteps
	messages, err := runner.Run(ctx, *task, *dir)
	for _, message := range messages {
		fmt.Printf("[%s]\n%s\n\n", message.Role, message.Content)
		if message.Action != nil {
			fmt.Printf("$ %s\n\n", message.Action.Command)
		}
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
