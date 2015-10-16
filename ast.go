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
)

type NodeType int

const (
	Document NodeType = iota
	BlockQuote
	List
	Item
	Paragraph
	Header
	Emph
	Strong
	Link
	Image
)

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
	}
}

func (n *Node) unlink() {
	if n.prev != nil {
		n.prev.next = n.next
	} else {
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
	reLeft := regexp.MustCompile("^ *#+ *$")
	reRight := regexp.MustCompile(" +#+ *$")
	match := reATXHeaderMarker.Find(p.currentLine[p.nextNonspace:])
	if !p.indented && match != nil {
		p.advanceNextNonspace()
		p.advanceOffset(uint32(len(match)), false)
		p.closeUnmatchedBlocks()
		container := p.addChild(Header, p.nextNonspace)
		container.level = uint32(len(bytes.Trim(match, " \t\n\r"))) // number of #s
		container.content = reRight.ReplaceAll(reLeft.ReplaceAll(p.currentLine[p.offset:], []byte{}), []byte{})
		//parser.currentLine.slice(parser.offset).replace(/^ *#+ *$/, '').replace(/ +#+ *$/, '');
		p.advanceOffset(uint32(len(p.currentLine))-p.offset, false)
		return LeafMatch
	}
	return NoMatch
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
	fmt.Printf("%3d: %s\n", p.lineNumber, string(line))
	//st := blockStartHeader(p, p.doc)
	//println(st)
}

func (p *Parser) finalize(block *Node, numLines uint32) {
}

func (p *Parser) processInlines(doc *Node) {
}

func (p *Parser) addChild(node NodeType, offset uint32) *Node {
	//while (!this.blocks[this.tip.type].canContain(tag)) {
	//    this.finalize(this.tip, this.lineNumber - 1);
	//}
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
	p.parse(bytes)
}
