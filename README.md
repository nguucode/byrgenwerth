# Byrgenwerth

A small Windows app that supports you while you play. Formerly Death Counter. It has two tools:

- **Death count**: counts how many times you die, automatically. It reads the screen with the OCR built into Windows ("YOU DIED", "WASTED", "DEAD"...) and shows the count, with a word or a picture as its label, as an on-screen overlay or in OBS.
- **Translate**: translates game dialogue from English to Vietnamese in real time, in a box next to the dialogue, and can read the translation aloud in Vietnamese.

**[Tiếng Việt bên dưới](#tiếng-việt)** · **[Install](#quick-start)** · **[User guide](docs/user-guide.md)** · **[Hướng dẫn sử dụng](docs/huong-dan-su-dung.md)**

## Features

- **Automatic death detection** with Windows OCR, the same way for every game: large death text near the middle of the screen counts.
- **Self-learning templates**: the first OCR hit saves a screenshot template, so stylized frames OCR misreads later are still caught. Games whose death text OCR can't read are taught with one hotkey press (F9).
- **Label next to the count**: a word (`{game}` = game name) or your own PNG/SVG picture, in any of the four corners around the number, at the size you choose.
- **Display anywhere**: click-through on-screen overlay, OBS Browser Source (keeps your style and label, works with Game Capture), or a plain text file.
- **Text style**: font, size, bold, color, outline width and color.
- **Real-time translation (EN → VI)**: draw a box around the dialogue and the translation appears next to it. Uses your own Anthropic API key, encrypted with your Windows account. Off until you turn it on; only the text in that box, the lines before it and the game name are sent. See [section 10 of the guide](docs/user-guide.md#10-translate-real-time-dialogue-translation).
- **Vietnamese narration**: press a key or a gamepad button combination (Xbox / XInput) to hear the translation, with a free female or male voice. It runs on your PC with [Piper](https://github.com/rhasspy/piper), downloaded once. See [section 11](docs/user-guide.md#11-thuyết-minh-vietnamese-narration).
- **Hotkeys**: F9 +1 and learn template · F10 −1 · F11 capture template · F8 toggle detection · F7 hide/show the app · Ctrl+Alt+T toggle translation · Ctrl+Alt+R redraw the dialogue region · Ctrl+Alt+V read the translation aloud.
- **Optional sound** on every count (bring your own .mp3 / .wav).
- **One-line install and update** from PowerShell. The app and its data stay in one folder; **uninstall** from the app removes everything. Death count needs no internet; Translate does.

![Text styles in OBS](docs/images/text-styles.png)

## Quick start

1. Open PowerShell and run the command below. The first time it asks where to install; run it again to update.
   ```powershell
   irm https://raw.githubusercontent.com/nguucode/byrgenwerth/main/install.ps1 | iex
   ```
2. Click **Thêm game** (add game) and type your game's name.
3. Play. When the death screen appears, the counter goes up.
   If it doesn't (stylized or missing text), press **F9** once on the death screen; the app learns it.
4. For OBS: tab **Overlay/OBS** → **Copy** the Browser Source URL → in OBS add a *Browser* source, width 1200, height 400.

Full walkthrough: **[docs/user-guide.md](docs/user-guide.md)**.

## Requirements and limits

- Windows 10 or 11, 64-bit. The macOS version was removed in v0.12.0.
- OCR uses the languages installed in Windows. English works out of the box on most PCs; for games in other languages add that language in *Settings → Time & language → Language & region*.
- Games must run in **Borderless** or **Windowed** mode. Exclusive fullscreen hides the screen from capture and hides the overlay.
- Games that only fade to black, with no death screen, can't be counted on their own: press F9.
- Project status: early. Death counting, overlay and OBS output have been used on a real setup. Translation and narration are new and lightly tested. Please open an issue with a screenshot if something is off.

## Build from source

Requires Go 1.24+. Releases are built on Windows by GitHub Actions when a `v*` tag is pushed (`.github/workflows/release.yml`).

```sh
go test ./...
GOOS=windows GOARCH=amd64 go build -ldflags "-H windowsgui -s -w" -o Byrgenwerth.exe .
```

`rsrc_windows_amd64.syso` holds the manifest and icon; regenerate it with
`go install github.com/akavel/rsrc@latest && rsrc -manifest app.manifest -ico icon.ico -arch amd64 -o rsrc_windows_amd64.syso`.
The UI uses [lxn/walk](https://github.com/lxn/walk) and OCR calls `Windows.Media.Ocr` through WinRT directly (no cgo). Narration downloads Piper 2023.11.14-2 and its Vietnamese voices at runtime and checks them against SHA-256 hashes in `piper.go`.

## License

[MIT](LICENSE). No sound files are bundled. The narration voices are downloaded from [rhasspy/piper-voices](https://huggingface.co/rhasspy/piper-voices) under their own licenses (`vais1000`: CC BY 4.0; `vivos`: CC BY-NC-SA 4.0).

---

## Tiếng Việt

App nhỏ gọn cho Windows, hỗ trợ bạn khi chơi game, tên cũ là Death Counter. App có hai công cụ:

- **Death count**: **tự đếm số lần chết**. App đọc màn hình bằng OCR có sẵn của Windows ("YOU DIED", "WASTED", "DEAD"...) rồi hiện số đếm, kèm nhãn là chữ hoặc hình, bằng overlay trên màn hình hoặc trong OBS.
- **Translate**: dịch hội thoại trong game từ tiếng Anh sang tiếng Việt theo thời gian thực, hiện trong một khung cạnh hội thoại, và đọc to bản dịch bằng tiếng Việt.

### Tính năng

- **Tự nhận màn chết** bằng OCR của Windows, cùng một cách cho mọi game: chữ chết to, gần giữa màn hình là được tính.
- **Tự học ảnh mẫu**: lần đầu OCR bắt được, app lưu ảnh màn chết làm mẫu, sau này OCR đọc trượt thì ảnh mẫu bắt thay. Game có chữ chết OCR không đọc được thì bấm F9 một lần để dạy.
- **Nhãn cạnh số đếm**: chữ (`{game}` = tên game) hoặc hình PNG/SVG của bạn, đặt ở một trong 4 góc quanh con số, cỡ tùy chỉnh.
- **Hiển thị**: overlay nổi trên game (chuột bấm xuyên qua), OBS Browser Source (giữ nguyên kiểu chữ và nhãn, dùng được với Game Capture), hoặc file text.
- **Kiểu chữ**: font, cỡ chữ, đậm, màu chữ, viền và màu viền.
- **Dịch real-time (Anh → Việt)**: khoanh vùng hội thoại là bản dịch hiện ngay bên cạnh. Dùng API key Anthropic của bạn, mã hóa theo tài khoản Windows. Chưa bật thì không gửi gì; khi bật chỉ gửi chữ trong vùng khoanh, các câu trước đó và tên game. Xem [mục 10 trong hướng dẫn](docs/huong-dan-su-dung.md#10-translate-dịch-hội-thoại-theo-thời-gian-thực).
- **Thuyết minh tiếng Việt**: bấm phím hoặc tổ hợp nút tay cầm (Xbox / XInput) để nghe bản dịch, giọng nữ hoặc nam miễn phí. Chạy ngay trên máy bằng [Piper](https://github.com/rhasspy/piper), chỉ tải một lần. Xem [mục 11](docs/huong-dan-su-dung.md#11-thuyết-minh-tiếng-việt).
- **Phím tắt**: F9 +1 và học mẫu · F10 −1 · F11 chụp mẫu · F8 bật/tắt nhận diện · F7 ẩn/hiện app · Ctrl+Alt+T bật/tắt dịch · Ctrl+Alt+R khoanh lại vùng hội thoại · Ctrl+Alt+V đọc bản dịch.
- **Âm thanh xác nhận** mỗi lần đếm (tự chọn file .mp3 / .wav).
- **Cài và cập nhật bằng một lệnh** PowerShell. App và dữ liệu nằm chung một thư mục; **gỡ cài đặt** ngay trong app sẽ xóa sạch. Death count không cần internet; Translate thì cần.

### Bắt đầu nhanh

1. Mở PowerShell, chạy lệnh dưới. Lần đầu script hỏi thư mục cài; chạy lại để cập nhật.
   ```powershell
   irm https://raw.githubusercontent.com/nguucode/byrgenwerth/main/install.ps1 | iex
   ```
2. Bấm **Thêm game**, gõ tên game.
3. Vào chơi. Màn chết hiện ra là số tự tăng. Nếu không tăng (chữ cách điệu hoặc không có chữ), bấm **F9** một lần ngay lúc màn chết đang hiện.
4. Dùng OBS: tab **Overlay/OBS** → **Copy** URL Browser Source → trong OBS thêm nguồn *Browser*, Width 1200, Height 400.

Hướng dẫn chi tiết: **[docs/huong-dan-su-dung.md](docs/huong-dan-su-dung.md)**.

### Yêu cầu và giới hạn

- Windows 10 hoặc 11, 64-bit. Bản macOS đã bỏ từ v0.12.0.
- OCR dùng ngôn ngữ đã cài trong Windows. Game tiếng Trung/Nhật/Việt... thì thêm ngôn ngữ đó trong *Settings → Time & language → Language & region*.
- Game phải chạy **Borderless** hoặc **Windowed**. Exclusive Fullscreen làm app không chụp được màn hình và che mất overlay.
- Game chỉ tối màn hình mà không có màn chết thì không tự đếm được: bấm F9.
- Tình trạng: bản sớm. Đếm số, overlay và OBS đã chạy trên máy thật. Dịch và thuyết minh mới thêm, còn ít được thử. Gặp lỗi thì mở issue kèm ảnh chụp màn hình.
