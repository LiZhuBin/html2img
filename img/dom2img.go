package img

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"regexp"
	"strconv"
	"strings"

	"github.com/golang/freetype/truetype"
	"github.com/nfnt/resize"
	"github.com/wnote/html2img/dom"
	"golang.org/x/image/font"
	"golang.org/x/image/math/fixed"
)

type DrawCursor struct {
	OffsetX int
	OffsetY int

	FromX int
	EndX  int

	FromY int
	EndY  int

	NeedNewLine bool
}

func BodyDom2Img(bodyDom *dom.Dom) ([]byte, error) {
	bodyWidth := getIntPx(bodyDom.TagStyle.Width, 0)
	bodyHeight := getIntPx(bodyDom.TagStyle.Height, 0)
	dst := image.NewRGBA(image.Rect(0, 0, bodyWidth, bodyHeight))
	bodyDom.CalcWidth = bodyWidth
	bodyDom.CalcHeight = bodyHeight
	if bodyDom.TagStyle.BackgroundColor != "" {
		col := getColor(bodyDom.TagStyle.BackgroundColor)
		draw.Draw(dst, dst.Bounds(), &image.Uniform{C: col}, image.ZP, draw.Src)
	}
	drawCursor := &DrawCursor{}
	DrawChildren(dst, bodyDom, bodyDom.TagStyle, bodyDom.Children, drawCursor)

	buf := &bytes.Buffer{}
	err := jpeg.Encode(buf, dst, &jpeg.Options{
		Quality: 100,
	})
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DrawChildren(dst *image.RGBA, parent *dom.Dom, pStyle *dom.TagStyle, children []*dom.Dom, drawCursor *DrawCursor) {
	for _, d := range children {
		calcStyle := getInheritStyle(pStyle, d.TagStyle)
		width := getIntPx(calcStyle.Width, parent.CalcWidth)
		height := getIntPx(calcStyle.Height, parent.CalcHeight)

		marginTop := getIntPx(calcStyle.MarginTop, parent.CalcHeight)
		marginBottom := getIntPx(calcStyle.MarginBottom, parent.CalcHeight)
		marginLeft := getIntPx(calcStyle.MarginLeft, parent.CalcWidth)
		marginRight := getIntPx(calcStyle.MarginRight, parent.CalcWidth)

		lineHeight := getIntPx(pStyle.LineHeight, parent.CalcHeight)

		drawCursor.OffsetY += marginTop
		drawCursor.OffsetX += marginLeft

		if d.DomType == dom.DOM_TYPE_ELEMENT {
			switch d.TagName {
			case "img":
				imgData := d.TagData.(dom.ImageData)
				img := imgData.Img
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

				draw.Draw(dst, dst.Bounds().Add(image.Pt(drawCursor.OffsetX, drawCursor.OffsetY)), img, image.ZP, draw.Over)
				drawCursor.OffsetY += height + marginBottom
			case "hr":
				drawCursor.OffsetY += height + marginBottom
			case "div":

			case "span":
			default:

			}
			drawCursor.OffsetX = drawCursor.OffsetX + marginLeft
			drawCursor.EndX = drawCursor.EndX - marginRight
			DrawChildren(dst, d, calcStyle, d.Children, drawCursor)
			drawCursor.OffsetY += marginTop
		} else if d.DomType == dom.DOM_TYPE_TEXT {
			f, exist := dom.FontMapping[calcStyle.FontFamily]
			if !exist {
				panic("Font-Family " + calcStyle.FontFamily + " not exist")
			}
			fontSize := getIntPx(calcStyle.FontSize, 0)
			AddText(f, float64(fontSize), 72, dst, image.NewUniform(color.RGBA{R: 0x44, G: 0x44, B: 0x44, A: 0xff}), d.TagData.(string), drawCursor.OffsetX, drawCursor.OffsetY+fontSize)
			lh := lineHeight
			if fontSize > lh {
				lh = fontSize
			}
			if parent.TagName == "span" {
				txtLen := float64(CalStrLen(d.TagData.(string)) * float64(fontSize) / 3)
				drawCursor.OffsetX += int(txtLen) + marginLeft
			} else {
				drawCursor.OffsetY += lh
			}
		} else {
			// Comments or other document type
		}
	}
}

func getIntPx(size string, pSize int) int {
	if size == "" {
		return 0
	}
	re := regexp.MustCompile("\\d+px")
	if re.MatchString(size) {
		ignoreUnitPx := strings.Replace(size, "px", "", 1)
		px, err := strconv.Atoi(ignoreUnitPx)
		if err != nil {
			panic(fmt.Sprintf("size err :%v", size))
		}
		return px
	}
	re = regexp.MustCompile("\\d+%")
	if re.MatchString(size) {
		sizePercent := strings.Replace(size, "%", "", 1)
		percent, err := strconv.Atoi(sizePercent)
		if err != nil {
			panic(fmt.Sprintf("size err :%v", size))
		}
		return percent * pSize / 100
	}
	return 0
}

func getColor(colorStr string) color.Color {
	escapeColor := strings.Replace(colorStr, "#", "", 1)
	if len(escapeColor) < 6 {
		panic(fmt.Sprintf("color err :%v", colorStr))
	}
	r, err := strconv.ParseInt(escapeColor[:2], 16, 32)
	if err != nil {
		panic(fmt.Sprintf("color err :%v", colorStr))
	}
	g, err := strconv.ParseInt(escapeColor[2:4], 16, 32)
	if err != nil {
		panic(fmt.Sprintf("color err :%v", colorStr))
	}
	b, err := strconv.ParseInt(escapeColor[4:6], 16, 32)
	if err != nil {
		panic(fmt.Sprintf("color err :%v", colorStr))
	}
	a := uint8(255)
	if len(escapeColor) == 8 {
		alp, err := strconv.ParseInt(escapeColor[6:8], 16, 32)
		if err != nil {
			panic(fmt.Sprintf("color err :%v", colorStr))
		}
		if alp > 0 {
			a = uint8(a)
		}
	}
	return color.RGBA{
		R: uint8(r),
		G: uint8(g),
		B: uint8(b),
		A: uint8(a),
	}
}

func AddText(f *truetype.Font, size float64, dpi float64, dst *image.RGBA, src *image.Uniform, text string, x int, y int) {
	h := font.HintingNone
	fd := &font.Drawer{
		Dst: dst,
		Src: src,
		Face: truetype.NewFace(f, &truetype.Options{
			Size:    size,
			DPI:     dpi,
			Hinting: h,
		}),
	}

	fd.Dot = fixed.Point26_6{
		X: fixed.I(x),
		Y: fixed.I(y),
	}
	fd.DrawString(text)
}

func getInheritStyle(pStyle *dom.TagStyle, curStyle *dom.TagStyle) *dom.TagStyle {
	if pStyle == nil {
		pStyle = &dom.TagStyle{}
	}
	if curStyle == nil {
		curStyle = &dom.TagStyle{}
	}
	if curStyle.Color == "" && pStyle.Color != "" {
		curStyle.Color = pStyle.Color
	}
	if curStyle.FontSize == "" && pStyle.FontSize != "" {
		curStyle.FontSize = pStyle.FontSize
	}
	if curStyle.LineHeight == "" && pStyle.LineHeight != "" {
		curStyle.LineHeight = pStyle.LineHeight
	}
	if curStyle.FontFamily == "" && pStyle.FontFamily != "" {
		curStyle.FontFamily = pStyle.FontFamily
	}
	return curStyle
}

func CalStrLen(str string) float64 {
	sl := 0.0
	rs := []rune(str)
	for _, r := range rs {
		rint := int(r)
		if rint < 128 {
			sl += 1.7
		} else {
			sl += 3
		}
	}
	return sl
}
