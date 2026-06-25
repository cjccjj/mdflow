package parser

type EventType int

const (
	TextEvent EventType = iota
	HeaderStartEvent
	HeaderEndEvent
	BoldStartEvent
	BoldEndEvent
	ItalicStartEvent
	ItalicEndEvent
	StrikethroughStartEvent
	StrikethroughEndEvent
	InlineCodeStartEvent
	InlineCodeEndEvent
	CodeBlockStartEvent
	CodeBlockEndEvent
	CodeBlockLangEvent
	HorizontalRuleEvent
	BulletItemEvent
	NewlineEvent
	TableStartEvent
	TableRowEvent
	TableEndEvent
	BlockquoteStartEvent
	BlockquoteEndEvent
	LinkEvent
	HTMLBlockStartEvent
	HTMLBlockEndEvent
	LinkRefDefEvent
	LinkRefEvent
)

type Event struct {
	Type   EventType
	Value  string
	Level  int
	Cells  []string
	Widths []int
	Aligns []int
	URL    string
}

func (e EventType) String() string {
	switch e {
	case TextEvent:
		return "Text"
	case HeaderStartEvent:
		return "HeaderStart"
	case HeaderEndEvent:
		return "HeaderEnd"
	case BoldStartEvent:
		return "BoldStart"
	case BoldEndEvent:
		return "BoldEnd"
	case ItalicStartEvent:
		return "ItalicStart"
	case ItalicEndEvent:
		return "ItalicEnd"
	case StrikethroughStartEvent:
		return "StrikethroughStart"
	case StrikethroughEndEvent:
		return "StrikethroughEnd"
	case InlineCodeStartEvent:
		return "InlineCodeStart"
	case InlineCodeEndEvent:
		return "InlineCodeEnd"
	case CodeBlockStartEvent:
		return "CodeBlockStart"
	case CodeBlockEndEvent:
		return "CodeBlockEnd"
	case CodeBlockLangEvent:
		return "CodeBlockLang"
	case HorizontalRuleEvent:
		return "HorizontalRule"
	case BulletItemEvent:
		return "BulletItem"
	case NewlineEvent:
		return "Newline"
	case TableStartEvent:
		return "TableStart"
	case TableRowEvent:
		return "TableRow"
	case TableEndEvent:
		return "TableEnd"
	case BlockquoteStartEvent:
		return "BlockquoteStart"
	case BlockquoteEndEvent:
		return "BlockquoteEnd"
	case LinkEvent:
		return "Link"
	case HTMLBlockStartEvent:
		return "HTMLBlockStart"
	case HTMLBlockEndEvent:
		return "HTMLBlockEnd"
	case LinkRefDefEvent:
		return "LinkRefDef"
	case LinkRefEvent:
		return "LinkRef"
	default:
		return "Unknown"
	}
}
