package html2img

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"net/http"
	"sort"
	"strings"

	"github.com/nfnt/resize"
	"golang.org/x/net/html"
)

const (
	DOM_TYPE_TEXT        = 1
	DOM_TYPE_ELEMENT     = 3
	DOM_TYPE_COMMENTNODE = 4
)

type ImageData struct {
	Fm  string
	Img image.Image
}

type EndOffset struct {
	X2 int
	Y2 int
}

type Rectangle struct {
	X1 int
	Y1 int
	X2 int
	Y2 int
}

type Dom struct {
	Outer     Rectangle
	Container Rectangle
	Inwall    Rectangle
	Inner     Rectangle

	DomType  int8
	TagName  string
	TagClass string
	TagData  interface{}

	TagStyle *TagStyle

	Children []*Dom
}

func (d *Dom) IsPositionAbsolute() bool {
	return d.TagStyle.Position == "absolute"
}

func (d *Dom) IsAutoHeight() bool {
	return d.TagStyle.Height == "auto" || d.TagStyle.Height == ""
}

func GetHtmlDom(htmlNode *html.Node, tagStyleList []*TagStyle) *Dom {
	bodyDom := &Dom{}
	SetDomAttr(bodyDom, htmlNode)
	domStyle := GetDomStyle(bodyDom, tagStyleList)
	bodyDom.Container.X1 = 0
	bodyDom.Container.Y1 = 0
	bodyDom.Inner.X1 = 0
	bodyDom.Inner.Y1 = 0
	bodyWidth := GetIntSize(domStyle.Width)
	if bodyWidth == 0 {
		panic("body with is required")
	}
	bodyDom.Container.X2 = bodyWidth
	bodyDom.Inner.X2 = bodyWidth

	if domStyle.Padding.Left != "" {
		bodyDom.Inner.X1 += GetIntSize(domStyle.Padding.Left)
	}
	if domStyle.Padding.Top != "" {
		bodyDom.Inner.Y1 += GetIntSize(domStyle.Padding.Top)
	}
	if domStyle.Padding.Right != "" {
		bodyDom.Inner.X2 -= GetIntSize(domStyle.Padding.Right)
	}

	bodyDom.TagStyle = domStyle
	children, endOffset := GetChildren(htmlNode, tagStyleList, []*Dom{bodyDom})
	bodyDom.Children = children
	bodyDom.Inner.Y2 = endOffset.Y2
	bodyDom.Container.Y2 = endOffset.Y2
	if domStyle.Padding.Bottom != "" {
		bodyDom.Inner.Y2 += GetIntSize(domStyle.Padding.Bottom)
	}
	bodyDom.Outer = bodyDom.Container
	return bodyDom
}

func GetChildren(htmlNode *html.Node, tagStyleList []*TagStyle, parents []*Dom) ([]*Dom, EndOffset) {
	var children []*Dom
	parent := parents[len(parents)-1]
	pX1 := parent.Inner.X1
	pY1 := parent.Inner.Y1
	pX2 := parent.Inner.X2
	pWidth := pX2 - pX1 + 1
	var endOffset EndOffset
CHILDREN:
	for ch := htmlNode.FirstChild; ch != nil; {
		if ch.Type != html.ElementNode && ch.Type != html.TextNode {
			ch = ch.NextSibling
			continue
		}
		// ignore empty text node
		if ch.Type == html.TextNode {
			textData := strings.Trim(ch.Data, CUT_SET_LIST)
			if textData == "" {
				ch = ch.NextSibling
				continue
			}
		}
		dom := &Dom{}
		SetDomAttr(dom, ch)
		domStyle := GetDomStyle(dom, tagStyleList)

		calcStyle := GetInheritStyle(parent.TagStyle, domStyle)
		width := GetIntPx(calcStyle.Width, pWidth)
		height := GetIntSize(calcStyle.Height)

		dom.TagStyle = domStyle

		dom.Outer.X1 = pX1
		dom.Container.X1 = dom.Outer.X1
		dom.Outer.Y1 = pY1
		dom.Container.Y1 = pY1
		if domStyle.Margin.Left != "" {
			dom.Container.X1 += GetIntSize(domStyle.Margin.Left)
		}
		if domStyle.Margin.Top != "" {
			dom.Container.Y1 += GetIntSize(domStyle.Margin.Top)
		}

		dom.Inner.X1 = dom.Container.X1
		dom.Inner.Y1 = dom.Container.Y1
		if domStyle.Padding.Left != "" {
			dom.Inner.X1 += GetIntSize(domStyle.Padding.Left)
		}
		if domStyle.Padding.Top != "" {
			dom.Inner.Y1 += GetIntSize(domStyle.Padding.Top)
		}

		switch ch.Data {
		case "img":
			src := GetAttr(ch, "src")
			resp, err := http.Get(src)
			if err != nil {
				panic(fmt.Sprintf("http.GetImage err :%v", err))
			}
			img, fm, err := image.Decode(resp.Body)
			resp.Body.Close()
			if err != nil {
				panic(fmt.Sprintf("image.Decode err :%v", err))
			}

			srcBounds := img.Bounds()
			if height > 0 || width > 0 {
				if height == 0 {
					height = width * srcBounds.Dy() / srcBounds.Dx()
				}
				if width == 0 {
					width = height * srcBounds.Dx() / srcBounds.Dy()
				}
				img = resize.Resize(uint(width), uint(height), img, resize.Lanczos3)
			}

			dom.Inner.X2 = dom.Inner.X1 + width - 1
			dom.Inner.Y2 = dom.Inner.Y1 + height - 1
			if domStyle.Margin.Right != "" {
				dom.Outer.X2 = dom.Inner.X2 + GetIntSize(domStyle.Margin.Right)
			}
			if domStyle.Margin.Bottom != "" {
				dom.Outer.Y2 = dom.Inner.Y2 + GetIntSize(domStyle.Margin.Bottom)
			}

			imgData := ImageData{
				Fm:  fm,
				Img: img,
			}
			dom.TagData = imgData

			pX1 = parent.Inner.X1
			endOffset.Y2 = dom.Inner.Y2

			dom.Outer = dom.Inner
			dom.Container = dom.Inner
			pY1 = dom.Outer.Y2 + 1
		case "span":
			par := append(parents, dom)
			var child []*Dom
			child, endOffset = GetChildren(ch, tagStyleList, par)
			dom.Children = child
			dom.Inner.Y2 = endOffset.Y2
			dom.Inner.X2 = endOffset.X2
			dom.Container.X2 = dom.Inner.X2
			if domStyle.Padding.Right != "" {
				dom.Container.X2 += GetIntSize(domStyle.Padding.Right)
			}

			dom.Outer.X2 = dom.Container.X2
			if domStyle.Margin.Right != "" {
				dom.Outer.X2 += GetIntSize(domStyle.Margin.Right)
			}

			dom.Container.Y2 = endOffset.Y2
			if domStyle.Padding.Bottom != "" {
				dom.Container.Y2 += GetIntSize(domStyle.Padding.Bottom)
			}
			dom.Outer.Y2 = dom.Container.Y2
			if domStyle.Margin.Bottom != "" {
				dom.Outer.Y2 += GetIntSize(domStyle.Margin.Bottom)
			}
			pX1 = dom.Outer.X2
			endOffset.Y2 = dom.Outer.Y2
		default:
			if ch.Type == html.TextNode {
				fontSize := GetIntSize(domStyle.FontSize)
				lineHeight := GetIntSize(domStyle.LineHeight)
				if fontSize > lineHeight {
					lineHeight = fontSize
				}
				var bParent *Dom
				for i := len(parents) - 1; i >= 0; i-- {
					p := parents[i]
					if p.Inner.X2 > 0 {
						bParent = p
						break
					}
				}
				multiTexts := SplitMultiLineText(ch.Data, float64(fontSize), dom.Inner.X1, bParent.Inner.X2, bParent.Inner.X1)
				maxX2 := 0
				maxY2 := dom.Inner.Y1
				for idx, text := range multiTexts {
					newDom := *dom
					if idx > 0 {
						newDom.Inner.X1 = bParent.Inner.X1
					}
					charWidth := CalcCharacterPx(text, float64(fontSize))
					newDom.Inner.X2 = newDom.Inner.X1 + int(charWidth)
					newDom.TagData = text
					newDom.Inner.Y1 = maxY2
					newDom.Inner.Y2 = maxY2 + lineHeight
					newDom.Container = newDom.Inner
					newDom.Outer = newDom.Inner
					if maxX2 < newDom.Outer.X2 {
						maxX2 = newDom.Outer.X2
					}
					maxY2 = newDom.Outer.Y2 + 1
					children = append(children, &newDom)
				}
				pX1 = maxX2
				endOffset.Y2 = maxY2
				endOffset.X2 = maxX2
				ch = ch.NextSibling
				continue CHILDREN
			} else {
				if dom.IsPositionAbsolute() {
					left := GetIntSize(domStyle.Offset.Left)
					top := GetIntSize(domStyle.Offset.Top)
					width := GetIntSize(domStyle.Width)
					height := GetIntSize(domStyle.Height)
					dom.Outer.X1 = left
					dom.Outer.X2 = left + width
					dom.Outer.Y1 = top
					dom.Outer.Y2 = top + height

					dom.Container = dom.Outer
					dom.Inner = dom.Outer
					break
				}
				if width > 0 {
					dom.Container.X2 = dom.Container.X1 + width
					dom.Outer.X2 = dom.Container.X2
				} else {
					dom.Outer.X2 = pX2
					dom.Container.X2 = pX2
				}
				if domStyle.Margin.Right != "" {
					dom.Container.X2 = pX2 - GetIntSize(domStyle.Margin.Right)
				}
				dom.Inner.X2 = dom.Container.X2
				if domStyle.Padding.Right != "" {
					dom.Inner.X2 -= GetIntSize(domStyle.Padding.Right)
				}
				par := append(parents, dom)
				var child []*Dom
				child, endOffset = GetChildren(ch, tagStyleList, par)
				dom.Children = child
				if len(child) != 0 {
					dom.Inner.Y2 = endOffset.Y2
					dom.Container.Y2 = endOffset.Y2
				} else {
					dom.Inner.Y2 = dom.Inner.Y1
					if domStyle.Height != "" {
						dom.Inner.Y2 += GetIntSize(domStyle.Height) - 1
					}
					dom.Container.Y2 = dom.Inner.Y2
				}

				if domStyle.Padding.Bottom != "" {
					dom.Container.Y2 += GetIntSize(domStyle.Padding.Bottom)
				}
				dom.Outer.Y2 = dom.Container.Y2
				if domStyle.Margin.Bottom != "" {
					dom.Outer.Y2 += GetIntSize(domStyle.Margin.Bottom)
				}

				endOffset.Y2 = dom.Outer.Y2
				endOffset.X2 = dom.Outer.X2
				pY1 = dom.Outer.Y2 + 1
			}
		}
		children = append(children, dom)
		ch = ch.NextSibling
	}
	return children, endOffset
}

func GetDomStyle(dom *Dom, tagStyleList []*TagStyle) *TagStyle {
	var selectedStyle []*TagStyle
	for _, style := range tagStyleList {
		if style.Selected(dom) {
			selectedStyle = append(selectedStyle, style)
		}
	}
	finalStyle := &TagStyle{}
	if len(selectedStyle) > 0 {
		// TODO Improved selector priority
		sort.SliceStable(selectedStyle, func(i, j int) bool {
			if len(selectedStyle[i].Selector) < len(selectedStyle[j].Selector) {
				return true
			}
			return false
		})
		for _, style := range selectedStyle {
			if style.Selector != "" {
				finalStyle.Selector = style.Selector
			}
			if style.Color != "" {
				finalStyle.Color = style.Color
			}
			if style.FontSize != "" {
				finalStyle.FontSize = style.FontSize
			}
			if style.LineHeight != "" {
				finalStyle.LineHeight = style.LineHeight
			}
			if style.FontFamily != "" {
				finalStyle.FontFamily = style.FontFamily
			}
			if style.BackgroundColor != "" {
				finalStyle.BackgroundColor = style.BackgroundColor
			}
			if style.BackgroundImage != "" {
				finalStyle.BackgroundImage = style.BackgroundImage
			}
			if style.Width != "" {
				finalStyle.Width = style.Width
			}
			if style.Height != "" {
				finalStyle.Height = style.Height
			}
			if style.Display != "" {
				finalStyle.Display = style.Display
			}
			if style.Position != "" {
				finalStyle.Position = style.Position
			}

			finalStyle.Offset = getSelectedPos(finalStyle.Offset, style.Offset)
			finalStyle.Margin = getSelectedPos(finalStyle.Margin, style.Margin)
			finalStyle.Padding = getSelectedPos(finalStyle.Padding, style.Padding)
			finalStyle.BorderRadius = getSelectedPos(finalStyle.BorderRadius, style.BorderRadius)
			finalStyle.BorderWidth = getSelectedPos(finalStyle.BorderWidth, style.BorderWidth)
			finalStyle.BorderColor = getSelectedPos(finalStyle.BorderColor, style.BorderColor)
			finalStyle.BorderStyle = getSelectedPos(finalStyle.BorderStyle, style.BorderStyle)
		}
	}

	return finalStyle
}

func SetDomAttr(dom *Dom, htmlNode *html.Node) {
	dom.DomType = int8(htmlNode.Type)
	if htmlNode.Type == html.ElementNode {
		dom.DomType = DOM_TYPE_ELEMENT
		dom.TagName = htmlNode.Data
		dom.TagClass = GetAttr(htmlNode, "class")
	} else if htmlNode.Type == html.TextNode {
		dom.DomType = DOM_TYPE_TEXT
		dom.TagData = htmlNode.Data
	}
}

func GetAttr(htmlNode *html.Node, attrKey string) string {
	for _, attr := range htmlNode.Attr {
		if attr.Key == attrKey {
			return attr.Val
		}
	}
	return ""
}

func getSelectedPos(oldPos Pos, selectedPos Pos) Pos {
	if selectedPos.Left != "" {
		oldPos.Left = selectedPos.Left
	}
	if selectedPos.Right != "" {
		oldPos.Right = selectedPos.Right
	}
	if selectedPos.Top != "" {
		oldPos.Top = selectedPos.Top
	}
	if selectedPos.Bottom != "" {
		oldPos.Bottom = selectedPos.Bottom
	}
	return oldPos
}