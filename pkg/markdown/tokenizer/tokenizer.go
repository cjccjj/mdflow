package tokenizer

func Tokenize(input []byte) []Token {
	var tokens []Token
	var textBuf []byte

	flushText := func() {
		if len(textBuf) > 0 {
			tokens = append(tokens, Token{Type: TextToken, Value: string(textBuf)})
			textBuf = textBuf[:0]
		}
	}

	for i := 0; i < len(input); i++ {
		b := input[i]
		switch b {
		case '\\':
			if i+1 < len(input) && (input[i+1] == '\n' || input[i+1] == '\r') {
				i++
				if input[i] == '\r' && i+1 < len(input) && input[i+1] == '\n' {
					i++
				}
				flushText()
				tokens = append(tokens, Token{Type: NewlineToken, Value: "\n"})
			} else {
				flushText()
				tokens = append(tokens, Token{Type: BackslashToken, Value: "\\"})
			}
		case '\n':
			flushText()
			tokens = append(tokens, Token{Type: NewlineToken, Value: "\n"})
		case '\r':
			flushText()
			if i+1 < len(input) && input[i+1] == '\n' {
				i++
			}
			tokens = append(tokens, Token{Type: NewlineToken, Value: "\n"})
		case '*':
			flushText()
			tokens = append(tokens, Token{Type: StarToken, Value: "*"})
		case '`':
			flushText()
			tokens = append(tokens, Token{Type: BacktickToken, Value: "`"})
		case '#':
			flushText()
			tokens = append(tokens, Token{Type: HashToken, Value: "#"})
		case '-':
			flushText()
			tokens = append(tokens, Token{Type: DashToken, Value: "-"})
		case '_':
			flushText()
			tokens = append(tokens, Token{Type: UnderscoreToken, Value: "_"})
		case '>':
			flushText()
			tokens = append(tokens, Token{Type: GreaterToken, Value: ">"})
		case '~':
			flushText()
			tokens = append(tokens, Token{Type: TildeToken, Value: "~"})
		case '|':
			flushText()
			tokens = append(tokens, Token{Type: PipeToken, Value: "|"})
		case '\t':
			flushText()
			tokens = append(tokens, Token{Type: TabToken, Value: "\t"})
		case '[':
			flushText()
			tokens = append(tokens, Token{Type: LeftBracketToken, Value: "["})
		case ']':
			flushText()
			tokens = append(tokens, Token{Type: RightBracketToken, Value: "]"})
		case '&':
			flushText()
			tokens = append(tokens, Token{Type: AmpersandToken, Value: "&"})
		case '(':
			flushText()
			tokens = append(tokens, Token{Type: LeftParenToken, Value: "("})
		case ')':
			flushText()
			tokens = append(tokens, Token{Type: RightParenToken, Value: ")"})
		case 0:
			textBuf = append(textBuf, []byte("\uFFFD")...)
		default:
			textBuf = append(textBuf, b)
		}
	}
	flushText()

	// Remove leading/trailing spaces from text tokens for cleaner parsing.
	// Actually the plan shows " hi " with spaces preserved. Let's keep spaces.
	// The parser should handle trimming.

	return tokens
}
