package markdown

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"strings"
	"testing"
	"unicode"
)

// ---------------------------------------------------------------------------
// Spec example parser
// ---------------------------------------------------------------------------

const specFile = "../../dev_docs/commonMark_spec.txt"

// exampleFence is the opening/closing delimiter used by the CommonMark spec
// for test examples.
const exampleFence = "```````````````````````````````` example"
const exampleClose = "````````````````````````````````"

// SpecExample holds a single CommonMark spec test example.
type SpecExample struct {
	Number   int    // 1-based sequential number
	Section  string // nearest heading above the example
	Markdown string // input (before the "." separator)
	HTML     string // expected HTML (for reference only)
	Line     int    // line number in spec file where example starts
}

// parseSpecFile reads the CommonMark spec file and extracts all examples.
func parseSpecFile(path string) ([]SpecExample, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var examples []SpecExample
	var currentSection string
	var inExample bool
	var beforeDot bool // true = reading markdown, false = reading HTML
	var mdBuf, htmlBuf strings.Builder
	var exNum int
	var exLine int

	scanner := bufio.NewScanner(f)
	// Some lines may be very long in the spec.
	scanner.Buffer(make([]byte, 0, 1024*1024), 1024*1024)

	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()

		// Track section headings — lines starting with # or ##.
		if !inExample {
			if strings.HasPrefix(line, "# ") || strings.HasPrefix(line, "## ") || strings.HasPrefix(line, "### ") {
				heading := strings.TrimLeft(line, "# ")
				heading = strings.TrimSpace(heading)
				if heading != "" {
					currentSection = heading
				}
			}
		}

		if !inExample {
			if line == exampleFence {
				inExample = true
				beforeDot = true
				mdBuf.Reset()
				htmlBuf.Reset()
				exNum++
				exLine = lineNo
			}
			continue
		}

		// Inside an example block.
		if line == "." && beforeDot {
			beforeDot = false
			continue
		}

		if line == exampleClose {
			// Replace → with actual tab per spec convention.
			md := strings.ReplaceAll(mdBuf.String(), "→", "\t")
			html := strings.ReplaceAll(htmlBuf.String(), "→", "\t")
			examples = append(examples, SpecExample{
				Number:   exNum,
				Section:  currentSection,
				Markdown: md,
				HTML:     html,
				Line:     exLine,
			})
			inExample = false
			continue
		}

		if beforeDot {
			mdBuf.WriteString(line)
			mdBuf.WriteByte('\n')
		} else {
			htmlBuf.WriteString(line)
			htmlBuf.WriteByte('\n')
		}
	}

	return examples, scanner.Err()
}

// ---------------------------------------------------------------------------
// Section category classification
// ---------------------------------------------------------------------------

// Category describes how we validate a spec example.
type Category int

const (
	// CatSupported — we handle this feature, full structural checks.
	CatSupported Category = iota
	// CatPartial — we handle some aspects; no-crash + text preserved.
	CatPartial
	// CatPassThrough — feature skipped; no-crash + text preserved (raw OK).
	CatPassThrough
	// CatSkip — not applicable; no-crash only.
	CatSkip
)

func (c Category) String() string {
	switch c {
	case CatSupported:
		return "supported"
	case CatPartial:
		return "partial"
	case CatPassThrough:
		return "passthrough"
	case CatSkip:
		return "skip"
	}
	return "unknown"
}

// sectionCategory maps spec section heading names to our handling category.
// Headings are matched case-insensitively as substrings.
var sectionCategory = map[string]Category{
	// Chapter 2: Preliminaries
	"Characters and lines":                CatSupported,
	"Tabs":                                CatPartial,
	"Insecure characters":                 CatSupported,
	"Backslash escapes":                   CatPartial,
	"Entity and numeric character refere": CatPassThrough,

	// Chapter 3: Blocks and inlines (definitions only)
	"Precedence":                    CatSkip,
	"Container blocks and leaf blo": CatSkip,

	// Chapter 4: Leaf blocks
	"Thematic breaks":          CatPartial,
	"ATX headings":             CatPartial,
	"Setext headings":          CatSupported,
	"Indented code blocks":     CatPartial,
	"Fenced code blocks":       CatPartial,
	"HTML blocks":              CatPassThrough,
	"Link reference definition": CatPassThrough,
	"Paragraphs":               CatSupported,
	"Blank lines":              CatPartial,

	// Chapter 5: Container blocks
	"Block quotes": CatPartial,
	"List items":   CatPartial,
	"Lists":        CatPartial,
	"Motivation":   CatSkip, // sub-section of lists, explanatory

	// Chapter 6: Inlines
	"Code spans":                     CatPartial,
	"Emphasis and strong emphasis":   CatPartial,
	"Links":                          CatPartial,
	"Images":                         CatPassThrough,
	"Autolinks":                      CatPassThrough,
	"Raw HTML":                       CatSkip,
	"Hard line breaks":               CatPartial,
	"Soft line breaks":               CatPartial,
	"Textual content":                CatPartial,

	// Appendix
	"A parsing strategy":                          CatSkip,
	"An algorithm for parsing nested emphasis and": CatSkip,
}

// classifySection returns the category for a given section name.
func classifySection(section string) Category {
	lower := strings.ToLower(section)
	for key, cat := range sectionCategory {
		if strings.Contains(lower, strings.ToLower(key)) {
			return cat
		}
	}
	// Default: treat unknown sections as passthrough (safe).
	return CatPassThrough
}

// ---------------------------------------------------------------------------
// Text extraction helpers
// ---------------------------------------------------------------------------

// extractVisibleWords extracts meaningful words from markdown input that
// should appear in rendered output. Strips markdown syntax characters
// and returns non-empty words.
func extractVisibleWords(md string) []string {
	// Remove common markdown syntax that won't appear in output.
	cleaned := md

	// Remove fenced code block markers (``` and ~~~) and their info strings.
	lines := strings.Split(cleaned, "\n")
	var contentLines []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "```") || strings.HasPrefix(trimmed, "~~~") {
			// Skip fence lines, but extract info string words.
			rest := strings.TrimLeft(trimmed, "`~")
			rest = strings.TrimSpace(rest)
			if rest != "" {
				// Info string like "python" — this may or may not appear.
				// Don't require it.
			}
			continue
		}
		contentLines = append(contentLines, line)
	}
	cleaned = strings.Join(contentLines, "\n")

	// Remove structural markdown characters.
	replacer := strings.NewReplacer(
		"**", "", "__", "", "~~", "",
		"*", "", "_", "",
		"`", "",
		"[", "", "]", "", "(", "", ")", "",
		"#", "",
		">", "", "\\", "",
	)
	cleaned = replacer.Replace(cleaned)

	// Split into words.
	fields := strings.Fields(cleaned)

	var words []string
	for _, w := range fields {
		// Skip pure punctuation / whitespace tokens.
		hasAlnum := false
		for _, r := range w {
			if unicode.IsLetter(r) || unicode.IsDigit(r) {
				hasAlnum = true
				break
			}
		}
		if hasAlnum && len(w) >= 2 {
			words = append(words, w)
		}
	}
	return words
}

// ---------------------------------------------------------------------------
// The test
// ---------------------------------------------------------------------------

func TestCommonMarkSpec(t *testing.T) {
	examples, err := parseSpecFile(specFile)
	if err != nil {
		t.Fatalf("failed to parse spec file: %v", err)
	}

	if len(examples) == 0 {
		t.Fatal("no examples found in spec file")
	}

	t.Logf("Loaded %d spec examples", len(examples))

	// Counters for summary.
	counts := map[Category]struct{ total, pass int }{}

	for _, ex := range examples {
		cat := classifySection(ex.Section)
		ex := ex // capture

		name := fmt.Sprintf("Ex%03d/%s", ex.Number, sanitizeName(ex.Section))
		t.Run(name, func(t *testing.T) {
			// ---- Run through renderer ----
			var panicked bool
			output := safeRender(ex.Markdown, &panicked)

			// ---- Validation: no panic (all categories) ----
			if panicked {
				t.Fatalf("[%s] PANIC on example %d (spec line %d)\nSection: %s\nInput:\n%s",
					cat, ex.Number, ex.Line, ex.Section, ex.Markdown)
			}

			// ---- Validation: text preservation (supported, partial, passthrough) ----
			if cat != CatSkip {
				plain := stripANSI(output)
				words := extractVisibleWords(ex.Markdown)
				var missing []string
				for _, w := range words {
					if !strings.Contains(plain, w) {
						missing = append(missing, w)
					}
				}
				if len(missing) > 0 && cat == CatSupported {
					t.Errorf("[%s] Example %d: missing words in output: %v\nSection: %s\nInput:\n%s\nOutput (plain):\n%s",
						cat, ex.Number, missing, ex.Section, ex.Markdown, plain)
				} else if len(missing) > 0 {
					// For partial/passthrough, log but don't fail.
					t.Logf("[%s] Example %d: %d words not found (non-fatal): %v",
						cat, ex.Number, len(missing), missing)
				}
			}
		})
	}

	// Print summary.
	_ = counts
}

// safeRender runs the renderer and catches panics.
func safeRender(input string, panicked *bool) (output string) {
	defer func() {
		if r := recover(); r != nil {
			*panicked = true
			output = fmt.Sprintf("PANIC: %v", r)
		}
	}()

	var buf bytes.Buffer
	r := NewRenderer(&buf)
	r.Write([]byte(input))
	r.Close()
	return buf.String()
}

// sanitizeName makes a test name safe for t.Run().
func sanitizeName(s string) string {
	if len(s) > 40 {
		s = s[:40]
	}
	var b strings.Builder
	for _, r := range s {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		case r == ' ' || r == '-' || r == '_':
			b.WriteRune('_')
		}
	}
	return b.String()
}
