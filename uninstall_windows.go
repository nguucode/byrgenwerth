//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/lxn/walk"
	"github.com/lxn/win"
)

// uninstallKey is written by install.ps1 so the app shows in Apps & features.
const uninstallKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Uninstall\DeathCounter`

func startMenuShortcut() string {
	return filepath.Join(os.Getenv("APPDATA"), `Microsoft\Windows\Start Menu\Programs\Death Counter.lnk`)
}

// runUninstall asks for confirmation, then removes the app and all its data.
// a is nil when started as "DeathCounter.exe --uninstall" from Apps & features.
func runUninstall(a *App) {
	var owner walk.Form
	if a != nil {
		owner = a.mw
	}
	msg := "Gỡ Death Counter khỏi máy?\n\nSẽ xóa:\n" +
		"• Toàn bộ dữ liệu: số đếm, ảnh mẫu, thống kê boss, cài đặt\n" +
		"• File DeathCounter.exe và thư mục cài đặt\n" +
		"• Shortcut trong Start Menu và mục trong Apps & features\n\n" +
		"Không hoàn tác được."
	if walk.MsgBox(owner, "Gỡ Death Counter", msg, walk.MsgBoxYesNo|walk.MsgBoxIconWarning|walk.MsgBoxDefButton2) != walk.DlgCmdYes {
		return
	}
	if a != nil {
		// Stop capture, overlays and config saving first; main finishes the job.
		a.uninstalling = true
		a.mw.Close()
		return
	}
	closeRunningApp()
	finishUninstall()
}

// closeRunningApp asks an open Death Counter window to close and waits for it,
// so it stops writing data and releases the exe.
func closeRunningApp() {
	cls, _ := syscall.UTF16PtrFromString(`\o/ Walk_MainWindow_Class \o/`)
	title, _ := syscall.UTF16PtrFromString("Death Counter")
	for i := 0; i < 50; i++ {
		h := win.FindWindow(cls, title)
		if h == 0 {
			return
		}
		if i == 0 {
			win.PostMessage(h, win.WM_CLOSE, 0, 0)
		}
		time.Sleep(200 * time.Millisecond)
	}
}

// finishUninstall deletes every data folder, then hands the exe, shortcut and
// registry entry to a hidden cmd that runs once this process has exited.
func finishUninstall() {
	exe, _ := os.Executable()
	exeDir := filepath.Dir(exe)
	appData := appDataDir()

	var failed []string
	if err := os.RemoveAll(filepath.Join(exeDir, "DeathCounterData")); err != nil {
		failed = append(failed, err.Error())
	}
	for _, d := range LegacyDataDirs(appData) {
		var err error
		if d == appData {
			err = os.RemoveAll(d) // the whole folder is ours
		} else {
			err = RemoveAppData(d) // a folder the user picked may hold other files
		}
		if err != nil {
			failed = append(failed, err.Error())
		}
	}

	done := "Đã gỡ Death Counter."
	if len(failed) > 0 {
		done += "\n\nMột số file chưa xóa được:\n" + strings.Join(failed, "\n")
	}
	walk.MsgBox(nil, "Gỡ Death Counter", done, walk.MsgBoxIconInformation)

	if exe == "" {
		return
	}
	// ping waits ~2s for this process to exit and unlock the exe. rmdir only
	// removes the install folder when it is empty, and only one install.ps1 made.
	line := `cmd.exe /c ping -n 3 127.0.0.1 >nul` +
		` & del /f /q "` + exe + `"` +
		` & del /f /q "` + startMenuShortcut() + `"` +
		` & reg delete "` + uninstallKey + `" /f >nul 2>&1`
	if strings.EqualFold(filepath.Base(exeDir), "DeathCounter") {
		line += ` & rmdir "` + exeDir + `"`
	}
	cmd := exec.Command("cmd.exe")
	cmd.SysProcAttr = &syscall.SysProcAttr{CmdLine: line, HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
	cmd.Start()
}
