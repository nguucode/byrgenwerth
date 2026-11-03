//go:build windows

package main

import (
	"bytes"
	_ "embed"
	"fmt"
	"image"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/lxn/walk"
	. "github.com/lxn/walk/declarative"
	"github.com/lxn/win"
)

//go:embed icon.png
var iconPNG []byte

type loopState struct {
	enabled      bool
	hp           bool
	hpZone       ROI
	hpColor      HPColor
	ocr          bool
	ocrLang      string
	hasTemplates bool
	mon          Monitor
	roi          ROI
	gen          int
}

type frameResult struct {
	gen     int
	hpRaw   float64 // -1 when not measured
	black   bool
	langs   []OcrLang // set when the OCR engine was (re)created
	small   *image.RGBA
	feat    Feature
	hasFeat bool
	ranOcr  bool
	lines   []OcrLine
	ocrErr  error
}

type App struct {
	store    *Store
	cfg      Config
	profiles []*Profile
	cur      *Profile

	tmplImgs  []image.Image
	tmplFeats []Feature
	det       *Detector
	monitors  []Monitor
	gen       int

	mu   sync.Mutex
	loop loopState
	quit chan struct{}
	wg   sync.WaitGroup

	ocrAvailable bool
	ocrInitErr   string
	migrated     bool
	dirWarn      string
	dataDirLE    *walk.LineEdit

	hk       *HotkeyManager
	hkErrors []string
	suppress bool

	mw         *walk.MainWindow
	profileCB  *walk.ComboBox
	countLbl   *walk.Label
	autoCB     *walk.CheckBox
	simLbl     *walk.Label
	thrSlider  *walk.Slider
	thrLbl     *walk.Label
	cooldownNE *walk.NumberEdit
	monCB      *walk.ComboBox
	roiLbl     *walk.Label
	tmplLB     *walk.ListBox
	tmplIV     *walk.ImageView
	tmplInfo   *walk.Label
	obsLE      *walk.LineEdit
	obsPathLE  *walk.LineEdit
	topCB      *walk.CheckBox
	soundCB    *walk.CheckBox
	obsCB      *walk.CheckBox
	overlayCB  *walk.CheckBox
	fontNE     *walk.NumberEdit
	fontCB     *walk.ComboBox
	presetCB   *walk.ComboBox
	presetLbl  *walk.Label
	presets    []Preset

	langCB      *walk.ComboBox
	ocrLangs    []OcrLang
	bossCB      *walk.CheckBox
	bossNowLbl  *walk.Label
	bossZoneLbl *walk.Label
	bossLB      *walk.ListBox
	bossList    []BossStat
	panelCB     *walk.CheckBox
	panelFontNE *walk.NumberEdit
	editPanelPB *walk.PushButton
	bossURLLE   *walk.LineEdit
	bossOverlay *Overlay
	hpCB        *walk.CheckBox
	hpLbl       *walk.Label
	hpZoneLbl   *walk.Label
	hpDet       *HPDeath
	tracker     *BossTracker
	curBoss     string
	lastBossHit string
	lastBossAt  time.Time
	boldCB      *walk.CheckBox
	colorSw     *walk.Composite
	colorLbl    *walk.Label
	outlineCB   *walk.CheckBox
	outlineNE   *walk.NumberEdit
	outlineSw   *walk.Composite
	outlineLbl  *walk.Label
	urlLE       *walk.LineEdit
	server      *OverlayServer
	swBrushes   []*walk.SolidColorBrush
	editPosPB   *walk.PushButton
	volSlider   *walk.Slider
	soundLbl    *walk.Label
	ocrCB       *walk.CheckBox
	ocrLbl      *walk.Label
	kwLE        *walk.LineEdit

	overlay    *Overlay
	sound      Sound
	statusLbl  *walk.Label
	hkLbl      *walk.Label
	previewBmp *walk.Bitmap
}

// defaultDataDir is %APPDATA%\\DeathCounter. It always holds the pointer to a
// custom data folder, if the user chose one.
func defaultDataDir() string {
	base := os.Getenv("APPDATA")
	if base == "" {
		if h, err := os.UserHomeDir(); err == nil {
			base = filepath.Join(h, "AppData", "Roaming")
		}
	}
	dir := filepath.Join(base, "DeathCounter")
	os.MkdirAll(dir, 0o755)
	return dir
}

// dataDir resolves the data folder. Data from very old versions, which lived in
// "DeathCounterData" next to the exe, is copied over once.
func dataDir() (dir string, migrated bool, warn string) {
	def := defaultDataDir()
	dir, warn = ResolveDataDir(def)
	if exe, err := os.Executable(); err == nil {
		migrated, _ = MigrateLegacyData(filepath.Join(filepath.Dir(exe), "DeathCounterData"), dir)
	}
	return dir, migrated, warn
}

// waitForPreviousInstance: after "change data folder" the app restarts itself with
// --after=<pid>; wait for the old process so hotkeys and the OBS port are free.
func waitForPreviousInstance() {
	for _, arg := range os.Args[1:] {
		if !strings.HasPrefix(arg, "--after=") {
			continue
		}
		var pid int
		fmt.Sscanf(strings.TrimPrefix(arg, "--after="), "%d", &pid)
		if pid <= 0 {
			return
		}
		p, err := os.FindProcess(pid)
		if err != nil {
			return
		}
		done := make(chan struct{})
		go func() { p.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(8 * time.Second):
		}
	}
}

func main() {
	waitForPreviousInstance()
	dir, migrated, warn := dataDir()
	a := &App{store: &Store{Dir: dir}, quit: make(chan struct{}), ocrAvailable: true, migrated: migrated, dirWarn: warn}
	a.cfg = a.store.LoadConfig()
	a.monitors = ListMonitors()
	if a.cfg.Monitor < 0 || a.cfg.Monitor >= len(a.monitors) {
		a.cfg.Monitor = 0
	}
	if err := a.buildUI(); err != nil {
		walk.MsgBox(nil, "Death Counter", "Lỗi tạo giao diện: "+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.init()
	a.mw.Run()
}

var (
	uiFont    = Font{Family: "Segoe UI", PointSize: 9}
	titleFont = Font{Family: "Segoe UI Semibold", PointSize: 9}
	hintColor = walk.RGB(110, 110, 110)
)

// title is a bold section heading.
func title(text string) Widget {
	return Label{Text: text, Font: titleFont}
}

// hint is a small gray helper line.
func hint(text string) Widget {
	return Label{Text: text, TextColor: hintColor}
}

// form lays out label/control pairs in two aligned columns.
func form(cells ...Widget) Widget {
	return Composite{Layout: Grid{Columns: 2, MarginsZero: true, Spacing: 6}, Children: cells}
}

// row puts several controls side by side inside one form cell.
func row(children ...Widget) Widget {
	return Composite{Layout: HBox{MarginsZero: true, Spacing: 6}, Children: children}
}

func (a *App) buildUI() error {
	monNames := make([]string, len(a.monitors))
	for i, m := range a.monitors {
		monNames[i] = m.Label(i)
	}
	pageLayout := VBox{Margins: Margins{Left: 10, Top: 10, Right: 10, Bottom: 10}, Spacing: 6}
	// scroll wraps a tab's content so a long tab scrolls instead of overflowing.
	scroll := func(tp TabPage) TabPage {
		return TabPage{Title: tp.Title, Layout: VBox{MarginsZero: true}, Children: []Widget{
			ScrollView{HorizontalFixed: true, Layout: tp.Layout, Children: tp.Children},
		}}
	}

	detectTab := TabPage{
		Title:  "Nhận diện",
		Layout: pageLayout,
		Children: []Widget{
			title("Tự nhận diện"),
			CheckBox{AssignTo: &a.autoCB, Text: "Bật tự nhận diện màn hình chết", OnCheckedChanged: a.onAutoChanged},
			CheckBox{AssignTo: &a.ocrCB, Text: "Đọc chữ bằng OCR (không cần ảnh mẫu)", OnCheckedChanged: a.onOcrChanged},
			form(
				Label{Text: "Trạng thái OCR"}, Label{AssignTo: &a.ocrLbl, Text: "--", EllipsisMode: EllipsisEnd},
				Label{Text: "Ngôn ngữ game"}, ComboBox{AssignTo: &a.langCB, Model: []string{"Theo Windows"}, OnCurrentIndexChanged: a.onLangChanged,
					ToolTipText: "Thiếu ngôn ngữ? Windows Settings → Time & language → Language & region → Add a language."},
				Label{Text: "Mẫu theo game"}, ComboBox{AssignTo: &a.presetCB, Model: a.presetNames(), OnCurrentIndexChanged: a.onPresetChanged},
				Label{Text: "Từ khóa"}, LineEdit{AssignTo: &a.kwLE, Text: strings.Join(a.cfg.Keywords, ", "), OnEditingFinished: a.onKeywordsChanged},
			),
			Label{AssignTo: &a.presetLbl, Text: "Cách nhau bằng dấu phẩy. Thêm * ở cuối để khớp phần đầu câu.", TextColor: hintColor},
			HSeparator{},
			title("So khớp ảnh mẫu"),
			form(
				Label{Text: "Độ giống"}, Label{AssignTo: &a.simLbl, Text: "--"},
				Label{Text: "Ngưỡng"}, row(
					Slider{AssignTo: &a.thrSlider, MinValue: 50, MaxValue: 99, Value: 80, StretchFactor: 3, OnValueChanged: a.onThresholdChanged},
					Label{AssignTo: &a.thrLbl, Text: "80%", MinSize: Size{Width: 36}},
				),
				Label{Text: "Cooldown"}, row(
					NumberEdit{AssignTo: &a.cooldownNE, MinValue: 1, MaxValue: 600, Decimals: 0, Value: 15.0, SpinButtonsVisible: true, MaxSize: Size{Width: 80}, OnValueChanged: a.onCooldownChanged},
					Label{Text: "giây"},
					HSpacer{},
				),
				Label{Text: "Màn hình game"}, ComboBox{AssignTo: &a.monCB, Model: monNames, CurrentIndex: a.cfg.Monitor, OnCurrentIndexChanged: a.onMonitorChanged},
				Label{Text: "Vùng nhận diện"}, row(
					Label{AssignTo: &a.roiLbl, Text: "Toàn màn hình", StretchFactor: 3},
					PushButton{Text: "Khoanh vùng...", OnClicked: a.editROI},
				),
			),
			HSeparator{},
			title("Ảnh mẫu"),
			Label{AssignTo: &a.tmplInfo, Text: "Chưa có mẫu.", TextColor: hintColor},
			Composite{
				Layout: HBox{MarginsZero: true, Spacing: 6},
				Children: []Widget{
					ListBox{AssignTo: &a.tmplLB, MinSize: Size{Width: 150, Height: 84}, OnCurrentIndexChanged: a.onTemplateSelected},
					ImageView{AssignTo: &a.tmplIV, Mode: ImageViewModeZoom, MinSize: Size{Width: 150, Height: 84}, StretchFactor: 2},
				},
			},
			row(
				PushButton{Text: "Chụp mẫu ngay", OnClicked: func() { a.captureTemplate(false) }},
				PushButton{Text: "Xóa mẫu đã chọn", OnClicked: a.removeTemplate},
				HSpacer{},
			),
			HSeparator{},
			title("Máu nhân vật (game không có màn chết)"),
			CheckBox{AssignTo: &a.hpCB, Text: "Đếm khi máu về 0 rồi màn hình tối đen", OnCheckedChanged: a.onHPChanged},
			form(
				Label{Text: "Thanh máu"}, row(
					Label{AssignTo: &a.hpZoneLbl, Text: "Chưa khoanh", StretchFactor: 3},
					PushButton{Text: "Khoanh thanh máu...", OnClicked: a.calibrateHP},
				),
				Label{Text: "Máu hiện tại"}, Label{AssignTo: &a.hpLbl, Text: "--"},
			),
			hint("Bấm khoanh lúc đang ĐẦY MÁU. Thanh máu gắn trên người nhân vật (Dead Space): khoanh rộng quanh chỗ nhân vật đứng."),
			VSpacer{},
		},
	}

	displayTab := TabPage{
		Title:  "Hiển thị",
		Layout: pageLayout,
		Children: []Widget{
			title("Nội dung"),
			form(
				Label{Text: "Định dạng"}, LineEdit{AssignTo: &a.obsLE, Text: a.cfg.ObsFormat, OnTextChanged: a.onObsFormatChanged},
			),
			hint("{count} = số lần chết, {game} = tên game."),
			HSeparator{},
			title("Kiểu chữ (overlay + OBS)"),
			form(
				Label{Text: "Font"}, ComboBox{AssignTo: &a.fontCB, Editable: true, Model: a.fontList(), OnCurrentIndexChanged: a.onStyleChanged, OnEditingFinished: a.onStyleChanged},
				Label{Text: "Cỡ chữ"}, row(
					NumberEdit{AssignTo: &a.fontNE, MinValue: 10, MaxValue: 300, Decimals: 0, Value: float64(a.cfg.OverlayFontSize), SpinButtonsVisible: true, MaxSize: Size{Width: 80}, OnValueChanged: a.onStyleChanged},
					Label{Text: "px"},
					CheckBox{AssignTo: &a.boldCB, Text: "Đậm", Checked: a.cfg.FontBold, OnCheckedChanged: a.onStyleChanged},
					HSpacer{},
				),
				Label{Text: "Màu chữ"}, row(
					Composite{AssignTo: &a.colorSw, MinSize: Size{Width: 40, Height: 22}, MaxSize: Size{Width: 40, Height: 22}},
					Label{AssignTo: &a.colorLbl, Text: a.cfg.TextColor, MinSize: Size{Width: 60}},
					PushButton{Text: "Chọn màu...", OnClicked: func() { a.pickColor(false) }},
					HSpacer{},
				),
				Label{Text: "Viền"}, row(
					CheckBox{AssignTo: &a.outlineCB, Text: "Bật", Checked: a.cfg.Outline, OnCheckedChanged: a.onStyleChanged},
					NumberEdit{AssignTo: &a.outlineNE, MinValue: 1, MaxValue: 30, Decimals: 0, Value: float64(maxInt(a.cfg.OutlineWidth, 1)), SpinButtonsVisible: true, MaxSize: Size{Width: 64}, OnValueChanged: a.onStyleChanged},
					Label{Text: "px"},
					HSpacer{},
				),
				Label{Text: "Màu viền"}, row(
					Composite{AssignTo: &a.outlineSw, MinSize: Size{Width: 40, Height: 22}, MaxSize: Size{Width: 40, Height: 22}},
					Label{AssignTo: &a.outlineLbl, Text: a.cfg.OutlineColor, MinSize: Size{Width: 60}},
					PushButton{Text: "Chọn màu...", OnClicked: func() { a.pickColor(true) }},
					HSpacer{},
				),
			),
			HSeparator{},
			title("Overlay trên màn hình"),
			CheckBox{AssignTo: &a.overlayCB, Text: "Hiện overlay nổi trên game", Checked: a.cfg.OverlayEnabled, OnCheckedChanged: a.onOverlayEnabledChanged},
			form(
				Label{Text: "Vị trí"}, row(
					PushButton{AssignTo: &a.editPosPB, Text: "Chỉnh vị trí overlay", OnClicked: a.toggleOverlayEdit},
					PushButton{Text: "Đặt lại", OnClicked: a.resetOverlayPos},
					HSpacer{},
				),
			),
			hint("OBS chỉ thấy overlay này khi dùng Display Capture."),
			HSeparator{},
			title("OBS"),
			form(
				Label{Text: "Browser Source"}, row(
					LineEdit{AssignTo: &a.urlLE, ReadOnly: true, Text: "(đang khởi động...)", StretchFactor: 3},
					PushButton{Text: "Copy", OnClicked: func() {
						if a.server != nil {
							walk.Clipboard().SetText(a.server.URL())
							a.setStatus("Đã copy URL. OBS: Sources → + → Browser → dán vào ô URL.")
						}
					}},
				),
			),
			hint("Giữ đúng kiểu chữ, dùng được với Game Capture. Đặt Width 1200, Height 250."),
			CheckBox{AssignTo: &a.obsCB, Text: "Ghi thêm file text (Text source, không có style)", Checked: a.cfg.ObsEnabled, OnCheckedChanged: a.onObsEnabledChanged},
			form(
				Label{Text: "File"}, row(
					LineEdit{AssignTo: &a.obsPathLE, ReadOnly: true, Text: a.store.ObsPath(), StretchFactor: 3},
					PushButton{Text: "Copy", OnClicked: func() {
						walk.Clipboard().SetText(a.store.ObsPath())
						a.setStatus("Đã copy đường dẫn file OBS.")
					}},
					PushButton{Text: "Mở thư mục", OnClicked: a.openDataDir},
				),
			),
			VSpacer{},
		},
	}

	soundTab := TabPage{
		Title:  "Cài đặt",
		Layout: pageLayout,
		Children: []Widget{
			title("Âm thanh xác nhận"),
			CheckBox{AssignTo: &a.soundCB, Text: "Phát âm thanh mỗi lần đếm +1", Checked: a.cfg.Sound, OnCheckedChanged: func() {
				if a.soundCB == nil {
					return
				}
				a.cfg.Sound = a.soundCB.Checked()
				a.saveConfig()
			}},
			form(
				Label{Text: "Âm lượng"}, row(
					Slider{AssignTo: &a.volSlider, MinValue: 0, MaxValue: 100, Value: a.cfg.SoundVolume, StretchFactor: 3, OnValueChanged: a.onVolumeChanged},
					PushButton{Text: "Nghe thử", OnClicked: a.testSound},
				),
				Label{Text: "File âm thanh"}, row(
					Label{AssignTo: &a.soundLbl, Text: "Chưa chọn", StretchFactor: 3, EllipsisMode: EllipsisPath},
					PushButton{Text: "Chọn file...", OnClicked: a.chooseSound},
					PushButton{Text: "Bỏ âm thanh", OnClicked: func() { a.setSoundFile("") }},
				),
			),
			hint("App không kèm sẵn âm thanh: tự chọn file .mp3 hoặc .wav của bạn. Âm lượng chỉ áp dụng cho mp3."),
			HSeparator{},
			title("Dữ liệu"),
			form(
				Label{Text: "Thư mục"}, row(
					LineEdit{AssignTo: &a.dataDirLE, ReadOnly: true, Text: a.store.Dir, StretchFactor: 3},
					PushButton{Text: "Đổi...", OnClicked: a.chooseDataDir},
					PushButton{Text: "Mặc định", OnClicked: func() { a.switchDataDir(defaultDataDir()) }},
				),
			),
			hint("Gồm số đếm, ảnh mẫu, thống kê boss, cài đặt. Đổi thư mục: app chép dữ liệu sang rồi tự mở lại; dữ liệu ở chỗ cũ được giữ nguyên."),
			row(
				PushButton{Text: "Mở thư mục dữ liệu", OnClicked: a.openDataDir},
				HSpacer{},
			),
			VSpacer{},
		},
	}

	bossTab := TabPage{
		Title:  "Boss",
		Layout: pageLayout,
		Children: []Widget{
			title("Theo dõi boss"),
			CheckBox{AssignTo: &a.bossCB, Text: "Ghi nhận boss khi chết (OCR tên trên thanh máu)", OnCheckedChanged: a.onBossTrackChanged},
			form(
				Label{Text: "Đang đánh"}, Label{AssignTo: &a.bossNowLbl, Text: "--", EllipsisMode: EllipsisEnd},
				Label{Text: "Vùng tên boss"}, row(
					Label{AssignTo: &a.bossZoneLbl, Text: "Tự động", StretchFactor: 3},
					PushButton{Text: "Khoanh vùng...", OnClicked: a.editBossZone},
					PushButton{Text: "Tự động", OnClicked: func() {
						if a.cur != nil {
							a.cur.BossROI = ROI{}
							a.store.SaveProfile(a.cur)
							a.refreshBossZoneLabel()
						}
					}},
				),
			),
			hint("Tên boss đứng yên trên màn hình vài giây mới được ghi nhận, phụ đề sẽ bị bỏ qua."),
			HSeparator{},
			title("Thống kê"),
			ListBox{AssignTo: &a.bossLB, MinSize: Size{Height: 110}},
			row(
				PushButton{Text: "Đổi tên...", OnClicked: a.renameBoss},
				PushButton{Text: "Gộp vào...", OnClicked: a.mergeBoss},
				PushButton{Text: "Xóa", OnClicked: a.removeBoss},
				HSpacer{},
				PushButton{Text: "Xóa hết", OnClicked: a.clearBosses},
			),
			HSeparator{},
			title("Bảng Top 3"),
			CheckBox{AssignTo: &a.panelCB, Text: "Hiện bảng nổi trên màn hình", Checked: a.cfg.BossPanelEnabled, OnCheckedChanged: a.onPanelEnabledChanged},
			form(
				Label{Text: "Cỡ chữ"}, row(
					NumberEdit{AssignTo: &a.panelFontNE, MinValue: 10, MaxValue: 80, Decimals: 0, Value: float64(a.cfg.BossPanelFontSize), SpinButtonsVisible: true, MaxSize: Size{Width: 80}, OnValueChanged: a.onPanelFontChanged},
					Label{Text: "px"},
					HSpacer{},
				),
				Label{Text: "Vị trí"}, row(
					PushButton{AssignTo: &a.editPanelPB, Text: "Chỉnh vị trí bảng", OnClicked: a.togglePanelEdit},
					PushButton{Text: "Đặt lại", OnClicked: a.resetPanelPos},
					HSpacer{},
				),
				Label{Text: "OBS"}, row(
					LineEdit{AssignTo: &a.bossURLLE, ReadOnly: true, Text: "(đang khởi động...)", StretchFactor: 3},
					PushButton{Text: "Copy", OnClicked: func() {
						if a.server != nil {
							walk.Clipboard().SetText(a.server.BossURL())
							a.setStatus("Đã copy URL bảng boss. OBS: Sources → + → Browser, Width 600, Height 300.")
						}
					}},
				),
			),
			hint("Kiểu bảng speedrun: hạng 1 vàng, boss đang đánh tô xanh."),
		},
	}

	return MainWindow{
		AssignTo: &a.mw,
		Title:    "Death Counter",
		Font:     uiFont,
		Size:     Size{Width: 480, Height: 850},
		MinSize:  Size{Width: 440, Height: 640},
		Layout:   VBox{Margins: Margins{Left: 12, Top: 10, Right: 12, Bottom: 10}, Spacing: 8},
		Children: []Widget{
			title("Game"),
			row(
				ComboBox{AssignTo: &a.profileCB, StretchFactor: 3, OnCurrentIndexChanged: a.onProfileChanged},
				PushButton{Text: "+ Thêm", OnClicked: a.addProfile},
				PushButton{Text: "Đổi tên", OnClicked: a.renameProfile},
				PushButton{Text: "Xóa", OnClicked: a.deleteProfile},
			),
			HSeparator{},
			row(
				Composite{
					Layout: VBox{MarginsZero: true, SpacingZero: true},
					Children: []Widget{
						Label{Text: "Số lần chết", Font: titleFont},
						Label{
							AssignTo:  &a.countLbl,
							Text:      "0",
							Font:      Font{Family: "Segoe UI", PointSize: 36, Bold: true},
							TextColor: walk.RGB(200, 30, 30),
						},
					},
				},
				HSpacer{},
				PushButton{Text: "+1", MinSize: Size{Width: 56}, OnClicked: func() { a.addDeath(1, "button") }},
				PushButton{Text: "−1", MinSize: Size{Width: 56}, OnClicked: func() { a.addDeath(-1, "button") }},
				PushButton{Text: "Reset", OnClicked: a.resetCount},
			),
			TabWidget{Pages: []TabPage{scroll(detectTab), scroll(displayTab), scroll(soundTab), scroll(bossTab)}},
			HSeparator{},
			CheckBox{AssignTo: &a.topCB, Text: "Cửa sổ app luôn nằm trên", Checked: a.cfg.AlwaysOnTop, OnCheckedChanged: a.onTopChanged},
			Label{AssignTo: &a.hkLbl, Text: "", TextColor: hintColor},
			Label{AssignTo: &a.statusLbl, Text: "", EllipsisMode: EllipsisEnd},
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
	a.tracker = NewBossTracker()
	if srv, err := StartOverlayServer(a.cfg.ServerPort); err == nil {
		a.server = srv
		a.urlLE.SetText(srv.URL())
		a.bossURLLE.SetText(srv.BossURL())
	} else {
		a.urlLE.SetText("Không mở được: " + err.Error())
	}
	a.refreshSwatches()
	a.suppress = true
	a.fontCB.SetText(a.cfg.FontFamily)
	a.suppress = false
	a.outlineNE.SetEnabled(a.cfg.Outline)
	a.initOverlay()
	a.initBossPanel()

	var err error
	a.profiles, err = a.store.LoadProfiles()
	if err != nil {
		a.setStatus("Lỗi đọc dữ liệu: " + err.Error())
	}
	if len(a.profiles) == 0 {
		if p, err := a.store.CreateProfile("Game mới"); err == nil {
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

	h := a.cfg.Hotkeys
	keys := map[HotkeyAction]string{ActAdd: h.Add, ActSub: h.Sub, ActCapture: h.Capture, ActToggle: h.Toggle}
	a.hk, a.hkErrors = StartHotkeys(keys, func(act HotkeyAction) {
		a.mw.Synchronize(func() { a.onHotkey(act) })
	})
	a.hkLbl.SetText(fmt.Sprintf("Phím tắt:  %s  +1   ·   %s  −1   ·   %s  chụp mẫu   ·   %s  bật/tắt nhận diện", h.Add, h.Sub, h.Capture, h.Toggle))
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
		close(a.quit)
		a.wg.Wait()
		a.hk.Stop()
		a.overlay.Destroy()
		a.bossOverlay.Destroy()
		a.server.Close()
		a.sound.Close()
		a.saveConfig()
	})

	a.wg.Add(2)
	go a.captureLoop()
	go func() { // keep overlay above borderless games
		defer a.wg.Done()
		for {
			select {
			case <-a.quit:
				return
			case <-time.After(3 * time.Second):
				a.mw.Synchronize(func() {
					if a.cfg.OverlayEnabled && !a.overlay.Editing() {
						a.overlay.KeepOnTop()
					}
					if a.cfg.BossPanelEnabled && !a.bossOverlay.Editing() {
						a.bossOverlay.KeepOnTop()
					}
				})
			}
		}
	}()
}

// ---------- capture loop ----------

func (a *App) captureLoop() {
	defer a.wg.Done()
	// WinRT OCR objects live on this thread for its whole life.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var ocr *OcrEngine
	defer func() { ocr.Close() }()
	ocrTried := false

	interval := time.Duration(a.cfg.IntervalMs) * time.Millisecond
	lastErr := ""
	for {
		select {
		case <-a.quit:
			return
		case <-time.After(interval):
		}
		a.mu.Lock()
		st := a.loop
		a.mu.Unlock()
		if !st.enabled {
			continue
		}
		var newLangs []OcrLang
		if st.ocr && (!ocrTried || (ocr != nil && ocr.Lang != st.ocrLang)) {
			ocrTried = true
			ocr.Close()
			var err error
			ocr, err = NewOcrEngine(st.ocrLang)
			if err != nil {
				ocr = nil
				msg := err.Error()
				a.mw.Synchronize(func() { a.onOcrUnavailable(msg) })
				continue
			}
			newLangs = ocr.Langs
		}

		fr := frameResult{gen: st.gen, langs: newLangs, hpRaw: -1}
		var err error
		if (st.ocr && ocr != nil) || st.hp {
			ow, oh := OcrSize(st.mon)
			var big *image.RGBA
			big, err = CaptureMonitorScaled(st.mon, ow, oh)
			if err == nil {
				fr.small = Downscale(big, CaptureW, CaptureH)
				if st.ocr && ocr != nil {
					fr.lines, fr.ocrErr = ocr.Recognize(big)
					fr.ranOcr = true
				}
				if st.hp {
					fr.hpRaw = MeasureHP(big, st.hpZone, st.hpColor)
				}
			}
		} else {
			fr.small, err = CaptureMonitor(st.mon)
		}
		if err == nil {
			fr.black = IsBlack(fr.small)
		}
		if err != nil {
			if err.Error() != lastErr {
				lastErr = err.Error()
				msg := err.Error()
				a.mw.Synchronize(func() { a.setStatus("Lỗi chụp màn hình: " + msg) })
			}
			continue
		}
		lastErr = ""
		if st.hasTemplates {
			fr.feat = ExtractFeature(fr.small, st.roi)
			fr.hasFeat = true
		}
		a.mw.Synchronize(func() { a.onFrame(fr) })
	}
}

func (a *App) onOcrUnavailable(msg string) {
	a.ocrAvailable = false
	a.ocrInitErr = msg
	a.ocrLbl.SetText("Không dùng được: " + msg + ". Vẫn dùng được F9 + ảnh mẫu.")
	a.updateLoop()
}

func describeOcr(lines []OcrLine, m OcrMatch, hit bool) string {
	if hit {
		return fmt.Sprintf("Thấy \"%s\" (cao %.1f%%) → KHỚP", m.Line, m.Height*100)
	}
	best, bh := "", 0.0
	for _, ln := range lines {
		for _, w := range ln.Words {
			if w.H > bh {
				bh, best = w.H, ln.Text
			}
		}
	}
	if best == "" {
		return "Không thấy chữ"
	}
	r := []rune(best)
	if len(r) > 40 {
		best = string(r[:40]) + "…"
	}
	return fmt.Sprintf("Chữ lớn nhất \"%s\" (%.1f%%), không phải từ khóa", best, bh*100)
}

func (a *App) onFrame(fr frameResult) {
	if fr.gen != a.gen || a.cur == nil {
		return
	}
	now := time.Now()
	sim, idx := 0.0, -1
	if fr.hasFeat {
		sim, idx = BestSimilarity(fr.feat, a.tmplFeats)
		a.simLbl.SetText(fmt.Sprintf("%d%%  (ngưỡng %d%%)", int(sim*100), int(a.cur.Threshold*100)))
	}
	if fr.langs != nil {
		a.setOcrLanguages(fr.langs)
	}
	if fr.hpRaw >= 0 && a.hpDet != nil {
		ratio := a.hpDet.Ratio(fr.hpRaw)
		if a.hpDet.Full > a.cur.HPFull*1.05 { // calibration grew: remember it
			a.cur.HPFull = a.hpDet.Full
			a.store.SaveProfile(a.cur)
		}
		switch {
		case fr.black:
			a.hpLbl.SetText("-- (màn hình đen)")
		case ratio < 0:
			a.hpLbl.SetText("-- (chưa thấy thanh máu)")
		default:
			a.hpLbl.SetText(fmt.Sprintf("%d%%", int(ratio*100+0.5)))
		}
		if a.hpDet.Feed(ratio, fr.black, now) && a.det.Trigger(now) {
			a.addDeath(1, "hp")
			a.setStatus(fmt.Sprintf("Tự đếm lúc %s (máu về 0 rồi màn hình tối). Đếm sai thì bấm %s.", now.Format("15:04:05"), a.cfg.Hotkeys.Sub))
		}
	}
	var m OcrMatch
	hit := false
	if fr.ranOcr {
		if fr.ocrErr != nil {
			a.ocrLbl.SetText("Lỗi: " + fr.ocrErr.Error())
		} else {
			kw := EffectiveKeywords(a.cur, a.cfg.Keywords)
			if a.cur.OcrEnabled {
				m, hit = MatchDeath(fr.lines, kw, a.cfg.OcrMinHeight)
				a.ocrLbl.SetText(describeOcr(fr.lines, m, hit))
			}
			if a.cur.BossTrack {
				a.tracker.Observe(BossCandidates(fr.lines, a.bossZones(), kw), now)
				b, _ := a.tracker.Current(now)
				if b != a.curBoss {
					a.curBoss = b
					a.refreshBosses()
				}
			}
		}
	}
	eff := sim
	if hit {
		eff = 1
	}
	if !a.det.Feed(eff, now) {
		return
	}
	a.addDeath(1, "auto")
	var why string
	if hit {
		why = fmt.Sprintf("đọc được chữ \"%s\"", m.Line)
	} else {
		why = fmt.Sprintf("giống mẫu #%d: %d%%", idx+1, int(sim*100))
	}
	msg := fmt.Sprintf("Tự đếm lúc %s (%s). Đếm sai thì bấm %s.", now.Format("15:04:05"), why, a.cfg.Hotkeys.Sub)
	if hit {
		msg += a.learnFromOcr(fr.small, m, sim)
	}
	a.setStatus(msg)
}

// learnFromOcr turns an OCR-detected death into a template (and a tight ROI the
// first time), so stylized frames OCR misreads later are still caught.
func (a *App) learnFromOcr(small *image.RGBA, m OcrMatch, sim float64) string {
	if small == nil || (len(a.tmplFeats) > 0 && sim >= a.cur.Threshold) {
		return ""
	}
	note := ""
	if a.cur.ROI.IsFull() {
		a.cur.ROI = AutoROI(m.Box)
		a.store.SaveProfile(a.cur)
		a.refreshROILabel()
		note = " Đã tự khoanh vùng quanh chữ."
	}
	if _, err := a.store.AddTemplate(a.cur, small, time.Now()); err != nil {
		return " (không lưu được ảnh mẫu: " + err.Error() + ")"
	}
	a.reloadTemplates()
	return fmt.Sprintf(" Tự học: đã lưu ảnh mẫu %d/%d.%s", len(a.cur.Templates), MaxTemplates, note)
}

func (a *App) updateLoop() {
	a.gen++
	st := loopState{gen: a.gen}
	if a.cur != nil {
		st.hasTemplates = len(a.tmplFeats) > 0
		st.ocr = (a.cur.OcrEnabled || a.cur.BossTrack) && a.ocrAvailable
		st.ocrLang = a.cur.OcrLang
		st.hp = a.cur.HPEnabled && a.cur.HPColor.Valid()
		st.hpZone, st.hpColor = a.cur.HPZone, a.cur.HPColor
		st.enabled = a.cur.AutoEnabled && (st.hasTemplates || st.ocr || st.hp)
		st.roi = a.cur.ROI
	}
	if a.cfg.Monitor >= 0 && a.cfg.Monitor < len(a.monitors) {
		st.mon = a.monitors[a.cfg.Monitor]
	} else if len(a.monitors) > 0 {
		st.mon = a.monitors[0]
	}
	a.mu.Lock()
	a.loop = st
	a.mu.Unlock()
	if a.cur == nil {
		return
	}
	if !st.hasTemplates {
		a.simLbl.SetText("-- (chưa có mẫu, sẽ tự học khi OCR bắt được)")
	}
	switch {
	case !a.cur.AutoEnabled:
		a.simLbl.SetText("-- (tự nhận diện đang tắt)")
		a.ocrLbl.SetText("--")
	case !a.ocrAvailable && a.ocrInitErr != "":
		// keep the error text set by onOcrUnavailable
	case !a.cur.OcrEnabled:
		a.ocrLbl.SetText("Đang tắt")
	case !st.enabled:
		a.ocrLbl.SetText("--")
	}
}

func (a *App) currentMonitor() Monitor {
	if a.cfg.Monitor >= 0 && a.cfg.Monitor < len(a.monitors) {
		return a.monitors[a.cfg.Monitor]
	}
	return a.monitors[0]
}

// ---------- hotkeys ----------

func (a *App) onHotkey(act HotkeyAction) {
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

func (a *App) addDeath(delta int, source string) {
	if a.cur == nil {
		return
	}
	if a.cur.Deaths+delta < 0 {
		return
	}
	a.cur.Deaths += delta
	if delta > 0 && a.cfg.Sound {
		a.playSound()
	}
	if a.cur.BossTrack && a.tracker != nil {
		now := time.Now()
		if delta > 0 {
			a.lastBossHit = ""
			if b, ok := a.tracker.Current(now); ok {
				a.lastBossHit = AddBossDeath(a.cur, b, 1)
				a.lastBossAt = now
			}
		} else if a.lastBossHit != "" && now.Sub(a.lastBossAt) < 3*time.Minute {
			AddBossDeath(a.cur, a.lastBossHit, -1) // undo the boss credit with the −1
			a.lastBossHit = ""
		}
	}
	a.store.SaveProfile(a.cur)
	a.store.AppendHistory(time.Now(), a.cur.Name, delta, a.cur.Deaths, source)
	a.refreshCount()
	a.refreshBosses()
}

func (a *App) refreshCount() {
	if a.cur == nil {
		return
	}
	a.countLbl.SetText(fmt.Sprint(a.cur.Deaths))
	if a.cfg.ObsEnabled {
		if err := a.store.WriteObs(a.cfg.ObsFormat, a.cur.Name, a.cur.Deaths); err != nil {
			a.setStatus("Không ghi được file OBS: " + err.Error())
		}
	}
	text := FormatObs(a.cfg.ObsFormat, a.cur.Name, a.cur.Deaths)
	if a.overlay != nil {
		a.overlay.Text = text
		a.overlay.Render()
	}
	a.server.Set(StateFromConfig(a.cfg, text))
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

func templateLabel(i int, name string) string {
	var d, t string
	var ms int
	if n, _ := fmt.Sscanf(strings.TrimSuffix(name, ".png"), "death_%8s_%6s_%d", &d, &t, &ms); n >= 2 && len(d) == 8 && len(t) == 6 {
		return fmt.Sprintf("#%d  %s/%s %s:%s:%s", i+1, d[6:8], d[4:6], t[0:2], t[2:4], t[4:6])
	}
	return fmt.Sprintf("#%d  %s", i+1, name)
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
		a.tmplInfo.SetText("Chưa có mẫu. OCR bắt được sẽ tự lưu; game có chữ chết khó đọc thì bấm " + a.cfg.Hotkeys.Add + " lần đầu.")
		return
	}
	s := fmt.Sprintf("%d/%d mẫu.", n, MaxTemplates)
	if ag := TemplateAgreement(a.tmplFeats); ag >= 0 {
		s += fmt.Sprintf(" Các mẫu khớp nhau: %d%%", int(ag*100))
		if ag < a.cur.Threshold {
			s += " → vùng nhận diện còn rộng hoặc có mẫu chụp sai, hãy khoanh sát hơn / xóa mẫu lạ."
		}
	}
	a.tmplInfo.SetText(s)
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
	a.bossCB.SetChecked(p.BossTrack)
	a.hpCB.SetChecked(p.HPEnabled)
	a.thrSlider.SetValue(int(p.Threshold*100 + 0.5))
	a.thrLbl.SetText(fmt.Sprintf("%d%%", int(p.Threshold*100+0.5)))
	a.cooldownNE.SetValue(p.CooldownSec)
	a.suppress = false
	a.cfg.CurrentProfile = p.ID
	a.saveConfig()
	a.refreshROILabel()
	a.refreshKeywordsUI()
	a.refreshCount()
	a.reloadTemplates()
	if a.tracker != nil {
		a.tracker.Reset()
	}
	a.curBoss, a.lastBossHit = "", ""
	a.refreshLangUI()
	a.refreshBossZoneLabel()
	a.hpDet = NewHPDeath(p.HPFull)
	a.refreshHPLabel()
	a.refreshBosses()
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
	if i := FindPreset(a.presets, p.Name); i >= 0 {
		p.Preset = a.presets[i].Game
		p.Keywords = append([]string(nil), a.presets[i].Keywords...)
		a.store.SaveProfile(p)
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
		if p, err := a.store.CreateProfile("Game mới"); err == nil {
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
	if a.thrSlider == nil || a.thrLbl == nil {
		return
	}
	v := a.thrSlider.Value()
	a.thrLbl.SetText(fmt.Sprintf("%d%%", v))
	if a.suppress || a.cur == nil {
		return
	}
	a.cur.Threshold = float64(v) / 100
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
}

func (a *App) onObsFormatChanged() {
	if a.obsLE == nil {
		return
	}
	a.cfg.ObsFormat = a.obsLE.Text()
	a.saveConfig()
	a.refreshCount()
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
		rs = append(rs, [4]int32{m.Rect.Left, m.Rect.Top, m.Rect.Right, m.Rect.Bottom})
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
		a.setStatus("Đã bật overlay. Bấm \"Chỉnh vị trí overlay\" để kéo nó tới chỗ muốn đặt.")
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

func (a *App) refreshSwatches() {
	set := func(sw *walk.Composite, lbl *walk.Label, hex string) {
		if sw == nil {
			return
		}
		c := MustHex(hex, RGB{})
		if br, err := walk.NewSolidColorBrush(walk.RGB(c.R, c.G, c.B)); err == nil {
			sw.SetBackground(br)
			a.swBrushes = append(a.swBrushes, br)
		}
		lbl.SetText(c.Hex())
	}
	set(a.colorSw, a.colorLbl, a.cfg.TextColor)
	set(a.outlineSw, a.outlineLbl, a.cfg.OutlineColor)
	// keep only the two brushes in use
	for len(a.swBrushes) > 2 {
		a.swBrushes[0].Dispose()
		a.swBrushes = a.swBrushes[1:]
	}
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
		a.editPosPB.SetText("Chỉnh vị trí overlay")
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
	a.editPosPB.SetText("Xong, lưu vị trí")
	a.setStatus("Kéo khung viền vàng tới vị trí bất kỳ trên màn hình, rồi bấm \"Xong, lưu vị trí\".")
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
	if a.soundLbl != nil {
		a.soundLbl.SetText(label)
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
	if a.volSlider == nil {
		return
	}
	a.cfg.SoundVolume = a.volSlider.Value()
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

func (a *App) presetNames() []string {
	a.presets = LoadPresets()
	names := []string{"Chung (mọi game)"}
	for _, p := range a.presets {
		names = append(names, p.Game)
	}
	return names
}

func (a *App) presetIndex(name string) int {
	for i, p := range a.presets {
		if p.Game == name {
			return i
		}
	}
	return -1
}

// refreshKeywordsUI syncs the preset combo, keyword box and hint with the profile.
func (a *App) refreshKeywordsUI() {
	if a.cur == nil || a.presetCB == nil {
		return
	}
	a.suppress = true
	defer func() { a.suppress = false }()
	i := a.presetIndex(a.cur.Preset)
	a.presetCB.SetCurrentIndex(i + 1) // 0 = generic
	a.kwLE.SetText(strings.Join(EffectiveKeywords(a.cur, a.cfg.Keywords), ", "))
	if i >= 0 {
		a.presetLbl.SetText(a.presets[i].HintText())
	} else {
		a.presetLbl.SetText("Cách nhau bằng dấu phẩy. Thêm * ở cuối để khớp phần đầu câu.")
	}
}

func (a *App) onPresetChanged() {
	if a.suppress || a.cur == nil || a.presetCB == nil {
		return
	}
	i := a.presetCB.CurrentIndex() - 1
	if i >= 0 && i < len(a.presets) {
		a.cur.Preset = a.presets[i].Game
		a.cur.Keywords = append([]string(nil), a.presets[i].Keywords...)
	} else {
		a.cur.Preset = ""
		a.cur.Keywords = nil
	}
	a.store.SaveProfile(a.cur)
	a.det.Reset()
	a.tracker.Reset()
	a.refreshKeywordsUI()
	a.refreshBossZoneLabel()
	a.setStatus("Đã áp dụng mẫu theo game.")
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
	a.tracker.Reset()
	a.updateLoop()
	a.setStatus("Đã đổi ngôn ngữ OCR cho game này.")
}

// ---------- boss tracking ----------

// bossZones: the user's region, else the game preset's, else the defaults.
func (a *App) bossZones() []ROI {
	if a.cur != nil && a.cur.BossROI.W > 0 && a.cur.BossROI.H > 0 {
		return []ROI{a.cur.BossROI.Normalized()}
	}
	if z := a.presetBossZone(); z != nil {
		return []ROI{*z}
	}
	return DefaultBossZones
}

func (a *App) presetBossZone() *ROI {
	if a.cur == nil {
		return nil
	}
	if i := a.presetIndex(a.cur.Preset); i >= 0 && a.presets[i].BossZone != nil {
		z := a.presets[i].BossZone.Normalized()
		return &z
	}
	return nil
}

func (a *App) refreshBossZoneLabel() {
	if a.cur == nil || a.bossZoneLbl == nil {
		return
	}
	if a.cur.BossROI.W > 0 && a.cur.BossROI.H > 0 {
		r := a.cur.BossROI
		a.bossZoneLbl.SetText(fmt.Sprintf("x %d%%, y %d%%, %d×%d%%", int(r.X*100), int(r.Y*100), int(r.W*100), int(r.H*100)))
	} else if a.presetBossZone() != nil {
		a.bossZoneLbl.SetText("Theo mẫu game (" + a.cur.Preset + ")")
	} else {
		a.bossZoneLbl.SetText("Tự động: trên + dưới")
	}
}

func (a *App) editBossZone() {
	if a.cur == nil {
		return
	}
	init := a.cur.BossROI
	if init.W <= 0 {
		if z := a.presetBossZone(); z != nil {
			init = *z
		} else {
			init = DefaultBossZones[0]
		}
	}
	a.editRegion("Vùng tên boss - "+a.cur.Name,
		"Kéo chuột khoanh vùng có TÊN BOSS (thường ngay trên thanh máu boss).\nĐể rộng một chút cho tên dài; tránh lấy phụ đề.",
		init, false, func(r ROI) {
			a.cur.BossROI = r
			a.store.SaveProfile(a.cur)
			a.tracker.Reset()
			a.refreshBossZoneLabel()
			a.setStatus("Đã lưu vùng tên boss.")
		})
}

func (a *App) onBossTrackChanged() {
	if a.suppress || a.cur == nil || a.bossCB == nil {
		return
	}
	a.cur.BossTrack = a.bossCB.Checked()
	a.store.SaveProfile(a.cur)
	a.tracker.Reset()
	a.curBoss = ""
	a.updateLoop()
	a.refreshBosses()
}

// refreshBosses updates the list, the on-screen panel and the OBS page.
func (a *App) refreshBosses() {
	if a.cur == nil || a.bossLB == nil {
		return
	}
	a.bossList = SortedBosses(a.cur)
	items := make([]string, len(a.bossList))
	for i, b := range a.bossList {
		items[i] = fmt.Sprintf("%3d   %s", b.Deaths, b.Name)
	}
	sel := a.bossLB.CurrentIndex()
	a.suppress = true
	a.bossLB.SetModel(items)
	if sel >= 0 && sel < len(items) {
		a.bossLB.SetCurrentIndex(sel)
	}
	a.suppress = false

	now := "--"
	if !a.cur.BossTrack {
		now = "(đang tắt)"
	} else if a.curBoss != "" {
		now = a.curBoss
	}
	a.bossNowLbl.SetText(now)

	st := SplitsState{
		Title: a.cur.Name, Rows: BuildSplits(a.cur, a.curBoss, 3), Total: a.cur.Deaths,
		FontSize: a.cfg.BossPanelFontSize, Font: a.cfg.FontFamily,
	}
	a.server.SetSplits(st)
	if a.bossOverlay != nil {
		a.bossOverlay.Splits = &st
		a.bossOverlay.FontSize = a.cfg.BossPanelFontSize
		a.bossOverlay.FontFamily = a.cfg.FontFamily
		a.bossOverlay.Render()
	}
}

func (a *App) selectedBoss() (BossStat, bool) {
	i := a.bossLB.CurrentIndex()
	if a.cur == nil || i < 0 || i >= len(a.bossList) {
		a.setStatus("Chọn một boss trong danh sách trước.")
		return BossStat{}, false
	}
	return a.bossList[i], true
}

func (a *App) renameBoss() {
	b, ok := a.selectedBoss()
	if !ok {
		return
	}
	name, ok := a.prompt("Đổi tên boss", "Tên đúng của boss:", b.Name)
	if !ok || strings.TrimSpace(name) == "" {
		return
	}
	RenameBoss(a.cur, b.Name, name)
	a.store.SaveProfile(a.cur)
	a.refreshBosses()
}

func (a *App) mergeBoss() {
	b, ok := a.selectedBoss()
	if !ok {
		return
	}
	var others []string
	for _, x := range a.bossList {
		if x.Name != b.Name {
			others = append(others, x.Name)
		}
	}
	if len(others) == 0 {
		a.setStatus("Không có boss nào khác để gộp.")
		return
	}
	i, ok := a.promptChoice("Gộp boss", fmt.Sprintf("Gộp \"%s\" (%d lần) vào:", b.Name, b.Deaths), others)
	if !ok {
		return
	}
	MergeBoss(a.cur, b.Name, others[i])
	a.store.SaveProfile(a.cur)
	a.refreshBosses()
	a.setStatus("Đã gộp vào \"" + others[i] + "\".")
}

func (a *App) removeBoss() {
	b, ok := a.selectedBoss()
	if !ok {
		return
	}
	if walk.MsgBox(a.mw, "Xóa boss", fmt.Sprintf("Xóa \"%s\" khỏi thống kê?\n(Tổng số lần chết không đổi.)", b.Name), walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
		return
	}
	RemoveBoss(a.cur, b.Name)
	a.store.SaveProfile(a.cur)
	a.refreshBosses()
}

func (a *App) clearBosses() {
	if a.cur == nil || len(a.cur.Bosses) == 0 {
		return
	}
	if walk.MsgBox(a.mw, "Xóa hết", "Xóa toàn bộ thống kê boss của game này?\n(Tổng số lần chết không đổi.)", walk.MsgBoxYesNo|walk.MsgBoxIconWarning) != walk.DlgCmdYes {
		return
	}
	a.cur.Bosses = nil
	a.store.SaveProfile(a.cur)
	a.refreshBosses()
}

// promptChoice asks the user to pick one option; the index is read before the dialog closes.
func (a *App) promptChoice(title, label string, options []string) (int, bool) {
	var dlg *walk.Dialog
	var cb *walk.ComboBox
	var okPB, cancelPB *walk.PushButton
	result := 0
	res, err := Dialog{
		AssignTo: &dlg, Title: title, DefaultButton: &okPB, CancelButton: &cancelPB,
		MinSize: Size{Width: 360, Height: 130}, Layout: VBox{}, Font: uiFont,
		Children: []Widget{
			Label{Text: label},
			ComboBox{AssignTo: &cb, Model: options, CurrentIndex: 0},
			Composite{Layout: HBox{MarginsZero: true}, Children: []Widget{
				HSpacer{},
				PushButton{AssignTo: &okPB, Text: "OK", OnClicked: func() {
					result = cb.CurrentIndex()
					dlg.Accept()
				}},
				PushButton{AssignTo: &cancelPB, Text: "Hủy", OnClicked: func() { dlg.Cancel() }},
			}},
		},
	}.Run(a.mw)
	if err != nil || res != walk.DlgCmdOK || result < 0 {
		return 0, false
	}
	return result, true
}

// ---------- boss panel overlay ----------

func (a *App) initBossPanel() {
	x, y, ok := ClampToMonitors(a.cfg.BossPanelX, a.cfg.BossPanelY, a.monitorRects())
	if !a.cfg.BossPanelPosSet || !ok {
		x, y = a.defaultPanelPos()
	}
	o, err := NewOverlay(x, y)
	if err != nil {
		a.setStatus("Không tạo được bảng boss: " + err.Error())
		return
	}
	a.bossOverlay = o
	o.FontSize = a.cfg.BossPanelFontSize
	o.FontFamily = a.cfg.FontFamily
	o.Splits = &SplitsState{}
	o.OnMoved = func(x, y int32) {
		a.cfg.BossPanelX, a.cfg.BossPanelY, a.cfg.BossPanelPosSet = x, y, true
		a.saveConfig()
	}
	o.Render()
	o.Show(a.cfg.BossPanelEnabled)
}

// defaultPanelPos: top-right area of the primary monitor.
func (a *App) defaultPanelPos() (int32, int32) {
	if len(a.monitors) == 0 {
		return 50, 50
	}
	r := a.monitors[0].Rect
	return r.Right - int32(a.cfg.BossPanelFontSize*22) - 40, r.Top + 40
}

func (a *App) onPanelEnabledChanged() {
	if a.panelCB == nil {
		return
	}
	a.cfg.BossPanelEnabled = a.panelCB.Checked()
	a.saveConfig()
	if !a.cfg.BossPanelEnabled && a.bossOverlay.Editing() {
		a.togglePanelEdit()
	}
	a.refreshBosses()
	a.bossOverlay.Show(a.cfg.BossPanelEnabled)
}

func (a *App) onPanelFontChanged() {
	if a.panelFontNE == nil {
		return
	}
	a.cfg.BossPanelFontSize = int(a.panelFontNE.Value())
	a.saveConfig()
	a.refreshBosses()
}

func (a *App) togglePanelEdit() {
	if a.bossOverlay == nil {
		return
	}
	if a.bossOverlay.Editing() {
		a.bossOverlay.SetEditing(false)
		a.editPanelPB.SetText("Chỉnh vị trí bảng")
		a.cfg.BossPanelX, a.cfg.BossPanelY, a.cfg.BossPanelPosSet = a.bossOverlay.X, a.bossOverlay.Y, true
		a.saveConfig()
		a.setStatus("Đã lưu vị trí bảng boss.")
		return
	}
	if !a.cfg.BossPanelEnabled {
		a.panelCB.SetChecked(true)
	}
	a.bossOverlay.SetEditing(true)
	a.bossOverlay.Show(true)
	a.editPanelPB.SetText("Xong, lưu vị trí")
	a.setStatus("Kéo bảng boss (viền vàng) tới vị trí bất kỳ, rồi bấm \"Xong, lưu vị trí\".")
}

func (a *App) resetPanelPos() {
	if a.bossOverlay == nil {
		return
	}
	x, y := a.defaultPanelPos()
	a.bossOverlay.MoveTo(x, y)
	a.cfg.BossPanelX, a.cfg.BossPanelY, a.cfg.BossPanelPosSet = x, y, true
	a.saveConfig()
}

// ---------- data folder ----------

func (a *App) chooseDataDir() {
	dlg := walk.FileDialog{Title: "Chọn thư mục lưu dữ liệu Death Counter", InitialDirPath: a.store.Dir}
	ok, err := dlg.ShowBrowseFolder(a.mw)
	if err != nil || !ok || strings.TrimSpace(dlg.FilePath) == "" {
		return
	}
	a.switchDataDir(dlg.FilePath)
}

// switchDataDir moves the app to target: copies the current data there when it is
// empty, or adopts the data already there, then restarts the app.
func (a *App) switchDataDir(target string) {
	target = filepath.Clean(target)
	if strings.EqualFold(target, filepath.Clean(a.store.Dir)) {
		a.setStatus("Đang dùng thư mục này rồi.")
		return
	}
	if err := CheckWritable(target); err != nil {
		walk.MsgBox(a.mw, "Thư mục dữ liệu", "Không ghi được vào thư mục này:\n"+target+"\n\n"+err.Error()+
			"\n\nThư mục hệ thống như C:\\Program Files cần quyền Admin; hãy chọn thư mục khác.", walk.MsgBoxIconWarning)
		return
	}
	a.saveConfig()
	if a.cur != nil {
		a.store.SaveProfile(a.cur)
	}
	if DirHasData(target) {
		msg := "Thư mục này đã có dữ liệu Death Counter:\n" + target +
			"\n\nDùng dữ liệu đó? Dữ liệu hiện tại vẫn giữ nguyên ở:\n" + a.store.Dir
		if walk.MsgBox(a.mw, "Thư mục dữ liệu", msg, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
			return
		}
	} else {
		msg := "Chép toàn bộ dữ liệu sang:\n" + target + "\n\nrồi mở lại app với thư mục mới?\nDữ liệu ở chỗ cũ vẫn giữ nguyên."
		if walk.MsgBox(a.mw, "Thư mục dữ liệu", msg, walk.MsgBoxYesNo|walk.MsgBoxIconQuestion) != walk.DlgCmdYes {
			return
		}
		if err := CopyDataDir(a.store.Dir, target); err != nil {
			walk.MsgBox(a.mw, "Thư mục dữ liệu", "Chép dữ liệu thất bại, vẫn dùng thư mục cũ:\n"+err.Error(), walk.MsgBoxIconError)
			return
		}
	}
	if err := SetDataDir(defaultDataDir(), target); err != nil {
		walk.MsgBox(a.mw, "Thư mục dữ liệu", "Không lưu được lựa chọn:\n"+err.Error(), walk.MsgBoxIconError)
		return
	}
	a.restart()
}

// restart launches a fresh copy that waits for this one to exit, then closes.
func (a *App) restart() {
	exe, err := os.Executable()
	if err != nil {
		walk.MsgBox(a.mw, "Death Counter", "Đã đổi thư mục. Hãy mở lại app.", walk.MsgBoxIconInformation)
		a.mw.Close()
		return
	}
	cmd := exec.Command(exe, fmt.Sprintf("--after=%d", os.Getpid()))
	if err := cmd.Start(); err != nil {
		walk.MsgBox(a.mw, "Death Counter", "Đã đổi thư mục. Hãy mở lại app.", walk.MsgBoxIconInformation)
	}
	a.mw.Close()
}

// ---------- health bar ----------

func (a *App) refreshHPLabel() {
	if a.cur == nil || a.hpZoneLbl == nil {
		return
	}
	if !a.cur.HPColor.Valid() {
		a.hpZoneLbl.SetText("Chưa khoanh")
		a.hpLbl.SetText("--")
		return
	}
	z := a.cur.HPZone
	a.hpZoneLbl.SetText(fmt.Sprintf("x %d%%, y %d%%, %d×%d%% · màu %.0f°", int(z.X*100), int(z.Y*100), int(z.W*100), int(z.H*100), a.cur.HPColor.Hue))
	if !a.cur.HPEnabled {
		a.hpLbl.SetText("-- (đang tắt)")
	}
}

func (a *App) onHPChanged() {
	if a.suppress || a.cur == nil || a.hpCB == nil {
		return
	}
	a.cur.HPEnabled = a.hpCB.Checked()
	a.store.SaveProfile(a.cur)
	if a.cur.HPEnabled && !a.cur.HPColor.Valid() {
		a.setStatus("Bấm \"Khoanh thanh máu...\" lúc đang đầy máu để app học màu thanh máu.")
	}
	a.refreshHPLabel()
	a.updateLoop()
}

// calibrateHP hides the app, grabs the game screen, then lets the user draw
// around the (full) health bar; the bar color is learned from that frame.
func (a *App) calibrateHP() {
	if a.cur == nil {
		return
	}
	mon := a.currentMonitor()
	a.mw.SetVisible(false)
	a.overlay.Show(false)
	a.bossOverlay.Show(false)
	time.AfterFunc(500*time.Millisecond, func() {
		a.mw.Synchronize(func() {
			ow, oh := OcrSize(mon)
			shot, err := CaptureMonitorScaled(mon, ow, oh)
			a.mw.SetVisible(true)
			a.overlay.Show(a.cfg.OverlayEnabled)
			a.bossOverlay.Show(a.cfg.BossPanelEnabled)
			if err != nil {
				a.setStatus("Không chụp được màn hình: " + err.Error())
				return
			}
			init := a.cur.HPZone
			if init.W <= 0 {
				init = ROI{0.05, 0.05, 0.4, 0.15}
			}
			a.editRegionOn(shot, "Khoanh thanh máu - "+a.cur.Name,
				"Kéo chuột khoanh THANH MÁU nhân vật (lúc đang đầy máu).\nHUD thường: khoanh sát thanh máu. Máu trên người nhân vật (Dead Space): khoanh rộng quanh nhân vật.",
				init, false, func(r ROI) {
					c, ok := LearnHPColor(shot, r)
					if !ok {
						walk.MsgBox(a.mw, "Thanh máu", "Không tìm thấy màu nổi bật trong vùng này.\nHãy khoanh lúc đầy máu, và khoanh sát thanh máu hơn.", walk.MsgBoxIconWarning)
						return
					}
					a.cur.HPZone, a.cur.HPColor = r, c
					a.cur.HPFull = MeasureHP(shot, r, c)
					a.cur.HPEnabled = true
					a.store.SaveProfile(a.cur)
					a.hpDet = NewHPDeath(a.cur.HPFull)
					a.suppress = true
					a.hpCB.SetChecked(true)
					a.suppress = false
					a.refreshHPLabel()
					a.updateLoop()
					a.setStatus(fmt.Sprintf("Đã học màu thanh máu (%.0f°). App sẽ đếm khi máu về 0 rồi màn hình tối.", c.Hue))
				})
		})
	})
}
