package tokenizer

type TokenType int

const (
	TextToken TokenType = iota
	NewlineToken
	StarToken
	BacktickToken
	HashToken
	DashToken
	TildeToken
	PipeToken
	TabToken
	UnderscoreToken
	GreaterToken
	LeftBracketToken
	RightBracketToken
	LeftParenToken
	RightParenToken
	BackslashToken
	AmpersandToken
	EOFToken
)

type Token struct {
	Type  TokenType
	Value string
}

func (t TokenType) String() string {
	switch t {
	case TextToken:
		return "Text"
	case NewlineToken:
		return "Newline"
	case StarToken:
		return "Star"
	case BacktickToken:
		return "Backtick"
	case HashToken:
		return "Hash"
	case DashToken:
		return "Dash"
	case TildeToken:
		return "Tilde"
	case PipeToken:
		return "Pipe"
	case TabToken:
		return "Tab"
	case UnderscoreToken:
		return "Underscore"
	case GreaterToken:
		return "Greater"
	case LeftBracketToken:
		return "LeftBracket"
	case RightBracketToken:
		return "RightBracket"
	case LeftParenToken:
		return "LeftParen"
	case RightParenToken:
		return "RightParen"
	case BackslashToken:
		return "Backslash"
	case AmpersandToken:
		return "Ampersand"
	case EOFToken:
		return "EOF"
	default:
		return "Unknown"
	}
}
