package main

import (
	"bytes"
	"regexp"
)

var (
	reMain = regexp.MustCompile("^[^\\n`\\[\\]\\!<&*_'\"]+")
)

type InlineParser struct {
	subject []byte
	pos     int
}

func NewInlineParser() *InlineParser {
	return &InlineParser{
		subject: []byte{},
		pos:     0,
	}
}

func text(s []byte) *Node {
	node := NewNode(Text, NewSourceRange())
	node.literal = s
	return node
}

func (p *InlineParser) peek() byte {
	if p.pos < len(p.subject) {
		return p.subject[p.pos]
	}
	return 255 // XXX: figure out invalid values
}

func (p *InlineParser) scanDelims(ch byte) (numDelims int, canOpen, canClose bool) {
	numDelims = 0
	startPos := p.pos
	if ch == '\'' || ch == '"' {
		numDelims += 1
		p.pos += 1
	} else {
		for p.peek() == ch {
			numDelims += 1
			p.pos += 1
		}
	}
	p.pos = startPos
	return numDelims, false, false
}

func (p *InlineParser) handleDelim(ch byte, block *Node) bool {
	numDelims, _, _ := p.scanDelims(ch)
	if numDelims < 1 {
		return false
	}
	startPos := p.pos
	println("startPos = ", startPos)
	p.pos += numDelims
	var contents []byte
	if ch == '\'' || ch == '"' {
		contents = []byte{ch}
	} else {
		contents = p.subject[startPos:p.pos]
		println("--- ", string(contents))
	}
	node := text(contents)
	block.appendChild(node)
	// TODO: add entry to stack
	return true
}

func (p *InlineParser) parseString(block *Node) bool {
	match := reMain.Find(p.subject[p.pos:])
	if match == nil {
		return false
	}
	p.pos += len(match)
	block.appendChild(text(match))
	return true
}

func (p *InlineParser) parseInline(block *Node) bool {
	res := false
	ch := p.peek()
	if ch == 255 { // XXX: invalid char
		return false
	}
	switch ch {
	case '*', '_':
		res = p.handleDelim(ch, block)
		break
	default:
		res = p.parseString(block)
		break
	}
	if !res {
		p.pos += 1
		block.appendChild(text([]byte{ch}))
	}
	return true
}

func (p *InlineParser) processEmphasis(stackBottom *Node) {
	// TODO
}

func (p *InlineParser) parse(block *Node) {
	p.subject = bytes.Trim(block.content, " \n\r")
	p.pos = 0
	for p.parseInline(block) {
	}
	block.content = nil // allow raw string to be garbage collected
	p.processEmphasis(nil)
}
