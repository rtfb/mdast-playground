package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"regexp"
)

var (
	reATXHeaderMarker = regexp.MustCompile("^#{1,6}(?: +|$)")
	reHrule           = regexp.MustCompile("^(?:(?:\\* *){3,}|(?:_ *){3,}|(?:- *){3,}) *$")
)

type NodeType int

const (
	Document NodeType = iota
	BlockQuote
	List
	Item
	Paragraph
	Header
	HorizontalRule
	Emph
	Strong
	Link
	Image
	Text
)

var nodeTypeNames = []string{
	Document:       "Document",
	BlockQuote:     "BlockQuote",
	List:           "List",
	Item:           "Item",
	Paragraph:      "Paragraph",
	Header:         "Header",
	HorizontalRule: "HorizontalRule",
	Emph:           "Emph",
	Strong:         "Strong",
	Link:           "Link",
	Image:          "Image",
	Text:           "Text",
}

func (t NodeType) String() string {
	return nodeTypeNames[t]
}

var blockHandlers = map[NodeType]BlockHandler{
	Document:       &DocumentBlockHandler{},
	Header:         &HeaderBlockHandler{},
	HorizontalRule: &HorizontalRuleBlockHandler{},
	BlockQuote:     &BlockQuoteBlockHandler{},
	Paragraph:      &ParagraphBlockHandler{},
}

type ContinueStatus int

const (
	Matched = iota
	NotMatched
	Completed
)

type BlockHandler interface {
	Continue(p *Parser, container *Node) ContinueStatus
	Finalize(p *Parser, block *Node)
	CanContain(t NodeType) bool
	AcceptsLines() bool
}

type HeaderBlockHandler struct {
}

func (h *HeaderBlockHandler) Continue(p *Parser, container *Node) ContinueStatus {
	// a header can never contain > 1 line, so fail to match:
	return NotMatched
}

func (h *HeaderBlockHandler) Finalize(p *Parser, block *Node) {
}

func (h *HeaderBlockHandler) CanContain(t NodeType) bool {
	return false
}

func (h *HeaderBlockHandler) AcceptsLines() bool {
	return false
}

type DocumentBlockHandler struct {
}

func (h *DocumentBlockHandler) Continue(p *Parser, container *Node) ContinueStatus {
	return Matched
}

func (h *DocumentBlockHandler) Finalize(p *Parser, block *Node) {
}

func (h *DocumentBlockHandler) CanContain(t NodeType) bool {
	return t != Item
}

func (h *DocumentBlockHandler) AcceptsLines() bool {
	return false
}

type HorizontalRuleBlockHandler struct {
}

func (h *HorizontalRuleBlockHandler) Continue(p *Parser, container *Node) ContinueStatus {
	// an hrule can never container > 1 line, so fail to match:
	return NotMatched
}

func (h *HorizontalRuleBlockHandler) Finalize(p *Parser, block *Node) {
}

func (h *HorizontalRuleBlockHandler) CanContain(t NodeType) bool {
	return false
}

func (h *HorizontalRuleBlockHandler) AcceptsLines() bool {
	return false
}

type BlockQuoteBlockHandler struct {
}

func (h *BlockQuoteBlockHandler) Continue(p *Parser, container *Node) ContinueStatus {
	ln := p.currentLine
	if !p.indented && peek(ln, p.nextNonspace) == '>' {
		p.advanceNextNonspace()
		p.advanceOffset(1, false)
		if peek(ln, p.offset) == ' ' {
			p.offset += 1
		}
	} else {
		return NotMatched
	}
	return Matched
}

func (h *BlockQuoteBlockHandler) Finalize(p *Parser, block *Node) {
}

func (h *BlockQuoteBlockHandler) CanContain(t NodeType) bool {
	return t != Item
}

func (h *BlockQuoteBlockHandler) AcceptsLines() bool {
	return false
}

type ParagraphBlockHandler struct {
}

func (h *ParagraphBlockHandler) Continue(p *Parser, container *Node) ContinueStatus {
	if p.blank {
		return NotMatched
	} else {
		return Matched
	}
}

func (h *ParagraphBlockHandler) Finalize(p *Parser, block *Node) {
	/*
		TODO:
			hasReferenceDefs := false
			for peek(block.content, 0) == '[' &&
				(pos := p.inlineParser.parseReference(block.content, p.refmap); pos != 0) {
				block.content = block.content[pos:]
				hasReferenceDefs = true
			}
			if hasReferenceDefs && isBlank(block.content) {
				block.unlink()
			}
	*/
}

func (h *ParagraphBlockHandler) CanContain(t NodeType) bool {
	return false
}

func (h *ParagraphBlockHandler) AcceptsLines() bool {
	return true
}

type SourceRange struct {
	line    uint32 // line # in the source document
	char    uint32 // char pos in line
	endLine uint32 // same as above pair, but for end of entity
	endChar uint32
}

func NewSourceRange() *SourceRange {
	return &SourceRange{
		line:    1,
		char:    1,
		endLine: 0,
		endChar: 0,
	}
}

type Node struct {
	Type       NodeType
	parent     *Node
	firstChild *Node
	lastChild  *Node
	prev       *Node // prev sibling
	next       *Node // next sibling
	sourcePos  *SourceRange
	content    []byte
	level      uint32
	open       bool
	//isFenced      bool
	lastLineBlank bool
	literal       []byte
}

func NewNode(typ NodeType, src *SourceRange) *Node {
	return &Node{
		Type:       typ,
		parent:     nil,
		firstChild: nil,
		lastChild:  nil,
		prev:       nil,
		next:       nil,
		sourcePos:  src,
		content:    nil,
		level:      0,
		open:       true,
		//isFenced:      false,
		lastLineBlank: false,
		literal:       nil,
	}
}

func (n *Node) unlink() {
	if n.prev != nil {
		n.prev.next = n.next
	} else if n.parent != nil {
		n.parent.firstChild = n.next
	}
	if n.next != nil {
		n.next.prev = n.prev
	} else if n.parent != nil {
		n.parent.lastChild = n.prev
	}
	n.parent = nil
	n.next = nil
	n.prev = nil
}

func (n *Node) appendChild(child *Node) {
	child.unlink()
	child.parent = n
	if n.lastChild != nil {
		n.lastChild.next = child
		child.prev = n.lastChild
		n.lastChild = child
	} else {
		n.firstChild = child
		n.lastChild = child
	}
}

func (n *Node) isContainer() bool {
	switch n.Type {
	case Document:
		fallthrough
	case BlockQuote:
		fallthrough
	case List:
		fallthrough
	case Item:
		fallthrough
	case Paragraph:
		fallthrough
	case Header:
		fallthrough
	case Emph:
		fallthrough
	case Strong:
		fallthrough
	case Link:
		fallthrough
	case Image:
		return true
	default:
		return false
	}
	return false
}

type NodeWalker struct {
	current  *Node
	root     *Node
	entering bool
}

func NewNodeWalker(root *Node) *NodeWalker {
	return &NodeWalker{
		current:  root,
		root:     nil,
		entering: true,
	}
}

func (nw *NodeWalker) next() (*Node, bool) {
	if nw.current == nil {
		return nil, false
	}
	if nw.root == nil {
		nw.root = nw.current
		return nw.current, nw.entering
	}
	if nw.entering && nw.current.isContainer() {
		if nw.current.firstChild != nil {
			nw.current = nw.current.firstChild
			nw.entering = true
		} else {
			nw.entering = false
		}
	} else if nw.current.next == nil {
		nw.current = nw.current.parent
		nw.entering = false
	} else {
		nw.current = nw.current.next
		nw.entering = true
	}
	if nw.current == nw.root {
		return nil, false
	}
	return nw.current, nw.entering
}

func (nw *NodeWalker) resumeAt(node *Node, entering bool) {
	nw.current = node
	nw.entering = entering
}

type Parser struct {
	doc    *Node
	tip    *Node // = doc
	oldTip *Node
	//refmap
	lineNumber           uint32
	lastLineLength       uint32
	offset               uint32
	column               uint32
	nextNonspace         uint32
	nextNonspaceColumn   uint32
	lastMatchedContainer *Node // = doc
	currentLine          []byte
	lines                [][]byte // input document.split(newlines)
	indent               uint32
	indented             bool
	blank                bool
	allClosed            bool
	inlineParser         *InlineParser
}

func NewParser() *Parser {
	docNode := NewNode(Document, NewSourceRange())
	return &Parser{
		doc:                  docNode,
		tip:                  docNode,
		oldTip:               docNode,
		lineNumber:           0,
		lastLineLength:       0,
		offset:               0,
		column:               0,
		lastMatchedContainer: docNode,
		currentLine:          []byte{},
		lines:                nil,
		allClosed:            true,
		inlineParser:         NewInlineParser(),
	}
}

type BlockStatus int

const (
	NoMatch = iota
	ContainerMatch
	LeafMatch
)

var blockTriggers = []func(p *Parser, container *Node) BlockStatus{
	atxHeaderTrigger,
	hruleTrigger,
	blockquoteTrigger,
}

func atxHeaderTrigger(p *Parser, container *Node) BlockStatus {
	match := reATXHeaderMarker.Find(p.currentLine[p.nextNonspace:])
	if !p.indented && match != nil {
		p.advanceNextNonspace()
		p.advanceOffset(uint32(len(match)), false)
		p.closeUnmatchedBlocks()
		container := p.addChild(Header, p.nextNonspace)
		container.level = uint32(len(bytes.Trim(match, " \t\n\r"))) // number of #s
		reLeft := regexp.MustCompile("^ *#+ *$")
		reRight := regexp.MustCompile(" +#+ *$")
		container.content = reRight.ReplaceAll(reLeft.ReplaceAll(p.currentLine[p.offset:], []byte{}), []byte{})
		//parser.currentLine.slice(parser.offset).replace(/^ *#+ *$/, '').replace(/ +#+ *$/, '');
		p.advanceOffset(uint32(len(p.currentLine))-p.offset, false)
		return LeafMatch
	}
	return NoMatch
}

func hruleTrigger(p *Parser, container *Node) BlockStatus {
	match := reHrule.Find(p.currentLine[p.nextNonspace:])
	if !p.indented && match != nil {
		p.closeUnmatchedBlocks()
		p.addChild(HorizontalRule, p.nextNonspace)
		p.advanceOffset(uint32(len(p.currentLine))-p.offset, false)
		return LeafMatch
	} else {
		return NoMatch
	}
}

func peek(line []byte, pos uint32) byte {
	if pos < uint32(len(line)) {
		return line[pos]
	}
	return 0
}

func blockquoteTrigger(p *Parser, container *Node) BlockStatus {
	if !p.indented && peek(p.currentLine, p.nextNonspace) == '>' {
		p.advanceNextNonspace()
		p.advanceOffset(1, false)
		if peek(p.currentLine, p.offset) == ' ' {
			p.advanceOffset(1, false)
		}
		p.closeUnmatchedBlocks()
		p.addChild(BlockQuote, p.nextNonspace)
		return ContainerMatch
	} else {
		return NoMatch
	}
}

func (p *Parser) incorporateLine(line []byte) {
	allMatched := true
	container := p.doc
	p.oldTip = p.tip
	p.offset = 0
	p.lineNumber += 1
	p.currentLine = line
	fmt.Printf("%3d: %s\n", p.lineNumber, string(line))
	lastChild := container.lastChild
	for lastChild != nil && lastChild.open {
		container = lastChild
		p.findNextNonspace()
		switch blockHandlers[container.Type].Continue(p, container) {
		case Matched: // matched, keep going
			break
		case NotMatched: // failed to match a block
			allMatched = false
			break
		case Completed: // we've hit end of line for fenced code close and can return
			p.lastLineLength = uint32(len(line))
			return
		default:
			panic("Continue returned illegal value, must be 0, 1, or 2")
		}
		if !allMatched {
			container = container.parent // back up to last matching block
			break
		}
	}
	p.allClosed = container == p.oldTip
	p.lastMatchedContainer = container
	matchedLeaf := container.Type != Paragraph && blockHandlers[container.Type].AcceptsLines()
	for !matchedLeaf {
		p.findNextNonspace()
		//if !p.indented && reMaybeSpecial.Find(line[p.nextNonspace:]) == nil {
		//	p.advanceNextNonspace()
		//	break
		//}
		nothingMatched := true
		for _, trigger := range blockTriggers {
			st := trigger(p, container)
			if st != NoMatch {
				container = p.tip
				nothingMatched = false
				if st == LeafMatch {
					matchedLeaf = true
				}
				break
			}
		}
		if nothingMatched {
			p.advanceNextNonspace()
			break
		}
	}
	if !p.allClosed && !p.blank && p.tip.Type == Paragraph {
		p.addLine()
	} else {
		p.closeUnmatchedBlocks()
		if p.blank && container.lastChild != nil {
			container.lastChild.lastLineBlank = true
		}
		t := container.Type
		lastLineBlank := p.blank /* &&
		!(t == BlockQuote || (t == CodeBlock && container.isFenced) ||
			(t == Item && container.firstChild == nil && container.sourcePos.line == p.lineNumber))
		*/
		cont := container
		for cont != nil {
			cont.lastLineBlank = lastLineBlank
			cont = cont.parent
		}
		if blockHandlers[t].AcceptsLines() {
			p.addLine()
			//if t == HtmlBlock &&
			//	container.htmlBlockType >= 1 &&
			//	container.htmlBlockType <= 5 &&
			//	reHtmlBlockClose() {
			//	p.finalize(container, p.lineNumber)
			//}
		} else if p.offset < uint32(len(line)) && !p.blank {
			container = p.addChild(Paragraph, p.offset)
			p.advanceNextNonspace()
			p.addLine()
		}
	}
	p.lastLineLength = uint32(len(line))
}

func (p *Parser) finalize(block *Node, lineNumber uint32) {
	above := block.parent
	block.open = false
	//block.sourcepos[1] = [lineNumber, this.lastLineLength];
	block.sourcePos.endLine = lineNumber
	block.sourcePos.endChar = p.lastLineLength
	blockHandlers[block.Type].Finalize(p, block)
	p.tip = above
}

func (p *Parser) processInlines(ast *Node) {
	walker := NewNodeWalker(ast)
	for node := ast; node != nil; node, _ = walker.next() {
		if node.Type == Paragraph || node.Type == Header {
			p.inlineParser.parse(node)
		}
	}
}

func (p *Parser) addLine() {
	p.tip.content = append(p.tip.content, p.currentLine[p.offset:]...)
	p.tip.content = append(p.tip.content, '\n')
}

func (p *Parser) addChild(node NodeType, offset uint32) *Node {
	for !blockHandlers[p.tip.Type].CanContain(node) {
		p.finalize(p.tip, p.lineNumber-1)
	}
	column := offset + 1 // offset 0 = column 1
	pos := NewSourceRange()
	pos.line = p.lineNumber
	pos.char = column
	newNode := NewNode(node, pos)
	newNode.content = []byte{}
	p.tip.appendChild(newNode)
	p.tip = newNode
	return newNode
}

func (p *Parser) advanceOffset(count uint32, columns bool) {
	var i uint32 = 0
	var cols uint32 = 0
	for {
		if columns {
			if cols >= count {
				break
			}
		} else {
			if i >= count {
				break
			}
		}
		if p.currentLine[p.offset+i] == '\t' {
			cols += (4 - ((p.column + cols) % 4))
		} else {
			cols += 1
		}
		i += 1
	}
	p.offset += i
	p.column += cols
}

func (p *Parser) advanceNextNonspace() {
	p.offset = p.nextNonspace
	p.column = p.nextNonspaceColumn
}

func (p *Parser) closeUnmatchedBlocks() {
	if !p.allClosed {
		for p.oldTip != p.lastMatchedContainer {
			parent := p.oldTip.parent
			p.finalize(p.oldTip, p.lineNumber-1)
			p.oldTip = parent
		}
		p.allClosed = true
	}
}

func (p *Parser) findNextNonspace() {
	i := p.offset
	cols := p.column
	var c byte
	for i < uint32(len(p.currentLine)) {
		c = p.currentLine[i]
		if c == ' ' {
			i += 1
			cols += 1
		} else if c == '\t' {
			i += 1
			cols += (4 - (cols % 4))
		} else {
			break
		}
	}
	p.blank = c == '\n' || c == '\r' || i == uint32(len(p.currentLine))
	p.nextNonspace = i
	p.nextNonspaceColumn = cols
	p.indent = p.nextNonspaceColumn - p.column
	p.indented = p.indent >= 4
}

func (p *Parser) parse(input []byte) *Node {
	p.lines = bytes.Split(input, []byte{'\n'})
	var numLines uint32 = uint32(len(p.lines))
	if input[len(input)-1] == '\n' {
		// ignore last blank line created by final newline
		numLines -= 1
	}
	var i uint32
	for i = 0; i < numLines; i += 1 {
		p.incorporateLine(p.lines[i])
	}
	for p.tip != nil {
		p.finalize(p.tip, numLines)
	}
	p.processInlines(p.doc)
	return p.doc
}

func forEachNode(root *Node, f func(node *Node, entering bool)) {
	walker := NewNodeWalker(root)
	node, entering := walker.next()
	for node != nil {
		f(node, entering)
		node, entering = walker.next()
	}
}

func dump(ast *Node, depth int) {
	forEachNode(ast, func(node *Node, entering bool) {
		indent := ""
		content := node.literal
		if content == nil {
			content = node.content
		}
		fmt.Printf("%s%s (%q)\n", indent, node.Type, content)
	})
	/*
		walker := NewNodeWalker(ast)
		_, node := walker.next()
		//for node := ast; node != nil; _, node = walker.next() {
		for node != nil {
			indent := ""
			content := node.literal
			if content == nil {
				content = node.content
			}
			fmt.Printf("%s%s (%q)\n", indent, node.Type, content)
			_, node = walker.next()
		}
	*/
	/*
		indent := ""
		for i := 0; i < depth; i += 1 {
			indent += "\t"
		}
		content := ast.literal
		if content == nil {
			content = ast.content
		}
		fmt.Printf("%s%s (%q)\n", indent, ast.Type, content)
		//fmt.Printf("%s%#v\n", indent, ast)
		//fmt.Printf("%s%#v\n", indent, ast.firstChild)
		for n := ast.firstChild; n != nil; n = n.next {
			dump(n, depth+1)
		}
	*/
}

func main() {
	//fmt.Printf("%#v\n", os.Args)
	if len(os.Args) < 2 {
		fmt.Println("usage: go run ast.go file.md")
		return
	}
	bytes, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	p := NewParser()
	ast := p.parse(bytes)
	dump(ast, 0)
	println("================")
	println(string(render(ast)))
}
