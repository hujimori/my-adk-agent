package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/loopagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

type ExitLoopArgs struct{}
type ExitLoopResults struct{}

// Q.ExitLoopArgsってなに？
// Q.ExitLoopResultsって何？
func ExitLoop(ctx tool.Context, input ExitLoopArgs) (ExitLoopResults, error) {
	fmt.Printf("[Tool Call] exitLoop triggerd by %s \n", ctx.AgentName())
	ctx.Actions().Escalate = true
	return ExitLoopResults{}, nil
}

const GEMINI_2_5_FLASH_LITE = "gemini-2.5-flash-lite"
const GEMINI_3_1_FLASH_LITE = "gemini-3.1-flash-lite"

func main() {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	ctx := context.Background()

	if err := runAgent(ctx, "Write a document about a cat"); err != nil {
		log.Fatalf("Agent execution failed: %v", err)
	}
}

func runAgent(ctx context.Context, promt string) error {
	model, err := gemini.NewModel(ctx, GEMINI_2_5_FLASH_LITE, &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})
	if err != nil {
		return fmt.Errorf("failed to create model: %v", err)
	}

	// STEP 1: Initial Writer Agent (Runs ONCE at the beginning)
	initialWriterAgent, err := llmagent.New(llmagent.Config{
		Name:        "InitialWriterAgent",
		Model:       model,
		Description: "Writes the initial document draft based on the topic.",
		Instruction: `You are a Creative Writing Assistant tasked with starting a story.
Write the *first draft* of a short story (aim for 2-4 sentences).
Base the content *only* on the topic provided in the user's prompt.
Output *only* the story/document text. Do not add introductions or explanations.`,
		OutputKey: "stateDoc",
	})
	if err != nil {
		return fmt.Errorf("failed to create initial writer agent: %v", err)
	}

	// state keys used in prompt templates ({stateDoc}, {stateCrit})
	stateDoc := "stateDoc"
	stateCrit := "stateCrit"
	donePhrase := "finish review"
	// STEP 2a: Critic Agent (Inside the Refinement Loop)
	criticAgentInLoop, err := llmagent.New(llmagent.Config{
		Name:        "CriticAgent",
		Model:       model,
		Description: "Reviews the current draft, providing critique or signaling completion.",
		Instruction: fmt.Sprintf(`You are a Constructive Critic AI reviewing a short document draft.
**Document to Review:**
"""
{%s}
"""
**Task:**
Review the document.
IF you identify 1-2 *clear and actionable* ways it could be improved:
Provide these specific suggestions concisely. Output *only* the critique text.
ELSE IF the document is coherent and addresses the topic adequately:
Respond *exactly* with the phrase "%s" and nothing else.`, stateDoc, donePhrase),
		OutputKey: "stateCrit",
	})
	if err != nil {
		return fmt.Errorf("failed to create critic agent: %v", err)
	}

	exitLoopTool, err := functiontool.New(
		functiontool.Config{
			Name:        "exitLoop",
			Description: "Call this function ONLY when the critique indicates no further changes are needed.",
		},
		ExitLoop,
	)
	if err != nil {
		return fmt.Errorf("failed to create exit loop tool: %v", err)
	}
	// STEP 2b: Refiner/Exiter Agent (Inside the Refinement Loop)
	refinerAgentInLoop, err := llmagent.New(llmagent.Config{
		Name:  "RefinerAgent",
		Model: model,
		Instruction: fmt.Sprintf(`You are a Creative Writing Assistant refining a document based on feedback OR exiting the process.
**Current Document:**

"""
{%s}
"""

**Critique/Suggestions:**
{%s}
**Task:**
Analyze the 'Critique/Suggestions'.
IF the critique is *exactly* "%s":
You MUST call the 'exitLoop' function. Do not output any text.
ELSE (the critique contains actionable feedback):
Carefully apply the suggestions to improve the 'Current Document'. Output *only* the refined document text.`, stateDoc, stateCrit, donePhrase),
		Description: "Refines the document based on critique, or calls exitLoop if critique indicates completion.",
		Tools:       []tool.Tool{exitLoopTool},
		OutputKey:   "stateDoc",
	})

	// STEP 2: Refinement Loop Agent
	refinementLoop, err := loopagent.New(loopagent.Config{
		AgentConfig: agent.Config{
			Name:      "RefinementLoop",
			SubAgents: []agent.Agent{criticAgentInLoop, refinerAgentInLoop},
		},
		// Q.ここをインタラクティブに制御するのはどうしたらいいか？
		MaxIterations: 2,
	})
	if err != nil {
		return fmt.Errorf("failed to create loop agent: %v", err)
	}

	// STEP 3: Overall Sequential Pipeline
	pipeline, err := sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:      "Loop Agent",
			SubAgents: []agent.Agent{initialWriterAgent, refinementLoop},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create sequential agent pipeline: %v", err)
	}

	// STEP 4: Launch the agent
	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(pipeline),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		return fmt.Errorf("run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}

	return nil
}
