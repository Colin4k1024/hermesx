package eino

import (
	"bytes"
	"context"
	"encoding/gob"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"github.com/Colin4k1024/hermesx/internal/llm"
)

type turnLoopItem struct {
	UserMessage string
}

type turnLoopRequest struct {
	item            turnLoopItem
	history         []llm.Message
	callbacks       *StreamCallbacks
	enableStreaming bool
	options         []Option
}

type turnLoopCheckpointEnvelope struct {
	RunnerCheckpoint []byte
	HasRunnerState   bool
	UnhandledItems   []turnLoopItem
	CanceledItems    []turnLoopItem
}

type turnLoopExecutionState struct {
	request    turnLoopRequest
	runtime    *EinoAgent
	safeWriter *safeStreamWriter
	result     *ConversationResult
}

type checkpointDeleter interface {
	Delete(ctx context.Context, checkPointID string) error
}

// Give the active turn a short grace window to checkpoint at a safe point
// before escalating request cancellation into an immediate stop.
const turnLoopRequestCancelTimeout = 2 * time.Second

// RunConversationTurnLoopSafe executes a single conversation request through
// Eino ADK TurnLoop so interrupted turns can resume on the next request.
func RunConversationTurnLoopSafe(ctx context.Context, userMessage string, history []llm.Message, callbacks *StreamCallbacks, opts ...Option) (*ConversationResult, error) {
	cfg := &agentConfig{
		maxIterations: 20,
		platform:      "api",
	}
	for _, opt := range opts {
		opt(cfg)
	}
	if cfg.transport == nil {
		return nil, fmt.Errorf("eino agent: transport is required")
	}
	ctx = contextWithAgentConfig(ctx, cfg)

	request := turnLoopRequest{
		item:            turnLoopItem{UserMessage: userMessage},
		history:         history,
		callbacks:       callbacks,
		enableStreaming: callbacks != nil,
		options:         opts,
	}

	if err := prepareTurnLoopCheckpoint(ctx, cfg, request.item); err != nil {
		return nil, err
	}

	state := &turnLoopExecutionState{request: request}
	loop := adk.NewTurnLoop(adk.TurnLoopConfig[turnLoopItem, *schema.Message]{
		GenInput: func(ctx context.Context, _ *adk.TurnLoop[turnLoopItem, *schema.Message], items []turnLoopItem) (*adk.GenInputResult[turnLoopItem, *schema.Message], error) {
			if len(items) == 0 {
				return nil, errors.New("eino turn loop: missing input item")
			}
			return &adk.GenInputResult[turnLoopItem, *schema.Message]{
				Input: &adk.TypedAgentInput[*schema.Message]{
					Messages:        buildTurnLoopMessages(state.request.history, state.request.item.UserMessage),
					EnableStreaming: state.request.enableStreaming,
				},
				Consumed:  []turnLoopItem{items[0]},
				Remaining: items[1:],
			}, nil
		},
		GenResume: func(ctx context.Context, _ *adk.TurnLoop[turnLoopItem, *schema.Message], interruptedItems, unhandledItems, newItems []turnLoopItem) (*adk.GenResumeResult[turnLoopItem, *schema.Message], error) {
			remaining := append([]turnLoopItem{}, unhandledItems...)
			if len(newItems) > 0 {
				if len(interruptedItems) > 0 && turnLoopItemsEquivalent(interruptedItems[0], newItems[0]) {
					remaining = append(remaining, newItems[1:]...)
				} else {
					remaining = append(remaining, newItems...)
				}
			}
			return &adk.GenResumeResult[turnLoopItem, *schema.Message]{
				Consumed:  append([]turnLoopItem{}, interruptedItems...),
				Remaining: remaining,
			}, nil
		},
		PrepareAgent: func(ctx context.Context, _ *adk.TurnLoop[turnLoopItem, *schema.Message], _ []turnLoopItem) (adk.TypedAgent[*schema.Message], error) {
			runtime, err := NewEinoAgent(ctx, state.request.options...)
			if err != nil {
				return nil, err
			}
			runtime.SetCallbacks(state.request.callbacks)
			if err := runtime.checkInput(ctx, state.request.item.UserMessage); err != nil {
				return nil, err
			}
			runtime.capture.Reset()
			state.runtime = runtime
			state.wrapStreamingCallbacks()
			if runtime.callbacks != nil {
				if runtime.callbacks.OnStatus != nil {
					runtime.callbacks.OnStatus("starting agent")
				}
				if runtime.callbacks.OnStep != nil {
					runtime.callbacks.OnStep(0, nil)
				}
			}
			return runtime.agent, nil
		},
		OnAgentEvents: func(ctx context.Context, tc *adk.TurnContext[turnLoopItem, *schema.Message], events *adk.AsyncIterator[*adk.TypedAgentEvent[*schema.Message]]) error {
			if state.runtime == nil {
				return errors.New("eino turn loop: runtime not prepared")
			}

			result := &ConversationResult{
				Model:    state.runtime.config.modelName,
				Provider: state.runtime.config.provider,
				BaseURL:  state.runtime.config.baseURL,
			}
			var finalMessages []*schema.Message
			var finalContent strings.Builder
			var finalReasoning strings.Builder
			var apiCalls int

			for {
				event, ok := events.Next()
				if !ok {
					break
				}
				if event == nil {
					continue
				}
				if event.Err != nil {
					var cancelErr *adk.CancelError
					if errors.As(event.Err, &cancelErr) {
						result.Interrupted = true
					}
					if state.runtime.callbacks != nil && state.runtime.callbacks.OnError != nil {
						state.runtime.callbacks.OnError(event.Err)
					}
					state.result = result
					return event.Err
				}
				if event.Output == nil || event.Output.MessageOutput == nil {
					continue
				}
				msg, err := state.runtime.consumeMessageVariant(event.Output.MessageOutput)
				if err != nil {
					state.result = result
					return err
				}
				if msg == nil {
					continue
				}
				finalMessages = append(finalMessages, msg)
				blocks := AgenticBlocksFromMessage(msg)
				state.runtime.capture.AddBlocks(blocks)
				state.runtime.emitBlocks(blocks)
				if msg.Role == schema.Assistant {
					apiCalls++
					if len(msg.ToolCalls) == 0 {
						finalContent.WriteString(msg.Content)
						finalReasoning.WriteString(msg.ReasoningContent)
					}
					addUsage(result, msg.ResponseMeta)
				}
			}

			result.FinalResponse = finalContent.String()
			result.LastReasoning = finalReasoning.String()
			result.Messages = SchemaMessagesToLLM(finalMessages)
			result.APICalls = apiCalls
			result.Completed = !result.Interrupted
			result.TotalTokens = result.InputTokens + result.OutputTokens
			result.AgenticBlocks = state.runtime.capture.Blocks()

			if state.safeWriter != nil {
				_ = state.safeWriter.Flush()
			}
			checked, err := state.runtime.checkOutput(ctx, result)
			if err != nil {
				state.result = result
				return err
			}
			state.result = checked
			tc.Loop.Stop(adk.WithSkipCheckpoint())
			return nil
		},
		Store:        cfg.checkpointStore,
		CheckpointID: turnLoopCheckpointID(cfg),
	})

	if ok, _ := loop.Push(request.item); !ok {
		return nil, errors.New("eino turn loop: failed to enqueue request")
	}

	runCtx, stopRun := context.WithCancel(context.WithoutCancel(ctx))
	defer stopRun()

	stopWatcherDone := make(chan struct{})
	defer close(stopWatcherDone)
	go func() {
		select {
		case <-ctx.Done():
			loop.Stop(
				adk.WithGracefulTimeout(turnLoopRequestCancelTimeout),
				adk.WithStopCause("request_canceled"),
			)
		case <-stopWatcherDone:
			return
		}
	}()

	go loop.Run(runCtx)
	exit := loop.Wait()
	if exit.CheckpointErr != nil {
		return state.result, fmt.Errorf("eino turn loop: checkpoint save failed: %w", exit.CheckpointErr)
	}
	if exit.ExitReason != nil {
		return state.result, exit.ExitReason
	}
	if state.result == nil {
		return nil, errors.New("eino turn loop: no result produced")
	}
	return state.result, nil
}

func (s *turnLoopExecutionState) wrapStreamingCallbacks() {
	if s.runtime == nil || s.runtime.callbacks == nil || s.runtime.callbacks.OnStreamDelta == nil {
		return
	}
	redactionHook := NewRedactionHook(s.runtime.config.leakScanner)
	originalCallbacks := s.runtime.callbacks
	cbCopy := *originalCallbacks
	s.safeWriter = newSafeStreamWriter(redactionHook, originalCallbacks)
	cbCopy.OnStreamDelta = func(text string) {
		s.safeWriter.Write(text)
	}
	s.runtime.callbacks = &cbCopy
}

func buildTurnLoopMessages(history []llm.Message, userMessage string) []*schema.Message {
	input := make([]*schema.Message, 0, len(history)+1)
	for _, msg := range history {
		input = append(input, convertLLMToSchema(&msg))
	}
	input = append(input, schema.UserMessage(userMessage))
	return input
}

func turnLoopCheckpointID(cfg *agentConfig) string {
	if cfg == nil || cfg.tenantID == "" || cfg.sessionID == "" {
		return ""
	}
	return cfg.tenantID + "/" + cfg.sessionID
}

func prepareTurnLoopCheckpoint(ctx context.Context, cfg *agentConfig, currentItem turnLoopItem) error {
	if cfg == nil || cfg.checkpointStore == nil {
		return nil
	}
	checkpointID := turnLoopCheckpointID(cfg)
	if checkpointID == "" {
		return nil
	}

	data, exists, err := cfg.checkpointStore.Get(ctx, checkpointID)
	if err != nil || !exists || len(data) == 0 {
		return err
	}

	envelope, err := decodeTurnLoopCheckpoint(data)
	if err != nil {
		slog.Debug("eino_turn_loop_reset_legacy_checkpoint", "checkpoint_id", checkpointID, "error", err)
		return deleteTurnLoopCheckpoint(ctx, cfg.checkpointStore, checkpointID)
	}

	if envelope.HasRunnerState && len(envelope.CanceledItems) > 0 && !turnLoopItemsEquivalent(envelope.CanceledItems[0], currentItem) {
		slog.Debug("eino_turn_loop_preempt_checkpoint", "checkpoint_id", checkpointID)
		return deleteTurnLoopCheckpoint(ctx, cfg.checkpointStore, checkpointID)
	}
	if !envelope.HasRunnerState && len(envelope.UnhandledItems) > 0 && !turnLoopItemsEquivalent(envelope.UnhandledItems[0], currentItem) {
		slog.Debug("eino_turn_loop_drop_stale_buffer", "checkpoint_id", checkpointID)
		return deleteTurnLoopCheckpoint(ctx, cfg.checkpointStore, checkpointID)
	}
	return nil
}

func decodeTurnLoopCheckpoint(data []byte) (*turnLoopCheckpointEnvelope, error) {
	var envelope turnLoopCheckpointEnvelope
	if err := gob.NewDecoder(bytes.NewReader(data)).Decode(&envelope); err != nil {
		return nil, err
	}
	return &envelope, nil
}

func deleteTurnLoopCheckpoint(ctx context.Context, store adk.CheckPointStore, checkpointID string) error {
	if deleter, ok := store.(checkpointDeleter); ok {
		return deleter.Delete(ctx, checkpointID)
	}
	return fmt.Errorf("eino turn loop: checkpoint store cannot delete stale checkpoint %q", checkpointID)
}

func turnLoopItemsEquivalent(left, right turnLoopItem) bool {
	return left.UserMessage == right.UserMessage
}
