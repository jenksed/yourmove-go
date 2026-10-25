package jobdocument

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

const (
	MaxInputBytes   = 1 << 20
	MaxVisitedNodes = 20_000
	MaxDepth        = 64
	MaxBlocks       = 10_000
	MaxOutputBytes  = 1 << 20
)

var (
	ErrInputTooLarge  = errors.New("job description input exceeds size limit")
	ErrInvalidInput   = errors.New("job description input is invalid")
	ErrInvalidBaseURL = errors.New("job description base URL is invalid")
	ErrNodeLimit      = errors.New("job description exceeds node limit")
	ErrDepthLimit     = errors.New("job description exceeds nesting limit")
	ErrBlockLimit     = errors.New("job description exceeds block limit")
	ErrOutputLimit    = errors.New("job description exceeds output limit")
	ErrNoContent      = errors.New("job description has no meaningful text")
)

// ParseDescription converts selected provider alternatives into owned blocks.
// Rich parsing failures use the complete plain alternative when it is valid;
// otherwise callers can render their current canonical description instead.
// Neither path mutates or normalizes the canonical value used by other features.
func ParseDescription(descriptionHTML, descriptionPlain, savedPostingURL string) (Document, error) {
	if len(descriptionHTML)+len(descriptionPlain) > MaxInputBytes {
		return Document{}, ErrInputTooLarge
	}
	if err := validateTextInput("HTML", descriptionHTML); err != nil {
		return Document{}, err
	}
	if err := validateTextInput("plain", descriptionPlain); err != nil {
		return Document{}, err
	}
	if strings.TrimSpace(descriptionHTML) == "" {
		return ParsePlainText(descriptionPlain)
	}

	base, err := displayBaseURL(savedPostingURL)
	if err != nil {
		if strings.TrimSpace(descriptionPlain) != "" {
			return ParsePlainText(descriptionPlain)
		}
		return Document{}, err
	}
	doc, parseErr := parseHTML(descriptionHTML, base)
	if parseErr == nil {
		return doc, nil
	}
	if strings.TrimSpace(descriptionPlain) != "" {
		if fallback, fallbackErr := ParsePlainText(descriptionPlain); fallbackErr == nil {
			return fallback, nil
		} else {
			return Document{}, fallbackErr
		}
	}
	return Document{}, parseErr
}

// ParsePlainText creates complete paragraph blocks for an owned canonical
// fallback. It does not interpret text that resembles HTML as markup.
func ParsePlainText(description string) (Document, error) {
	if len(description) > MaxInputBytes {
		return Document{}, ErrInputTooLarge
	}
	if err := validateTextInput("plain", description); err != nil {
		return Document{}, err
	}
	if strings.TrimSpace(description) == "" {
		return Document{}, ErrNoContent
	}

	var doc Document
	outputBytes := 0
	var paragraph []string
	flush := func() {
		if len(paragraph) > 0 {
			doc.Blocks = append(doc.Blocks, Block{Kind: BlockParagraph, Content: []Inline{{Text: strings.Join(paragraph, "\n")}}})
			paragraph = nil
		}
	}
	for _, line := range strings.Split(strings.ReplaceAll(description, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		outputBytes += len(line)
		if outputBytes > MaxOutputBytes {
			return Document{}, ErrOutputLimit
		}
		switch {
		case line == "":
			flush()
		case classifyHeading(2, line) != SectionUnknown:
			flush()
			doc.Blocks = append(doc.Blocks, Block{Kind: BlockHeading, Level: 2, Content: []Inline{{Text: line}}})
		case strings.HasPrefix(line, "• ") || strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* "):
			flush()
			text := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(strings.TrimPrefix(line, "• "), "- "), "* "))
			item := ListItem{Content: []Inline{{Text: text}}}
			if len(doc.Blocks) > 0 && doc.Blocks[len(doc.Blocks)-1].Kind == BlockList {
				doc.Blocks[len(doc.Blocks)-1].Items = append(doc.Blocks[len(doc.Blocks)-1].Items, item)
			} else {
				doc.Blocks = append(doc.Blocks, Block{Kind: BlockList, Items: []ListItem{item}})
			}
		default:
			paragraph = append(paragraph, line)
		}
		pending := 0
		if len(paragraph) > 0 {
			pending = 1
		}
		if len(doc.Blocks)+pending > MaxBlocks {
			return Document{}, ErrBlockLimit
		}
	}
	flush()
	if len(doc.Blocks) == 0 {
		return Document{}, ErrNoContent
	}
	return doc, nil
}

func validateTextInput(label, value string) error {
	if !utf8.ValidString(value) {
		return fmt.Errorf("%w: %s is not valid UTF-8", ErrInvalidInput, label)
	}
	if strings.IndexByte(value, 0) >= 0 {
		return fmt.Errorf("%w: %s contains NUL", ErrInvalidInput, label)
	}
	return nil
}

func parseHTML(source string, base *url.URL) (Document, error) {
	context := &html.Node{Type: html.ElementNode, DataAtom: atom.Div, Data: "div"}
	roots, err := html.ParseFragment(strings.NewReader(source), context)
	if err != nil {
		return Document{}, fmt.Errorf("parse posting HTML: %w", err)
	}
	if err := validateTreeBounds(roots); err != nil {
		return Document{}, err
	}

	parser := documentParser{base: base}
	if err := parser.walk(roots); err != nil {
		return Document{}, err
	}
	if err := parser.flushParagraph(); err != nil {
		return Document{}, err
	}
	if len(parser.blocks) == 0 {
		return Document{}, ErrNoContent
	}
	return Document{Blocks: parser.blocks}, nil
}

type nodeAtDepth struct {
	node  *html.Node
	depth int
}

// validateTreeBounds runs before producing output. The byte limit is checked
// before ParseFragment; this pass bounds all nodes, including skipped subtrees.
func validateTreeBounds(roots []*html.Node) error {
	stack := make([]nodeAtDepth, 0, len(roots))
	for i := len(roots) - 1; i >= 0; i-- {
		stack = append(stack, nodeAtDepth{node: roots[i], depth: 1})
	}
	visited := 0
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		visited++
		if visited > MaxVisitedNodes {
			return ErrNodeLimit
		}
		if current.depth > MaxDepth {
			return ErrDepthLimit
		}
		children := make([]*html.Node, 0)
		for child := current.node.FirstChild; child != nil; child = child.NextSibling {
			children = append(children, child)
		}
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, nodeAtDepth{node: children[i], depth: current.depth + 1})
		}
	}
	return nil
}

type documentParser struct {
	base          *url.URL
	blocks        []Block
	current       []Inline
	blockCount    int
	outputBytes   int
	inlineBytes   int
	inlineErr     error
	resolvedLinks map[string]resolvedDisplayLink
}

func (p *documentParser) walk(nodes []*html.Node) error {
	for _, node := range nodes {
		if node.Type == html.TextNode {
			p.current = append(p.current, Inline{Text: safeText(node.Data)})
			continue
		}
		if node.Type != html.ElementNode {
			continue
		}
		tag := strings.ToLower(node.Data)
		if skippedElements[tag] {
			continue
		}
		switch {
		case headingLevel(tag) != 0:
			if err := p.flushParagraph(); err != nil {
				return err
			}
			block := Block{Kind: BlockHeading, Level: headingLevel(tag), Content: p.inlineChildren(node)}
			if p.inlineErr != nil {
				return p.inlineErr
			}
			if err := p.appendBlock(block); err != nil {
				return err
			}
		case tag == "p":
			if err := p.flushParagraph(); err != nil {
				return err
			}
			block := Block{Kind: BlockParagraph, Content: p.inlineChildren(node)}
			if p.inlineErr != nil {
				return p.inlineErr
			}
			if err := p.appendBlock(block); err != nil {
				return err
			}
		case tag == "ul" || tag == "ol":
			if err := p.flushParagraph(); err != nil {
				return err
			}
			block := p.listBlock(node)
			if p.inlineErr != nil {
				return p.inlineErr
			}
			if err := p.appendBlock(block); err != nil {
				return err
			}
		case tag == "br":
			p.current = append(p.current, p.textInline("\n", inlineStyle{})...)
			if p.inlineErr != nil {
				return p.inlineErr
			}
		case tag == "hr":
			if err := p.flushParagraph(); err != nil {
				return err
			}
		case blockContainers[tag]:
			if err := p.flushParagraph(); err != nil {
				return err
			}
			if err := p.walk(childNodes(node)); err != nil {
				return err
			}
		default:
			p.current = append(p.current, p.inlineChildren(node)...)
			if p.inlineErr != nil {
				return p.inlineErr
			}
		}
	}
	return nil
}

var skippedElements = map[string]bool{
	"script": true, "style": true, "noscript": true, "template": true,
	"iframe": true, "frame": true, "frameset": true, "object": true, "embed": true,
	"svg": true, "math": true, "img": true, "picture": true, "source": true, "track": true,
	"video": true, "audio": true, "canvas": true,
	"base": true, "head": true, "meta": true, "link": true,
	"form": true, "input": true, "button": true, "select": true, "option": true, "textarea": true,
}

var blockContainers = map[string]bool{
	"address": true, "article": true, "aside": true, "blockquote": true,
	"div": true, "dl": true, "dt": true, "dd": true, "fieldset": true,
	"figcaption": true, "figure": true, "footer": true, "header": true,
	"main": true, "nav": true, "section": true, "table": true, "tbody": true,
	"td": true, "tfoot": true, "th": true, "thead": true, "tr": true,
	"pre": true,
}

func childNodes(node *html.Node) []*html.Node {
	var children []*html.Node
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		children = append(children, child)
	}
	return children
}

func headingLevel(tag string) int {
	if len(tag) == 2 && tag[0] == 'h' && tag[1] >= '1' && tag[1] <= '6' {
		return int(tag[1] - '0')
	}
	return 0
}

func (p *documentParser) inlineChildren(node *html.Node) []Inline {
	var result []Inline
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		result = append(result, p.inlineNode(child, inlineStyle{})...)
	}
	return mergeInlines(result)
}

type inlineStyle struct {
	strong   bool
	emphasis bool
	href     string
}

func (p *documentParser) inlineNode(node *html.Node, style inlineStyle) []Inline {
	if p.inlineErr != nil {
		return nil
	}
	switch node.Type {
	case html.TextNode:
		return p.textInline(node.Data, style)
	case html.ElementNode:
		tag := strings.ToLower(node.Data)
		if skippedElements[tag] {
			return nil
		}
		switch tag {
		case "br":
			return p.textInline("\n", style)
		case "strong", "b":
			style.strong = true
		case "em", "i":
			style.emphasis = true
		case "a":
			style.href = linkHref(node)
		}
		var result []Inline
		for child := node.FirstChild; child != nil; child = child.NextSibling {
			if p.inlineErr != nil {
				break
			}
			result = append(result, p.inlineNode(child, style)...)
		}
		return mergeInlines(result)
	default:
		return nil
	}
}

func (p *documentParser) textInline(raw string, style inlineStyle) []Inline {
	text := safeText(raw)
	if text == "" {
		return nil
	}
	linkURL := ""
	if style.href != "" {
		if p.resolvedLinks == nil {
			p.resolvedLinks = make(map[string]resolvedDisplayLink)
		}
		resolved, ok := p.resolvedLinks[style.href]
		if !ok {
			resolved.url, resolved.safe = safeDisplayLink(style.href, p.base)
			p.resolvedLinks[style.href] = resolved
		}
		if resolved.safe {
			linkURL = resolved.url
		}
	}
	inline := Inline{Text: text, Strong: style.strong, Emphasis: style.emphasis, LinkURL: linkURL}
	inlineBytes := len(inline.Text) + len(inline.LinkURL)
	if p.inlineBytes+inlineBytes > MaxOutputBytes {
		p.inlineErr = ErrOutputLimit
		return nil
	}
	p.inlineBytes += inlineBytes
	return []Inline{inline}
}

type resolvedDisplayLink struct {
	url  string
	safe bool
}

func safeText(value string) string {
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		if unicode.IsControl(r) && r != '\n' && r != '\r' && r != '\t' {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func mergeInlines(inlines []Inline) []Inline {
	if len(inlines) < 2 {
		return inlines
	}
	merged := make([]Inline, 0, len(inlines))
	var builder strings.Builder
	var current Inline
	hasCurrent := false
	flush := func() {
		if !hasCurrent {
			return
		}
		current.Text = builder.String()
		merged = append(merged, current)
		builder.Reset()
		hasCurrent = false
	}
	for _, inline := range inlines {
		if inline.Text == "" {
			continue
		}
		if !hasCurrent || current.Strong != inline.Strong || current.Emphasis != inline.Emphasis || current.LinkURL != inline.LinkURL {
			flush()
			current = inline
			hasCurrent = true
		}
		builder.WriteString(inline.Text)
	}
	flush()
	return merged
}

func (p *documentParser) listBlock(node *html.Node) Block {
	block := Block{Kind: BlockList, Ordered: strings.EqualFold(node.Data, "ol")}
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && strings.EqualFold(child.Data, "li") {
			block.Items = append(block.Items, p.listItem(child))
			continue
		}
		// Malformed lists can contain useful direct text. Keep it as an item in
		// source order instead of dropping it or rebuilding browser markup.
		content := p.inlineNode(child, inlineStyle{})
		if hasText(content) {
			block.Items = append(block.Items, ListItem{Content: mergeInlines(content)})
		}
	}
	return block
}

func (p *documentParser) listItem(node *html.Node) ListItem {
	item := ListItem{}
	seenNestedList := false
	for child := node.FirstChild; child != nil; child = child.NextSibling {
		if child.Type == html.ElementNode && (strings.EqualFold(child.Data, "ul") || strings.EqualFold(child.Data, "ol")) {
			item.Lists = append(item.Lists, p.listBlock(child))
			seenNestedList = true
			continue
		}
		content := p.inlineNode(child, inlineStyle{})
		if !hasText(content) {
			continue
		}
		if !seenNestedList {
			item.Content = append(item.Content, content...)
			continue
		}
		// The frozen model has a separate nested-block slice. A paragraph after
		// a nested list is retained there so its text stays after that list.
		item.Lists = append(item.Lists, Block{Kind: BlockParagraph, Content: mergeInlines(content)})
	}
	item.Content = mergeInlines(item.Content)
	return item
}

func hasText(inlines []Inline) bool {
	for _, inline := range inlines {
		if strings.TrimSpace(inline.Text) != "" {
			return true
		}
	}
	return false
}

func (p *documentParser) flushParagraph() error {
	content := mergeInlines(p.current)
	p.current = nil
	if !hasText(content) {
		return nil
	}
	return p.appendBlock(Block{Kind: BlockParagraph, Content: content})
}

func (p *documentParser) appendBlock(block Block) error {
	if !blockHasText(block) {
		return nil
	}
	count, outputBytes := blockSize(block)
	if p.blockCount+count > MaxBlocks {
		return ErrBlockLimit
	}
	if p.outputBytes+outputBytes > MaxOutputBytes {
		return ErrOutputLimit
	}
	p.blockCount += count
	p.outputBytes += outputBytes
	p.blocks = append(p.blocks, block)
	return nil
}

func blockHasText(block Block) bool {
	if hasText(block.Content) {
		return true
	}
	for _, item := range block.Items {
		if hasText(item.Content) {
			return true
		}
		for _, nested := range item.Lists {
			if blockHasText(nested) {
				return true
			}
		}
	}
	return false
}

func blockSize(block Block) (blocks, outputBytes int) {
	blocks = 1
	for _, inline := range block.Content {
		outputBytes += len(inline.Text) + len(inline.LinkURL)
	}
	for _, item := range block.Items {
		blocks++ // Bound list items as emitted document blocks too.
		for _, inline := range item.Content {
			outputBytes += len(inline.Text) + len(inline.LinkURL)
		}
		for _, nested := range item.Lists {
			nestedBlocks, nestedBytes := blockSize(nested)
			blocks += nestedBlocks
			outputBytes += nestedBytes
		}
	}
	return blocks, outputBytes
}

func linkHref(node *html.Node) string {
	for _, attr := range node.Attr {
		if strings.EqualFold(attr.Key, "href") {
			return attr.Val
		}
	}
	return ""
}

func displayBaseURL(raw string) (*url.URL, error) {
	if raw == "" || len(raw) > 2048 || hasControl(raw) || strings.Contains(raw, "\\") {
		return nil, ErrInvalidBaseURL
	}
	base, err := url.Parse(raw)
	if err != nil || !strings.EqualFold(base.Scheme, "https") || base.Host == "" || base.User != nil || base.Opaque != "" {
		return nil, ErrInvalidBaseURL
	}
	if port := base.Port(); port != "" && port != "443" {
		return nil, ErrInvalidBaseURL
	}
	host := base.Hostname()
	if host == "" || unsafeHost(host) || hasEncodedSeparator(base.EscapedPath()) {
		return nil, ErrInvalidBaseURL
	}
	copy := *base
	copy.Scheme = "https"
	copy.RawQuery = ""
	copy.ForceQuery = false
	copy.Fragment = ""
	copy.RawFragment = ""
	return &copy, nil
}

func safeDisplayLink(raw string, base *url.URL) (string, bool) {
	if raw == "" || strings.HasPrefix(raw, "//") || hasControl(raw) || strings.ContainsAny(raw, " \\") || hasEncodedSeparator(raw) {
		return "", false
	}
	// Fragment-only references are not links to application-generated anchors.
	if strings.HasPrefix(raw, "#") {
		return "", false
	}
	ref, err := url.Parse(raw)
	if err != nil || ref.User != nil || ref.Opaque != "" {
		return "", false
	}
	if ref.Scheme != "" && !strings.EqualFold(ref.Scheme, "https") {
		return "", false
	}
	if ref.Host != "" && ref.Scheme == "" {
		return "", false
	}
	resolved := base.ResolveReference(ref)
	if !strings.EqualFold(resolved.Scheme, "https") || resolved.User != nil || resolved.Opaque != "" {
		return "", false
	}
	if port := resolved.Port(); port != "" && port != "443" {
		return "", false
	}
	if !strings.EqualFold(resolved.Hostname(), base.Hostname()) || unsafeHost(resolved.Hostname()) {
		return "", false
	}
	if hasControl(resolved.String()) || strings.Contains(resolved.Path, "\\") || hasEncodedSeparator(resolved.EscapedPath()) {
		return "", false
	}
	return resolved.String(), true
}

func unsafeHost(host string) bool {
	host = strings.TrimSuffix(strings.ToLower(host), ".")
	if host == "" || !strings.Contains(host, ".") || net.ParseIP(host) != nil || browserNumericHost(host) {
		return true
	}
	for _, suffix := range []string{".localhost", ".local", ".internal", ".lan", ".home", ".home.arpa"} {
		if strings.HasSuffix(host, suffix) {
			return true
		}
	}
	return host == "localhost"
}

// Browsers accept several non-canonical IPv4 spellings (such as 127.1 and
// 0x7f000001). Reject numeric final labels so those cannot turn a relative
// display link into a loopback destination.
func browserNumericHost(host string) bool {
	last := host[strings.LastIndex(host, ".")+1:]
	if last == "" {
		return false
	}
	if allDigits(last) {
		return true
	}
	if len(last) <= 2 || last[:2] != "0x" {
		return false
	}
	for _, r := range last[2:] {
		if !('0' <= r && r <= '9') && !('a' <= r && r <= 'f') {
			return false
		}
	}
	return true
}

func allDigits(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		if !('0' <= r && r <= '9') {
			return false
		}
	}
	return true
}

func hasControl(value string) bool {
	for _, r := range value {
		if unicode.IsControl(r) {
			return true
		}
	}
	return false
}

func hasEncodedSeparator(value string) bool {
	probe := value
	for range 8 {
		lower := strings.ToLower(probe)
		if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
			return true
		}
		decoded, err := url.PathUnescape(probe)
		if err != nil || decoded == probe {
			return false
		}
		probe = decoded
	}
	// A still-changing value after the bounded decoding pass is ambiguous.
	// Reject it instead of allowing deeper encoding to bypass the check.
	return true
}
