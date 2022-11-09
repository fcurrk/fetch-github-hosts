package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/theme"
	"image/color"
)

type fghGuiTheme struct {
}

func (f *fghGuiTheme) Font(s fyne.TextStyle) fyne.Resource {
	font, err := assetsFs.ReadFile("assets/zcool-cryyt.ttf")
	if err != nil {
		return theme.DefaultTheme().Font(s)
	}
	return fyne.NewStaticResource("fgh-font", font)
}

func (*fghGuiTheme) Color(c fyne.ThemeColorName, _ fyne.ThemeVariant) color.Color {
//	return theme.DefaultTheme().Color(c, theme.VariantDark)
	return theme.DefaultTheme().Color(c, theme.VariantLight)
}

func (*fghGuiTheme) Icon(n fyne.ThemeIconName) fyne.Resource {
	return theme.DefaultTheme().Icon(n)
}

func (*fghGuiTheme) Size(n fyne.ThemeSizeName) float32 {
	return theme.DefaultTheme().Size(n)
}
