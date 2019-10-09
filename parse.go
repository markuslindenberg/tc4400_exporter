package main

import (
	"bytes"
	"io"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

func parseTables(r io.ReadCloser) (tables [][][]string, err error) {
	doc, err := html.Parse(r)
	if err != nil {
		return nil, err
	}

	tables = [][][]string{}
	n := doc
	for {
		if n.Type == html.ElementNode && n.DataAtom == atom.Table {
			tables = append(tables, parseTable(n))
		} else if n.FirstChild != nil {
			n = n.FirstChild
			continue
		}

		if n.NextSibling != nil {
			n = n.NextSibling
		} else if n.Parent != doc && n.Parent.NextSibling != nil {
			n = n.Parent.NextSibling
		} else {
			break
		}
	}

	return tables, nil
}

func parseTable(tableNode *html.Node) (table [][]string) {
	table = [][]string{}

	var contentBuffer bytes.Buffer
	bodyNode := tableNode.FirstChild
	for {
		if bodyNode == nil {
			break
		}
		if bodyNode.Type == html.ElementNode && (bodyNode.DataAtom == atom.Thead || bodyNode.DataAtom == atom.Tbody) {
			rowNode := bodyNode.FirstChild
			for {
				if rowNode == nil {
					break
				}
				if rowNode.Type == html.ElementNode && rowNode.DataAtom == atom.Tr {
					row := []string{}
					cellNode := rowNode.FirstChild
					for {
						if cellNode == nil {
							break
						}
						if cellNode.Type == html.ElementNode && (cellNode.DataAtom == atom.Th || cellNode.DataAtom == atom.Td) {
							contentBuffer.Reset()
							contentNode := cellNode.FirstChild
							for {
								if contentNode == nil {
									break
								}
								if contentNode.Type == html.TextNode {
									contentBuffer.WriteString(contentNode.Data)
								} else if contentNode.FirstChild != nil {
									contentNode = contentNode.FirstChild
									continue
								}

								if contentNode.NextSibling != nil {
									contentNode = contentNode.NextSibling
								} else if contentNode.Parent != cellNode {
									contentNode = contentNode.Parent.NextSibling
								} else {
									contentNode = nil
								}
							}
							row = append(row, strings.TrimSpace(contentBuffer.String()))
						}
						cellNode = cellNode.NextSibling
					}
					table = append(table, row)
				}
				rowNode = rowNode.NextSibling
			}
		}
		bodyNode = bodyNode.NextSibling
	}
	return table
}
