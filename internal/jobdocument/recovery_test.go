package jobdocument

import "testing"

func TestPastedPlainTextKeepsKnownSectionsAndLists(t *testing.T) {
	text := "Responsibilities\n• Build tools\n• Keep them accessible\n\nQualifications\n- Go experience\n\n<script>alert(1)</script>"
	doc, err := ParsePlainText(text)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) != 5 || doc.Blocks[0].Kind != BlockHeading || doc.Blocks[1].Kind != BlockList || len(doc.Blocks[1].Items) != 2 || doc.Blocks[2].Kind != BlockHeading || doc.Blocks[4].Content[0].Text != "<script>alert(1)</script>" {
		t.Fatalf("pasted structure: %+v", doc)
	}
	sections := SplitSections(doc)
	if len(sections) != 2 || sections[0].Kind != SectionResponsibilities || sections[1].Kind != SectionQualifications {
		t.Fatalf("section navigation unavailable: %+v", sections)
	}
}
