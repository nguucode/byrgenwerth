# Hướng dẫn sử dụng Byrgenwerth

[English](user-guide.md) · [Về trang chính](../README.md)

## Mục lục

1. [Cài đặt](#1-cài-đặt)
2. [Màn hình chính](#2-màn-hình-chính)
3. [Tab Nhận diện: tự đếm số lần chết](#3-tab-nhận-diện-tự-đếm-số-lần-chết)
4. [Tab Hiển thị: nhãn và kiểu chữ](#4-tab-hiển-thị-nhãn-và-kiểu-chữ)
5. [Tab Overlay/OBS](#5-tab-overlayobs)
6. [Tab Cài đặt](#6-tab-cài-đặt)
7. [Phím tắt](#7-phím-tắt)
8. [Dữ liệu được lưu ở đâu](#8-dữ-liệu-được-lưu-ở-đâu)
9. [Xử lý sự cố](#9-xử-lý-sự-cố)
10. [Translate: dịch hội thoại theo thời gian thực](#10-translate-dịch-hội-thoại-theo-thời-gian-thực)
11. [Thuyết minh tiếng Việt](#11-thuyết-minh-tiếng-việt)

---

## 1. Cài đặt

Mở **PowerShell** (Start menu → gõ "PowerShell") rồi chạy:

```powershell
irm https://raw.githubusercontent.com/nguucode/byrgenwerth/main/install.ps1 | iex
```

- **Lần đầu** script hỏi thư mục cài; bấm Enter để dùng `%LOCALAPPDATA%\Programs\Byrgenwerth`. Thư mục bạn gõ không tên là `Byrgenwerth` thì script tạo thư mục `Byrgenwerth` bên trong. App và toàn bộ dữ liệu nằm trong thư mục này.
- Script thêm **Byrgenwerth** vào Start menu và vào *Apps & features* của Windows, rồi mở app.
- **Cập nhật**: chạy lại đúng lệnh đó. Script đóng app, thay file exe trong cùng thư mục, giữ nguyên dữ liệu.
- Lần đầu mở app, Windows có thể **hỏi quyền mạng**: app mở một trang web nội bộ ở `127.0.0.1` để OBS đọc. Trang này chỉ chạy trong máy. Chọn cho phép.

Cài tay: tải `Byrgenwerth.exe` ở trang [Releases](https://github.com/nguucode/byrgenwerth/releases/latest) vào thư mục bạn có quyền ghi (không phải `C:\Program Files`). SmartScreen có thể cảnh báo vì exe chưa ký số: *More info → Run anyway*. Khi đó cập nhật cũng phải làm tay.

Game phải chạy ở chế độ **Borderless** hoặc **Windowed**. Ở Exclusive Fullscreen, Windows không cho app chụp màn hình game và overlay bị che.

Byrgenwerth chỉ chạy trên Windows 10 và 11. Bản macOS đã bỏ từ v0.12.0.

## 2. Màn hình chính

- **Death count** / **Translate** chuyển giữa hai công cụ. **Luôn trên** giữ cửa sổ app luôn nằm trên. Khi game toàn màn hình (kể cả Borderless) hiện lên phía trước, app tự thu nhỏ; bấm F7 để mở lại.
- **Game**: game bạn đang chơi. Mỗi game có số đếm, ảnh mẫu và từ khóa riêng. **Thêm game** để thêm. Đổi tên và xóa game nằm ở tab Cài đặt.
- Số màu đỏ là số lần chết, kèm nút **+1**, **−1** và **Reset**.
- **4 tab**: Nhận diện, Hiển thị, Overlay/OBS, Cài đặt.
- Dòng cuối là **trạng thái**: app vừa làm gì, hoặc báo lỗi.

Rê chuột lên ô hoặc nút để xem giải thích ngắn.

## 3. Tab Nhận diện: tự đếm số lần chết

### App nhận ra lúc chết bằng cách nào

App chụp màn hình khoảng 2 lần mỗi giây và dùng hai cách cùng lúc:

1. **OCR**: đọc chữ trên màn hình bằng tính năng nhận dạng chữ có sẵn của Windows, tìm từ khóa như "YOU DIED", "WASTED", "DEAD". Chữ phải **to** và **gần giữa màn hình**; chữ nhỏ trong menu, tên vật phẩm, HUD ở góc đều bị bỏ qua. Cách này dùng chung cho mọi game, không còn danh sách mẫu riêng cho từng game.
2. **Ảnh mẫu**: so màn hình với ảnh màn chết đã lưu. Dùng cho chữ cách điệu hoặc game không có chữ. Khi OCR đã đọc được chữ chết trong phiên chơi, ảnh mẫu khớp một mình không còn được tính: ảnh mẫu màn chết tối cũng khớp lúc hồi sinh trong cảnh tối hay khi công cụ chụp màn hình làm tối màn hình.

Mỗi màn chết chỉ tính **một lần**, dù hiện bao lâu.

### Các mục cài đặt

| Mục | Ý nghĩa |
|---|---|
| Tự nhận diện | Bật tự đếm. Tắt nếu muốn đếm tay (F9/F10). |
| Đọc chữ (OCR) | Bật OCR. |
| Màn hình | Màn hình đang chạy game. |
| Ngôn ngữ | Ngôn ngữ OCR. "Theo Windows" là đủ cho game tiếng Anh. Ngôn ngữ khác xem [xử lý sự cố](#ocr-không-đọc-được-game-tiếng-trungnhậtviệt). |
| Từ khóa | Từ khóa, cách nhau bằng dấu phẩy. Thêm `*` ở cuối để khớp phần đầu câu, ví dụ `You were slain by *`. Xóa hết để quay về bộ từ khóa chung. |
| Ngưỡng | Ngưỡng so ảnh mẫu, mặc định **70%**. Hạ xuống nếu hay sót, tăng lên (85–90%) nếu đếm nhầm. |
| Cooldown | Số giây chờ sau mỗi lần đếm, mặc định **10 giây**. |
| Vùng | Vùng dùng để so ảnh mẫu. **Khoanh sát dòng chữ chết** (nút Khoanh...) để chính xác nhất. Lần đầu OCR bắt được, app tự khoanh. |
| Ảnh mẫu | Ảnh mẫu đã lưu (tối đa 10). **Chụp mẫu** lưu màn hình hiện tại, **Xóa mẫu** xóa mẫu đang chọn. Xóa mẫu chụp sai để tránh đếm nhầm. |
| OCR | Chữ OCR đang đọc được, ví dụ `Thấy "DEAD" (cao 4.6%) → KHỚP`. Xem dòng này đầu tiên khi bị sót. |
| Độ giống | Độ giống giữa màn hình và ảnh mẫu, cập nhật liên tục. |

### Game không tự đếm được

1. Xem dòng **OCR** lúc màn chết đang hiện.
2. Nếu OCR không đọc được (chữ cách điệu, chữ Hán, không có chữ): bấm **F9** ngay lúc màn chết. App cộng 1 và lưu ảnh mẫu.
3. Bấm **Khoanh...** cạnh **Vùng**, khoanh sát dòng chữ hoặc biểu tượng chết.
4. Từ lần chết sau app tự đếm.

Game chỉ tối màn hình mà không có màn chết thì app không tự đếm được: bấm F9 mỗi lần chết.

## 4. Tab Hiển thị: nhãn và kiểu chữ

### Nhãn

Nhãn hiện cùng con số, cả trên overlay lẫn trong OBS.

| Mục | Ý nghĩa |
|---|---|
| Kiểu | **Chữ**, **Hình** (file PNG hoặc SVG) hoặc **Không có nhãn** (chỉ có số). |
| Chữ | Chữ của nhãn, ví dụ `Deaths`. `{game}` được thay bằng tên game. |
| Hình | **Chọn...** để chọn file `.png` hoặc `.svg`. File được chép vào thư mục dữ liệu, nên có thể xóa hay di chuyển file gốc. |
| Cỡ | Cỡ nhãn (px): cỡ chữ, hoặc chiều cao của hình. |
| Vị trí | Nhãn nằm ở góc nào quanh con số: trên trái, trên phải, dưới trái, dưới phải. "Trên" là phía trên con số, "dưới" là phía dưới; "trái"/"phải" là canh theo mép đó của con số. |

### Kiểu chữ

Font (font có trong máy, hoặc gõ tên), cỡ của con số, đậm, độ dày viền (bật/tắt), và hai màu: **Chữ** và **Viền**. Rê chuột lên nút màu để xem mã màu.

## 5. Tab Overlay/OBS

### Overlay trên màn hình

1. Tích **Hiện**.
2. Bấm **Kéo vị trí**: hiện khung viền vàng, kéo đi đâu cũng được.
3. Bấm **Lưu vị trí**. Bình thường chuột bấm xuyên qua overlay. **Đặt lại** đưa overlay về góc trên bên trái.

OBS chỉ quay được overlay này khi dùng **Display Capture**. Dùng Game Capture thì dùng Browser Source bên dưới.

### OBS (nên dùng Browser Source)

1. Bấm **Copy** cạnh **Browser** (`http://127.0.0.1:17321/`).
2. Trong OBS: *Sources → + → Browser*, dán vào ô **URL**, **Width 1200**, **Height 400**.
3. Đặt vị trí. Nền trong suốt, đúng kiểu chữ và nhãn, tự cập nhật.

Cách cũ: tích **Ghi file** rồi dùng OBS *Text (GDI+) → Read from file* với đường dẫn lấy từ nút **Copy** bên cạnh. File chỉ có chữ nhãn và con số, không có kiểu chữ hay hình.

## 6. Tab Cài đặt

### Âm thanh mỗi lần +1

App không kèm sẵn âm thanh. **Chọn...** để chọn file `.mp3` hoặc `.wav` của bạn, **Âm lượng** chỉnh âm lượng (chỉ với mp3), **Bật** để bật, **Nghe thử** để nghe, **Bỏ file** để bỏ. Âm thanh phát mỗi lần +1 (tự đếm, F9 hay bấm nút), không phát khi −1.

### Game đang chọn

**Đổi tên** đổi tên game đang chọn. **Xóa game** xóa game cùng số đếm và ảnh mẫu.

### Phím tắt và dữ liệu

Danh sách phím tắt hiện ở đây (xem [mục 7](#7-phím-tắt)). **Thư mục** mở thư mục dữ liệu.

### Gỡ cài đặt

Bấm **Gỡ app...**, hoặc vào Windows *Settings → Apps → Byrgenwerth → Uninstall*. Sau khi xác nhận, app xóa:

- toàn bộ dữ liệu, gồm API key, hình nhãn và giọng đọc đã tải: `ByrgenwerthData`, `%APPDATA%\Byrgenwerth`, cùng `%APPDATA%\DeathCounter` và thư mục bạn chọn ở bản cũ (chỉ các file của app trong đó, không đụng file khác),
- `Byrgenwerth.exe`, và thư mục cài nếu không còn gì bên trong,
- shortcut trong Start menu và mục trong *Apps & features*.

Không hoàn tác được. Muốn giữ số đếm thì chép thư mục `ByrgenwerthData` ra chỗ khác trước.

## 7. Phím tắt

| Phím | Tác dụng |
|---|---|
| F9 | +1. Nếu màn hình hiện tại chưa là ảnh mẫu thì lưu làm mẫu. |
| F10 | −1 (sửa khi đếm nhầm). |
| F11 | Chỉ lưu ảnh mẫu, không đếm. |
| F8 | Bật/tắt tự nhận diện. |
| F7 | Ẩn/hiện cửa sổ app (cũng dùng để mở lại sau khi app tự thu nhỏ vì game). Nhận diện, overlay và OBS vẫn chạy. |
| Ctrl+Alt+T | Bật/tắt dịch. |
| Ctrl+Alt+R | Khoanh lại vùng hội thoại để dịch. |
| Ctrl+Alt+V | Đọc bản dịch. Đổi ở Translate → Thuyết minh → **Gán...**. |

Phím tắt dùng được khi đang trong game; game không nhận các phím này. Muốn đổi các phím còn lại, sửa `hotkeys` trong `config.json` (ví dụ `"add": "Ctrl+F9"`) rồi mở lại app.

## 8. Dữ liệu được lưu ở đâu

Trong `ByrgenwerthData` cạnh `Byrgenwerth.exe` (nút **Thư mục** ở tab Cài đặt):

| File | Nội dung |
|---|---|
| `config.json` | Cài đặt chung: phím tắt, kiểu chữ, nhãn, vị trí overlay, thuyết minh... |
| `profiles\<game>\profile.json` | Số đếm và từ khóa từng game. |
| `profiles\<game>\templates\` | Ảnh mẫu màn chết. |
| `label.png` / `label.svg` | Hình nhãn, nếu bạn chọn. |
| `piper\` | Bộ đọc và giọng thuyết minh, sau khi tải. |
| `history.csv` | Nhật ký mọi lần +1/−1, mở được bằng Excel. |
| `obs_deaths.txt` | File text cho OBS (nếu bật). |
| `ocr-debug.log` | Mọi lần OCR đọc khi game toàn màn hình đang ở phía trước (chữ, vị trí, cỡ chữ), và mọi lần đếm. Đầy 4 MB thì chuyển sang `ocr-debug.old.log`. Gửi kèm khi báo lỗi. |
| `translate.key` | API key Anthropic của bạn, mã hóa theo tài khoản Windows. |
| `ocr-misses.log` | Chữ OCR đọc được trong 10 giây trước mỗi lần bấm F9 +1, tức các lần app bỏ sót. Gửi kèm khi báo lỗi. |

Sao lưu bằng cách chép cả thư mục.

## 9. Xử lý sự cố

### Chết mà không đếm
- Bấm F9 ngay: app cộng 1 và ghi chữ OCR đọc được vào `ocr-misses.log` trong thư mục dữ liệu. File này cho biết vì sao bị sót.
- Xem dòng **OCR** lúc màn chết đang hiện.
  - Không đọc được gì hoặc đọc ra chữ lạ: chữ cách điệu. Dùng F9 + khoanh vùng (mục 3).
  - Đọc đúng chữ nhưng báo "không phải từ khóa": thêm chữ đó vào **Từ khóa**.
- Game đang Exclusive Fullscreen: chuyển sang Borderless.
- Nhiều màn hình: kiểm tra **Màn hình**.

### Đếm nhầm
- Bấm F10.
- Xóa ảnh mẫu sai, tăng **Ngưỡng**, khoanh **Vùng** sát hơn.

### Phím tắt không chạy
Dòng trạng thái báo "Cảnh báo phím tắt" khi phím đã bị chương trình khác dùng. Đổi phím trong `config.json`, hoặc với phím đọc bản dịch thì dùng **Gán...**.

### Không thấy overlay trong game
Game đang Exclusive Fullscreen. Chuyển sang Borderless.

### OCR không đọc được game tiếng Trung/Nhật/Việt
1. Vào Windows *Settings → Time & language → Language & region → Add a language*, thêm ngôn ngữ của game.
2. Mở lại app và chọn ngôn ngữ đó ở **Ngôn ngữ** (tab Nhận diện).
3. Thêm chữ chết bằng ngôn ngữ đó vào **Từ khóa**. Dấu không quan trọng: "BẠN ĐÃ CHẾT" khớp cả "BAN DA CHET".

### OBS không hiện gì
- App phải đang chạy.
- Cổng có thể khác 17321 nếu bị chiếm; luôn copy URL từ app.

## 10. Translate: dịch hội thoại theo thời gian thực

Tab **Dịch** đọc vùng hội thoại trong game rồi hiện bản dịch tiếng Việt trong một khung ngay cạnh đó. Chỉ dịch từ tiếng Anh sang tiếng Việt.

1. Lấy API key ở [console.anthropic.com](https://console.anthropic.com) rồi dán vào ô **API key**. Bạn trả phí cho Anthropic theo lượng dùng; chi phí ước tính được cộng dồn từ số token và bảng giá. Chi phí tính lại từ đầu khi đổi key hoặc đổi game.
2. Mở game, bấm **Ctrl+Alt+R** rồi kéo chuột khoanh khung hội thoại. Khoanh càng hẹp càng đỡ tốn phí và càng ít đọc nhầm. Nút **Khoanh...** trong tab cũng làm việc này.
3. Tích **Bật dịch Anh → Việt** hoặc bấm **Ctrl+Alt+T**. Mỗi lần mở app, dịch luôn ở trạng thái tắt, nên chưa bật thì không gửi gì.

Cần biết:

- **Dữ liệu gửi ra ngoài**: chữ OCR đọc được trong vùng khoanh, 8 câu trước đó và tên game đang chọn được gửi tới API của Anthropic (`claude-haiku-5-5`). Ảnh chụp màn hình không được gửi, phần ngoài vùng khoanh cũng không.
- **Key** được mã hóa theo tài khoản Windows (DPAPI) trong file `translate.key`. Gỡ app là xóa luôn.
- Một câu chỉ được dịch khi chữ đã đứng yên, nên chữ hiện từng ký tự sẽ đợi hiện xong. Câu lặp lại lấy từ cache, không tốn phí.
- Khi lỗi, khung hiện câu tiếng Anh kèm lý do. Vùng khoanh có hơn 500 ký tự thì không dịch: hãy khoanh hẹp lại.
- Windows cần có OCR tiếng Anh: *Settings → Time & language → Language & region*.
- Game phải chạy **Borderless** hoặc **Windowed**, nếu không khung dịch không hiện đè lên được.
- Tính năng dịch chưa được thử với nhiều game. Có thể còn lỗi.

## 11. Thuyết minh tiếng Việt

Tab **Thuyết minh** đọc to bản dịch tiếng Việt khi bạn bấm phím hoặc nút tay cầm.

1. Chọn giọng ở **Giọng**: **Nữ** hoặc **Nam**. Bấm **Tải** một lần. App tải [Piper](https://github.com/rhasspy/piper) (22 MB) và giọng đọc (nữ 63 MB, nam 28 MB), kiểm tra từng file theo mã hash cố định, rồi lưu trong `ByrgenwerthData\piper`.
2. **Nghe thử** để nghe mẫu. **Tốc độ** chỉnh tốc độ đọc (0 = bình thường, +50 = nhanh hơn, −50 = chậm hơn), **Âm lượng** chỉnh âm lượng.
3. Gán phím bằng **Gán...** cạnh **Bàn phím** (mặc định **Ctrl+Alt+V**), gán tổ hợp nút tay cầm bằng **Gán...** cạnh **Tay cầm**: giữ cùng lúc các nút rồi thả ra.
4. Trong game, bấm phím hoặc tổ hợp nút: bản dịch đang hiện (hoặc bản dịch gần nhất) được đọc to. Bấm lần nữa để dừng. Nút **Đọc dịch** trong app cũng làm việc này.

Cần biết:

- Thuyết minh chạy **ngay trên máy**. Tải xong thì không cần internet và không gửi chữ đi đâu.
- Giọng đọc là giọng tiếng Việt miễn phí của Piper: giọng nữ `vais1000` (chất lượng trung bình), giọng nam là một người đọc trong bộ `vivos` (chất lượng thấp hơn). Nghe vẫn rõ là giọng máy.
- Tay cầm: tay Xbox, hoặc tay được Windows nhận như tay Xbox (Steam Input, DS4Windows). Game cũng nhận nút bạn bấm, nên chọn tổ hợp game không dùng, ví dụ **LB+RB+Y**.
- Phải có bản dịch mới đọc được, nên bật **Bật dịch** trước.
