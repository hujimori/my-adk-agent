package main

import (
	"context"
	"log"
	"os"

	"github.com/joho/godotenv"
	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/agent/workflowagents/sequentialagent"
	"google.golang.org/adk/cmd/launcher"
	"google.golang.org/adk/cmd/launcher/full"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/genai"
)

type Response struct {
	Status string `json:"status"`
	Report string `json:"report"`
}

type CityArgs struct {
	City string `json:"city"`
}

const GEMINI_2_5_FLASH_LITE = "gemini-2.5-flash-lite"
const GEMINI_3_1_FLASH_LITE = "gemini-3.1-flash-lite"

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	ctx := context.Background()

	model, err := gemini.NewModel(ctx, GEMINI_2_5_FLASH_LITE, &genai.ClientConfig{
		APIKey: os.Getenv("GOOGLE_API_KEY"),
	})

	if err != nil {
		log.Fatalf("Failed to create model: %v", err)
	}

	codeWriteAgent, err := llmagent.New(llmagent.Config{
		Name:        "CodeWriterAgent",
		Model:       model,
		Description: "Writes initial Go code based on a specification.",
		Instruction: `You are a Go Code Generator.

First, decide whether the user's message is:
(A) A request to generate / write / implement Go code for some functionality.
(B) A conversational or meta question (e.g., "what can you do?", "hello", "who are you?", "help").

If (A): Write Go code that fulfills the requirement, and output *only* the complete Go code block enclosed in triple backticks ('''go ... '''). Do not add any other text before or after the code block.

If (B): Reply in natural conversational language (no code blocks, no backticks). Briefly explain that you generate Go code based on user requests and invite the user to describe what they want built. Do not output any Go code in this case.`,
		OutputKey: "generated_code",
	})
	if err != nil {
		log.Fatalf("failed to create code writer agent: %v", err)
	}

	codeReviewerAgent, err := llmagent.New(llmagent.Config{
		Name:        "CordeReviewerAgent",
		Model:       model,
		Description: "Reviews code and provides feedback.",
		Instruction: `You are an expert Go Code Reviewer.
Your task is to provide constructive feedback on the provided code.

**Code to Review:**
'''go
{generated_code}
'''

**Review Criteria:**
1.  **Correctness:** Does the code work as intended? Are there logic errors?
2.  **Readability:** Is the code clear and easy to understand? Follows Go style guidelines?
3.  **Idiomatic Go:** Does the code use Go's features in a natural and standard way?
4.  **Edge Cases:** Does the code handle potential edge cases or invalid inputs gracefully?
5.  **Best Practices:** Does the code follow common Go best practices?

**Output:**
Provide your feedback as a concise, bulleted list. Focus on the most important points for improvement.
If the code is excellent and requires no changes, simply state: "No major issues found."
Output *only* the review comments or the "No major issues" statement.`,
		OutputKey: "review_comments"})

	if err != nil {
		log.Fatalf("failed to create code reviewer agent: %v", err)
	}

	codeRefactorAgent, err := llmagent.New(llmagent.Config{
		Name:        "CodeRefactorAgent",
		Model:       model,
		Description: "Refactors code based on review comments",
		Instruction: `You are a Go Code Refactoring AI.
Your goal is to improve the given Go code based on the provided review comments.

**Original Code:**
'''go
{generated_code}
'''

**Review Comments:**
{review_comments}

**Task:**
Carefully apply the suggestions from the review comments to refactor the original code.
If the review comments state "No major issues found," return the original code unchanged.
Ensure the final code is complete, functional, and includes necessary imports.

**Output:**
Output *only* the final, refactored Go code block, enclosed in triple backticks ('''go ... ''').
Do not add any other text before or after the code block.`,
		OutputKey: "refactored_code"})

	if err != nil {
		log.Fatalf("failed to create code refactorer agent: %v", err)
	}

	codePipelineAgent, err := sequentialagent.New(sequentialagent.Config{
		AgentConfig: agent.Config{
			Name:        "code pipeline agent",
			Description: "Executes a sequence of code writing, reviewing, and refactoring.",
			SubAgents: []agent.Agent{
				codeWriteAgent,
				codeReviewerAgent,
				codeRefactorAgent,
			},
		},
	})

	if err != nil {
		log.Fatalf("failed to create sequential agent: %v", err)
	}

	config := &launcher.Config{
		AgentLoader: agent.NewSingleLoader(codePipelineAgent),
	}

	l := full.NewLauncher()
	if err = l.Execute(ctx, config, os.Args[1:]); err != nil {
		log.Fatalf("Run failed: %v\n\n%s", err, l.CommandLineSyntax())
	}
}
