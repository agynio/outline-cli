package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatYAML Format = "yaml"
	FormatMD   Format = "md"
	FormatJSON Format = "json"
)

func ParseFormat(value string) (Format, error) {
	if value == "" {
		return FormatYAML, nil
	}
	switch strings.ToLower(value) {
	case string(FormatYAML):
		return FormatYAML, nil
	case string(FormatMD), "markdown":
		return FormatMD, nil
	case string(FormatJSON):
		return FormatJSON, nil
	default:
		return "", fmt.Errorf("unsupported output format: %s", value)
	}
}

func Print(w io.Writer, format Format, data any) error {
	switch format {
	case FormatYAML:
		payload, err := yaml.Marshal(data)
		if err != nil {
			return fmt.Errorf("marshal yaml: %w", err)
		}
		_, err = w.Write(payload)
		return err
	case FormatJSON:
		encoder := json.NewEncoder(w)
		encoder.SetIndent("", "  ")
		if err := encoder.Encode(data); err != nil {
			return fmt.Errorf("encode json: %w", err)
		}
		return nil
	case FormatMD:
		markdown, ok := data.(Markdown)
		if !ok {
			return fmt.Errorf("markdown output is only supported for markdown responses")
		}
		_, err := fmt.Fprint(w, markdown.Text)
		return err
	default:
		return fmt.Errorf("unsupported output format: %s", format)
	}
}

type Markdown struct {
	Text string
}
