package render

type Style struct {
	Prefix string
	Suffix string
}

type Theme struct {
	H1            Style
	H2            Style
	H3            Style
	H4            Style
	H5            Style
	H6            Style
	Bold          Style
	Italic        Style
	Strikethrough Style
	InlineCode    Style
	CodeBlock     Style
	CodeBlockLang Style
	HorizontalRule Style
	BulletItem    Style
	Blockquote    Style
	TableBorder   Style
	TableHeader   Style
	TableCell     Style
	LinkText      Style
	LinkURL       Style
	Text          Style
}

var DefaultTheme = Theme{
	H1: Style{
		Prefix: "\033[1;48;5;63;38;5;228m",
		Suffix: "\033[0m",
	},
	H2: Style{
		Prefix: "\033[1;34m",
		Suffix: "\033[0m",
	},
	H3: Style{
		Prefix: "\033[1;33m",
		Suffix: "\033[0m",
	},
	H4: Style{
		Prefix: "\033[1;36m",
		Suffix: "\033[0m",
	},
	H5: Style{
		Prefix: "\033[1;35m",
		Suffix: "\033[0m",
	},
	H6: Style{
		Prefix: "\033[1;32m",
		Suffix: "\033[0m",
	},
	Bold: Style{
		Prefix: "\033[1m",
		Suffix: "\033[0m",
	},
	Italic: Style{
		Prefix: "\033[3m",
		Suffix: "\033[0m",
	},
	Strikethrough: Style{
		Prefix: "\033[9m",
		Suffix: "\033[0m",
	},
	InlineCode: Style{
		Prefix: "\033[38;5;215;48;5;236m",
		Suffix: "\033[0m",
	},
	CodeBlock: Style{
		Prefix: "\033[38;5;245m",
		Suffix: "\033[0m",
	},
	CodeBlockLang: Style{
		Prefix: "\033[3;38;5;221m",
		Suffix: "\033[0m",
	},
	HorizontalRule: Style{
		Prefix: "\033[2m",
		Suffix: "\033[0m",
	},
	BulletItem: Style{
		Prefix: "",
		Suffix: "",
	},
	Blockquote: Style{
		Prefix: "\033[2m",
		Suffix: "\033[0m",
	},
	TableBorder: Style{
		Prefix: "\033[2m",
		Suffix: "\033[0m",
	},
	TableHeader: Style{
		Prefix: "\033[1m",
		Suffix: "\033[0m",
	},
	TableCell: Style{
		Prefix: "",
		Suffix: "",
	},
	LinkText: Style{
		Prefix: "\033[4;34m",
		Suffix: "\033[0m",
	},
	LinkURL: Style{
		Prefix: "\033[2;34m",
		Suffix: "\033[0m",
	},
	Text: Style{},
}
