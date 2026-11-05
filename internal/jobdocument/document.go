// Package jobdocument defines the small owned document model used to render
// employer posting content safely. It has no provider, app, or storage policy.
package jobdocument

type BlockKind string

const (
	BlockHeading   BlockKind = "heading"
	BlockParagraph BlockKind = "paragraph"
	BlockList      BlockKind = "list"
)

type Inline struct {
	Text     string
	Strong   bool
	Emphasis bool
	// LinkURL is a checked, display-only target. It grants no posting or
	// application-navigation authority; unsafe links retain their label as text.
	LinkURL string
}

type ListItem struct {
	Content []Inline
	Lists   []Block
}

type Block struct {
	Kind    BlockKind
	Level   int
	Content []Inline
	Ordered bool
	Items   []ListItem
}

// Document preserves the sequence of safe, owned blocks from one posting.
type Document struct {
	Blocks []Block
}
