package conf

import (
	"os"
)

var GConf = map[string]string{
	"font_path": "",
}

var DPI = float64(72)

func init() {
	setDefaultFontsPath()
}

// SetFontsPath 自定义字体路径
func SetFontsPath(fontPath string){
	GConf["font_path"] = fontPath
}

func setDefaultFontsPath()  {
	goPath, _ := os.LookupEnv("GOPATH")
	SetFontsPath(goPath + "/src/github.com/LiZhuBin/html2img/conf/fonts/")
}