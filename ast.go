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
}

func (t NodeType) String() string {
	return nodeTypeNames[t]
}

var blockHandlers = map[NodeType]BlockHandler{
	Document:       &DocumentBlockHandler{},
	Header:         &HeaderBlockHandler{},
	HorizontalRule: &HorizontalRuleBlockHandler{},
}

type BlockHandler interface {
	Continue(p *Parser) bool
	Finalize(p *Parser, block *Node)
	CanContain(t NodeType) bool
	AcceptsLines() bool
}

type HeaderBlockHandler struct {
}

func (h *HeaderBlockHandler) Continue(p *Parser) bool {
	// a header can never contain > 1 line, so fail to match:
	return true
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

func (h *DocumentBlockHandler) Continue(p *Parser) bool {
	return false
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

func (h *HorizontalRuleBlockHandler) Continue(p *Parser) bool {
	// an hrule can never container > 1 line, so fail to match:
	return true
}

func (h *HorizontalRuleBlockHandler) Finalize(p *Parser, block *Node) {
}

func (h *HorizontalRuleBlockHandler) CanContain(t NodeType) bool {
	return false
}

func (h *HorizontalRuleBlockHandler) AcceptsLines() bool {
	return false
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
	// ...
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

type Parser struct {
	doc *Node
	tip *Node // = doc
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
}

type BlockStatus int

const (
	NoMatch = iota
	ContainerMatch
	LeafMatch
)

func blockStartHeader(p *Parser, container *Node) BlockStatus {
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

func blockStartHrule(p *Parser, container *Node) BlockStatus {
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

func NewParser() *Parser {
	docNode := NewNode(Document, NewSourceRange())
	return &Parser{
		doc:                  docNode,
		tip:                  docNode,
		lineNumber:           0,
		lastLineLength:       0,
		offset:               0,
		column:               0,
		lastMatchedContainer: docNode,
		currentLine:          []byte{},
		lines:                nil,
	}
}

func (p *Parser) incorporateLine(line []byte) {
	p.lineNumber += 1
	p.currentLine = line
	fmt.Printf("%3d: %s\n", p.lineNumber, string(line))
	st := blockStartHeader(p, p.doc)
	if st == NoMatch {
		st = blockStartHrule(p, p.doc)
	}
	//println(st)
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

func (p *Parser) processInlines(doc *Node) {
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
			if cols < count {
				break
			}
		} else {
			if i < count {
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
	// TODO
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
	//for p.tip != nil {
	//	p.finalize(p.tip, numLines)
	//}
	p.processInlines(p.doc)
	return p.doc
}

func dump(ast *Node, depth int) {
	indent := ""
	for i := 0; i < depth; i += 1 {
		indent += "\t"
	}
	fmt.Printf("%s%s\n", indent, ast.Type)
	//fmt.Printf("%s%#v\n", indent, ast)
	//fmt.Printf("%s%#v\n", indent, ast.firstChild)
	for n := ast.firstChild; n != nil; n = n.next {
		dump(n, depth+1)
	}
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
}
