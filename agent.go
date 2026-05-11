package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/geminitool"
	"google.golang.org/genai"
)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-2.5-flash-lite", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_APLI_KEY"),
	})

	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	// Define the agent.
	a, err := llmagent.New(llmagent.Config{
		Name:        "multi_tool_agent",
		Model:       model,
		Description: "An agent that can awnser questions using Google Search.",
		Instruction: "You are a helpful assistant. Use the avaialable tools to anwser questions",
		Tools: []tool.Tool{
			geminitool.GoogleSearch{},
		},
	})

	if err != nil {
		log.Fatalf("Failed to Create agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(a),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
