package main

import (
	"context"
	"fmt"
	"log"
	"os"

	fieldservice "fieldservice-agent-telemetry"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

type openAIChat struct{ client *openai.Client }

func (c openAIChat) Complete(ctx context.Context, prompt string) (string, error) {
	completion, err := c.client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Model: "auto",
		Messages: []openai.ChatCompletionMessageParamUnion{
			openai.UserMessage(prompt),
		},
	})
	if err != nil {
		return "", err
	}
	return completion.Choices[0].Message.Content, nil
}

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	// One key and one base URL cover the model call and the telemetry calls.
	client := openai.NewClient(option.WithAPIKey(apiKey), option.WithBaseURL(fieldservice.BaseURL))
	telemetry := fieldservice.NewTelemetryClient(fieldservice.BaseURL, apiKey)
	decision, err := fieldservice.RunDispatchLoop(context.Background(), fieldservice.WorkOrder{
		ID: "WO-1042", DispatchStatus: "awaiting_photo_review", PhotoNotes: "Outdoor disconnect cover is cracked.",
	}, openAIChat{client: &client}, telemetry)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("work order=%s action=%s tokens=%d\n", decision.WorkOrderID, decision.Action, decision.TokenCount)
}
