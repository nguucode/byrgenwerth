# Byrgenwerth user guide

[Tiếng Việt](huong-dan-su-dung.md) · [Back to README](../README.md)

The app's interface is in Vietnamese. This guide gives each label in Vietnamese with its meaning.

## Contents

1. [Install](#1-install)
2. [Main window](#2-main-window)
3. [Nhận diện tab (detection)](#3-nhận-diện-tab-detection)
4. [Hiển thị tab (how the count looks)](#4-hiển-thị-tab-how-the-count-looks)
5. [Overlay/OBS tab](#5-overlayobs-tab)
6. [Cài đặt tab (settings)](#6-cài-đặt-tab-settings)
7. [Hotkeys](#7-hotkeys)
8. [Where data is stored](#8-where-data-is-stored)
9. [Troubleshooting](#9-troubleshooting)
10. [Translate: real-time dialogue translation](#10-translate-real-time-dialogue-translation)
11. [Thuyết minh: Vietnamese narration](#11-thuyết-minh-vietnamese-narration)

---

## 1. Install

Open **PowerShell** (Start menu → type "PowerShell") and run:

```powershell
irm https://raw.githubusercontent.com/nguucode/byrgenwerth/main/install.ps1 | iex
```

- **First time** it asks for an install folder; press Enter for `%LOCALAPPDATA%\Programs\Byrgenwerth`. If the folder you type is not named `Byrgenwerth`, a `Byrgenwerth` folder is created inside it. The app and all its data live there.
- It adds **Byrgenwerth** to the Start menu and to Windows *Apps & features*, then starts the app.
- **Update**: run the same command again. It closes the app, replaces the exe in the same folder and keeps your data.
- On first launch Windows may ask for **network permission**: the app serves a local page on `127.0.0.1` for OBS. That page stays on your PC. Allow it.

Manual install: download `Byrgenwerth.exe` from [Releases](https://github.com/nguucode/byrgenwerth/releases/latest) into a folder you can write to (not `C:\Program Files`). SmartScreen may warn because the exe is not code-signed: *More info → Run anyway*. Updates are then manual too.

Run your game in **Borderless** or **Windowed** mode. In exclusive fullscreen, Windows blocks screen capture and the overlay is hidden.

Byrgenwerth runs on Windows 10 and 11 only. The macOS version was removed in v0.12.0.

## 2. Main window

- **Death count** / **Translate** switch between the two tools. **Luôn trên** keeps the app window on top. When a full-screen game (Borderless included) comes to the front, the app window minimizes itself; F7 brings it back.
- **Game**: the game you are playing. Each game has its own count, templates and keywords. **Thêm game** adds one. Rename and delete are in the Cài đặt tab.
- The big red number is the death count, with **+1**, **−1** and **Reset**.
- **4 tabs**: Nhận diện (detection), Hiển thị (look), Overlay/OBS, Cài đặt (settings).
- The last line is the **status**: what the app just did, or any error.

Hover over a field or button for a short explanation.

## 3. Nhận diện tab (detection)

### How deaths are detected

The app captures the screen about twice per second and uses two methods together:

1. **OCR**: reads the screen with the text recognition built into Windows and looks for keywords like "YOU DIED", "WASTED", "DEAD". The text must be **large** and **near the center**; small menu text, item names and corner HUD are ignored. This works the same way for every game: there are no per-game presets.
2. **Templates**: compares the screen with saved death-screen images. Used for stylized text or games with no text. Once OCR has read the death text in this session, a template match alone no longer counts: a dark death-screen template also matches dark respawns and dimmed screenshot tools.

Each death screen is counted **once**, however long it stays.

### Settings

| Label | Meaning |
|---|---|
| Tự nhận diện | Enable automatic detection. Turn off to count manually (F9/F10). |
| Đọc chữ (OCR) | Enable OCR. |
| Màn hình | Which monitor runs the game. |
| Ngôn ngữ | OCR language. "Theo Windows" (system default) is fine for English games. See [troubleshooting](#ocr-cannot-read-a-game-in-chinesejapanesevietnamese) for others. |
| Từ khóa | Keywords, comma separated. End with `*` to match the start of a sentence, e.g. `You were slain by *`. Clear the box to go back to the generic list. |
| Ngưỡng | Template threshold, **70%** by default. Lower it if deaths are missed, raise it (85–90%) if it miscounts. |
| Cooldown | Seconds to wait after a count before another one, **10 s** by default. |
| Vùng | Region used for template matching. **Draw it tightly around the death text** (Khoanh...) for best accuracy. Set automatically on the first OCR hit. |
| Ảnh mẫu | Saved templates (max 10). **Chụp mẫu** saves the screen now, **Xóa mẫu** deletes the selected one. Delete wrong ones to avoid false counts. |
| OCR | What OCR currently reads, e.g. `Thấy "DEAD" (cao 4.6%) → KHỚP` (found, matched). Check this first when a death is missed. |
| Độ giống | Live similarity between the screen and the templates. |

### A game is not counted automatically

1. Watch the **OCR** line while the death screen is visible.
2. If OCR can't read it (stylized text, kanji, no text at all): press **F9** on the death screen. The app adds 1 and saves a template.
3. Click **Khoanh...** next to **Vùng** and draw tightly around the death text or icon.
4. Next deaths are counted automatically.

Games that only fade to black without any death screen can't be counted on their own: press F9 when you die.

## 4. Hiển thị tab (how the count looks)

### Label (Nhãn)

The label is shown with the number, on the overlay and in OBS.

| Label | Meaning |
|---|---|
| Kiểu | **Chữ** (a word), **Hình** (a PNG or SVG picture) or **Không có nhãn** (number only). |
| Chữ | The word, e.g. `Deaths`. `{game}` is replaced by the game name. |
| Hình | **Chọn...** picks a `.png` or `.svg`. It is copied into the data folder, so the original can move. |
| Cỡ | Size of the label in px: the text size, or the picture's height. |
| Vị trí | Which corner around the number the label sits in: top left, top right, bottom left or bottom right. "Top" puts it above the number, "bottom" below; "left"/"right" lines it up with that edge of the number. |

### Text style (Kiểu chữ)

Font (any installed font, or type a name), size of the number, bold, outline width (on/off), and the two colors: **Chữ** (text) and **Viền** (outline). Hover a color button to see its value.

## 5. Overlay/OBS tab

### On-screen overlay

1. Tick **Hiện** (show).
2. Click **Kéo vị trí** (move): a yellow frame appears; drag it anywhere.
3. Click **Lưu vị trí** (save). Normally the overlay is click-through. **Đặt lại** puts it back in the top-left corner.

OBS only records this overlay with **Display Capture**. With Game Capture, use the Browser Source below.

### OBS (Browser Source recommended)

1. Click **Copy** next to **Browser** (`http://127.0.0.1:17321/`).
2. In OBS: *Sources → + → Browser*, paste into **URL**, **Width 1200**, **Height 400**.
3. Position it. Transparent background, your text style and label, updates live.

Legacy option: tick **Ghi file** and use OBS *Text (GDI+) → Read from file* with the path from its **Copy** button. The file holds the label word and the number, without style or picture.

## 6. Cài đặt tab (settings)

### Sound on every +1

No sound file is bundled. **Chọn...** picks your own `.mp3` or `.wav`, **Âm lượng** sets the volume (mp3 only), **Bật** turns it on, **Nghe thử** plays it, **Bỏ file** removes it. It plays on every +1 (automatic, F9 or button), not on −1.

### Game đang chọn (current game)

**Đổi tên** renames the current game. **Xóa game** deletes it with its count and templates.

### Hotkeys and data

The hotkey list is shown here (see [section 7](#7-hotkeys)). **Thư mục** opens the data folder.

### Uninstall

Click **Gỡ app...**, or use Windows *Settings → Apps → Byrgenwerth → Uninstall*. After you confirm, it deletes:

- all data, including your API key, label picture and downloaded voices: `ByrgenwerthData`, `%APPDATA%\Byrgenwerth`, plus `%APPDATA%\DeathCounter` and the folder you picked in older versions (only the app's own files there, nothing else),
- `Byrgenwerth.exe`, and the install folder when it is left empty,
- the Start menu shortcut and the *Apps & features* entry.

It cannot be undone. To keep your counts, copy `ByrgenwerthData` somewhere first.

## 7. Hotkeys

| Key | Action |
|---|---|
| F9 | +1. If the current screen is not yet a template, save it as one. |
| F10 | −1 (fix a wrong count). |
| F11 | Save a template only, no count. |
| F8 | Toggle automatic detection. |
| F7 | Hide/show the app window (also brings it back after it minimized for a game). Detection, overlay and OBS keep running. |
| Ctrl+Alt+T | Turn translation on or off. |
| Ctrl+Alt+R | Draw the dialogue region for translation again. |
| Ctrl+Alt+V | Read the translation aloud. Change it in Translate → Thuyết minh → **Gán...**. |

Hotkeys work while in game; the game won't receive these keys. To change the others, edit `hotkeys` in `config.json` (e.g. `"add": "Ctrl+F9"`) and restart the app.

## 8. Where data is stored

In `ByrgenwerthData` next to `Byrgenwerth.exe` (button **Thư mục** in the Cài đặt tab):

| File | Content |
|---|---|
| `config.json` | Global settings: hotkeys, text style, label, overlay position, narration... |
| `profiles\<game>\profile.json` | Count and keywords per game. |
| `profiles\<game>\templates\` | Death-screen templates. |
| `label.png` / `label.svg` | The label picture, if you chose one. |
| `piper\` | The narration engine and voices, once downloaded. |
| `history.csv` | Log of every +1/−1, opens in Excel. |
| `obs_deaths.txt` | Text file for OBS (if enabled). |
| `ocr-debug.log` | Every OCR read while a full-screen game is in front (text, position, height), and every count. Moved to `ocr-debug.old.log` at 4 MB. Send it with a bug report. |
| `translate.key` | Your Anthropic API key, encrypted with your Windows account. |
| `ocr-misses.log` | What OCR read in the 10 seconds before each F9 +1, i.e. deaths the app missed. Send it with a bug report. |

Back up by copying the whole folder.

## 9. Troubleshooting

### A death is not counted
- Press F9 right away: it counts the death and writes what OCR read to `ocr-misses.log` in the data folder. That file shows why the death was missed.
- Check the **OCR** line during the death screen.
  - Nothing or garbage read: stylized text. Use F9 + draw region (section 3).
  - Correct text but "không phải từ khóa" (not a keyword): add it to **Từ khóa**.
- Exclusive fullscreen: switch to Borderless.
- Multiple monitors: check **Màn hình**.

### Wrong counts
- Press F10.
- Delete wrong templates, raise **Ngưỡng**, draw **Vùng** tighter.

### Hotkeys don't work
The status line says "Cảnh báo phím tắt" (hotkey warning) when another program owns the key. Change it in `config.json`, or for narration with **Gán...**.

### Overlay not visible in game
The game is in exclusive fullscreen. Switch to Borderless.

### OCR cannot read a game in Chinese/Japanese/Vietnamese
1. Windows *Settings → Time & language → Language & region → Add a language*, add the game's language.
2. Restart the app and pick it in **Ngôn ngữ** (Nhận diện tab).
3. Add that language's death text to **Từ khóa**. Accents don't matter: "BẠN ĐÃ CHẾT" also matches "BAN DA CHET".

### OBS shows nothing
- The app must be running.
- The port may differ from 17321 if it was busy; always copy the URL from the app.

## 10. Translate: real-time dialogue translation

The **Dịch** tab reads the dialogue region of your game and shows a Vietnamese translation in a box right next to it. English to Vietnamese only.

1. Get an API key at [console.anthropic.com](https://console.anthropic.com) and paste it into **API key**. You pay Anthropic for what you use; the estimated cost adds up from token counts and list prices. It restarts when you change the key or the game.
2. Open the game, press **Ctrl+Alt+R** and drag over the dialogue box. A tighter region costs less and misreads less. **Khoanh...** in the tab does the same.
3. Tick **Bật dịch Anh → Việt** or press **Ctrl+Alt+T**. It always starts off, so nothing is sent until you turn it on.

Good to know:

- **What leaves your PC**: the text OCR reads in the region, the 8 lines before it and the current game's name go to Anthropic's API (`claude-haiku-5-5`). Screenshots do not, and neither does anything outside the region.
- **The key** is encrypted with your Windows account (DPAPI) in `translate.key`. Uninstalling deletes it.
- A line is translated once the text has stopped changing, so typewriter text waits until it finishes. Repeated lines come from a cache and cost nothing.
- On an error the box shows the English line and the reason. More than 500 characters in the region is refused: draw a tighter region.
- English OCR must be installed in Windows: *Settings → Time & language → Language & region*.
- The game must run **Borderless** or **Windowed**, or the box cannot show over it.
- Translation has not been tried on many games yet. Expect rough edges.

## 11. Thuyết minh: Vietnamese narration

The **Thuyết minh** tab reads the Vietnamese translation aloud when you press a key or a gamepad button.

1. Pick a voice in **Giọng**: **Nữ** (female) or **Nam** (male). Click **Tải** (download) once. The app downloads [Piper](https://github.com/rhasspy/piper) (22 MB) and the voice (female 63 MB, male 28 MB), checks each file against a fixed hash, and keeps them in `ByrgenwerthData\piper`.
2. **Nghe thử** plays a sample. **Tốc độ** changes the speed (0 = normal, +50 = faster, −50 = slower), **Âm lượng** the volume.
3. Bind a key with **Gán...** next to **Bàn phím** (default **Ctrl+Alt+V**), a gamepad combination with **Gán...** next to **Tay cầm**: hold the buttons together and let go.
4. In game, press the key or the buttons: the translation on screen (or the last one) is read aloud. Press again to stop. **Đọc dịch** does the same from the app.

Good to know:

- Narration runs **on your PC**. After the download it needs no internet and sends nothing anywhere.
- The voices are Piper's free Vietnamese voices: female `vais1000` (medium quality), male one speaker of `vivos` (lower quality). They sound synthetic.
- Gamepads: Xbox controllers and anything shown to Windows as one (Steam Input, DS4Windows). The game also receives the buttons, so pick a combination it does not use, such as **LB+RB+Y**.
- Reading needs a translation, so turn on **Bật dịch** first.
