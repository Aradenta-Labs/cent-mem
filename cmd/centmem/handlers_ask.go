package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/aradenta-labs/cent-mem/internal/agent"
	"github.com/aradenta-labs/cent-mem/internal/cli"
	"github.com/aradenta-labs/cent-mem/internal/config"
	"github.com/aradenta-labs/cent-mem/internal/scope"
	"github.com/aradenta-labs/cent-mem/internal/store"
)

// cmdAsk handles conversational Q&A synthesized from stored memories.
// Syntax:
//   centmem ask "<question>" [--scope <scope>] [--top N] [--interactive]
func cmdAsk(args []string) int {
	fs := newFlagSet("ask")
	fs.String("scope", "", "scope path filter")
	fs.Int("top", 5, "number of citations/recall results")
	fs.Bool("interactive", false, "run multi-turn interactive terminal inquiry")

	return runCommand(reorderFlags(args, fs), fs, func(cfg config.Config, fs *flag.FlagSet) error {
		interactive := fs.Lookup("interactive").Value.String() == "true"
		question := strings.TrimSpace(strings.Join(fs.Args(), " "))

		if question == "" && !interactive {
			return cli.Invalidf("ask: question is required (or run with --interactive)")
		}

		scopePath := fs.Lookup("scope").Value.String()
		if scopePath != "" {
			if _, err := scope.Parse(scopePath); err != nil {
				return cli.Invalidf("ask: invalid --scope %q: %v", scopePath, err)
			}
		}

		s, err := store.Open(cfg)
		if err != nil {
			return cli.Internalf("ask: %v", err)
		}
		defer s.Close()

		engine := initAgentEngine(cfg, s)
		top := intFlag(fs, "top", 5)
		if top <= 0 {
			top = 5
		} else if top > 20 {
			top = 20
		}

		if interactive {
			return runInteractiveAsk(os.Stdin, os.Stdout, engine, scopePath, top, question)
		}

		ctx, cancel := signalContext()
		defer cancel()

		res, err := engine.Ask(ctx, question, agent.InquiryOptions{
			Scope: scopePath,
			Top:   top,
		})
		if err != nil {
			return cli.Internalf("ask: %v", err)
		}

		citations := res.Citations
		if citations == nil {
			citations = []store.Citation{}
		}
		gaps := res.KnowledgeGaps
		if gaps == nil {
			gaps = []string{}
		}

		out := map[string]any{
			"ok":              true,
			"answer":          res.Answer,
			"citations":       citations,
			"knowledge_gaps":  gaps,
			"reasoning_steps": res.ReasoningSteps,
			"fallback_used":   res.FallbackUsed,
		}
		if res.ConversationID != "" {
			out["conversation_id"] = res.ConversationID
		}

		return prettyPrint(fs, out)
	})
}

func runInteractiveAsk(in io.Reader, out io.Writer, engine *agent.Engine, scopePath string, top int, initialQuestion string) error {
	ctx := context.Background()
	var convID string

	if initialQuestion != "" {
		fmt.Fprintf(out, "User: %s\n", initialQuestion)
		res, err := engine.Ask(ctx, initialQuestion, agent.InquiryOptions{
			Scope: scopePath,
			Top:   top,
		})
		if err != nil {
			return fmt.Errorf("ask: %w", err)
		}
		convID = res.ConversationID
		fmt.Fprintf(out, "Assistant:\n%s\n\n", res.Answer)
		if len(res.Citations) > 0 {
			fmt.Fprintln(out, "Citations:")
			for _, c := range res.Citations {
				fmt.Fprintf(out, "  [%d] (%s) %s\n", c.ID, c.Scope, c.Snippet)
			}
			fmt.Fprintln(out)
		}
	}

	scanner := bufio.NewScanner(in)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 1024*1024)

	for {
		fmt.Fprint(out, "centmem> ")
		if !scanner.Scan() {
			break
		}
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if line == "exit" || line == "quit" || line == "/q" {
			break
		}
		if line == "help" || line == "/help" {
			fmt.Fprintln(out, "Interactive Commands:")
			fmt.Fprintln(out, "  exit, quit, /q : Exit inquiry session")
			fmt.Fprintln(out, "  clear, /clear  : Reset conversation context thread")
			fmt.Fprintln(out, "  help, /help    : Show this help")
			fmt.Fprintln(out)
			continue
		}
		if line == "clear" || line == "/clear" {
			convID = ""
			fmt.Fprintln(out, "Cleared conversation context. Started new thread.")
			continue
		}

		res, err := engine.Ask(ctx, line, agent.InquiryOptions{
			Scope:          scopePath,
			Top:            top,
			ConversationID: convID,
		})
		if err != nil {
			fmt.Fprintf(out, "Error: %v\n", err)
			continue
		}
		convID = res.ConversationID
		fmt.Fprintf(out, "\nAssistant:\n%s\n\n", res.Answer)
		if len(res.Citations) > 0 {
			fmt.Fprintln(out, "Citations:")
			for _, c := range res.Citations {
				fmt.Fprintf(out, "  [%d] (%s) %s\n", c.ID, c.Scope, c.Snippet)
			}
			fmt.Fprintln(out)
		}
	}
	return scanner.Err()
}
