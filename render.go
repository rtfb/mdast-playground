package main

import (
	"bytes"
	"fmt"
)

func tag(name string, attrs []string, selfClosing bool) []byte {
	result := "<" + name
	if attrs != nil && len(attrs) > 0 {
		// TODO
	}
	if selfClosing {
		result += " /"
	}
	return []byte(result + ">")
}

func render(ast *Node) []byte {
	var buff bytes.Buffer
	var lastOutput []byte
	out := func(text []byte) {
		buff.Write(text)
		lastOutput = text
	}
	esc := func(text []byte, preserveEntities bool) []byte {
		// XXX: impl
		return text
	}
	cr := func() {
		if !bytes.Equal(lastOutput, []byte("\n")) {
			buff.WriteString("\n")
			lastOutput = []byte("\n")
		}
	}
	forEachNode(ast, func(node *Node, entering bool) {
		attrs := []string{}
		switch node.Type {
		case Text:
			out(esc(node.literal, false))
			break
		case Emph:
			if entering {
				out(tag("em", nil, false))
			} else {
				out(tag("/em", nil, false))
			}
			break
		case Strong:
			if entering {
				out(tag("strong", nil, false))
			} else {
				out(tag("/strong", nil, false))
			}
			break
		case Document:
			break
		case Paragraph:
			/*
			   grandparent = node.parent.parent;
			   if (grandparent !== null &&
			       grandparent.type === 'List') {
			       if (grandparent.listTight) {
			           break;
			       }
			   }
			*/
			if entering {
				cr()
				out(tag("p", attrs, false))
			} else {
				out(tag("/p", attrs, false))
				cr()
			}
			break
		case BlockQuote:
			if entering {
				cr()
				out(tag("blockquote", attrs, false))
				cr()
			} else {
				cr()
				out(tag("/blockquote", nil, false))
				cr()
			}
			break
		case Header:
			tagname := fmt.Sprintf("h%d", node.level)
			if entering {
				cr()
				out(tag(tagname, attrs, false))
			} else {
				out(tag("/"+tagname, nil, false))
				cr()
			}
			break
		case HorizontalRule:
			cr()
			out(tag("hr", attrs, true))
			cr()
			break
		default:
			panic("Unknown node type " + node.Type.String())
		}
	})
	return buff.Bytes()
}
