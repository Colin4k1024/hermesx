package tooladapter

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/Colin4k1024/hermesx/internal/egress"
	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/schema"
	"github.com/eino-contrib/jsonschema"

	"github.com/Colin4k1024/hermesx/internal/eino/ctxkeys"
	"github.com/Colin4k1024/hermesx/internal/tools"
)

// WrappedTool bridges a HermesX ToolEntry to Eino's InvokableTool interface.
type WrappedTool struct {
	entry *tools.ToolEntry
}

var _ tool.InvokableTool = (*WrappedTool)(nil)

// Wrap creates an Eino InvokableTool from a HermesX ToolEntry.
func Wrap(entry *tools.ToolEntry) *WrappedTool {
	return &WrappedTool{entry: entry}
}

// WrapAll converts a slice of ToolEntry into Eino InvokableTools and their ToolInfos.
func WrapAll(entries []*tools.ToolEntry) ([]tool.InvokableTool, []*schema.ToolInfo, error) {
	invokables := make([]tool.InvokableTool, 0, len(entries))
	infos := make([]*schema.ToolInfo, 0, len(entries))
	for _, e := range entries {
		w := Wrap(e)
		info, err := w.Info(context.Background())
		if err != nil {
			return nil, nil, fmt.Errorf("tool %q: %w", e.Name, err)
		}
		invokables = append(invokables, w)
		infos = append(infos, info)
	}
	return invokables, infos, nil
}

func (w *WrappedTool) Info(_ context.Context) (*schema.ToolInfo, error) {
	desc := w.entry.Description
	if desc == "" {
		if d, ok := w.entry.Schema["description"].(string); ok {
			desc = d
		}
	}

	params, err := extractParamsSchema(w.entry.Schema)
	if err != nil {
		return nil, fmt.Errorf("tool %q schema: %w", w.entry.Name, err)
	}

	info := &schema.ToolInfo{
		Name: w.entry.Name,
		Desc: desc,
	}
	if params != nil {
		info.ParamsOneOf = schema.NewParamsOneOfByJSONSchema(params)
	}
	return info, nil
}

func (w *WrappedTool) InvokableRun(ctx context.Context, argumentsInJSON string, _ ...tool.Option) (string, error) {
	var args map[string]any
	if argumentsInJSON != "" && argumentsInJSON != "{}" {
		if err := json.Unmarshal([]byte(argumentsInJSON), &args); err != nil {
			return "", fmt.Errorf("tool %q: invalid arguments JSON: %w", w.entry.Name, err)
		}
	}
	if args == nil {
		args = make(map[string]any)
	}

	tctx := ctxkeys.ToolContextFrom(ctx)
	if tctx == nil {
		tctx = &tools.ToolContext{}
	}
	tctx = enrichToolContext(tctx, w.entry)

	result := w.entry.Handler(ctx, args, tctx)
	return result, nil
}

func enrichToolContext(base *tools.ToolContext, entry *tools.ToolEntry) *tools.ToolContext {
	if base == nil {
		base = &tools.ToolContext{}
	}
	cloned := *base
	if cloned.HTTPClient == nil {
		if transport, ok := cloned.Extra["egress_transport"].(*http.Transport); ok && transport != nil {
			maxRedirects := 0
			if entry != nil {
				maxRedirects = entry.MaxRedirects
			}
			cloned.HTTPClient = &http.Client{
				Transport: transport,
				Timeout:   30 * time.Second,
				CheckRedirect: func(req *http.Request, via []*http.Request) error {
					if maxRedirects == 0 {
						return http.ErrUseLastResponse
					}
					if len(via) >= maxRedirects {
						return egress.ErrNotAllowed
					}
					return nil
				},
			}
		}
	}
	return &cloned
}

// extractParamsSchema converts the "parameters" field from the OpenAI function
// schema format (map[string]any) into a *jsonschema.Schema.
func extractParamsSchema(schemaMap map[string]any) (*jsonschema.Schema, error) {
	params, ok := schemaMap["parameters"]
	if !ok || params == nil {
		return nil, nil
	}

	data, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal parameters: %w", err)
	}

	var js jsonschema.Schema
	if err := json.Unmarshal(data, &js); err != nil {
		return nil, fmt.Errorf("unmarshal to jsonschema: %w", err)
	}
	return &js, nil
}
