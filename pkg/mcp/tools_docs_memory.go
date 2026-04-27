package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/Cobliteam/workflow-toolkit/pkg/docstore"
	"github.com/Cobliteam/workflow-toolkit/pkg/memory"
)

func registerDocMemoryTools(s *server.MCPServer, workflowHome string) {
	s.AddTool(toolDocGet(), handleDocGet(workflowHome))
	s.AddTool(toolDocList(), handleDocList(workflowHome))
	s.AddTool(toolDocSearch(), handleDocSearch(workflowHome))
	s.AddTool(toolDocAdd(), handleDocAdd(workflowHome))
	s.AddTool(toolMemoryGet(), handleMemoryGet(workflowHome))
	s.AddTool(toolMemoryList(), handleMemoryList(workflowHome))
	s.AddTool(toolMemorySet(), handleMemorySet(workflowHome))
}

// --- doc_get ---

func toolDocGet() mcplib.Tool {
	return mcplib.Tool{
		Name:        "doc_get",
		Description: "Get a workflow document (discovery, savepoint, runbook, 1on1, postmortem, incident) by ID. Returns full content.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"id": prop("string", "Document ID (e.g. discovery-2026-04-10-kafka-lag)"),
			},
			Required: []string{"id"},
		},
	}
}

func handleDocGet(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		id := req.GetString("id", "")
		if id == "" {
			return mcplib.NewToolResultError("id is required"), nil
		}

		db, err := docstore.Open(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to open docs.db: %v", err)), nil
		}
		defer db.Close()

		doc, err := db.Get(id)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("document %q not found: %v", id, err)), nil
		}

		out := fmt.Sprintf("# %s\ntype: %s | date: %s | repo: %s\ntags: %s\n\n%s",
			doc.Title, doc.Type, doc.DocDate, doc.Repo, doc.Tags, doc.Content)
		return mcplib.NewToolResultText(out), nil
	}
}

// --- doc_list ---

func toolDocList() mcplib.Tool {
	return mcplib.Tool{
		Name:        "doc_list",
		Description: "List workflow documents with optional filters. Returns ID, type, title, date, repo.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"type":  prop("string", "Filter by type: discovery | savepoint | runbook | 1on1 | postmortem | incident | review | poc | draft | reference | config | template"),
				"repo":  prop("string", "Filter by repo name"),
				"since": prop("string", "Filter by date (YYYY-MM-DD) — returns documents on or after this date"),
				"tag":   prop("string", "Filter by tag"),
			},
		},
	}
}

func handleDocList(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		db, err := docstore.Open(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to open docs.db: %v", err)), nil
		}
		defer db.Close()

		filter := docstore.DocFilter{
			Type:  req.GetString("type", ""),
			Repo:  req.GetString("repo", ""),
			Since: req.GetString("since", ""),
			Tag:   req.GetString("tag", ""),
		}

		docs, err := db.List(filter)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("list failed: %v", err)), nil
		}

		if len(docs) == 0 {
			return mcplib.NewToolResultText("no documents found"), nil
		}

		type docSummary struct {
			ID    string `json:"id"`
			Type  string `json:"type"`
			Title string `json:"title"`
			Date  string `json:"date"`
			Repo  string `json:"repo,omitempty"`
			Tags  string `json:"tags,omitempty"`
		}

		var summaries []docSummary
		for _, d := range docs {
			summaries = append(summaries, docSummary{
				ID:    d.ID,
				Type:  d.Type,
				Title: d.Title,
				Date:  d.DocDate,
				Repo:  d.Repo,
				Tags:  strings.Join(d.Tags, ","),
			})
		}

		data, _ := json.Marshal(summaries)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- doc_search ---

func toolDocSearch() mcplib.Tool {
	return mcplib.Tool{
		Name:        "doc_search",
		Description: "Full-text search across all workflow documents (discovery, savepoint, runbook, etc.).",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"keyword": prop("string", "Search keyword — matched against title and content"),
			},
			Required: []string{"keyword"},
		},
	}
}

func handleDocSearch(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		keyword := req.GetString("keyword", "")
		if keyword == "" {
			return mcplib.NewToolResultError("keyword is required"), nil
		}

		db, err := docstore.Open(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to open docs.db: %v", err)), nil
		}
		defer db.Close()

		docs, err := db.Search(keyword)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("search failed: %v", err)), nil
		}

		if len(docs) == 0 {
			return mcplib.NewToolResultText(fmt.Sprintf("no results for %q", keyword)), nil
		}

		var lines []string
		for _, d := range docs {
			lines = append(lines, fmt.Sprintf("%s | %s | %s | %s", d.ID, d.Type, d.DocDate, d.Title))
		}
		return mcplib.NewToolResultText(strings.Join(lines, "\n")), nil
	}
}

// --- doc_add ---

func toolDocAdd() mcplib.Tool {
	return mcplib.Tool{
		Name:        "doc_add",
		Description: "Create a new workflow document. If no content is provided, the template for the given type is applied.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"type":    prop("string", "Document type: discovery | savepoint | runbook | 1on1 | postmortem | incident | review | poc | draft | reference"),
				"title":   prop("string", "Document title"),
				"date":    prop("string", "Document date YYYY-MM-DD (default: today)"),
				"repo":    prop("string", "Associated repo name (optional)"),
				"tags":    prop("string", "Comma-separated tags (optional)"),
				"content": prop("string", "Document content in Markdown (optional — uses template if omitted)"),
			},
			Required: []string{"type", "title"},
		},
	}
}

func handleDocAdd(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		db, err := docstore.Open(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to open docs.db: %v", err)), nil
		}
		defer db.Close()

		docType := req.GetString("type", "")
		title := req.GetString("title", "")
		content := req.GetString("content", "")

		if docType == "" || title == "" {
			return mcplib.NewToolResultError("type and title are required"), nil
		}
		if !db.IsValidType(docType) {
			return mcplib.NewToolResultError(fmt.Sprintf("invalid type %q — valid: %s", docType, strings.Join(db.Types, ", "))), nil
		}

		if content == "" {
			if tmpl, ok := db.GetTemplate(docType); ok {
				content = tmpl
			}
		}

		var tags []string
		if t := req.GetString("tags", ""); t != "" {
			tags = strings.Split(t, ",")
		}

		doc, err := db.Add(docstore.DocInput{
			Type:    docType,
			Title:   title,
			DocDate: req.GetString("date", ""),
			Repo:    req.GetString("repo", ""),
			Tags:    tags,
			Content: content,
		})
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("add failed: %v", err)), nil
		}

		return mcplib.NewToolResultText(fmt.Sprintf("created: %s", doc.ID)), nil
	}
}

// --- memory_get ---

func toolMemoryGet() mcplib.Tool {
	return mcplib.Tool{
		Name:        "memory_get",
		Description: "Get a specific memory entry by key, or list all entries for a topic.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"key":   prop("string", "Exact key to retrieve (e.g. checkpoint_interval_ms)"),
				"topic": prop("string", "Topic to list all entries (e.g. kafka, repoindex)"),
			},
		},
	}
}

func handleMemoryGet(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		store, err := memory.LoadStore(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to load memory: %v", err)), nil
		}

		key := req.GetString("key", "")
		topic := req.GetString("topic", "")

		if key != "" {
			entry, ok := store.Get(key)
			if !ok {
				return mcplib.NewToolResultError(fmt.Sprintf("key %q not found", key)), nil
			}
			return mcplib.NewToolResultText(fmt.Sprintf("key: %s\nvalue: %s\ntype: %s\ntopic: %s\ndesc: %s",
				entry.Key, entry.Value, entry.Type, entry.Topic, entry.Description)), nil
		}

		entries := store.FilterByTopic(topic)
		if len(entries) == 0 {
			msg := "no memory entries found"
			if topic != "" {
				msg = fmt.Sprintf("no entries for topic %q", topic)
			}
			return mcplib.NewToolResultText(msg), nil
		}

		type entrySummary struct {
			Key   string `json:"key"`
			Value string `json:"value"`
			Type  string `json:"type"`
			Topic string `json:"topic"`
			Desc  string `json:"desc,omitempty"`
		}

		var summaries []entrySummary
		for _, e := range entries {
			summaries = append(summaries, entrySummary{
				Key:   e.Key,
				Value: e.Value,
				Type:  e.Type,
				Topic: e.Topic,
				Desc:  e.Description,
			})
		}

		data, _ := json.Marshal(summaries)
		return mcplib.NewToolResultText(string(data)), nil
	}
}

// --- memory_list ---

func toolMemoryList() mcplib.Tool {
	return mcplib.Tool{
		Name:        "memory_list",
		Description: "List all memory entries. Optionally filter by topic or show only stale entries.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"topic": prop("string", "Filter by topic (e.g. kafka, repoindex, webhook)"),
				"stale": prop("number", "Show only entries not verified in N+ days"),
			},
		},
	}
}

func handleMemoryList(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		store, err := memory.LoadStore(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to load memory: %v", err)), nil
		}

		topic := req.GetString("topic", "")
		staleDays := int(req.GetFloat("stale", 0))

		var entries []memory.Entry
		if staleDays > 0 {
			entries = store.Stale(staleDays)
		} else {
			entries = store.FilterByTopic(topic)
		}

		if len(entries) == 0 {
			return mcplib.NewToolResultText("no entries found"), nil
		}

		var lines []string
		for _, e := range entries {
			line := fmt.Sprintf("[%s] %s=%s", e.Topic, e.Key, e.Value)
			if e.Description != "" {
				line += " // " + e.Description
			}
			lines = append(lines, line)
		}
		return mcplib.NewToolResultText(strings.Join(lines, "\n")), nil
	}
}

// --- memory_set ---

func toolMemorySet() mcplib.Tool {
	return mcplib.Tool{
		Name:        "memory_set",
		Description: "Store or update a memory entry in docs.db.",
		InputSchema: mcplib.ToolInputSchema{
			Type: "object",
			Properties: map[string]any{
				"key":   prop("string", "Entry key (e.g. checkpoint_interval_ms)"),
				"value": prop("string", "Entry value"),
				"type":  prop("string", "Entry type: threshold | config | fact | limit"),
				"topic": prop("string", "Topic grouping (e.g. kafka, repoindex, webhook)"),
				"desc":  prop("string", "Human-readable description of the entry"),
			},
			Required: []string{"key", "value", "type", "topic"},
		},
	}
}

func handleMemorySet(workflowHome string) server.ToolHandlerFunc {
	return func(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
		store, err := memory.LoadStore(workflowHome)
		if err != nil {
			return mcplib.NewToolResultError(fmt.Sprintf("failed to load memory: %v", err)), nil
		}

		key := req.GetString("key", "")
		value := req.GetString("value", "")
		entryType := req.GetString("type", "")
		topic := req.GetString("topic", "")
		desc := req.GetString("desc", "")

		if key == "" || value == "" || entryType == "" || topic == "" {
			return mcplib.NewToolResultError("key, value, type, and topic are required"), nil
		}

		store.Set(key, value, entryType, topic, desc)
		return mcplib.NewToolResultText(fmt.Sprintf("set: %s=%s [%s/%s]", key, value, topic, entryType)), nil
	}
}
