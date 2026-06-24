package parser

import "testing"

func TestCharacterizationFlushThenCloseStates(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		flushTypes []string
		closeTypes []string
	}{
		{
			name:       "bold",
			input:      "**bold",
			flushTypes: []string{"BoldStart", "Text"},
			closeTypes: []string{"BoldEnd"},
		},
		{
			name:       "multi backtick inline code",
			input:      "``code",
			flushTypes: []string{"InlineCodeStart", "Text"},
			closeTypes: []string{"InlineCodeEnd"},
		},
		{
			name:       "fenced code",
			input:      "```go\ncode",
			flushTypes: []string{"CodeBlockStart", "CodeBlockLang", "Newline", "Text"},
			closeTypes: []string{"CodeBlockEnd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			flushEvents := p.Parse(tokenize(tt.input))
			flushEvents = append(flushEvents, p.Flush()...)
			if got := eventTypes(flushEvents); !equal(got, tt.flushTypes) {
				t.Fatalf("flush: expected %v, got %v", tt.flushTypes, got)
			}

			closeEvents := p.CloseStates()
			if got := eventTypes(closeEvents); !equal(got, tt.closeTypes) {
				t.Fatalf("close: expected %v, got %v", tt.closeTypes, got)
			}
		})
	}
}

func TestCharacterizationStreamingBoundaries(t *testing.T) {
	tests := []struct {
		name   string
		chunks []string
		want   []string
	}{
		{
			name:   "split bold opener and closer",
			chunks: []string{"*", "*bo", "ld*", "*"},
			want:   []string{"BoldStart", "Text", "Text", "BoldEnd"},
		},
		{
			name:   "split link",
			chunks: []string{"[la", "bel](", "https://example.test", ")"},
			want:   []string{"Link"},
		},
		{
			name:   "split named entity",
			chunks: []string{"&", "amp;"},
			want:   []string{"Text"},
		},
		{
			name:   "split setext heading",
			chunks: []string{"Heading\n", "---", "\n"},
			want:   []string{"HeaderStart", "Text", "HeaderEnd", "Newline"},
		},
		{
			name:   "split table",
			chunks: []string{"| A | B |\n", "| -", "-- | --- |\n", "| 1 | 2 |\n"},
			want:   []string{"TableStart", "TableRow", "TableEnd"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			var events []Event
			for _, chunk := range tt.chunks {
				events = append(events, p.Parse(tokenize(chunk))...)
			}
			events = append(events, p.Flush()...)
			if got := eventTypes(events); !equal(got, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestCharacterizationFalsePositiveFallbacks(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "table header without separator",
			input: "| H |\nnot separator\n",
			want:  []string{"Text", "Newline", "Text", "Newline"},
		},
		{
			name:  "link without url",
			input: "[label]\n",
			want:  []string{"Text", "Text", "Text", "Newline"},
		},
		{
			name:  "too many hashes",
			input: "####### heading\n",
			want:  []string{"Text", "Text", "Text", "Text", "Text", "Text", "Text", "Text", "Newline"},
		},
		{
			name:  "short dash run",
			input: "--\n",
			want:  []string{"Text", "Text", "Newline"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			p := New()
			events := p.Parse(tokenize(tt.input))
			events = append(events, p.Flush()...)
			if got := eventTypes(events); !equal(got, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, got)
			}
		})
	}
}
