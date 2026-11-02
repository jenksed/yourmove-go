package jobdocument

import (
	"encoding/json"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
)

const testSavedPostingURL = "https://jobs.ashbyhq.com/fable/11111111-1111-4111-8111-111111111111?source=old#overview"

func TestParseDescriptionPreservesMeaningfulStructureAndSourceOrder(t *testing.T) {
	htmlSource := `<h2>Responsibilities</h2><p>Build <strong>reliable</strong> services with <em>care</em>.</p><ul><li>First<ol><li>Nested</li></ol></li><li>Second</li></ul><ol><li>One</li></ol><p>After the lists.</p>`
	doc, err := ParseDescription(htmlSource, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) != 5 {
		t.Fatalf("top-level block count = %d, want 5: %+v", len(doc.Blocks), doc.Blocks)
	}
	if doc.Blocks[0].Kind != BlockHeading || doc.Blocks[0].Level != 2 || inlineText(doc.Blocks[0].Content) != "Responsibilities" {
		t.Fatalf("heading was not preserved: %+v", doc.Blocks[0])
	}
	paragraph := doc.Blocks[1]
	if paragraph.Kind != BlockParagraph || len(paragraph.Content) != 5 {
		t.Fatalf("paragraph inline structure = %+v", paragraph)
	}
	if paragraph.Content[1].Text != "reliable" || !paragraph.Content[1].Strong || paragraph.Content[3].Text != "care" || !paragraph.Content[3].Emphasis {
		t.Fatalf("strong/emphasis was not retained: %+v", paragraph.Content)
	}
	unordered := doc.Blocks[2]
	if unordered.Kind != BlockList || unordered.Ordered || len(unordered.Items) != 2 || len(unordered.Items[0].Lists) != 1 {
		t.Fatalf("unordered/nested list = %+v", unordered)
	}
	if nested := unordered.Items[0].Lists[0]; nested.Kind != BlockList || !nested.Ordered || inlineText(nested.Items[0].Content) != "Nested" {
		t.Fatalf("nested ordered list = %+v", nested)
	}
	if ordered := doc.Blocks[3]; ordered.Kind != BlockList || !ordered.Ordered || inlineText(ordered.Items[0].Content) != "One" {
		t.Fatalf("ordered list = %+v", ordered)
	}
	if doc.Blocks[4].Kind != BlockParagraph || inlineText(doc.Blocks[4].Content) != "After the lists." {
		t.Fatalf("source order trailer = %+v", doc.Blocks[4])
	}
}

func TestParseM0AshbyFixtureKeepsFullReadingStructure(t *testing.T) {
	data, err := os.ReadFile("../platforms/testdata/job-intelligence/ashby-long-synthetic.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Jobs []struct {
			DescriptionHTML  string `json:"descriptionHtml"`
			DescriptionPlain string `json:"descriptionPlain"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Jobs) != 1 {
		t.Fatalf("M0 fixture jobs = %d, want one", len(fixture.Jobs))
	}
	doc, err := ParseDescription(fixture.Jobs[0].DescriptionHTML, fixture.Jobs[0].DescriptionPlain, testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) < 12 {
		t.Fatalf("fixture structure collapsed into %d blocks", len(doc.Blocks))
	}
	text := documentText(doc)
	for _, want := range []string{
		"Senior Platform Engineer", "What you will do", "Design and maintain Go services",
		"Define useful service-level indicators", "Compensation", "€120,000 – €150,000 per year",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("fixture reading omitted %q", want)
		}
	}
	if hasAnyLink(doc) {
		t.Fatal("foreign fable.example handbook link became clickable")
	}
}

func BenchmarkParseM0AshbyFixture(b *testing.B) {
	data, err := os.ReadFile("../platforms/testdata/job-intelligence/ashby-long-synthetic.json")
	if err != nil {
		b.Fatal(err)
	}
	var fixture struct {
		Jobs []struct {
			DescriptionHTML  string `json:"descriptionHtml"`
			DescriptionPlain string `json:"descriptionPlain"`
		} `json:"jobs"`
	}
	if err := json.Unmarshal(data, &fixture); err != nil {
		b.Fatal(err)
	}
	if len(fixture.Jobs) != 1 {
		b.Fatalf("M0 fixture jobs = %d, want one", len(fixture.Jobs))
	}
	job := fixture.Jobs[0]
	b.SetBytes(int64(len(job.DescriptionHTML) + len(job.DescriptionPlain)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseDescription(job.DescriptionHTML, job.DescriptionPlain, testSavedPostingURL); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseNearOutputLimit(b *testing.B) {
	htmlSource := "<p>" + strings.Repeat("x", MaxOutputBytes-16) + "</p>"
	if len(htmlSource) > MaxInputBytes {
		b.Fatalf("near-limit input size = %d, max = %d", len(htmlSource), MaxInputBytes)
	}
	b.SetBytes(int64(len(htmlSource)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := ParseDescription(htmlSource, "", testSavedPostingURL); err != nil {
			b.Fatal(err)
		}
	}
}

func TestParseDescriptionResolvesDisplayLinksAgainstSavedPostingURL(t *testing.T) {
	htmlSource := `<base href="https://foreign.example/ignore/"><p><a href="guide">Guide</a> <a href="/help">Help</a> <a href="../outside">Outside posting path</a> <a href="https://jobs.ashbyhq.com:443/terms">Terms</a></p>`
	doc, err := ParseDescription(htmlSource, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	got := linkTargets(doc)
	want := []string{
		"https://jobs.ashbyhq.com/fable/guide",
		"https://jobs.ashbyhq.com/help",
		"https://jobs.ashbyhq.com/outside",
		"https://jobs.ashbyhq.com:443/terms",
	}
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("resolved links = %q, want %q", got, want)
	}
	if strings.Contains(strings.Join(got, "\n"), "source=old") || strings.Contains(strings.Join(got, "\n"), "overview") {
		t.Fatal("saved URL query or fragment leaked into resolved base")
	}
}

func TestParseDescriptionRejectsUnsafeLinkTargetsButKeepsLabels(t *testing.T) {
	unsafe := []string{
		`https://foreign.example/guide`, `https://jobs.ashbyhq.com.evil.test/guide`,
		`//foreign.example/guide`, `javascript:alert(1)`, `data:text/html,hi`, `file:///etc/passwd`,
		`mailto:people@example.test`, `http://jobs.ashbyhq.com/guide`,
		`https://jobs.ashbyhq.com:8443/guide`, `https://user@jobs.ashbyhq.com/guide`,
		`%2f%2fforeign.example/guide`, `%252f%252fforeign.example/guide`,
		`%252525252f%252525252fforeign.example/guide`, "guide\nnext",
		`#not-generated-by-yourmove`,
	}
	for _, href := range unsafe {
		t.Run(strings.ReplaceAll(href, "/", "_"), func(t *testing.T) {
			htmlSource := `<p><a href="` + href + `">Visible label</a></p>`
			doc, err := ParseDescription(htmlSource, "", testSavedPostingURL)
			if err != nil {
				t.Fatal(err)
			}
			if got := documentText(doc); !strings.Contains(got, "Visible label") {
				t.Fatalf("unsafe link label disappeared: %q", got)
			}
			if got := linkTargets(doc); len(got) != 0 {
				t.Fatalf("unsafe target %q became clickable: %q", href, got)
			}
		})
	}
}

func TestDisplayBaseRejectsPrivateLocalAndBrowserNumericHosts(t *testing.T) {
	for _, host := range []string{
		"127.0.0.1", "10.1.2.3", "[::1]", "127.1", "2130706433", "0x7f000001",
		"localhost", "intranet", "printer.local", "service.internal",
	} {
		t.Run(host, func(t *testing.T) {
			if _, err := displayBaseURL("https://" + host + "/posting"); !errors.Is(err, ErrInvalidBaseURL) {
				t.Fatalf("unsafe base host error = %v", err)
			}
		})
	}
}

func TestParseDescriptionSkipsActiveEmbeddedAndAttributeContent(t *testing.T) {
	htmlSource := `<div onclick="run()"><h2 onmouseover="run()">Useful heading</h2><p>Visible text <a href="https://jobs.ashbyhq.com/guide" onclick="run()">safe guide</a></p><script>steal()</script><style>.secret{display:block}</style><noscript>hidden fallback</noscript><template>template text</template><iframe>frame text</iframe><object>object text</object><embed src="https://foreign.example/embed"><svg><text>svg text</text></svg><math><mi>math text</mi></math><img alt="image alt" src="https://foreign.example/image"><video>video text</video><audio>audio text</audio><canvas>canvas text</canvas></div>`
	doc, err := ParseDescription(htmlSource, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	text := documentText(doc)
	if !strings.Contains(text, "Useful heading") || !strings.Contains(text, "Visible text") || !strings.Contains(text, "safe guide") {
		t.Fatalf("safe employer text missing: %q", text)
	}
	for _, forbidden := range []string{
		"run()", "steal()", ".secret", "hidden fallback", "template text", "frame text",
		"object text", "svg text", "math text", "image alt", "video text", "audio text", "canvas text",
	} {
		if strings.Contains(text, forbidden) {
			t.Errorf("active/embedded content %q survived: %q", forbidden, text)
		}
	}
	if got := linkTargets(doc); len(got) != 1 || got[0] != "https://jobs.ashbyhq.com/guide" {
		t.Fatalf("owned safe link = %q", got)
	}
}

func TestParseDescriptionRecoversMalformedHTMLAndKeepsEncodedMarkupLiteral(t *testing.T) {
	doc, err := ParseDescription(`<h2>Role<p>Build <b>carefully<ul><li>First<li>Second`, "", testSavedPostingURL)
	if err != nil {
		t.Fatalf("malformed fragment failed: %v", err)
	}
	text := documentText(doc)
	for _, want := range []string{"Role", "Build carefully", "First", "Second"} {
		if !strings.Contains(text, want) {
			t.Errorf("malformed fragment lost %q: %q", want, text)
		}
	}

	literal, err := ParseDescription(`<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>`, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	if got := documentText(literal); got != "<script>alert(1)</script>" {
		t.Fatalf("encoded literal markup = %q", got)
	}
}

func TestParseDescriptionFallsBackToCompletePlainAlternative(t *testing.T) {
	doc, err := ParseDescription(`<script>unusable rich content</script>`, "Canonical paragraph one.\n\nCanonical paragraph two.", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	if len(doc.Blocks) != 2 || documentText(doc) != "Canonical paragraph one. Canonical paragraph two." {
		t.Fatalf("plain alternative fallback = %+v (%q)", doc, documentText(doc))
	}
	if _, err := ParseDescription(`<p>rich</p>`, "plain", "http://jobs.ashbyhq.com/fable/id"); err != nil {
		t.Fatalf("invalid link base did not retain complete plain alternative: %v", err)
	}
}

func TestParseOutputLimitStopsRepeatedRelativeLinksInsideParagraph(t *testing.T) {
	baseURL, err := displayBaseURL("https://jobs.ashbyhq.com/" + strings.Repeat("b", 1750))
	if err != nil {
		t.Fatal(err)
	}
	var source strings.Builder
	source.WriteString("<p>")
	for i := 0; i < 900; i++ {
		source.WriteString(`<a href="?detail=`)
		source.WriteString(strconv.Itoa(i))
		source.WriteString(`">x</a>`)
	}
	source.WriteString("</p>")
	if _, err := parseHTML(source.String(), baseURL); !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("repeated expanded links error = %v, want output limit", err)
	}
}

func TestParseDescriptionInputUTF8NULAndAggregateLimits(t *testing.T) {
	for name, htmlSource := range map[string]string{
		"invalid utf8": string([]byte{'<', 'p', '>', 0xff, '<', '/', 'p', '>'}),
		"nul":          "<p>text\x00more</p>",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ParseDescription(htmlSource, "plain fallback", testSavedPostingURL); !errors.Is(err, ErrInvalidInput) {
				t.Fatalf("invalid source error = %v", err)
			}
		})
	}
	if _, err := ParseDescription("<p>café 世界</p>", "", testSavedPostingURL); err != nil {
		t.Fatalf("valid Unicode failed: %v", err)
	}
	if _, err := ParseDescription(strings.Repeat("x", MaxInputBytes), "", testSavedPostingURL); err != nil {
		t.Fatalf("exact input boundary failed: %v", err)
	}
	if _, err := ParseDescription(strings.Repeat("x", MaxInputBytes+1), "", testSavedPostingURL); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("input +1 error = %v", err)
	}
	part := strings.Repeat("x", MaxInputBytes/2)
	if _, err := ParseDescription(part, part, testSavedPostingURL); err != nil {
		t.Fatalf("exact aggregate alternatives boundary failed: %v", err)
	}
	if _, err := ParseDescription(part, part+"x", testSavedPostingURL); !errors.Is(err, ErrInputTooLarge) {
		t.Fatalf("aggregate alternatives +1 error = %v", err)
	}
}

func TestParseDescriptionTreeNodeAndDepthLimitsAtExactBoundary(t *testing.T) {
	comment := `<!--x-->`
	exactNodes := strings.Repeat(comment, MaxVisitedNodes-2) + `<p>x</p>`
	if _, err := ParseDescription(exactNodes, "", testSavedPostingURL); err != nil {
		t.Fatalf("exact node limit failed: %v", err)
	}
	tooManyNodes := strings.Repeat(comment, MaxVisitedNodes-1) + `<p>x</p>`
	if _, err := ParseDescription(tooManyNodes, "", testSavedPostingURL); !errors.Is(err, ErrNodeLimit) {
		t.Fatalf("node limit +1 error = %v", err)
	}

	nested := func(count int) string {
		return strings.Repeat("<div>", count) + "x" + strings.Repeat("</div>", count)
	}
	if _, err := ParseDescription(nested(MaxDepth-1), "", testSavedPostingURL); err != nil {
		t.Fatalf("exact depth boundary failed: %v", err)
	}
	if _, err := ParseDescription(nested(MaxDepth), "", testSavedPostingURL); !errors.Is(err, ErrDepthLimit) {
		t.Fatalf("depth limit +1 error = %v", err)
	}
}

func TestParserEnforcesEmittedBlockLimitWithoutTruncating(t *testing.T) {
	parser := documentParser{}
	items := make([]ListItem, MaxBlocks-1)
	items[0].Content = []Inline{{Text: "one meaningful item"}}
	if err := parser.appendBlock(Block{Kind: BlockList, Items: items}); err != nil {
		t.Fatalf("exact emitted-block limit failed: %v", err)
	}
	if parser.blockCount != MaxBlocks {
		t.Fatalf("count = %d, want %d", parser.blockCount, MaxBlocks)
	}
	if err := parser.appendBlock(Block{Kind: BlockParagraph, Content: []Inline{{Text: "one too many"}}}); !errors.Is(err, ErrBlockLimit) {
		t.Fatalf("block limit +1 error = %v", err)
	}
}

func TestParserEnforcesExactOutputLimitAndPlusOne(t *testing.T) {
	baseURL := "https://jobs.ashbyhq.com/" + strings.Repeat("a", 160) + "/posting"
	base, err := displayBaseURL(baseURL)
	if err != nil {
		t.Fatal(err)
	}
	target, ok := safeDisplayLink("guide", base)
	if !ok {
		t.Fatal("test relative link was rejected")
	}
	perLink := len(target) + 1
	links := MaxOutputBytes / perLink
	remaining := MaxOutputBytes - links*perLink
	lastLabel := strings.Repeat("x", 1+remaining)
	var markup strings.Builder
	for i := 0; i < links-1; i++ {
		markup.WriteString(`<p><a href="guide">x</a></p>`)
	}
	markup.WriteString(`<p><a href="guide">`)
	markup.WriteString(lastLabel)
	markup.WriteString(`</a></p>`)
	if markup.Len() > MaxInputBytes {
		t.Fatalf("boundary input too large: %d", markup.Len())
	}
	doc, err := ParseDescription(markup.String(), "", baseURL)
	if err != nil {
		t.Fatalf("exact output boundary failed: %v", err)
	}
	output := 0
	for _, block := range doc.Blocks {
		_, size := blockSize(block)
		output += size
	}
	if output != MaxOutputBytes {
		t.Fatalf("output bytes = %d, want %d", output, MaxOutputBytes)
	}

	markup.WriteString("x")
	if _, err := ParseDescription(markup.String(), "", baseURL); !errors.Is(err, ErrOutputLimit) {
		t.Fatalf("output limit +1 error = %v", err)
	}
}

func TestParserProducesDeterministicOutput(t *testing.T) {
	source := `<h3>Role</h3><p>A <b>safe</b> description with <a href="guide">a guide</a>.</p><ul><li>One</li><li>Two</li></ul>`
	first, err := ParseDescription(source, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	second, err := ParseDescription(source, "", testSavedPostingURL)
	if err != nil {
		t.Fatal(err)
	}
	if !equalDocument(first, second) {
		t.Fatalf("same input produced different models:\nfirst:  %+v\nsecond: %+v", first, second)
	}
}

func FuzzParseDescription(f *testing.F) {
	f.Add([]byte(`<h2>Role</h2><p>Build <strong>carefully</strong>.</p>`))
	f.Add([]byte(`<script>ignored()</script><a href="javascript:alert(1)">label</a>`))
	f.Add([]byte{0xff, 0x00, '<', 'p', '>'})
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 8192 {
			t.Skip()
		}
		doc, err := ParseDescription(string(data), "", testSavedPostingURL)
		if err != nil {
			return
		}
		blocks, outputBytes := 0, 0
		for _, block := range doc.Blocks {
			count, size := blockSize(block)
			blocks += count
			outputBytes += size
		}
		if blocks > MaxBlocks || outputBytes > MaxOutputBytes {
			t.Fatalf("successful output exceeded bounds: blocks=%d bytes=%d", blocks, outputBytes)
		}
		for _, target := range linkTargets(doc) {
			if !strings.HasPrefix(target, "https://jobs.ashbyhq.com/") {
				t.Fatalf("unsafe target emitted: %q", target)
			}
		}
	})
}

func inlineText(inlines []Inline) string {
	var b strings.Builder
	for _, inline := range inlines {
		b.WriteString(inline.Text)
	}
	return b.String()
}

func documentText(doc Document) string {
	var chunks []string
	var walk func(Block)
	walk = func(block Block) {
		if text := inlineText(block.Content); text != "" {
			chunks = append(chunks, text)
		}
		for _, item := range block.Items {
			if text := inlineText(item.Content); text != "" {
				chunks = append(chunks, text)
			}
			for _, nested := range item.Lists {
				walk(nested)
			}
		}
	}
	for _, block := range doc.Blocks {
		walk(block)
	}
	return strings.Join(strings.Fields(strings.Join(chunks, " ")), " ")
}

func linkTargets(doc Document) []string {
	var result []string
	var add func([]Inline)
	var walk func(Block)
	add = func(inlines []Inline) {
		for _, inline := range inlines {
			if inline.LinkURL != "" {
				result = append(result, inline.LinkURL)
			}
		}
	}
	walk = func(block Block) {
		add(block.Content)
		for _, item := range block.Items {
			add(item.Content)
			for _, nested := range item.Lists {
				walk(nested)
			}
		}
	}
	for _, block := range doc.Blocks {
		walk(block)
	}
	return result
}

func hasAnyLink(doc Document) bool {
	return len(linkTargets(doc)) > 0
}

func equalDocument(left, right Document) bool {
	return strings.TrimSpace(documentText(left)) == strings.TrimSpace(documentText(right)) &&
		strings.Join(linkTargets(left), "\n") == strings.Join(linkTargets(right), "\n") &&
		len(left.Blocks) == len(right.Blocks)
}
