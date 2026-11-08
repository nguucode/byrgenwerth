//go:build windows

package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/lxn/walk"
)

// refreshKeywordsUI shows the profile's keywords (or the generic list).
func (a *App) refreshKeywordsUI() {
	if a.cur == nil || a.kwLE == nil {
		return
	}
	a.suppress = true
	a.kwLE.SetText(strings.Join(EffectiveKeywords(a.cur, a.cfg.Keywords), ", "))
	a.suppress = false
}

// ---------- label next to the count ----------

var labelModes = []string{LabelModeText, LabelModeImage, LabelModeNone}

// initLabelUI fills the label controls from the config and loads the picture.
func (a *App) initLabelUI() {
	a.suppress = true
	for i, m := range labelModes {
		if m == a.cfg.LabelMode {
			a.labelModeCB.SetCurrentIndex(i)
		}
	}
	a.suppress = false
	a.loadLabelImage()
	a.refreshLabelControls()
}

func (a *App) labelImagePath() string {
	if a.cfg.LabelImage == "" {
		return ""
	}
	return filepath.Join(a.store.Dir, a.cfg.LabelImage)
}

// loadLabelImage reads the chosen picture at the label size for the overlay,
// and hands the file to the OBS page.
func (a *App) loadLabelImage() {
	a.labelImg = nil
	path := a.labelImagePath()
	a.server.SetLabelImage(path)
	if path == "" {
		a.labelImgLE.SetText("Chưa chọn")
		return
	}
	img, err := LoadLabelImage(path, a.cfg.LabelSize)
	if err != nil {
		a.labelImgLE.SetText("Không mở được hình")
		a.setStatus("Không mở được hình nhãn: " + err.Error())
		return
	}
	a.labelImg = img
	a.labelImgLE.SetText(a.cfg.LabelImage)
}

func (a *App) refreshLabelControls() {
	mode := a.cfg.LabelMode
	a.labelTextLE.SetEnabled(mode == LabelModeText)
	a.labelSizeNE.SetEnabled(mode != LabelModeNone)
	a.cornerCB.SetEnabled(mode != LabelModeNone)
}

func (a *App) onLabelModeChanged() {
	if a.suppress || a.labelModeCB == nil {
		return
	}
	i := a.labelModeCB.CurrentIndex()
	if i < 0 || i >= len(labelModes) {
		return
	}
	mode := labelModes[i]
	if mode == LabelModeImage && a.labelImg == nil {
		if !a.chooseLabelImage() {
			a.suppress = true
			for j, m := range labelModes {
				if m == a.cfg.LabelMode {
					a.labelModeCB.SetCurrentIndex(j)
				}
			}
			a.suppress = false
			return
		}
	}
	a.cfg.LabelMode = mode
	a.saveConfig()
	a.refreshLabelControls()
	a.refreshCount()
}

func (a *App) onLabelTextChanged() {
	if a.suppress || a.labelTextLE == nil {
		return
	}
	a.cfg.LabelText = a.labelTextLE.Text()
	a.saveConfig()
	a.refreshCount()
}

func (a *App) onLabelSizeChanged() {
	if a.suppress || a.labelSizeNE == nil || a.labelImgLE == nil {
		return
	}
	a.cfg.LabelSize = int(a.labelSizeNE.Value())
	a.saveConfig()
	if a.cfg.LabelImage != "" {
		a.loadLabelImage()
	}
	a.refreshCount()
}

func (a *App) onCornerChanged() {
	if a.suppress || a.cornerCB == nil {
		return
	}
	if i := a.cornerCB.CurrentIndex(); i >= 0 && i < len(LabelCorners) {
		a.cfg.LabelCorner = LabelCorners[i]
		a.saveConfig()
		a.refreshCount()
	}
}

// chooseLabelImage copies a PNG or SVG into the data folder as the label
// picture and switches the label to it. Returns false when nothing was chosen.
func (a *App) chooseLabelImage() bool {
	dlg := walk.FileDialog{Title: "Chọn hình nhãn", Filter: "Hình (*.png;*.svg)|*.png;*.svg"}
	ok, err := dlg.ShowOpen(a.mw)
	if err != nil || !ok {
		return false
	}
	ext := strings.ToLower(filepath.Ext(dlg.FilePath))
	if ext != ".png" && ext != ".svg" {
		walk.MsgBox(a.mw, "Hình nhãn", "Chỉ nhận file .png hoặc .svg.", walk.MsgBoxIconWarning)
		return false
	}
	if _, err := LoadLabelImage(dlg.FilePath, a.cfg.LabelSize); err != nil {
		walk.MsgBox(a.mw, "Hình nhãn", "Không đọc được hình này: "+err.Error(), walk.MsgBoxIconWarning)
		return false
	}
	b, err := os.ReadFile(dlg.FilePath)
	if err != nil {
		a.setStatus("Không đọc được file: " + err.Error())
		return false
	}
	name := "label" + ext
	if err := writeAtomic(filepath.Join(a.store.Dir, name), b); err != nil {
		a.setStatus("Không lưu được hình: " + err.Error())
		return false
	}
	for _, e := range LabelImageExts {
		if e != ext {
			os.Remove(filepath.Join(a.store.Dir, "label"+e))
		}
	}
	a.cfg.LabelImage = name
	a.cfg.LabelMode = LabelModeImage
	a.labelImgVer++
	a.saveConfig()
	a.loadLabelImage()
	a.suppress = true
	a.labelModeCB.SetCurrentIndex(1)
	a.suppress = false
	a.refreshLabelControls()
	a.refreshCount()
	a.setStatus("Đã đặt hình nhãn. OBS tự cập nhật.")
	return true
}
