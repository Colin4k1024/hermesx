# Notion Integration

This document describes how to set up and use the Notion platform adapter for HermesX.

## Overview

The Notion adapter allows HermesX to interact with Notion pages by appending content blocks. It uses the Notion API v2022-06-28.

## Prerequisites

1. A Notion account
2. An integration token (API key) from Notion
3. Pages shared with your integration

## Setup

### 1. Create a Notion Integration

1. Go to [https://www.notion.so/my-integrations](https://www.notion.so/my-integrations)
2. Click "New integration"
3. Give it a name (e.g., "HermesX")
4. Select the workspace
5. Click "Submit"
6. Copy the "Internal Integration Secret" (starts with `ntn_` or `secret_`)

### 2. Share Pages with Integration

1. Open the Notion page you want to use
2. Click the "..." menu in the top right
3. Select "Add connections"
4. Find and select your integration

### 3. Configure HermesX

Add the following environment variable:

```bash
NOTION_API_KEY=ntn_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx
```

Or add to your configuration file:

```yaml
platforms:
  notion:
    enabled: true
    token: "ntn_xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"
```

## Usage

### Sending Messages

Messages are sent as paragraph blocks appended to the specified page.

```go
// chatID is the Notion page ID
adapter.Send(ctx, "page-id-123", "Hello from HermesX!", nil)
```

### Page ID

The page ID can be found in the Notion page URL:
- URL: `https://www.notion.so/My-Page-abc123def456`
- Page ID: `abc123def456`

## Capabilities

| Feature | Supported |
|---------|-----------|
| Text Messages | ✅ |
| Images | ✅ (as links) |
| Voice Messages | ❌ |
| Documents | ✅ (as links) |
| Stickers | ❌ |
| Threads | ✅ |
| Reactions | ❌ |
| Message Edits | ✅ |
| Max Message Length | 2000 characters |

## Limitations

1. **Read-Only Polling**: The adapter currently supports sending messages only. Incoming message polling is not yet implemented.
2. **Image Hosting**: Images must be hosted externally (URLs). Local file upload is not supported.
3. **Rich Text**: Only plain text is supported. Notion's rich text formatting is not utilized.

## API Reference

- [Notion API Documentation](https://developers.notion.com/)
- [Notion API Version 2022-06-28](https://developers.notion.com/reference/versioning)

## Troubleshooting

### "Unauthorized" Error

- Verify your API key is correct
- Ensure the integration has access to the page

### "Object Not Found" Error

- Check that the page ID is correct
- Ensure the page is shared with your integration

### Rate Limiting

The Notion API has rate limits. If you encounter rate limit errors:
- Reduce the frequency of requests
- Implement exponential backoff

## Example Configuration

```yaml
platforms:
  notion:
    enabled: true
    token: "${NOTION_API_KEY}"
    settings:
      default_page_id: "abc123def456"
```

## Development

### Running Tests

```bash
go test ./internal/gateway/platforms/... -run TestNotion
```

### Adding Features

To extend the Notion adapter:

1. Edit `internal/gateway/platforms/notion.go`
2. Add tests in `internal/gateway/platforms/notion_test.go`
3. Update this documentation

## References

- Issue: #44
- Implementation: `internal/gateway/platforms/notion.go`
