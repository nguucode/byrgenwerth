# Death Counter user guide

[Tiếng Việt](huong-dan-su-dung.md) · [Back to README](../README.md)

The app's interface is in Vietnamese. This guide gives each label in Vietnamese with its meaning.

## Contents

1. [Install](#1-install)
2. [Main window](#2-main-window)
3. [Add a game](#3-add-a-game)
4. [Nhận diện tab (detection)](#4-nhận-diện-tab-detection)
5. [Hiển thị tab (display: overlay and OBS)](#5-hiển-thị-tab-display-overlay-and-obs)
6. [Cài đặt tab (settings: sound and data)](#6-cài-đặt-tab-settings-sound-and-data)
7. [Boss tab: top 3 bosses](#7-boss-tab-top-3-bosses)
8. [Hotkeys](#8-hotkeys)
9. [Where data is stored](#9-where-data-is-stored)
10. [Troubleshooting](#10-troubleshooting)

---

## 1. Install

1. Download `DeathCounter.exe` from [Releases](https://github.com/nguucode/death-counter/releases/latest).
2. Put it anywhere, e.g. `C:\Program Files\DeathCounter\`. No admin rights needed.
3. Run it. On first launch you may see:
   - **SmartScreen** ("Windows protected your PC"): the exe is not code-signed. Click *More info → Run anyway*.
   - **A network permission prompt**: the app serves a local page on `127.0.0.1` for OBS. It never goes to the internet. Allow it.

Run your game in **Borderless** or **Windowed** mode. In exclusive fullscreen, Windows blocks screen capture and the overlay is hidden.

## 2. Main window

- **Game**: the game you are playing. Each game has its own count, templates, keywords and boss stats.
- **Số lần chết** (deaths): the big red number, with **+1**, **−1** and **Reset** buttons.
- **4 tabs**: Nhận diện (detection), Hiển thị (display), Cài đặt (settings), Boss.
- **Cửa sổ app luôn nằm trên**: keep the app window always on top.
- The last line is the **status**: what the app just did, or any error.

## 3. Add a game

1. Click **+ Thêm** (add), type the game name, OK.
2. If the name matches one of the 100 presets (close is fine: "elden ring", "wuchang"), its death keywords and boss-name region are loaded automatically.
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
2. **Templates**: compares the screen with saved death-screen images. Used for stylized text or games with no text.

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

Data lives in `%APPDATA%\DeathCounter` by default. To keep it elsewhere (drive D:, a OneDrive folder to sync two PCs...):

1. Click **Đổi...** (change) and pick a folder.
2. Empty folder: the app asks, then **copies** your current data there. Folder that already has Death Counter data: the app asks, then **uses** that data.
3. The app restarts with the new folder. The old copy is kept; delete it yourself when you no longer need it.

**Mặc định** (default) goes back to `%APPDATA%\DeathCounter`. System folders like `C:\Program Files` can't be used (they need admin rights).
If the chosen folder becomes unreachable (USB unplugged, drive letter changed), the app falls back to the default folder and tells you.

## 7. Boss tab: top 3 bosses

### How the current boss is found

- OCR reads the **top** and **bottom** bands of the screen, where most games print the boss name above its health bar.
- A label must **stay on screen for several seconds** to count as a boss, so subtitles are ignored. A label visible continuously for over 15 minutes is treated as HUD.
- When you die, the death goes to the boss seen in the last **30 seconds**. −1 removes it from that boss.

### Settings

| Label | Meaning |
|---|---|
| Ghi nhận boss khi chết | Track bosses for this game. |
| Đang đánh | The boss currently detected. Empty during a fight? See troubleshooting. |
| Vùng tên boss | "Tự động" (top + bottom bands), "Theo mẫu game" (preset, e.g. Sekiro: top-left) or your own region. |
| Thống kê | Bosses and death counts. Fix OCR mistakes with **Đổi tên** (rename), **Gộp vào...** (merge two names of one boss), **Xóa** (delete). |
| Bảng Top 3 | Speedrun-style panel: #1 in gold, current boss in blue. A current boss outside the top 3 gets its own extra row. |

### Show the panel

- **On screen**: tick **Hiện bảng nổi trên màn hình**, move it like the overlay.
- **In OBS**: copy `http://127.0.0.1:17321/bosses` → OBS *Browser* source, Width 600, Height 300.

## 8. Hotkeys

| Key | Action |
|---|---|
| F9 | +1. If the current screen is not yet a template, save it as one. |
| F10 | −1 (fix a wrong count). |
| F11 | Save a template only, no count. |
| F8 | Toggle automatic detection. |

Hotkeys work while in game; the game won't receive these keys.
To change them, edit `hotkeys` in `config.json` (e.g. `"add": "Ctrl+F9"`) and restart the app.

## 9. Where data is stored

By default `C:\Users\<you>\AppData\Roaming\DeathCounter\`, or the folder you chose in the Cài đặt tab (button **Mở thư mục dữ liệu**):

| File | Content |
|---|---|
| `config.json` | Global settings: hotkeys, text style, overlay positions... |
| `profiles\<game>\profile.json` | Count, keywords, boss stats per game. |
| `profiles\<game>\templates\` | Death-screen templates. |
| `history.csv` | Log of every +1/−1, opens in Excel. |
| `obs_deaths.txt` | Text file for OBS (if enabled). |

Back up by copying the whole folder. To move it, see [section 6](#data-folder).

## 10. Troubleshooting

### A death is not counted
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
3. Add that language's death text to **Từ khóa**.

### Boss not detected
- Watch **Đang đánh** during the fight.
- Empty: the name may be outside the automatic bands or too small. Use **Khoanh vùng...** in the Boss tab.
- Wrong name: draw a tighter region and rename it in **Thống kê**.

### OBS shows nothing
- The app must be running.
- The port may differ from 17321 if it was busy; always copy the URL from the app.
