package jobdocument

import "testing"

func TestDocumentModelCanRepresentOrderedHeadingsListsEmphasisAndLinks(t *testing.T) {
	document := Document{Blocks: []Block{
		{Kind: BlockHeading, Level: 2, Content: []Inline{{Text: "Responsibilities"}}},
		{Kind: BlockParagraph, Content: []Inline{{Text: "Build "}, {Text: "reliable systems", Strong: true}, {Text: " and "}, {Text: "share patterns", Emphasis: true}, {Text: "."}}},
		{Kind: BlockList, Ordered: false, Items: []ListItem{
			{Content: []Inline{{Text: "First"}}, Lists: []Block{{Kind: BlockList, Ordered: true, Items: []ListItem{{Content: []Inline{{Text: "Nested"}}}}}}},
			{Content: []Inline{{Text: "Read the "}, {Text: "team guide", LinkURL: "https://jobs.ashbyhq.com/fable/guide"}}},
		}},
	}}
	if len(document.Blocks) != 3 || document.Blocks[0].Kind != BlockHeading || document.Blocks[0].Level != 2 {
		t.Fatalf("heading/order was not represented: %+v", document)
	}
	if !document.Blocks[1].Content[1].Strong || !document.Blocks[1].Content[3].Emphasis {
		t.Fatalf("inline emphasis was not represented: %+v", document.Blocks[1].Content)
	}
	list := document.Blocks[2]
	if list.Ordered || len(list.Items[0].Lists) != 1 || !list.Items[0].Lists[0].Ordered {
		t.Fatalf("nested ordered/unordered list was not represented: %+v", list)
	}
	if got := list.Items[1].Content[1].LinkURL; got != "https://jobs.ashbyhq.com/fable/guide" {
		t.Fatalf("link target = %q", got)
	}
}
