package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
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
	prev       *Node
	next       *Node
	sourcePos  *SourceRange
	content    []byte
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

type Parser struct {
	doc *Node
	tip *Node // = doc
	//refmap
	lineNumber           uint32
	lastLineLength       uint32
	offset               uint32
	column               uint32
	lastMatchedContainer *Node // = doc
	currentLine          []byte
	lines                [][]byte // input document.split(newlines)
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
	fmt.Println(string(line))
}

func (p *Parser) finalize(block *Node, numLines uint32) {
}

func (p *Parser) processInlines(doc *Node) {
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
	fmt.Printf("%#v\n", os.Args)
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
