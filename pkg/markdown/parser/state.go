package parser

type State int

const (
	NormalState State = iota
	HeaderState
	BoldState
	ItalicState
	StrikethroughState
	InlineCodeState
	CodeBlockState
	IndentedCodeBlockState
	BlockquoteState
	TablePendingState
	TableBodyState
	SetextPendingState
)
