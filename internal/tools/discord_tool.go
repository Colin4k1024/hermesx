package tools

import (
	"encoding/json"
	"os"
	"strings"

	"github.com/bwmarrin/discordgo"
)

func init() {
	if os.Getenv("DISCORD_BOT_TOKEN") == "" {
		return
	}

	Register(&ToolEntry{
		Name:        "discord_send",
		Toolset:     "discord",
		Description: "Send a message to a Discord channel",
		Emoji:       "💬",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"channel_id": map[string]any{"type": "string", "description": "Discord channel ID"},
				"content":    map[string]any{"type": "string", "description": "Message content"},
			},
			"required": []string{"channel_id", "content"},
		},
		Handler:     handleDiscordSend,
		RequiresEnv: []string{"DISCORD_BOT_TOKEN"},
	})

	Register(&ToolEntry{
		Name:        "discord_search",
		Toolset:     "discord",
		Description: "Search messages in a Discord channel",
		Emoji:       "🔍",
		Schema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"channel_id": map[string]any{"type": "string", "description": "Discord channel ID"},
				"query":      map[string]any{"type": "string", "description": "Search query"},
				"limit":      map[string]any{"type": "integer", "description": "Max results", "default": 10},
			},
			"required": []string{"channel_id"},
		},
		Handler:     handleDiscordSearch,
		RequiresEnv: []string{"DISCORD_BOT_TOKEN"},
	})
}

func handleDiscordSend(args map[string]any, ctx *ToolContext) string {
	channelID, _ := args["channel_id"].(string)
	content, _ := args["content"].(string)

	sess, err := discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		return toJSON(map[string]any{"error": err.Error()})
	}

	msg, err := sess.ChannelMessageSend(channelID, content)
	if err != nil {
		return toJSON(map[string]any{"error": err.Error()})
	}

	return toJSON(map[string]any{"status": "sent", "message_id": msg.ID})
}

func handleDiscordSearch(args map[string]any, ctx *ToolContext) string {
	channelID, _ := args["channel_id"].(string)
	query, _ := args["query"].(string)
	limit := 10
	if l, ok := args["limit"].(float64); ok {
		limit = int(l)
	}

	sess, err := discordgo.New("Bot " + os.Getenv("DISCORD_BOT_TOKEN"))
	if err != nil {
		return toJSON(map[string]any{"error": err.Error()})
	}

	messages, err := sess.ChannelMessages(channelID, limit, "", "", "")
	if err != nil {
		return toJSON(map[string]any{"error": err.Error()})
	}

	var results []map[string]any
	for _, m := range messages {
		if query != "" && !strings.Contains(strings.ToLower(m.Content), strings.ToLower(query)) {
			continue
		}
		results = append(results, map[string]any{
			"id":      m.ID,
			"author":  m.Author.Username,
			"content": m.Content,
			"time":    m.Timestamp,
		})
	}

	b, _ := json.Marshal(map[string]any{"messages": results, "count": len(results)})
	return string(b)
}
