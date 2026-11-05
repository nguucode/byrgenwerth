# Byrgenwerth user guide

[Tiếng Việt](huong-dan-su-dung.md) · [Back to README](../README.md)

The app's interface is in Vietnamese. This guide gives each label in Vietnamese with its meaning.

## Contents

1. [Install](#1-install)
2. [Main window](#2-main-window)
3. [Add a game](#3-add-a-game)
4. [Nhận diện tab (detection)](#4-nhận-diện-tab-detection)
5. [Hiển thị tab (display: overlay and OBS)](#5-hiển-thị-tab-display-overlay-and-obs)
6. [Cài đặt tab (settings: sound and data)](#6-cài-đặt-tab-settings-sound-and-data)
7. [Hotkeys](#7-hotkeys)
8. [Where data is stored](#8-where-data-is-stored)
9. [Troubleshooting](#9-troubleshooting)
10. [Translate tab: real-time dialogue translation](#10-translate-tab-real-time-dialogue-translation)

---

## 1. Install

Open **PowerShell** (Start menu → type "PowerShell") and run:

```powershell
irm https://raw.githubusercontent.com/nguucode/death-counter/main/install.ps1 | iex
```

- **First time** it asks for an install folder; press Enter for `%LOCALAPPDATA%\Programs\Byrgenwerth`. If the folder you type is not named `Byrgenwerth`, a `Byrgenwerth` folder is created inside it. The app and all its data live there.
- It adds **Byrgenwerth** to the Start menu and to Windows *Apps & features*, then starts the app.
- **Update**: run the same command again. It closes the app, replaces the exe in the same folder and keeps your data. An existing Death Counter install is renamed to Byrgenwerth, with its counts and settings. In OBS, update the path of the text file if you use it.
- On first launch Windows may ask for **network permission**: the app serves a local page on `127.0.0.1` for OBS. That page stays on your PC. Allow it. Only the Translate tab goes to the internet, and only while it is on.

Manual install: download `Byrgenwerth.exe` from [Releases](https://github.com/nguucode/death-counter/releases/latest) into a folder you can write to (not `C:\Program Files`). SmartScreen may warn because the exe is not code-signed: *More info → Run anyway*. Updates are then manual too.

Run your game in **Borderless** or **Windowed** mode. In exclusive fullscreen, Windows blocks screen capture and the overlay is hidden.

### macOS

macOS 14 or newer, Apple Silicon or Intel. It has the same two tools as Windows. Run this in Terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/nguucode/death-counter/main/install.sh | sh
```

The app goes to `~/Applications`, with its data in `~/Library/Application Support/Byrgenwerth`. Run the command again to update. It has no Dock icon: use **Byrgenwerth** in the menu bar to open the window or quit.

- Allow **Screen Recording** when macOS asks, then open the app again. Because the app is not notarized, macOS may ask again after an update.
- The API key is kept in the Keychain.
- F keys need **fn** on most Mac keyboards, unless *Use F1, F2 as standard function keys* is on. F11 is also the default Show Desktop shortcut, so **Chụp mẫu ngay** may be the easier way to capture a template. To change hotkeys, edit `hotkeys` in `config.json`.
- The Mac version has not been tried with a game yet. Most Windows games reach a Mac through a translation layer, so expect differences in what can be read.

## 2. Main window

The header shows the app name and version. **Death count** and **Translate** switch between the two tools. The items below belong to Death count; see [section 11](#11-translate-tab-real-time-dialogue-translation) for Translate.

- **Game**: the game you are playing. Each game has its own count, templates and keywords.
- **Số lần chết** (deaths): the big red number, with **+1**, **−1** and **Reset** buttons.
- **3 tabs**: Nhận diện (detection), Hiển thị (display), Cài đặt (settings).
- **Cửa sổ app luôn nằm trên**: keep the app window always on top. When a full-screen game (Borderless included) comes to the front, the app window minimizes itself; F7 brings it back.
- The last line is the **status**: what the app just did, or any error.

## 3. Add a game

1. Click **+ Thêm** (add), type the game name, OK.
2. If the name matches one of the 100 presets (close is fine: "elden ring", "wuchang"), its death keywords are loaded automatically.
3. **Đổi tên** (rename) / **Xóa** (delete). Deleting a game also deletes its count and templates.

### Supported games

Any Windows game can be counted: if OCR doesn't catch the death screen, press **F9** once to teach a template, or use [the health-bar mode](#games-without-a-death-screen-fade-to-black-reload-checkpoint). What differs between games is how much works out of the box. "Verified" below means the death text was found in wikis and screenshots ([sources](death-screens-top100.csv)); not every game has been tested in the app yet.

**Counted automatically, readable text (27):** Elden Ring, Elden Ring Nightreign, Dark Souls III, Dark Souls Remastered, Bloodborne, Demon's Souls (2020), Wuchang: Fallen Feathers, Stellar Blade, The First Berserker: Khazan, Diablo IV, The Witcher 3: Wild Hunt, Fortnite, Apex Legends, PUBG: Battlegrounds, Overwatch 2, Escape from Tarkov, Team Fortress 2, Rust, DayZ, Left 4 Dead 2, Resident Evil 4 (2023 remake), Assassin's Creed Shadows, Metal Gear Solid Delta: Snake Eater, Minecraft, Subnautica, Lethal Company, Risk of Rain 2.
Some only count part of the deaths: Apex Legends and Risk of Rain 2 only when the whole squad or run is lost; PUBG and Escape from Tarkov on the end-of-match screen, not the moment you die; Assassin's Creed Shadows also counts failed missions (DESYNCHRONIZED).

**Counted automatically, stylized text: OCR may miss it, press F9 once (10):** Sekiro: Shadows Die Twice (needs Japanese OCR), Hunt: Showdown 1896, Grand Theft Auto V, Red Dead Redemption 2, Resident Evil 2 (2019 remake), Hades, Terraria, Cuphead, Balatro, Fall Guys. Balatro and Fall Guys count lost runs and eliminations, not deaths.

**Preset but death text not verified, generic keywords (54):** Lies of P, Black Myth: Wukong, Lords of the Fallen (2023), Nioh 2, Mortal Shell, Hollow Knight: Silksong, Monster Hunter Wilds, Monster Hunter: World, Dragon's Dogma 2, Baldur's Gate 3, Path of Exile 2, Cyberpunk 2077, Counter-Strike 2, Valorant, Call of Duty: Warzone, Call of Duty: Black Ops 6, Call of Duty: Black Ops 7, Battlefield 6, Marvel Rivals, Tom Clancy's Rainbow Six Siege, ARC Raiders, Helldivers 2, The Finals, Destiny 2, Deadlock, Delta Force, Deep Rock Galactic, Resident Evil Village, Silent Hill 2 (2024 remake), Dead Space (2023 remake), The Last of Us Part II, God of War Ragnarök, Ghost of Tsushima, Ghost of Yōtei, Marvel's Spider-Man 2, Horizon Forbidden West, Assassin's Creed Valhalla, Kingdom Come: Deliverance II, Death Stranding 2: On the Beach, Ninja Gaiden 2 Black, Devil May Cry 5, Hades II, Dead Cells, The Binding of Isaac: Rebirth, Valheim, 7 Days to Die, Project Zomboid, Palworld, Enshrouded, R.E.P.O., PEAK, Vampire Survivors, Slay the Spire, League of Legends.
These may count on their own if the death screen matches a generic keyword; expect to press F9 once.

**No death text, or text too small: F9 templates or health-bar mode (9):** Hollow Knight, The Last of Us Part I, Phasmophobia, Celeste, Super Meat Boy, Geometry Dash (no death text); Don't Starve Together, Dead by Daylight, Dota 2 (text too small).

For every game: Windows only, and the game must run **Borderless** or **Windowed**. Console games (e.g. Bloodborne, Demon's Souls) only work when the picture is shown in a window on the PC, such as PS Remote Play or a capture-card preview.

## 4. Nhận diện tab (detection)

### How deaths are detected

The app captures the screen about twice per second and uses two methods together:

1. **OCR**: looks for keywords like "YOU DIED", "WASTED", "DEAD". The text must be **large** and **near the center**; small menu text, item names and corner HUD are ignored.
2. **Templates**: compares the screen with saved death-screen images. Used for stylized text or games with no text. Once OCR has read the death text in this session, a template match alone no longer counts: a dark death-screen template also matches dark respawns and dimmed screenshot tools.

Each death screen is counted **once**, however long it stays.

### Settings

| Label | Meaning |
|---|---|
| Bật tự nhận diện | Enable automatic detection. Turn off to count manually (F9/F10). |
| Đọc chữ bằng OCR | Enable OCR. |
| Trạng thái OCR | What OCR currently reads, e.g. `Thấy "DEAD" (cao 4.6%) → KHỚP` (found, matched). Check this first when a death is missed. |
| Ngôn ngữ game | OCR language. "Theo Windows" (system default) is fine for English games. See [troubleshooting](#ocr-cannot-read-a-game-in-chinesejapanesevietnamese) for others. |
| Mẫu theo game | Game preset. "Chung (mọi game)" = generic list. The gray line below says whether the preset is verified. |
| Từ khóa | Keywords, comma separated. End with `*` to match the start of a sentence, e.g. `You were slain by *`. |
| Độ giống | Live similarity between the screen and templates. |
| Ngưỡng | Threshold. Lower it (70%) if deaths are missed, raise it (90%) if it miscounts. |
| Cooldown | Seconds to wait after a count before another one. 8–10 s for fast-respawn games. |
| Màn hình game | Which monitor runs the game. |
| Vùng nhận diện | Region used for template matching. **Draw it tightly around the death text** for best accuracy. Set automatically on the first OCR hit. |
| Ảnh mẫu | Saved templates (max 10). Delete wrong ones to avoid false counts. |

### Games without a death screen (fade to black, reload checkpoint)

Some games just fade to black and reload. Use the **Máu nhân vật** (character health) section:

1. In game, at **full health**, click **Khoanh thanh máu...** (draw health bar). The app hides for half a second to capture the game screen.
2. Draw the region:
   - HUD health bar: draw tightly around it.
   - Health shown on the character (e.g. Isaac's spine in Dead Space): draw a **wide** region around where the character usually stands.
3. The app learns the bar's color. **Máu hiện tại** shows current health %.

A death is counted when **health drops to ~0%, then within 8 seconds the screen goes almost fully black for at least 1 second**. Loading screens while you still have health are ignored.

Limits: instant deaths where the bar has no time to drop (falling) may be missed: press F9. Walking into a black loading zone at very low health may count wrongly: press F10.

### A game is not counted automatically

1. Watch **Trạng thái OCR** while the death screen is visible.
2. If OCR can't read it (stylized text, kanji, no text): press **F9** on the death screen. The app adds 1 and saves a template.
3. Click **Khoanh vùng...** (draw region) and draw tightly around the death text or icon.
4. Next deaths are counted automatically.

## 5. Hiển thị tab (display: overlay and OBS)

### Content

**Định dạng** (format): `{count}` = deaths, `{game}` = game name. Example: `{game} · Deaths: {count}`.

### Text style (shared by overlay and OBS)

Font (any installed font, or type a name), size, bold, text color, outline (on/off, width) and outline color.

### On-screen overlay

1. Tick **Hiện overlay nổi trên game** (show overlay).
2. Click **Chỉnh vị trí overlay** (move overlay): it shows a yellow frame; drag it anywhere.
3. Click **Xong, lưu vị trí** (done). Normally the overlay is click-through.

OBS only records this overlay with **Display Capture**. With Game Capture, use the Browser Source below.

### OBS (Browser Source recommended)

1. Click **Copy** next to Browser Source (`http://127.0.0.1:17321/`).
2. In OBS: *Sources → + → Browser*, paste into **URL**, **Width 1200**, **Height 250**.
3. Position it. Transparent background, your text style, updates live.

Legacy option: tick **Ghi thêm file text** and use OBS *Text (GDI+) → Read from file*. This does not keep the style.

## 6. Cài đặt tab (settings: sound and data)

### Sound

No sound file is bundled.

1. **Chọn file...**: pick your own `.mp3` or `.wav`.
2. Tick **Phát âm thanh mỗi lần đếm +1** (play on every +1).
3. **Âm lượng** (volume, mp3 only), **Nghe thử** (test).

Plays on every +1 (automatic, F9 or button), not on −1. **Bỏ âm thanh** removes it.

### Data folder

Data lives in `ByrgenwerthData` next to `Byrgenwerth.exe`, so the app folder holds everything. **Mở** (open) shows it in Explorer.

- Coming from Death Counter, `DeathCounterData` is renamed on the first launch. Coming from 0.8 or older, where data was in `%APPDATA%\DeathCounter` or a folder you picked: the first launch copies it over once. The old copy stays until you uninstall.
- If the app folder is not writable, data goes to `%APPDATA%\Byrgenwerth` for now and the app tells you. Reinstall into a folder you own.

### Uninstall

Click **Gỡ Byrgenwerth...** (uninstall) in this tab, or use Windows *Settings → Apps → Byrgenwerth → Uninstall*. After you confirm, it deletes:

- all data, including your API key: `ByrgenwerthData`, `%APPDATA%\Byrgenwerth`, plus `%APPDATA%\DeathCounter` and the folder you picked in older versions (only the app's own files there, nothing else),
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

Hotkeys work while in game; the game won't receive these keys.
To change them, edit `hotkeys` in `config.json` (e.g. `"add": "Ctrl+F9"`) and restart the app.

## 8. Where data is stored

In `ByrgenwerthData` next to `Byrgenwerth.exe` (button **Mở** in the Cài đặt tab):

| File | Content |
|---|---|
| `config.json` | Global settings: hotkeys, text style, overlay positions... |
| `profiles\<game>\profile.json` | Count and keywords per game. |
| `profiles\<game>\templates\` | Death-screen templates. |
| `history.csv` | Log of every +1/−1, opens in Excel. |
| `obs_deaths.txt` | Text file for OBS (if enabled). |
| `ocr-debug.log` | Every OCR read while a full-screen game is in front (text, position, height), and every count. Moved to `ocr-debug.old.log` at 4 MB. Send it with a bug report. |
| `translate.key` | Your Anthropic API key, encrypted with your Windows account. |
| `ocr-misses.log` | What OCR read in the 10 seconds before each F9 +1, i.e. deaths the app missed. Send it with a bug report. |

Back up by copying the whole folder.

## 9. Troubleshooting

### A death is not counted
- Press F9 right away: it counts the death and writes what OCR read to `ocr-misses.log` in the data folder. That file shows why the death was missed.
- Check **Trạng thái OCR** during the death screen.
  - Nothing or garbage read: stylized text. Use F9 + draw region (section 4).
  - Correct text but "không phải từ khóa" (not a keyword): add it to **Từ khóa**.
- Exclusive fullscreen: switch to Borderless.
- Multiple monitors: check **Màn hình game**.

### Wrong counts
- Press F10.
- Delete wrong templates, raise **Ngưỡng**, draw **Vùng nhận diện** tighter.

### Hotkeys don't work
The status line says "Cảnh báo phím tắt" (hotkey warning) when another program owns the key. Change it in `config.json`.

### Overlay not visible in game
The game is in exclusive fullscreen. Switch to Borderless.

### OCR cannot read a game in Chinese/Japanese/Vietnamese
1. Windows *Settings → Time & language → Language & region → Add a language*, add the game's language.
2. Restart the app and pick it in **Ngôn ngữ game** (Nhận diện tab).
3. Add that language's death text to **Từ khóa**. Accents don't matter: "BẠN ĐÃ CHẾT" also matches "BAN DA CHET". Presets of games that show "YOU DIED" already include the Vietnamese fan translation "BẠN ĐÃ CHẾT".

### OBS shows nothing
- The app must be running.
- The port may differ from 17321 if it was busy; always copy the URL from the app.

## 10. Translate tab: real-time dialogue translation

The app reads the dialogue region of your game and shows a Vietnamese translation in a box right next to it. English to Vietnamese only.

1. Get an API key at [console.anthropic.com](https://console.anthropic.com) and paste it into **API key**. You pay Anthropic for what you use; **Chi phí ước tính** (estimated cost) adds up from token counts and list prices. It restarts when you change the key or the game.
2. Open the game, press **Ctrl+Alt+R** and drag over the dialogue box. A tighter region costs less and misreads less. Pressing the shortcut in the game also works; so does **Khoanh vùng...** in the tab.
3. Tick **Bật dịch** (translate on) or press **Ctrl+Alt+T**. It always starts off, so nothing is sent until you turn it on.

Good to know:

- **What leaves your PC**: the text OCR reads in the region, the 8 lines before it and the current game's name go to Anthropic's API (`claude-haiku-5-5`). Screenshots do not, and neither does anything outside the region.
- **The key** is encrypted with your Windows account (DPAPI) in `translate.key`. Uninstalling deletes it.
- A line is translated once the text has stopped changing, so typewriter text waits until it finishes. Repeated lines come from a cache and cost nothing.
- On an error the box shows the English line and the reason. More than 500 characters in the region is refused: draw a tighter region.
- English OCR must be installed in Windows: *Settings → Time & language → Language & region*.
- The game must run **Borderless** or **Windowed**, or the box cannot show over it.
- Translation is new and has not been tried on a real game yet. Expect rough edges.
