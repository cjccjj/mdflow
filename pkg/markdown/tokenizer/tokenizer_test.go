package tokenizer

import (
	"strings"
	"testing"
)

func TestTokenize(t *testing.T) {
	input := []byte("## hi **abc**")

	tokens := Tokenize(input)

	expected := []Token{
		{Type: HashToken, Value: "#"},
		{Type: HashToken, Value: "#"},
		{Type: TextToken, Value: " hi "},
		{Type: StarToken, Value: "*"},
		{Type: StarToken, Value: "*"},
		{Type: TextToken, Value: "abc"},
		{Type: StarToken, Value: "*"},
		{Type: StarToken, Value: "*"},
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d: %v", len(expected), len(tokens), tokens)
	}

	for i, tok := range tokens {
		if tok.Type != expected[i].Type {
			t.Errorf("token %d: expected type %s, got %s", i, expected[i].Type, tok.Type)
		}
		if tok.Value != expected[i].Value {
			t.Errorf("token %d: expected value %q, got %q", i, expected[i].Value, tok.Value)
		}
	}
}

func TestTokenizeEmpty(t *testing.T) {
	tokens := Tokenize([]byte{})
	if len(tokens) != 0 {
		t.Fatalf("expected 0 tokens, got %d", len(tokens))
	}
}

func TestTokenizeOnlyText(t *testing.T) {
	tokens := Tokenize([]byte("hello world"))
	expected := []Token{
		{Type: TextToken, Value: "hello world"},
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d", len(expected), len(tokens))
	}
	if tokens[0].Type != TextToken || tokens[0].Value != "hello world" {
		t.Errorf("expected TextToken 'hello world', got %v", tokens[0])
	}
}

func TestTokenizeNewlines(t *testing.T) {
	tokens := Tokenize([]byte("a\nb\n"))
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(tokens), tokens)
	}
	expectedTypes := []TokenType{TextToken, NewlineToken, TextToken, NewlineToken}
	for i, tok := range tokens {
		if tok.Type != expectedTypes[i] {
			t.Errorf("token %d: expected %s, got %s", i, expectedTypes[i], tok.Type)
		}
	}
}

func TestTokenizeCRLF(t *testing.T) {
	tokens := Tokenize([]byte("a\r\nb\nc\r"))
	if len(tokens) != 6 {
		t.Fatalf("expected 6 tokens, got %d: %v", len(tokens), tokens)
	}
	expectedTypes := []TokenType{TextToken, NewlineToken, TextToken, NewlineToken, TextToken, NewlineToken}
	for i, tok := range tokens {
		if tok.Type != expectedTypes[i] {
			t.Errorf("token %d: expected %s, got %s (value=%q)", i, expectedTypes[i], tok.Type, tok.Value)
		}
	}
}

func TestTokenizeTab(t *testing.T) {
	tokens := Tokenize([]byte("a\tb"))
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d: %v", len(tokens), tokens)
	}
	expectedTypes := []TokenType{TextToken, TabToken, TextToken}
	for i, tok := range tokens {
		if tok.Type != expectedTypes[i] {
			t.Errorf("token %d: expected %s, got %s (value=%q)", i, expectedTypes[i], tok.Type, tok.Value)
		}
	}
}

func TestTokenizeNullReplacement(t *testing.T) {
	tokens := Tokenize([]byte("a\x00b"))
	if len(tokens) != 1 {
		t.Fatalf("expected 1 token, got %d: %v", len(tokens), tokens)
	}
	if tokens[0].Type != TextToken {
		t.Errorf("expected TextToken, got %s", tokens[0].Type)
	}
	if !strings.Contains(tokens[0].Value, "\uFFFD") {
		t.Errorf("expected replacement character U+FFFD in value, got %q", tokens[0].Value)
	}
}

func TestTokenizeBackslashEscape(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []Token
	}{
		{
			name:  "escape star",
			input: []byte(`\*text`),
			expected: []Token{
				{Type: BackslashToken, Value: "\\"},
				{Type: StarToken, Value: "*"},
				{Type: TextToken, Value: "text"},
			},
		},
		{
			name:  "escape hash",
			input: []byte(`\# not heading`),
			expected: []Token{
				{Type: BackslashToken, Value: "\\"},
				{Type: HashToken, Value: "#"},
				{Type: TextToken, Value: " not heading"},
			},
		},
		{
			name:  "escaped backtick",
			input: []byte{'\\', '`', 'n', 'o', 't', ' ', 'c', 'o', 'd', 'e', '`'},
			expected: []Token{
				{Type: BackslashToken, Value: "\\"},
				{Type: BacktickToken, Value: "`"},
				{Type: TextToken, Value: "not code"},
				{Type: BacktickToken, Value: "`"},
			},
		},
		{
			name:  "escaped backslash",
			input: []byte(`\\`),
			expected: []Token{
				{Type: BackslashToken, Value: "\\"},
				{Type: BackslashToken, Value: "\\"},
			},
		},
		{
			name:  "backslash before letter (literal)",
			input: []byte(`\a`),
			expected: []Token{
				{Type: BackslashToken, Value: "\\"},
				{Type: TextToken, Value: "a"},
			},
		},
		{
			name:  "backslash followed by space (literal)",
			input: []byte(`\ word`),
			expected: []Token{
				{Type: BackslashToken, Value: "\\"},
				{Type: TextToken, Value: " word"},
			},
		},
		{
			name:  "mixed: escaped star followed by real star",
			input: []byte(`\**bold*`),
			expected: []Token{
				{Type: BackslashToken, Value: "\\"},
				{Type: StarToken, Value: "*"},
				{Type: StarToken, Value: "*"},
				{Type: TextToken, Value: "bold"},
				{Type: StarToken, Value: "*"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens := Tokenize(tt.input)
			if len(tokens) != len(tt.expected) {
				t.Fatalf("expected %d tokens, got %d: %v", len(tt.expected), len(tokens), tokens)
			}
			for i, tok := range tokens {
				if tok.Type != tt.expected[i].Type {
					t.Errorf("token %d: expected type %s, got %s", i, tt.expected[i].Type, tok.Type)
				}
				if tok.Value != tt.expected[i].Value {
					t.Errorf("token %d: expected value %q, got %q", i, tt.expected[i].Value, tok.Value)
				}
			}
		})
	}
}

func TestTokenizeUnderscore(t *testing.T) {
	tokens := Tokenize([]byte("___ foo"))
	if len(tokens) != 4 {
		t.Fatalf("expected 4 tokens, got %d: %v", len(tokens), tokens)
	}
	expectedTypes := []TokenType{UnderscoreToken, UnderscoreToken, UnderscoreToken, TextToken}
	for i, tok := range tokens {
		if tok.Type != expectedTypes[i] {
			t.Errorf("token %d: expected %s, got %s (value=%q)", i, expectedTypes[i], tok.Type, tok.Value)
		}
	}
}

func TestTokenizeSpecialChars(t *testing.T) {
	tokens := Tokenize([]byte("* ` # -"))
	if len(tokens) != 7 {
		t.Fatalf("expected 7 tokens, got %d: %v", len(tokens), tokens)
	}
	expectedTypes := []TokenType{
		StarToken,
		TextToken,
		BacktickToken,
		TextToken,
		HashToken,
		TextToken,
		DashToken,
	}
	for i, tok := range tokens {
		if tok.Type != expectedTypes[i] {
			t.Errorf("token %d: expected %s, got %s (value=%q)", i, expectedTypes[i], tok.Type, tok.Value)
		}
	}
}
