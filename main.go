package main

import (
	"bufio"
	"context"
	"deep-research/internal/config"
	"deep-research/internal/llm"
	"deep-research/internal/workflows"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/instructor-ai/instructor-go/pkg/instructor"
	"github.com/sashabaranov/go-openai"
)

const (
	// userPromptColor applies blue color to user prompts in the terminal
	userPromptColor = "\u001b[94mYou\u001b[0m: "

	// gptResponseColor applies yellow color to AI responses in the terminal
	// Uses %s placeholder for dynamic content insertion
	gptResponseColor = "\u001b[93mGPT\u001b[0m: %s\n"

	// welcomeMessage displays when the application starts
	welcomeMessage = "Chat with GPT (use 'ctrl-c' to quit)"
)

type ChatSessionState struct {
	conversation            []openai.ChatCompletionMessage
	researchConversation    []openai.ChatCompletionMessage
	researchBrief           string
	compressedResearchNotes []string
}

type WorkflowManager struct {
	clarifyWithUser          workflows.ChatSessionWorkflow
	researchBriefGeneration  workflows.ChatSessionWorkflow
	webResearch              workflows.ChatSessionWorkflow
	researchReportGeneration workflows.ChatSessionWorkflow
}

type ChatSession struct {
	client                 *openai.Client
	structuredOutputClient *instructor.InstructorOpenAI
	logger                 *slog.Logger
	ctx                    context.Context
	cancel                 context.CancelFunc
	state                  *ChatSessionState
	workflows              *WorkflowManager
	getUserMessage         func() (string, bool)
}

func (cs *ChatSession) addUserMessage(message string) error {
	if len(message) == 0 {
		return fmt.Errorf("user message cannot be empty")
	}

	cs.state.conversation = append(cs.state.conversation, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: message,
	})

	cs.logger.Debug("User message added to conversation",
		"message_length", len(message),
		"conversation_length", len(cs.state.conversation))
	return nil
}

func (cs *ChatSession) processUserInput() bool {
	fmt.Print(userPromptColor)
	userInput, ok := cs.getUserMessage()
	if !ok {
		cs.logger.Debug("User input terminated")
		return false
	}

	if err := cs.addUserMessage(userInput); err != nil {
		cs.logger.Error("Failed to add user message", "error", err)
		fmt.Printf("Error processing your messages: %v\nPlease try again.\n", err)
		// Continue conversation despite error
		return true
	}

	return true
}

func (cs *ChatSession) Run() error {
	fmt.Println(welcomeMessage)
	cs.logger.Debug("Starting chat session")

	for {
		if !cs.processUserInput() {
			cs.logger.Debug("User terminated input, ending session")
			break
		}

		// Workflow 1: Clarify with research scope with user
		resp, cont, err := cs.workflows.clarifyWithUser.Execute(cs.ctx)
		if err != nil {
			cs.logger.Error("Failed to execute clarify with user workflow", "error", err)
			fmt.Printf("Error generating response: %v. Please try again.\n", err)
			break
		}
		fmt.Printf(gptResponseColor, resp)
		if !cont {
			break
		}
	}

	// Workflow 2: Generate research brief based on scoping interactions
	resp, _, err := cs.workflows.researchBriefGeneration.Execute(cs.ctx)
	if err != nil {
		cs.logger.Error("Failed to execute research brief generation workflow", "error", err)
		return fmt.Errorf("error generating response: %v", err)
	}
	cs.state.researchBrief = resp.(string)
	cs.state.researchConversation = append(cs.state.researchConversation, openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: resp.(string),
	})

	// Workflow 3: Generate web search for information
	for {
		_, cont, err := cs.workflows.webResearch.Execute(cs.ctx)
		if err != nil {
			cs.logger.Error("Failed to execute web research workflow", "error", err)
			break
		}
		if !cont {
			break
		}
	}

	// Workflow 4: Write research report based on all available information
	resp, _, err = cs.workflows.researchReportGeneration.Execute(cs.ctx)
	if err != nil {
		cs.logger.Error("Failed to execute research report generation workflow", "error", err)
		return fmt.Errorf("error generating response: %v", err)
	}
	fmt.Printf(gptResponseColor, resp)

	cs.logger.Debug("Chat session ended")
	return nil
}

func NewChatSession(ctx context.Context) (*ChatSession, error) {
	cfg, err := config.LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load configuration: %w", err)
	}

	client, structuredOutputClient, err := llm.InitializeClients(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize LLM client: %w", err)
	}

	sessionCtx, cancel := context.WithCancel(ctx)

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level:     slog.LevelInfo,
		AddSource: true,
	}))

	scanner := bufio.NewScanner(os.Stdin)
	getUserMessage := func() (string, bool) {
		// Check if context is cancelled before waiting for input
		select {
		case <-sessionCtx.Done():
			return "", false
		default:
		}

		// Use a channel to make scanner.Scan() interruptible
		inputChan := make(chan bool, 1)
		go func() {
			inputChan <- scanner.Scan()
		}()

		select {
		case <-sessionCtx.Done():
			return "", false
		case success := <-inputChan:
			if !success {
				return "", false
			}
			return scanner.Text(), true
		}
	}

	state := &ChatSessionState{
		conversation:            make([]openai.ChatCompletionMessage, 0),
		researchConversation:    make([]openai.ChatCompletionMessage, 0),
		researchBrief:           "",
		compressedResearchNotes: make([]string, 0),
	}

	session := &ChatSession{
		client:                 client,
		structuredOutputClient: structuredOutputClient,
		logger:                 logger,
		ctx:                    sessionCtx,
		cancel:                 cancel,
		state:                  state,
		workflows: &WorkflowManager{
			clarifyWithUser:          workflows.NewClarifyWithUser(&state.conversation, structuredOutputClient, logger),
			researchBriefGeneration:  workflows.NewResearchBriefGeneration(&state.conversation, structuredOutputClient, logger),
			webResearch:              workflows.NewWebResearch(&state.researchConversation, &state.compressedResearchNotes, client, structuredOutputClient, logger),
			researchReportGeneration: workflows.NewResearchReportGeneration(&state.researchBrief, &state.compressedResearchNotes, structuredOutputClient, logger),
		},
		getUserMessage: getUserMessage,
	}

	logger.Debug("ChatSession created successfully")

	return session, nil
}

func (cs *ChatSession) Close() error {
	cs.logger.Debug("Shutting down ChatSession")

	if cs.cancel != nil {
		cs.cancel()
	}

	cs.logger.Debug("ChatSession shutdown completed")
	return nil
}

func setupGracefulShutdown() (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancel(context.Background())

	sigChan := make(chan os.Signal, 1)

	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigChan
		fmt.Printf("\nReceived signal %v, shutting down gracefully...\n", sig)

		cancel()
	}()

	return ctx, cancel
}

func main() {
	ctx, cleanup := setupGracefulShutdown()
	defer cleanup()

	chatSession, err := NewChatSession(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to initialize application: %v", err)
		os.Exit(1)
	}

	defer func() {
		if closeErr := chatSession.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Error during shutdown: %v\n", closeErr)
		}
	}()

	if err := chatSession.Run(); err != nil {
		chatSession.logger.Error("Application error", "error", err)
		fmt.Fprintf(os.Stderr, "Application error: %v\n", err)
		os.Exit(1)
	}

	chatSession.logger.Debug("Application completed successfully")
}
