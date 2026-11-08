//go:build windows

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

//go:embed icon.png
var iconPNG []byte

type App struct {
	Engine

	subtitle *Overlay
	profiles []*Profile

	tmplImgs []image.Image

	migrated     bool
	dirWarn      string
	uninstalling bool
	lastFg       win.HWND // foreground window last seen by autoHideForGame

	navDeath     *walk.PushButton
	navTranslate *walk.PushButton
	navNormal    *walk.Font
	navBold      *walk.Font
	deathPage    *walk.Composite
	transPage    *walk.Composite
	trOnCB       *walk.CheckBox
	trRegionLbl  *walk.Label
	trKeyLE      *walk.LineEdit
	trFontNE     *walk.NumberEdit
	trSrcLbl     *walk.TextLabel
	trDstLbl     *walk.TextLabel
	trCostLbl    *walk.Label

	hk       *HotkeyManager
	hkErrors []string
	suppress bool

	mw         *walk.MainWindow
	profileCB  *walk.ComboBox
	countLbl   *walk.Label
	autoCB     *walk.CheckBox
	simLbl     *walk.Label
	thrNE      *walk.NumberEdit
	cooldownNE *walk.NumberEdit
	monCB      *walk.ComboBox
	roiLbl     *walk.Label
	tmplLB     *walk.ListBox
	tmplIV     *walk.ImageView
	tmplInfo   *walk.Label
	topCB      *walk.CheckBox
	soundCB    *walk.CheckBox
	obsCB      *walk.CheckBox
	overlayCB  *walk.CheckBox
	fontNE     *walk.NumberEdit
	fontCB     *walk.ComboBox

	labelModeCB *walk.ComboBox
	labelTextLE *walk.LineEdit
	labelImgLE  *walk.LineEdit
	labelSizeNE *walk.NumberEdit
	cornerCB    *walk.ComboBox
	labelImg    *image.RGBA // the picture at its current size, nil when none
	labelImgVer int         // bumped on each new picture so OBS reloads it

	narr narration

	langCB    *walk.ComboBox
	ocrLangs  []OcrLang
	boldCB    *walk.CheckBox
	colorPB   *walk.PushButton
	outlineCB *walk.CheckBox
	outlineNE *walk.NumberEdit
	outlinePB *walk.PushButton
	urlLE     *walk.LineEdit
	server    *OverlayServer
	editPosPB *walk.PushButton
	volNE     *walk.NumberEdit
	soundLE   *walk.LineEdit
	ocrCB     *walk.CheckBox
	ocrLbl    *walk.Label
	kwLE      *walk.LineEdit

	overlay    *Overlay
	sound      Sound
	statusLbl  *walk.Label
	hkLbl      *walk.TextLabel
	previewBmp *walk.Bitmap
}

func roamingDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if h, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(h, "AppData", "Roaming")
		}
	}
	return base
}

// appDataDir is only a fallback when the app folder is not writable.
func appDataDir() string { return filepath.Join(roamingDir(), "Byrgenwerth") }

// legacyAppDataDir is where versions before 0.9 kept data.
func legacyAppDataDir() string { return filepath.Join(roamingDir(), "DeathCounter") }

// dataDir is "ByrgenwerthData" next to the exe, so the app folder holds
// everything. Data from older versions is taken over once.
func dataDir() (dir string, migrated bool, warn string) {
	appData := appDataDir()
	dir = appData
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		migrated = AdoptLegacyDataDir(exeDir)
		dir = filepath.Join(exeDir, dataDirName)
		// The rename failed, perhaps a locked file: keep using the old folder for now.
		if _, err := os.Stat(dir); err != nil {
			if old := filepath.Join(exeDir, legacyDataDirName); DirHasData(old) {
				dir = old
			}
		}
	}
	if err := CheckWritable(dir); err != nil {
		warn = "Không ghi được dữ liệu vào " + dir + " (" + err.Error() + "). Đang tạm lưu ở " + appData +
			". Hãy cài app vào thư mục bạn có quyền ghi (ví dụ không phải C:\\Program Files)."
		os.MkdirAll(appData, 0o755)
		dir = appData
	}
	// appData first: a fallback copy from a folder that was not writable is newest.
	for _, src := range append([]string{appData}, LegacyDataDirs(legacyAppDataDir())...) {
		if src == dir {
			continue
		}
		if ok, _ := MigrateLegacyData(src, dir); ok {
			return dir, true, warn
		}
	}
	return dir, migrated, warn
}

func main() {
	for _, arg := range os.Args[1:] {
		if arg == "--uninstall" { // from Windows "Apps & features"
			runUninstall(nil)
			return
		}
	}
	dir, migrated, warn := dataDir()
	a := &App{Engine: Engine{store: &Store{Dir: dir}, quit: make(chan struct{}), ocrAvailable: true}, migrated: migrated, dirWarn: warn}
	a.ui = a
	a.cfg = a.store.LoadConfig()
	a.monitors = ListMonitors()
	if a.cfg.Monitor < 0 || a.cfg.Monitor >= len(a.monitors) {
		a.cfg.Monitor = 0
	}
	if err := a.buildUI(); err != nil {
		walk.MsgBox(nil, "Byrgenwerth", "Lỗi tạo giao diện: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.init()
	a.mw.Run()
	if a.uninstalling {
		finishUninstall()
	}
}

var (
	uiFont    = Font{Family: "Segoe UI", PointSize: 9}
	titleFont = Font{Family: "Segoe UI Semibold", PointSize: 9}
	hintColor = walk.RGB(110, 110, 110)
)

// Every input shares one size and every button another, so the columns line up.
const (
	inputW  = 150
	buttonW = 80
)

var (
	inSize = Size{Width: inputW}
	// Heights stay native: a box (HBox) sizes rows from native heights, so a
	// taller forced button gets its frame cut.
	btnSize = Size{Width: buttonW}
	labelSz = Size{Width: 64} // first column of every form
)

// sized gives inputs and buttons their shared size.
func sized(w Widget) Widget {
	switch c := w.(type) {
	case PushButton:
		c.MinSize, c.MaxSize = btnSize, btnSize
		return c
	case LineEdit:
		c.MinSize, c.MaxSize = inSize, inSize
		return c
	case ComboBox:
		c.MinSize, c.MaxSize = inSize, inSize
		return c
	case NumberEdit:
		c.MinSize, c.MaxSize = inSize, inSize
		return c
	}
	return w
}

// title is a bold section heading.
func title(text string) Widget {
	return Label{Text: text, Font: titleFont}
}

// hint is a small gray helper line. TextLabel with a MinSize width wraps
// instead of widening the window.
func hint(text string) Widget {
	return TextLabel{Text: text, TextColor: hintColor, MinSize: Size{Width: 100}}
}

// sep is a thin horizontal rule. walk's HSeparator is greedy vertically
// and eats all free height, so it is not used.
func sep() Widget {
	return Composite{
		MinSize:    Size{Height: 1},
		MaxSize:    Size{Height: 1},
		Background: SolidColorBrush{Color: walk.RGB(220, 220, 220)},
	}
}

// form lays out label/control pairs in two aligned columns, left-aligned.
func form(cells ...Widget) Widget {
	for i := 0; i < len(cells); i += 2 {
		if l, ok := cells[i].(Label); ok {
			l.MinSize = labelSz
			cells[i] = l
		}
		// A grid stretches its cells to the column width and ignores MaxSize:
		// wrapped in a row, a box keeps the shared input width.
		if i+1 < len(cells) {
			if _, ok := cells[i+1].(Composite); !ok {
				cells[i+1] = row(cells[i+1])
			}
		}
	}
	return Composite{Layout: Grid{Columns: 2, MarginsZero: true, Spacing: 4, Alignment: AlignHNearVCenter}, Children: cells}
}

// row puts several controls side by side, packed to the left.
func row(children ...Widget) Widget {
	for i, c := range children {
		children[i] = sized(c)
	}
	return Composite{Layout: HBox{MarginsZero: true, Spacing: 4, Alignment: AlignHNearVCenter}, Children: append(children, HSpacer{})}
}

// pageLayout is the layout of every tab page.
func pageLayout() Layout {
	return VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 8}, Spacing: 4, Alignment: AlignHNearVNear}
}

func (a *App) buildUI() error {
	monNames := make([]string, len(a.monitors))
	for i, m := range a.monitors {
		monNames[i] = m.Label(i)
	}
	cornerNames := make([]string, len(LabelCorners))
	for i, c := range LabelCorners {
		cornerNames[i] = CornerName(c)
	}

	// Death count: what to look for on screen.
	detectTab := TabPage{
		Title:  "Nhận diện",
		Layout: pageLayout(),
		Children: []Widget{
			row(
				CheckBox{AssignTo: &a.autoCB, Text: "Tự nhận diện", OnCheckedChanged: a.onAutoChanged,
					ToolTipText: "Tự đếm khi màn hình chết hiện ra (" + a.cfg.Hotkeys.Toggle + " bật/tắt)."},
				CheckBox{AssignTo: &a.ocrCB, Text: "Đọc chữ (OCR)", OnCheckedChanged: a.onOcrChanged,
					ToolTipText: "Đọc chữ trên màn hình bằng OCR của Windows, không cần ảnh mẫu."},
			),
			form(
				Label{Text: "Màn hình"}, ComboBox{AssignTo: &a.monCB, Model: monNames, CurrentIndex: a.cfg.Monitor, MinSize: inSize, MaxSize: inSize, OnCurrentIndexChanged: a.onMonitorChanged},
				Label{Text: "Ngôn ngữ"}, ComboBox{AssignTo: &a.langCB, Model: []string{"Theo Windows"}, MinSize: inSize, MaxSize: inSize, OnCurrentIndexChanged: a.onLangChanged,
					ToolTipText: "Ngôn ngữ chữ trong game. Thiếu ngôn ngữ? Windows Settings → Time & language → Language & region → Add a language."},
				Label{Text: "Từ khóa"}, LineEdit{AssignTo: &a.kwLE, Text: strings.Join(a.cfg.Keywords, ", "), MinSize: inSize, MaxSize: inSize, OnEditingFinished: a.onKeywordsChanged,
					ToolTipText: "Chữ hiện trên màn hình chết, cách nhau bằng dấu phẩy. Thêm * ở cuối để khớp phần đầu câu. Xóa hết để dùng bộ từ khóa chung."},
				Label{Text: "Ngưỡng"}, row(
					NumberEdit{AssignTo: &a.thrNE, MinValue: 50, MaxValue: 99, Decimals: 0, Value: float64(DefaultThreshold * 100), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onThresholdChanged,
						ToolTipText: "Ảnh mẫu giống màn hình từ mức này trở lên thì tính là chết."},
					Label{Text: "%"},
				),
				Label{Text: "Cooldown"}, row(
					NumberEdit{AssignTo: &a.cooldownNE, MinValue: 1, MaxValue: 600, Decimals: 0, Value: float64(DefaultCooldownSec), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onCooldownChanged,
						ToolTipText: "Thời gian tối thiểu giữa hai lần đếm."},
					Label{Text: "giây"},
				),
				Label{Text: "Vùng"}, row(
					Label{AssignTo: &a.roiLbl, Text: "Toàn màn hình"},
					PushButton{Text: "Khoanh...", OnClicked: a.editROI, ToolTipText: "Khoanh sát dòng chữ chết (vd: YOU DIED) để nhận diện chính xác hơn."},
				),
			),
			sep(),
			row(
				title("Ảnh mẫu"),
				Label{AssignTo: &a.tmplInfo, Text: "Chưa có mẫu.", TextColor: hintColor, EllipsisMode: EllipsisEnd},
			),
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 4, Alignment: AlignHNearVNear},
				Children: []Widget{
					ListBox{AssignTo: &a.tmplLB, MinSize: Size{Width: 108, Height: 56}, MaxSize: Size{Width: 108, Height: 56}, OnCurrentIndexChanged: a.onTemplateSelected},
					ImageView{AssignTo: &a.tmplIV, Mode: ImageViewModeZoom, MinSize: Size{Width: 100, Height: 56}, MaxSize: Size{Width: 100, Height: 56}},
					Composite{
						Layout: VBox{MarginsZero: true, Spacing: 4},
						Children: []Widget{
							sized(PushButton{Text: "Chụp mẫu", OnClicked: func() { a.captureTemplate(false) }, ToolTipText: "Chụp màn hình hiện tại làm ảnh mẫu (" + a.cfg.Hotkeys.Capture + ")."}),
							sized(PushButton{Text: "Xóa mẫu", OnClicked: a.removeTemplate}),
						},
					},
					HSpacer{},
				},
			},
			sep(),
			form(
				Label{Text: "OCR", TextColor: hintColor}, Label{AssignTo: &a.ocrLbl, Text: "--", TextColor: hintColor, EllipsisMode: EllipsisEnd, MinSize: inSize},
				Label{Text: "Độ giống", TextColor: hintColor}, Label{AssignTo: &a.simLbl, Text: "--", TextColor: hintColor, EllipsisMode: EllipsisEnd, MinSize: inSize},
			),
			VSpacer{},
		},
	}

	// Death count: how the count looks.
	displayTab := TabPage{
		Title:  "Hiển thị",
		Layout: pageLayout(),
		Children: []Widget{
			title("Nhãn"),
			form(
				Label{Text: "Kiểu"}, ComboBox{AssignTo: &a.labelModeCB, Model: []string{"Chữ", "Hình (PNG, SVG)", "Không có nhãn"}, MinSize: inSize, MaxSize: inSize, OnCurrentIndexChanged: a.onLabelModeChanged},
				Label{Text: "Chữ"}, LineEdit{AssignTo: &a.labelTextLE, Text: a.cfg.LabelText, MinSize: inSize, MaxSize: inSize, OnTextChanged: a.onLabelTextChanged,
					ToolTipText: "{game} = tên game."},
				Label{Text: "Hình"}, row(
					LineEdit{AssignTo: &a.labelImgLE, ReadOnly: true, Text: "Chưa chọn", MinSize: inSize, MaxSize: inSize},
					PushButton{Text: "Chọn...", OnClicked: func() { a.chooseLabelImage() }},
				),
				Label{Text: "Cỡ"}, row(
					NumberEdit{AssignTo: &a.labelSizeNE, MinValue: 8, MaxValue: 400, Decimals: 0, Value: float64(a.cfg.LabelSize), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onLabelSizeChanged,
						ToolTipText: "Cỡ chữ của nhãn, hoặc chiều cao của hình."},
					Label{Text: "px"},
				),
				Label{Text: "Vị trí"}, ComboBox{AssignTo: &a.cornerCB, Model: cornerNames, CurrentIndex: max(CornerIndex(a.cfg.LabelCorner), 0), MinSize: inSize, MaxSize: inSize, OnCurrentIndexChanged: a.onCornerChanged,
					ToolTipText: "Nhãn nằm ở góc nào quanh số đếm."},
			),
			sep(),
			title("Kiểu chữ"),
			form(
				Label{Text: "Font"}, ComboBox{AssignTo: &a.fontCB, Editable: true, Model: a.fontList(), MinSize: inSize, MaxSize: inSize, OnCurrentIndexChanged: a.onStyleChanged, OnEditingFinished: a.onStyleChanged},
				Label{Text: "Cỡ số"}, row(
					NumberEdit{AssignTo: &a.fontNE, MinValue: 10, MaxValue: 300, Decimals: 0, Value: float64(a.cfg.OverlayFontSize), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onStyleChanged},
					Label{Text: "px"},
					CheckBox{AssignTo: &a.boldCB, Text: "Đậm", Checked: a.cfg.FontBold, OnCheckedChanged: a.onStyleChanged},
				),
				Label{Text: "Viền"}, row(
					NumberEdit{AssignTo: &a.outlineNE, MinValue: 1, MaxValue: 30, Decimals: 0, Value: float64(maxInt(a.cfg.OutlineWidth, 1)), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onStyleChanged},
					Label{Text: "px"},
					CheckBox{AssignTo: &a.outlineCB, Text: "Bật", Checked: a.cfg.Outline, OnCheckedChanged: a.onStyleChanged},
				),
				Label{Text: "Màu"}, row(
					PushButton{AssignTo: &a.colorPB, Text: "Chữ", OnClicked: func() { a.pickColor(false) }},
					PushButton{AssignTo: &a.outlinePB, Text: "Viền", OnClicked: func() { a.pickColor(true) }},
				),
			),
			VSpacer{},
		},
	}

	// Death count: where the count shows up.
	outputTab := TabPage{
		Title:  "Overlay/OBS",
		Layout: pageLayout(),
		Children: []Widget{
			title("Overlay trên game"),
			row(
				CheckBox{AssignTo: &a.overlayCB, Text: "Hiện", Checked: a.cfg.OverlayEnabled, OnCheckedChanged: a.onOverlayEnabledChanged},
				PushButton{AssignTo: &a.editPosPB, Text: "Kéo vị trí", OnClicked: a.toggleOverlayEdit},
				PushButton{Text: "Đặt lại", OnClicked: a.resetOverlayPos},
			),
			hint("OBS chỉ thấy overlay này khi dùng Display Capture."),
			sep(),
			title("OBS"),
			form(
				Label{Text: "Browser"}, row(
					LineEdit{AssignTo: &a.urlLE, ReadOnly: true, Text: "(đang khởi động...)", MinSize: inSize, MaxSize: inSize},
					PushButton{Text: "Copy", OnClicked: func() {
						if a.server != nil {
							walk.Clipboard().SetText(a.server.URL())
							a.setStatus("Đã copy URL. OBS: Sources → + → Browser → dán vào ô URL.")
						}
					}},
				),
				Label{Text: "File text"}, row(
					CheckBox{AssignTo: &a.obsCB, Text: "Ghi file", Checked: a.cfg.ObsEnabled, OnCheckedChanged: a.onObsEnabledChanged,
						ToolTipText: "Ghi số đếm ra file cho Text source của OBS (không có kiểu chữ)."},
					PushButton{Text: "Copy", ToolTipText: "Copy đường dẫn file", OnClicked: func() {
						walk.Clipboard().SetText(a.store.ObsPath())
						a.setStatus("Đã copy đường dẫn file OBS.")
					}},
				),
			),
			hint("Browser Source giữ đúng kiểu chữ và nhãn, dùng được với Game Capture. Đặt Width 1200, Height 400."),
			VSpacer{},
		},
	}

	settingsTab := TabPage{
		Title:  "Cài đặt",
		Layout: pageLayout(),
		Children: []Widget{
			title("Âm thanh mỗi lần +1"),
			form(
				Label{Text: "File"}, row(
					LineEdit{AssignTo: &a.soundLE, ReadOnly: true, Text: "Chưa chọn", MinSize: inSize, MaxSize: inSize},
					PushButton{Text: "Chọn...", OnClicked: a.chooseSound, ToolTipText: "File .mp3 hoặc .wav của bạn."},
				),
				Label{Text: "Âm lượng"}, row(
					NumberEdit{AssignTo: &a.volNE, MinValue: 0, MaxValue: 100, Decimals: 0, Value: float64(a.cfg.SoundVolume), SpinButtonsVisible: true, MinSize: inSize, MaxSize: inSize, OnValueChanged: a.onVolumeChanged,
						ToolTipText: "Chỉ áp dụng cho mp3."},
					Label{Text: "%"},
				),
				Label{Text: ""}, row(
					CheckBox{AssignTo: &a.soundCB, Text: "Bật", Checked: a.cfg.Sound, OnCheckedChanged: func() {
						if a.soundCB == nil {
							return
						}
						a.cfg.Sound = a.soundCB.Checked()
						a.saveConfig()
					}},
					PushButton{Text: "Nghe thử", OnClicked: a.testSound},
					PushButton{Text: "Bỏ file", OnClicked: func() { a.setSoundFile("") }},
				),
			),
			sep(),
			title("Game đang chọn"),
			row(
				PushButton{Text: "Đổi tên", OnClicked: a.renameProfile},
				PushButton{Text: "Xóa game", OnClicked: a.deleteProfile, ToolTipText: "Xóa game cùng số đếm và ảnh mẫu."},
			),
			sep(),
			title("Phím tắt"),
			TextLabel{AssignTo: &a.hkLbl, Text: "", TextColor: hintColor, MinSize: Size{Width: 100}},
			sep(),
			title("Dữ liệu"),
			row(
				PushButton{Text: "Thư mục", OnClicked: a.openDataDir, ToolTipText: "Mở " + a.store.Dir},
				PushButton{Text: "Gỡ app...", OnClicked: func() { runUninstall(a) },
					ToolTipText: "Xóa app, toàn bộ dữ liệu, shortcut và mục trong Apps & features. Không hoàn tác được."},
			),
			hint("Số đếm, ảnh mẫu, cài đặt và giọng đọc nằm cạnh Byrgenwerth.exe."),
			VSpacer{},
		},
	}

	return MainWindow{
		AssignTo: &a.mw,
		Title:    "Byrgenwerth v" + version,
		Font:     uiFont,
		Size:     Size{Width: 360, Height: 560},
		MinSize:  Size{Width: 350, Height: 480},
		Layout:   VBox{Margins: Margins{Left: 8, Top: 8, Right: 8, Bottom: 6}, Spacing: 6, Alignment: AlignHNearVNear},
		Children: []Widget{
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 4, Alignment: AlignHNearVCenter},
				Children: []Widget{
					sized(PushButton{AssignTo: &a.navDeath, Text: "Death count", OnClicked: func() { a.showPage(false) }}),
					sized(PushButton{AssignTo: &a.navTranslate, Text: "Translate", OnClicked: func() { a.showPage(true) }}),
					HSpacer{},
					CheckBox{AssignTo: &a.topCB, Text: "Luôn trên", Checked: a.cfg.AlwaysOnTop, OnCheckedChanged: a.onTopChanged,
						ToolTipText: "Cửa sổ app luôn nằm trên các cửa sổ khác"},
				},
			},
			Composite{
				AssignTo: &a.deathPage,
				Layout:   VBox{MarginsZero: true, Spacing: 6, Alignment: AlignHNearVNear},
				Children: []Widget{
					row(
						ComboBox{AssignTo: &a.profileCB, OnCurrentIndexChanged: a.onProfileChanged, ToolTipText: "Game đang đếm (đổi tên, xóa: tab Cài đặt)"},
						PushButton{Text: "Thêm game", OnClicked: a.addProfile},
					),
					row(
						Label{
							AssignTo:    &a.countLbl,
							Text:        "0",
							Font:        Font{Family: "Segoe UI", PointSize: 22, Bold: true},
							TextColor:   walk.RGB(200, 30, 30),
							MinSize:     Size{Width: 70},
							ToolTipText: "Số lần chết",
						},
						PushButton{Text: "+1", OnClicked: func() { a.addDeath(1, "button") }, ToolTipText: a.cfg.Hotkeys.Add},
						PushButton{Text: "−1", OnClicked: func() { a.addDeath(-1, "button") }, ToolTipText: a.cfg.Hotkeys.Sub},
						PushButton{Text: "Reset", OnClicked: a.resetCount},
					),
					TabWidget{Pages: []TabPage{detectTab, displayTab, outputTab, settingsTab}},
				},
			},
			a.translatePage(),
			Label{AssignTo: &a.statusLbl, Text: "", EllipsisMode: EllipsisEnd, TextColor: hintColor},
		},
	}.Create()
}

func (a *App) init() {
	if img, err := png.Decode(bytes.NewReader(iconPNG)); err == nil {
		if ic, err := walk.NewIconFromImageForDPI(img, 96); err == nil {
			a.mw.SetIcon(ic)
		}
	}

	a.loadSound()
	if srv, err := StartOverlayServer(a.cfg.ServerPort); err == nil {
		a.server = srv
		a.urlLE.SetText(srv.URL())
	} else {
		a.urlLE.SetText("Không mở được: " + err.Error())
	}
	a.refreshSwatches()
	a.suppress = true
	a.fontCB.SetText(a.cfg.FontFamily)
	a.suppress = false
	a.outlineNE.SetEnabled(a.cfg.Outline)
	a.initOverlay()
	a.initTranslation()
	a.initLabelUI()
	a.initNarration()

	var err error
	a.profiles, err = a.store.LoadProfiles()
	if err != nil {
		a.setStatus("Lỗi đọc dữ liệu: " + err.Error())
	}
	if len(a.profiles) == 0 {
		if p, err := a.store.CreateProfile(newProfileName); err == nil {
			a.profiles = []*Profile{p}
		}
	}
	idx := 0
	for i, p := range a.profiles {
		if p.ID == a.cfg.CurrentProfile {
			idx = i
		}
	}
	a.refreshProfileList(idx)
	if len(a.profiles) > 0 {
		a.loadProfile(a.profiles[idx])
	}
	a.applyTopMost()

	a.startHotkeys()
	h := a.cfg.Hotkeys
	a.hkLbl.SetText(fmt.Sprintf("%s +1 và học mẫu  ·  %s −1  ·  %s chụp mẫu  ·  %s bật/tắt nhận diện  ·  %s ẩn/hiện app  ·  %s bật/tắt dịch  ·  %s khoanh vùng dịch",
		h.Add, h.Sub, h.Capture, h.Toggle, h.Window, h.TranslateToggle, h.TranslateRegion))
	if len(a.hkErrors) > 0 {
		a.setStatus("Cảnh báo phím tắt: " + strings.Join(a.hkErrors, "; "))
	} else {
		a.setStatus("Sẵn sàng. Khi chết mà app chưa tự nhận ra, bấm " + h.Add + ".")
	}
	if a.dirWarn != "" {
		a.setStatus(a.dirWarn)
		walk.MsgBox(a.mw, "Thư mục dữ liệu", a.dirWarn, walk.MsgBoxIconWarning)
	}
	if a.migrated {
		a.setStatus("Đã chuyển dữ liệu cũ sang " + a.store.Dir + ". Nhớ cập nhật đường dẫn file trong OBS (nút Copy).")
	}

	a.mw.Closing().Attach(func(canceled *bool, reason walk.CloseReason) {
		a.mw.SetVisible(false) // vanish now; stopping the capture loop can take seconds
		close(a.quit)
		a.translation.cancel()
		a.wg.Wait()
		a.hk.Stop()
		a.overlay.Destroy()
		a.subtitle.Destroy()
		a.server.Close()
		a.sound.Close()
		a.stopNarration()
		a.saveConfig()
	})

	a.wg.Add(3)
	go a.captureLoop()
	go a.translateLoop()
	go func() { // keep overlay above borderless games
		defer a.wg.Done()
		for {
			select {
			case <-a.quit:
				return
			case <-time.After(time.Second):
				a.mw.Synchronize(func() {
					a.autoHideForGame()
					if a.cfg.OverlayEnabled && !a.overlay.Editing() {
						a.overlay.KeepOnTop()
					}
					if a.translation.on {
						a.subtitle.KeepOnTop()
					}
				})
			}
		}
	}()
}

// ---------- windows on screen ----------

// ownRects returns the app's visible windows (main window, overlay, subtitle)
// normalized to the game monitor, so OCR can ignore the app's own text.
func (a *App) ownRects() []ROI {
	mon := a.currentMonitor().Rect
	mw, mh := float64(mon.R-mon.L), float64(mon.B-mon.T)
	hwnds := []win.HWND{a.mw.Handle()}
	if a.overlay != nil {
		hwnds = append(hwnds, a.overlay.hwnd)
	}
	if s := a.subtitle; s != nil {
		hwnds = append(hwnds, s.hwnd)
	}
	var out []ROI
	for _, h := range hwnds {
		if h == 0 || !win.IsWindowVisible(h) || win.IsIconic(h) {
			continue
		}
		var r win.RECT
		win.GetWindowRect(h, &r)
		out = append(out, ROI{
			X: float64(r.Left-mon.L) / mw, Y: float64(r.Top-mon.T) / mh,
			W: float64(r.Right-r.Left) / mw, H: float64(r.Bottom-r.Top) / mh,
		})
	}
	return out
}

// autoHideForGame minimizes the app window when another program's full-screen
// window (a game, borderless or exclusive) comes to the front. It acts once per
// switch, so F7 can still bring the app back while the game runs.
func (a *App) autoHideForGame() {
	fg := win.GetForegroundWindow()
	if fg == a.lastFg {
		return
	}
	a.lastFg = fg
	h := a.mw.Handle()
	if isOtherFullscreen(fg) && a.mw.Visible() && !win.IsIconic(h) {
		win.ShowWindow(h, win.SW_SHOWMINNOACTIVE)
	}
}

// isOtherFullscreen reports whether h belongs to another process and covers its
// whole monitor. The desktop and taskbar are not games.
func isOtherFullscreen(h win.HWND) bool {
	if h == 0 {
		return false
	}
	var pid uint32
	win.GetWindowThreadProcessId(h, &pid)
	if int(pid) == os.Getpid() {
		return false
	}
	cls := make([]uint16, 64)
	win.GetClassName(h, &cls[0], len(cls))
	switch syscall.UTF16ToString(cls) {
	case "Progman", "WorkerW", "Shell_TrayWnd", "Shell_SecondaryTrayWnd":
		return false
	}
	var r win.RECT
	if !win.GetWindowRect(h, &r) {
		return false
	}
	mi := win.MONITORINFO{CbSize: uint32(unsafe.Sizeof(win.MONITORINFO{}))}
	if !win.GetMonitorInfo(win.MonitorFromWindow(h, win.MONITOR_DEFAULTTONEAREST), &mi) {
		return false
	}
	m := mi.RcMonitor
	return r.Left <= m.Left && r.Top <= m.Top && r.Right >= m.Right && r.Bottom >= m.Bottom
}

// ---------- hotkeys ----------

func (a *App) bringToFront() {
	h := a.mw.Handle()
	a.mw.SetVisible(true)
	win.ShowWindow(h, win.SW_RESTORE)
	a.applyTopMost()
	win.SetForegroundWindow(h)
}

func (a *App) onHotkey(act HotkeyAction) {
	if act == ActWindow {
		if a.mw.Visible() && !win.IsIconic(a.mw.Handle()) {
			a.mw.SetVisible(false)
			return
		}
		a.bringToFront()
		return
	}
	if act == ActTranslate {
		if !a.translation.picking {
			a.trOnCB.SetChecked(!a.trOnCB.Checked())
		}
		return
	}
	if act == ActTranslateRegion {
		a.editTranslateRegion()
		return
	}
	if act == ActNarrate {
		a.onNarrate()
		return
	}
	if a.cur == nil {
		return
	}
	switch act {
	case ActAdd:
		now := time.Now()
		if a.det.AutoCountedRecently(now) {
			a.setStatus("App vừa tự đếm lần chết này rồi, bỏ qua " + a.cfg.Hotkeys.Add + ".")
			return
		}
		img, err := CaptureMonitor(a.currentMonitor())
		a.addDeath(1, "hotkey")
		a.store.AppendMissLog(a.cur.Name, a.recentOcr, now)
		a.det.MarkManual(now)
		if err != nil {
			a.setStatus("Đã +1, nhưng không chụp được màn hình: " + err.Error())
			return
		}
		f := ExtractFeature(img, a.cur.ROI)
		sim, _ := BestSimilarity(f, a.tmplFeats)
		if len(a.tmplFeats) > 0 && sim >= a.cur.Threshold {
			a.setStatus(fmt.Sprintf("Đã +1. Màn hình này đã khớp mẫu (%d%%), không lưu thêm.", int(sim*100)))
			return
		}
		a.saveTemplate(img, "Đã +1 và lưu ảnh mẫu mới")
	case ActSub:
		a.addDeath(-1, "hotkey")
	case ActCapture:
		a.captureTemplate(true)
	case ActToggle:
		a.autoCB.SetChecked(!a.autoCB.Checked())
	}
}

// ---------- counting ----------

func (a *App) refreshCount() {
	if a.cur == nil {
		return
	}
	a.countLbl.SetText(fmt.Sprint(a.cur.Deaths))
	if a.cfg.ObsEnabled {
		if err := writeAtomic(a.store.ObsPath(), []byte(TextFileLine(a.cfg, a.cur.Name, a.cur.Deaths))); err != nil {
			a.setStatus("Không ghi được file OBS: " + err.Error())
		}
	}
	if o := a.overlay; o != nil {
		o.Text = fmt.Sprint(a.cur.Deaths)
		o.Label = LabelTextFor(a.cfg, a.cur.Name)
		o.LabelImg = nil
		if a.cfg.LabelMode == LabelModeImage {
			o.LabelImg = a.labelImg
		}
		o.LabelSize = a.cfg.LabelSize
		o.Corner = a.cfg.LabelCorner
		o.Render()
	}
	a.server.Set(StateFromConfig(a.cfg, a.cur.Name, a.cur.Deaths, a.labelImgVer))
}

func (a *App) resetCount() {
	if a.cur == nil {
		return
	}
	if walk.MsgBox(a.mw, "Reset", fmt.Sprintf("Đưa số lần chết của \"%s\" về 0?", a.cur.Name), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	old := a.cur.Deaths
	a.cur.Deaths = 0
	a.store.SaveProfile(a.cur)
	a.store.AppendHistory(time.Now(), a.cur.Name, -old, 0, "reset")
	a.refreshCount()
}

// ---------- templates ----------

func (a *App) captureTemplate(fromHotkey bool) {
	if a.cur == nil {
		return
	}
	img, err := CaptureMonitor(a.currentMonitor())
	if err != nil {
		a.setStatus("Không chụp được màn hình: " + err.Error())
		return
	}
	if fromHotkey {
		// The death on screen right now is assumed to be counted already.
		a.det.MarkManual(time.Now())
	}
	a.saveTemplate(img, "Đã lưu ảnh mẫu mới")
}

func (a *App) saveTemplate(img image.Image, msg string) {
	if _, err := a.store.AddTemplate(a.cur, img, time.Now()); err != nil {
		a.setStatus("Lỗi lưu ảnh mẫu: " + err.Error())
		return
	}
	a.reloadTemplates()
	a.tmplLB.SetCurrentIndex(len(a.cur.Templates) - 1)
	extra := ""
	if a.cur.ROI.IsFull() {
		extra = " Nên bấm \"Khoanh vùng nhận diện\" để khoanh sát chữ chết."
	}
	a.setStatus(fmt.Sprintf("%s (%d/%d).%s", msg, len(a.cur.Templates), MaxTemplates, extra))
}

func (a *App) removeTemplate() {
	i := a.tmplLB.CurrentIndex()
	if a.cur == nil || i < 0 || i >= len(a.cur.Templates) {
		return
	}
	a.store.RemoveTemplate(a.cur, i)
	a.reloadTemplates()
	a.setStatus("Đã xóa mẫu.")
}

func (a *App) reloadTemplates() {
	a.tmplImgs, a.tmplFeats = nil, nil
	if a.cur != nil {
		for _, name := range a.cur.Templates {
			img, err := a.store.LoadTemplateImage(a.cur, name)
			if err != nil {
				continue
			}
			a.tmplImgs = append(a.tmplImgs, img)
			a.tmplFeats = append(a.tmplFeats, ExtractFeature(img, a.cur.ROI))
		}
	}
	labels := make([]string, 0, len(a.tmplImgs))
	if a.cur != nil {
		for i, n := range a.cur.Templates {
			labels = append(labels, templateLabel(i, n))
		}
	}
	a.suppress = true
	a.tmplLB.SetModel(labels)
	a.suppress = false
	a.showPreview(nil)
	if len(labels) > 0 {
		a.tmplLB.SetCurrentIndex(0)
		a.showPreview(a.tmplImgs[0])
	}
	a.refreshTemplateInfo()
	a.updateLoop()
}

func (a *App) refreshTemplateInfo() {
	n := len(a.tmplFeats)
	if n == 0 {
		a.tmplInfo.SetText("Chưa có. OCR bắt được sẽ tự lưu, hoặc bấm " + a.cfg.Hotkeys.Add + " lúc chết.")
		return
	}
	s := fmt.Sprintf("%d/%d mẫu.", n, MaxTemplates)
	if ag := TemplateAgreement(a.tmplFeats); ag >= 0 {
		s += fmt.Sprintf(" Khớp nhau %d%%.", int(ag*100))
		if ag < a.cur.Threshold {
			s += " Nên khoanh vùng sát hơn hoặc xóa mẫu lạ."
		}
	}
	a.tmplInfo.SetText(s)
	a.tmplInfo.SetToolTipText(s)
}

func (a *App) onTemplateSelected() {
	if a.suppress {
		return
	}
	i := a.tmplLB.CurrentIndex()
	if i >= 0 && i < len(a.tmplImgs) {
		a.showPreview(a.tmplImgs[i])
	}
}

func (a *App) showPreview(img image.Image) {
	var bmp *walk.Bitmap
	if img != nil {
		bmp, _ = walk.NewBitmapFromImageForDPI(img, 96)
	}
	if bmp != nil {
		a.tmplIV.SetImage(bmp)
	} else {
		a.tmplIV.SetImage(nil)
	}
	if a.previewBmp != nil {
		a.previewBmp.Dispose()
	}
	a.previewBmp = bmp
}

// ---------- ROI editor ----------

func fitRect(container walk.Rectangle, w, h int) walk.Rectangle {
	if container.Width <= 0 || container.Height <= 0 {
		return container
	}
	rw := container.Width
	rh := rw * h / w
	if rh > container.Height {
		rh = container.Height
		rw = rh * w / h
	}
	return walk.Rectangle{X: container.X + (container.Width-rw)/2, Y: container.Y + (container.Height-rh)/2, Width: rw, Height: rh}
}

func (a *App) editROI() {
	if a.cur == nil {
		return
	}
	a.editRegion("Khoanh vùng nhận diện - "+a.cur.Name,
		"Kéo chuột để khoanh SÁT dòng chữ / biểu tượng chỉ hiện khi chết (vd: YOU DIED).\nKhoanh càng sát, nhận diện càng chính xác. Tránh lấy phần cảnh game phía sau.",
		a.cur.ROI, true, a.saveDeathROI)
}

func (a *App) saveDeathROI(roi ROI) {
	a.cur.ROI = roi
	a.store.SaveProfile(a.cur)
	a.det.Reset()
	a.refreshROILabel()
	sel := a.tmplLB.CurrentIndex()
	a.reloadTemplates()
	if sel >= 0 && sel < len(a.tmplImgs) {
		a.tmplLB.SetCurrentIndex(sel)
	}
	a.setStatus("Đã lưu vùng nhận diện.")
}

// editRegion shows a screenshot and lets the user drag a rectangle.
func (a *App) editRegion(title, instr string, initial ROI, showAgreement bool, save func(ROI)) {
	a.editRegionOn(nil, title, instr, initial, showAgreement, save)
}

// editRegionOn is editRegion over a given screenshot (nil = template or fresh capture).
func (a *App) editRegionOn(shot image.Image, title, instr string, initial ROI, showAgreement bool, save func(ROI)) {
	if a.cur == nil {
		return
	}
	base := shot
	if base != nil {
		// use the given screenshot
	} else if i := a.tmplLB.CurrentIndex(); i >= 0 && i < len(a.tmplImgs) {
		base = a.tmplImgs[i]
	} else if len(a.tmplImgs) > 0 {
		base = a.tmplImgs[0]
	} else {
		img, err := CaptureMonitor(a.currentMonitor())
		if err != nil {
			a.setStatus("Không chụp được màn hình: " + err.Error())
			return
		}
		base = img
	}
	bmp, err := walk.NewBitmapFromImageForDPI(base, 96)
	if err != nil {
		a.setStatus("Lỗi ảnh: " + err.Error())
		return
	}
	defer bmp.Dispose()
	bg, _ := walk.NewSolidColorBrush(walk.RGB(20, 20, 20))
	defer bg.Dispose()
	pen, _ := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(255, 60, 60))
	defer pen.Dispose()
	pen2, _ := walk.NewCosmeticPen(walk.PenSolid, walk.RGB(255, 255, 255))
	defer pen2.Dispose()

	roi := initial.Normalized()
	var prev ROI
	dragging := false
	var sx, sy int

	var dlg *walk.Dialog
	var cw *walk.CustomWidget
	var okPB, cancelPB *walk.PushButton
	var infoLbl *walk.Label

	imgRect := func() walk.Rectangle {
		return fitRect(cw.ClientBoundsPixels(), CaptureW, CaptureH)
	}
	toRel := func(x, y int) (float64, float64) {
		r := imgRect()
		fx := float64(x-r.X) / float64(r.Width)
		fy := float64(y-r.Y) / float64(r.Height)
		clamp := func(v float64) float64 {
			if v < 0 {
				return 0
			}
			if v > 1 {
				return 1
			}
			return v
		}
		return clamp(fx), clamp(fy)
	}
	updateInfo := func() {
		var feats []Feature
		for _, im := range a.tmplImgs {
			feats = append(feats, ExtractFeature(im, roi))
		}
		s := fmt.Sprintf("Vùng: x %d%%, y %d%%, rộng %d%%, cao %d%%.", int(roi.X*100), int(roi.Y*100), int(roi.W*100), int(roi.H*100))
		if ag := TemplateAgreement(feats); ag >= 0 && showAgreement {
			s += fmt.Sprintf("  Các mẫu khớp nhau trong vùng này: %d%%", int(ag*100))
		}
		infoLbl.SetText(s)
	}

	paint := func(c *walk.Canvas, _ walk.Rectangle) error {
		cb := cw.ClientBoundsPixels()
		c.FillRectanglePixels(bg, cb)
		r := imgRect()
		c.DrawImageStretchedPixels(bmp, r)
		rr := walk.Rectangle{
			X:      r.X + int(roi.X*float64(r.Width)),
			Y:      r.Y + int(roi.Y*float64(r.Height)),
			Width:  int(roi.W * float64(r.Width)),
			Height: int(roi.H * float64(r.Height)),
		}
		c.DrawRectanglePixels(pen2, walk.Rectangle{X: rr.X - 1, Y: rr.Y - 1, Width: rr.Width + 2, Height: rr.Height + 2})
		c.DrawRectanglePixels(pen, rr)
		c.DrawRectanglePixels(pen, walk.Rectangle{X: rr.X + 1, Y: rr.Y + 1, Width: rr.Width - 2, Height: rr.Height - 2})
		return nil
	}

	err = Dialog{
		AssignTo:      &dlg,
		Title:         title,
		DefaultButton: &okPB,
		CancelButton:  &cancelPB,
		MinSize:       Size{Width: 760, Height: 560},
		Layout:        VBox{},
		Children: []Widget{
			Label{Text: instr},
			CustomWidget{
				AssignTo:            &cw,
				MinSize:             Size{Width: 720, Height: 405},
				InvalidatesOnResize: true,
				PaintPixels:         paint,
				OnMouseDown: func(x, y int, b walk.MouseButton) {
					if b != walk.LeftButton {
						return
					}
					dragging = true
					prev = roi
					sx, sy = x, y
					win.SetCapture(cw.Handle())
				},
				OnMouseMove: func(x, y int, b walk.MouseButton) {
					if !dragging {
						return
					}
					x0, y0 := toRel(sx, sy)
					x1, y1 := toRel(x, y)
					if x1 < x0 {
						x0, x1 = x1, x0
					}
					if y1 < y0 {
						y0, y1 = y1, y0
					}
					roi = ROI{x0, y0, x1 - x0, y1 - y0}
					cw.Invalidate()
				},
				OnMouseUp: func(x, y int, b walk.MouseButton) {
					if !dragging {
						return
					}
					dragging = false
					win.ReleaseCapture()
					if roi.W < 0.02 || roi.H < 0.02 {
						roi = prev
					}
					roi = roi.Normalized()
					cw.Invalidate()
					updateInfo()
				},
			},
			Label{AssignTo: &infoLbl, Text: ""},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					PushButton{Text: "Toàn màn hình", OnClicked: func() {
						roi = FullROI()
						cw.Invalidate()
						updateInfo()
					}},
					HSpacer{},
					PushButton{AssignTo: &okPB, Text: "Lưu", OnClicked: func() { dlg.Accept() }},
					PushButton{AssignTo: &cancelPB, Text: "Hủy", OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}.Create(a.mw)
	if err != nil {
		a.setStatus("Lỗi mở hộp thoại: " + err.Error())
		return
	}
	updateInfo()
	if dlg.Run() != walk.DlgCmdOK {
		return
	}
	save(roi.Normalized())
}

func (a *App) refreshROILabel() {
	if a.cur == nil || a.cur.ROI.IsFull() {
		a.roiLbl.SetText("Toàn màn hình")
		return
	}
	r := a.cur.ROI
	a.roiLbl.SetText(fmt.Sprintf("x %d%%, y %d%%, %d×%d%%", int(r.X*100), int(r.Y*100), int(r.W*100), int(r.H*100)))
}

// ---------- profiles ----------

func (a *App) refreshProfileList(sel int) {
	names := make([]string, len(a.profiles))
	for i, p := range a.profiles {
		names[i] = p.Name
	}
	a.suppress = true
	a.profileCB.SetModel(names)
	if sel >= 0 && sel < len(names) {
		a.profileCB.SetCurrentIndex(sel)
	}
	a.suppress = false
}

func (a *App) loadProfile(p *Profile) {
	a.cur = p
	a.det = NewDetector(p.Threshold, time.Duration(p.CooldownSec*float64(time.Second)))
	a.suppress = true
	a.autoCB.SetChecked(p.AutoEnabled)
	a.ocrCB.SetChecked(p.OcrEnabled)
	a.thrNE.SetValue(float64(int(p.Threshold*100 + 0.5)))
	a.cooldownNE.SetValue(p.CooldownSec)
	a.suppress = false
	a.cfg.CurrentProfile = p.ID
	a.saveConfig()
	a.refreshROILabel()
	a.refreshKeywordsUI()
	a.refreshCount()
	a.reloadTemplates()
	a.ocrProven = false
	a.refreshLangUI()
}

func (a *App) onProfileChanged() {
	if a.suppress {
		return
	}
	i := a.profileCB.CurrentIndex()
	if i >= 0 && i < len(a.profiles) && a.profiles[i] != a.cur {
		a.loadProfile(a.profiles[i])
	}
}

func (a *App) addProfile() {
	name, ok := a.prompt("Thêm game", "Tên game:", "")
	if !ok {
		return
	}
	p, err := a.store.CreateProfile(name)
	if err != nil {
		walk.MsgBox(a.mw, "Thêm game", "Không tạo được: "+err.Error(), walk.MsgBoxIconWarning)
		return
	}
	a.profiles = append(a.profiles, p)
	a.refreshProfileList(len(a.profiles) - 1)
	a.loadProfile(p)
	a.setStatus("Đã thêm \"" + p.Name + "\".")
}

func (a *App) renameProfile() {
	if a.cur == nil {
		return
	}
	name, ok := a.prompt("Đổi tên", "Tên game mới:", a.cur.Name)
	if !ok || strings.TrimSpace(name) == "" {
		return
	}
	a.cur.Name = strings.TrimSpace(name)
	a.store.SaveProfile(a.cur)
	for i, p := range a.profiles {
		if p == a.cur {
			a.refreshProfileList(i)
		}
	}
	a.refreshCount()
}

func (a *App) deleteProfile() {
	if a.cur == nil {
		return
	}
	if walk.MsgBox(a.mw, "Xóa game", fmt.Sprintf("Xóa \"%s\" cùng số đếm và ảnh mẫu?", a.cur.Name), walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
		return
	}
	a.store.DeleteProfile(a.cur)
	var rest []*Profile
	for _, p := range a.profiles {
		if p != a.cur {
			rest = append(rest, p)
		}
	}
	a.profiles = rest
	if len(a.profiles) == 0 {
		if p, err := a.store.CreateProfile(newProfileName); err == nil {
			a.profiles = []*Profile{p}
		}
	}
	a.refreshProfileList(0)
	a.loadProfile(a.profiles[0])
}

func (a *App) prompt(title, label, initial string) (string, bool) {
	var dlg *walk.Dialog
	var le *walk.LineEdit
	var okPB, cancelPB *walk.PushButton
	result := initial // captured while the dialog still exists
	res, err := Dialog{
		AssignTo:      &dlg,
		Title:         title,
		DefaultButton: &okPB,
		CancelButton:  &cancelPB,
		MinSize:       Size{Width: 320, Height: 120},
		Layout:        VBox{},
		Children: []Widget{
			Label{Text: label},
			LineEdit{AssignTo: &le, Text: initial},
			Composite{
				Layout: HBox{MarginsZero: true},
				Children: []Widget{
					HSpacer{},
					PushButton{AssignTo: &okPB, Text: "OK", OnClicked: func() {
						result = le.Text()
						dlg.Accept()
					}},
					PushButton{AssignTo: &cancelPB, Text: "Hủy", OnClicked: func() { dlg.Cancel() }},
				},
			},
		},
	}.Run(a.mw)
	if err != nil || res != walk.DlgCmdOK {
		return "", false
	}
	return result, true
}

// ---------- settings handlers ----------

func (a *App) onAutoChanged() {
	if a.suppress || a.cur == nil {
		return
	}
	a.cur.AutoEnabled = a.autoCB.Checked()
	a.store.SaveProfile(a.cur)
	a.det.Reset()
	a.updateLoop()
	if a.cur.AutoEnabled {
		a.setStatus("Đã bật tự nhận diện.")
	} else {
		a.setStatus("Đã tắt tự nhận diện.")
	}
}

func (a *App) onThresholdChanged() {
	if a.thrNE == nil || a.suppress || a.cur == nil {
		return
	}
	a.cur.Threshold = a.thrNE.Value() / 100
	a.det.Threshold = a.cur.Threshold
	a.store.SaveProfile(a.cur)
	a.refreshTemplateInfo()
}

func (a *App) onCooldownChanged() {
	if a.suppress || a.cur == nil {
		return
	}
	a.cur.CooldownSec = a.cooldownNE.Value()
	a.det.Cooldown = time.Duration(a.cur.CooldownSec * float64(time.Second))
	a.store.SaveProfile(a.cur)
}

func (a *App) onMonitorChanged() {
	if a.monCB == nil {
		return
	}
	i := a.monCB.CurrentIndex()
	if i < 0 || i >= len(a.monitors) {
		return
	}
	a.cfg.Monitor = i
	a.saveConfig()
	a.updateLoop()
	if a.trRegionLbl != nil {
		a.refreshTranslateRegionLabel()
		a.syncTranslateLoop()
	}
}

func (a *App) onTopChanged() {
	if a.topCB == nil {
		return
	}
	a.cfg.AlwaysOnTop = a.topCB.Checked()
	a.saveConfig()
	a.applyTopMost()
}

func (a *App) applyTopMost() {
	if a.mw == nil {
		return
	}
	after := win.HWND_NOTOPMOST
	if a.cfg.AlwaysOnTop {
		after = win.HWND_TOPMOST
	}
	win.SetWindowPos(a.mw.Handle(), after, 0, 0, 0, 0, win.SWP_NOMOVE|win.SWP_NOSIZE|win.SWP_NOACTIVATE)
}

func (a *App) openDataDir() {
	verb, _ := syscall.UTF16PtrFromString("open")
	dir, _ := syscall.UTF16PtrFromString(a.store.Dir)
	win.ShellExecute(0, verb, dir, nil, nil, win.SW_SHOWNORMAL)
}

func (a *App) saveConfig() {
	a.store.SaveConfig(a.cfg)
}

func (a *App) setStatus(s string) {
	if a.statusLbl != nil {
		a.statusLbl.SetText(s)
	}
}

// ---------- display: OBS + overlay ----------

func colorIndex(name string) int {
	for i, n := range OverlayColorNames {
		if n == name {
			return i
		}
	}
	return 0
}

func (a *App) monitorRects() [][4]int32 {
	var rs [][4]int32
	for _, m := range a.monitors {
		rs = append(rs, [4]int32{m.Rect.L, m.Rect.T, m.Rect.R, m.Rect.B})
	}
	return rs
}

func (a *App) initOverlay() {
	x, y, ok := ClampToMonitors(a.cfg.OverlayX, a.cfg.OverlayY, a.monitorRects())
	if !a.cfg.OverlayPosSet || !ok {
		x, y, _ = ClampToMonitors(-1<<30, -1<<30, a.monitorRects())
	}
	o, err := NewOverlay(x, y)
	if err != nil {
		a.setStatus("Không tạo được overlay: " + err.Error())
		return
	}
	a.overlay = o
	a.applyStyleToOverlay()
	o.OnMoved = func(x, y int32) {
		a.cfg.OverlayX, a.cfg.OverlayY, a.cfg.OverlayPosSet = x, y, true
		a.saveConfig()
	}
	o.Text = " "
	o.Render()
	o.Show(a.cfg.OverlayEnabled)
}

func (a *App) onObsEnabledChanged() {
	if a.obsCB == nil {
		return
	}
	a.cfg.ObsEnabled = a.obsCB.Checked()
	a.saveConfig()
	if a.cfg.ObsEnabled {
		a.refreshCount()
		a.setStatus("Đã bật hiển thị qua OBS.")
	} else {
		a.setStatus("Đã tắt ghi file OBS.")
	}
}

func (a *App) onOverlayEnabledChanged() {
	if a.overlayCB == nil {
		return
	}
	a.cfg.OverlayEnabled = a.overlayCB.Checked()
	a.saveConfig()
	if !a.cfg.OverlayEnabled && a.overlay.Editing() {
		a.toggleOverlayEdit()
	}
	a.refreshCount()
	a.overlay.Show(a.cfg.OverlayEnabled)
	if a.cfg.OverlayEnabled {
		a.setStatus("Đã bật overlay. Bấm \"Kéo vị trí\" để kéo nó tới chỗ muốn đặt.")
	}
}

func (a *App) applyStyleToOverlay() {
	o := a.overlay
	if o == nil {
		return
	}
	o.FontSize = a.cfg.OverlayFontSize
	o.FontFamily = a.cfg.FontFamily
	o.Bold = a.cfg.FontBold
	o.Color = MustHex(a.cfg.TextColor, RGB{255, 255, 255})
	o.OutlineColor = MustHex(a.cfg.OutlineColor, RGB{})
	o.OutlineWidth = 0
	if a.cfg.Outline {
		o.OutlineWidth = a.cfg.OutlineWidth
	}
}

func (a *App) onStyleChanged() {
	if a.suppress || a.fontNE == nil || a.fontCB == nil || a.boldCB == nil || a.outlineCB == nil || a.outlineNE == nil {
		return
	}
	if f := strings.TrimSpace(a.fontCB.Text()); f != "" {
		a.cfg.FontFamily = f
	}
	a.cfg.OverlayFontSize = int(a.fontNE.Value())
	a.cfg.FontBold = a.boldCB.Checked()
	a.cfg.Outline = a.outlineCB.Checked()
	a.cfg.OutlineWidth = int(a.outlineNE.Value())
	a.outlineNE.SetEnabled(a.cfg.Outline)
	a.saveConfig()
	a.applyStyleToOverlay()
	a.refreshCount()
}

func (a *App) pickColor(outline bool) {
	cur := a.cfg.TextColor
	if outline {
		cur = a.cfg.OutlineColor
	}
	c, ok := PickColor(a.mw.Handle(), MustHex(cur, RGB{255, 255, 255}))
	if !ok {
		return
	}
	if outline {
		a.cfg.OutlineColor = c.Hex()
	} else {
		a.cfg.TextColor = c.Hex()
	}
	a.saveConfig()
	a.refreshSwatches()
	a.applyStyleToOverlay()
	a.refreshCount()
}

// refreshSwatches shows the two colors in their buttons' tooltips.
func (a *App) refreshSwatches() {
	if a.colorPB == nil || a.outlinePB == nil {
		return
	}
	a.colorPB.SetToolTipText("Màu chữ " + MustHex(a.cfg.TextColor, RGB{255, 255, 255}).Hex())
	a.outlinePB.SetToolTipText("Màu viền " + MustHex(a.cfg.OutlineColor, RGB{}).Hex())
}

func (a *App) fontList() []string {
	fs := InstalledFonts()
	if len(fs) == 0 {
		fs = []string{"Segoe UI", "Arial", "Impact", "Consolas"}
	}
	return fs
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (a *App) toggleOverlayEdit() {
	if a.overlay == nil {
		return
	}
	if a.overlay.Editing() {
		a.overlay.SetEditing(false)
		a.editPosPB.SetText("Kéo vị trí")
		a.cfg.OverlayX, a.cfg.OverlayY, a.cfg.OverlayPosSet = a.overlay.X, a.overlay.Y, true
		a.saveConfig()
		a.setStatus("Đã lưu vị trí overlay.")
		return
	}
	if !a.cfg.OverlayEnabled {
		a.overlayCB.SetChecked(true) // triggers onOverlayEnabledChanged
	}
	a.overlay.SetEditing(true)
	a.overlay.Show(true)
	a.editPosPB.SetText("Lưu vị trí")
	a.setStatus("Kéo khung viền vàng tới vị trí bất kỳ trên màn hình, rồi bấm \"Lưu vị trí\".")
}

func (a *App) resetOverlayPos() {
	if a.overlay == nil {
		return
	}
	x, y, _ := ClampToMonitors(-1<<30, -1<<30, a.monitorRects())
	a.overlay.MoveTo(x, y)
	a.cfg.OverlayX, a.cfg.OverlayY, a.cfg.OverlayPosSet = x, y, true
	a.saveConfig()
	a.setStatus("Đã đưa overlay về góc trên bên trái màn hình chính.")
}

// ---------- sound ----------

func (a *App) loadSound() {
	a.sound.Close()
	path := a.cfg.SoundFile
	label := "Chưa chọn"
	if path != "" {
		if _, err := os.Stat(path); err != nil {
			label = "Không tìm thấy: " + filepath.Base(path)
		} else if err := a.sound.Load(path, a.cfg.SoundVolume); err != nil {
			label = "Không mở được: " + err.Error()
		} else {
			label = filepath.Base(path)
		}
	}
	if a.soundLE != nil {
		a.soundLE.SetText(label)
	}
}

// playSound is used on each +1; silent when no file is chosen.
func (a *App) playSound() {
	if !a.sound.Loaded() {
		return
	}
	if err := a.sound.Play(); err != nil {
		a.setStatus("Lỗi phát âm thanh: " + err.Error())
	}
}

func (a *App) testSound() {
	if !a.sound.Loaded() {
		a.setStatus("Chưa chọn file âm thanh: bấm \"Chọn file...\".")
		return
	}
	a.playSound()
}

func (a *App) onVolumeChanged() {
	if a.volNE == nil {
		return
	}
	a.cfg.SoundVolume = int(a.volNE.Value())
	a.sound.SetVolume(a.cfg.SoundVolume)
	a.saveConfig()
}

func (a *App) chooseSound() {
	dlg := walk.FileDialog{Title: "Chọn âm thanh", Filter: "Âm thanh (*.mp3;*.wav)|*.mp3;*.wav"}
	ok, err := dlg.ShowOpen(a.mw)
	if err != nil || !ok {
		return
	}
	a.setSoundFile(dlg.FilePath)
}

func (a *App) setSoundFile(path string) {
	a.cfg.SoundFile = path
	a.saveConfig()
	a.loadSound()
	if path == "" {
		a.setStatus("Đã bỏ âm thanh.")
		return
	}
	a.playSound()
}

func (a *App) onOcrChanged() {
	if a.suppress || a.cur == nil || a.ocrCB == nil {
		return
	}
	a.cur.OcrEnabled = a.ocrCB.Checked()
	a.store.SaveProfile(a.cur)
	a.det.Reset()
	a.updateLoop()
}

func (a *App) onKeywordsChanged() {
	if a.kwLE == nil || a.cur == nil || a.suppress {
		return
	}
	kw := ParseKeywords(a.kwLE.Text())
	if len(kw) == 0 || strings.Join(kw, ",") == strings.Join(a.cfg.Keywords, ",") {
		a.cur.Keywords = nil // back to the generic list
	} else {
		a.cur.Keywords = kw
	}
	a.store.SaveProfile(a.cur)
	a.refreshKeywordsUI()
	a.setStatus(fmt.Sprintf("Đã lưu %d từ khóa cho \"%s\".", len(EffectiveKeywords(a.cur, a.cfg.Keywords)), a.cur.Name))
}

// ---------- OCR language ----------

func (a *App) setOcrLanguages(langs []OcrLang) {
	a.ocrLangs = langs
	names := []string{"Theo Windows"}
	for _, l := range langs {
		names = append(names, fmt.Sprintf("%s (%s)", l.Name, l.Tag))
	}
	a.suppress = true
	a.langCB.SetModel(names)
	a.suppress = false
	a.refreshLangUI()
}

func (a *App) refreshLangUI() {
	if a.cur == nil || a.langCB == nil {
		return
	}
	idx := 0
	for i, l := range a.ocrLangs {
		if strings.EqualFold(l.Tag, a.cur.OcrLang) {
			idx = i + 1
		}
	}
	a.suppress = true
	a.langCB.SetCurrentIndex(idx)
	a.suppress = false
	if a.cur.OcrLang != "" && idx == 0 && len(a.ocrLangs) > 0 {
		a.setStatus("Máy chưa có OCR cho " + a.cur.OcrLang + ": đang dùng tiếng Anh. Cài thêm trong Settings → Language & region.")
	}
}

func (a *App) onLangChanged() {
	if a.suppress || a.cur == nil || a.langCB == nil {
		return
	}
	i := a.langCB.CurrentIndex() - 1
	if i >= 0 && i < len(a.ocrLangs) {
		a.cur.OcrLang = a.ocrLangs[i].Tag
	} else {
		a.cur.OcrLang = ""
	}
	a.store.SaveProfile(a.cur)
	a.updateLoop()
	a.setStatus("Đã đổi ngôn ngữ OCR cho game này.")
}
