package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

type Response struct {
	Status string `json:"status"`
	Report string `json:"report"`
}

type CityArgs struct {
	City string `json:"city"`
}

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	ctx := context.Background()

	model, err := gemini.NewModel(ctx, "gemini-2.5-flash-lite", &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})

	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	weatherTool, err := functiontool.New(functiontool.Config{
		Name:        "getWeather",
		Description: "get weather",
	}, getWeather)
	if err != nil {
		log.Fatalf("Failed to create weather tool: %v", err)
	}

	currentTimeTool, err := functiontool.New(functiontool.Config{
		Name:        "getCurrentTime",
		Description: "get current time",
	}, getCurrentTime)
	if err != nil {
		log.Fatalf("Failed to create currentTime tool: %v", err)
	}

	// Define the agent.
	a, err := llmagent.New(llmagent.Config{
		Name:        "multi_tool_agent",
		Model:       model,
		Description: "An agent that can answer questions about weather and current time.",
		Instruction: `You are a helpful assistant that answers questions about weather and current time.

When the user asks about weather or time:
1. Call the appropriate tool (getWeather or getCurrentTime) with the city name.
2. The tool returns a JSON object with "status" and "report" fields.
3. If status is "success", answer the user in natural language using the content of the "report" field.
4. If the tool returns an error, apologize and explain the issue to the user.

Always present the tool's report content to the user — do not stop after just calling the tool.`,
		Tools: []tool.Tool{
			weatherTool, currentTimeTool,
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

func getWeather(ctx tool.Context, args CityArgs) (*Response, error) {

	if strings.ToLower(args.City) == "new york" {
		return &Response{
			Status: "success",
			Report: "The weather in New York is sunny with a temperature of 25 degrees Celsius (77 degrees Fahrenheit).",
		}, nil
	}

	return nil, fmt.Errorf("Weather information for %s is not available.", args.City)
}

func getCurrentTime(ctx tool.Context, args CityArgs) (*Response, error) {

	var tzIdentifier string

	if strings.ToLower(args.City) == "new york" {
		tzIdentifier = "America/New_York"
	} else {
		return nil, fmt.Errorf("Sorry, I don't have timezone information for %s.", args.City)
	}

	loc, _ := time.LoadLocation(tzIdentifier)
	now := time.Now().In(loc)

	return &Response{
		Status: "success",
		Report: fmt.Sprintf("The current time in %s is %s", args.City, now.Format("2006-01-02 15:04:05")),
	}, nil
}
