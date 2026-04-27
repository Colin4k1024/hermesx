package llm

import "encoding/json"

// ToolDef is a provider-neutral tool definition.
type ToolDef struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Parameters  map[string]any `json:"parameters,omitempty"`
}

// ToRawParameters returns Parameters as json.RawMessage for SDKs that require it.
func (t ToolDef) ToRawParameters() json.RawMessage {
	if t.Parameters == nil {
		return nil
	}
	b, err := json.Marshal(t.Parameters)
	if err != nil {
		return nil
	}
	return b
}
