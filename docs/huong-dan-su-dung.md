# Hướng dẫn sử dụng Death Counter

[English](user-guide.md) · [Về trang chính](../README.md)

## Mục lục

1. [Cài đặt](#1-cài-đặt)
2. [Màn hình chính](#2-màn-hình-chính)
3. [Thêm game](#3-thêm-game)
4. [Tab Nhận diện: tự đếm số lần chết](#4-tab-nhận-diện-tự-đếm-số-lần-chết)
5. [Tab Hiển thị: overlay và OBS](#5-tab-hiển-thị-overlay-và-obs)
6. [Tab Cài đặt: âm thanh và dữ liệu](#6-tab-cài-đặt-âm-thanh-và-dữ-liệu)
7. [Tab Boss: Top 3 boss](#7-tab-boss-top-3-boss)
8. [Phím tắt](#8-phím-tắt)
9. [Dữ liệu được lưu ở đâu](#9-dữ-liệu-được-lưu-ở-đâu)
10. [Xử lý sự cố](#10-xử-lý-sự-cố)

---

## 1. Cài đặt

1. Tải `DeathCounter.exe` từ trang [Releases](https://github.com/nguucode/death-counter/releases/latest).
2. Đặt file ở đâu cũng được, ví dụ `C:\Program Files\DeathCounter\`. App không cần quyền Admin.
3. Chạy file. Lần đầu có thể gặp:
   - **SmartScreen** ("Windows protected your PC"): exe chưa ký số. Bấm *More info → Run anyway*.
   - **Hỏi quyền mạng**: app mở một trang web nội bộ ở `127.0.0.1` để OBS đọc. Trang này chỉ chạy trong máy, không ra internet. Chọn cho phép.

Game phải chạy ở chế độ **Borderless** hoặc **Windowed**. Ở Exclusive Fullscreen, Windows không cho app chụp màn hình game và overlay bị che.

## 2. Màn hình chính

- **Game**: chọn game đang chơi; mỗi game có số đếm, ảnh mẫu, từ khóa và thống kê boss riêng.
- **Số lần chết**: số lớn màu đỏ. Các nút **+1**, **−1** và **Reset** để chỉnh tay.
- **4 tab**: Nhận diện, Hiển thị, Cài đặt, Boss.
- **Cửa sổ app luôn nằm trên**: giữ cửa sổ app nổi trên các cửa sổ khác.
- Dòng cuối là **trạng thái**: app vừa làm gì, lỗi gì nếu có.

## 3. Thêm game

1. Bấm **+ Thêm**, gõ tên game, OK.
2. Nếu tên trùng một trong 100 game có sẵn mẫu (gõ gần đúng là được, ví dụ "elden ring", "wuchang"), app tự nạp từ khóa màn chết và vùng tên boss của game đó.
3. **Đổi tên** / **Xóa** để quản lý. Xóa game sẽ xóa luôn số đếm và ảnh mẫu của game đó.

## 4. Tab Nhận diện: tự đếm số lần chết

### Cách app nhận ra bạn vừa chết

App chụp màn hình khoảng 2 lần mỗi giây và dùng hai cách song song:

1. **OCR (đọc chữ)**: tìm từ khóa như "YOU DIED", "WASTED", "DEAD"... Chữ phải **to** và **nằm gần giữa màn hình**; chữ nhỏ trong menu, tên item hay chữ ở góc đều bị bỏ qua.
2. **Ảnh mẫu**: so màn hình với ảnh màn chết đã lưu. Dùng cho game có chữ cách điệu hoặc không có chữ.

Mỗi màn chết chỉ được đếm **một lần**, dù nó hiện bao lâu.

### Các mục trong tab

| Mục | Ý nghĩa |
|---|---|
| Bật tự nhận diện | Tắt nếu chỉ muốn đếm bằng tay (F9/F10). |
| Đọc chữ bằng OCR | Bật/tắt phần đọc chữ. |
| Trạng thái OCR | App đang đọc thấy gì. Ví dụ `Thấy "DEAD" (cao 4.6%) → KHỚP`. Không tự đếm thì nhìn dòng này trước. |
| Ngôn ngữ game | Ngôn ngữ OCR. Để "Theo Windows" nếu game tiếng Anh. Game tiếng khác: xem [mục 10](#ocr-không-đọc-được-game-tiếng-trungnhậtviệt). |
| Mẫu theo game | Chọn game để nạp đúng từ khóa. "Chung (mọi game)" dùng danh sách chung. Dòng chữ xám bên dưới cho biết game đó đã được xác minh chưa. |
| Từ khóa | Sửa tay được, cách nhau bằng dấu phẩy. Thêm `*` ở cuối để khớp phần đầu câu, ví dụ `You were slain by *`. |
| Độ giống | Mức giống giữa màn hình và ảnh mẫu, theo thời gian thực. |
| Ngưỡng | Giống từ mức này trở lên là tính chết. Đếm sót thì hạ (70%), đếm nhầm thì tăng (90%). |
| Cooldown | Sau mỗi lần đếm, không đếm thêm trong số giây này. Game hồi sinh nhanh thì để 8–10 giây. |
| Màn hình game | Chọn màn hình đang chạy game nếu có nhiều màn hình. |
| Vùng nhận diện | Vùng dùng để so ảnh mẫu. **Khoanh sát chữ chết** cho chính xác nhất. Lần đầu OCR bắt được, app tự khoanh. |
| Ảnh mẫu | Danh sách ảnh màn chết đã lưu (tối đa 10). Xóa ảnh chụp sai để tránh đếm nhầm. |

### Game không có màn chết (màn hình tối rồi về điểm lưu)

Một số game khi chết chỉ tối màn hình rồi tải lại, không có chữ nào. Dùng mục **Máu nhân vật**:

1. Vào game, lúc nhân vật **đang đầy máu**, bấm **Khoanh thanh máu...**. App tự ẩn đi nửa giây để chụp màn hình game.
2. Khoanh vùng thanh máu:
   - Thanh máu trên HUD: khoanh sát thanh máu.
   - Máu nằm trên người nhân vật (ví dụ cột sống của Isaac trong Dead Space): khoanh **rộng** quanh chỗ nhân vật hay đứng.
3. App tự học màu thanh máu. Dòng **Máu hiện tại** hiện % máu khi chơi.

App đếm 1 lần chết khi **máu về ~0%, rồi trong vòng 8 giây màn hình đen gần như hoàn toàn ít nhất 1 giây**. Màn tải lúc còn máu không bị tính.

Giới hạn: chết tức thì mà thanh máu không kịp tụt (rơi vực) có thể bị sót, khi đó bấm F9. Lúc máu rất thấp mà đi vào vùng tải màn đen thì có thể bị đếm nhầm, bấm F10 để trừ.

### Game không tự đếm thì làm gì

1. Nhìn **Trạng thái OCR** khi màn chết đang hiện.
2. Nếu OCR không đọc được (chữ cách điệu, chữ Hán, không có chữ): bấm **F9** đúng lúc màn chết hiện. App cộng 1 và lưu ảnh mẫu.
3. Bấm **Khoanh vùng...** và khoanh sát phần chữ/biểu tượng chết.
4. Từ lần chết sau, app tự đếm.

## 5. Tab Hiển thị: overlay và OBS

### Nội dung

**Định dạng** quyết định chữ hiển thị. `{count}` là số lần chết, `{game}` là tên game. Ví dụ `{game} · Deaths: {count}`.

### Kiểu chữ (dùng chung cho overlay và OBS)

Font (chọn từ font đã cài hoặc gõ tên), cỡ chữ, đậm, màu chữ, viền (bật/tắt, độ dày) và màu viền.

### Overlay trên màn hình

1. Tick **Hiện overlay nổi trên game**.
2. Bấm **Chỉnh vị trí overlay**: overlay hiện khung viền vàng, kéo tới chỗ muốn đặt.
3. Bấm **Xong, lưu vị trí**. Lúc bình thường, chuột bấm xuyên qua overlay nên không vướng khi chơi.

OBS chỉ quay được overlay này khi dùng **Display Capture**. Dùng Game Capture thì xem phần dưới.

### OBS (khuyên dùng Browser Source)

1. Bấm **Copy** cạnh ô Browser Source (dạng `http://127.0.0.1:17321/`).
2. Trong OBS: *Sources → + → Browser*, dán vào ô **URL**, đặt **Width 1200**, **Height 250**.
3. Kéo nguồn tới vị trí muốn đặt. Nền trong suốt, giữ đúng kiểu chữ, số tự cập nhật.

Cách cũ: tick **Ghi thêm file text**, rồi trong OBS dùng *Text (GDI+) → Read from file* với đường dẫn file. Cách này không giữ kiểu chữ.

## 6. Tab Cài đặt: âm thanh và dữ liệu

### Âm thanh

App **không kèm sẵn âm thanh**.

1. Bấm **Chọn file...**, chọn file `.mp3` hoặc `.wav` của bạn.
2. Tick **Phát âm thanh mỗi lần đếm +1**.
3. Chỉnh **Âm lượng** (chỉ có tác dụng với mp3), bấm **Nghe thử**.

Âm thanh phát mỗi lần +1 (tự đếm, F9 hay nút bấm), không phát khi −1. **Bỏ âm thanh** để tắt hẳn.

### Dữ liệu

Mặc định dữ liệu nằm ở `%APPDATA%\DeathCounter`. Muốn để chỗ khác (ổ D, thư mục OneDrive để đồng bộ giữa hai máy...):

1. Bấm **Đổi...**, chọn thư mục.
2. Thư mục trống: app hỏi rồi **chép** dữ liệu hiện tại sang. Thư mục đã có dữ liệu Death Counter: app hỏi rồi **dùng luôn** dữ liệu đó.
3. App tự mở lại với thư mục mới. Dữ liệu ở chỗ cũ vẫn giữ nguyên, bạn tự xóa khi không cần.

**Mặc định** đưa về `%APPDATA%\DeathCounter`. Không chọn được thư mục hệ thống như `C:\Program Files` (cần quyền Admin).
Nếu thư mục đã chọn không còn truy cập được (rút USB, đổi ổ), app tạm dùng thư mục mặc định và báo cho bạn.

## 7. Tab Boss: Top 3 boss

### Cách app biết bạn đang đánh boss nào

- App đọc chữ ở **dải trên** và **dải dưới** màn hình, nơi hầu hết game in tên boss trên thanh máu.
- Một dòng chữ phải **đứng yên vài giây** mới được coi là tên boss, nên phụ đề bị loại. Chữ đứng liên tục hơn 15 phút bị coi là HUD.
- Khi bạn chết, lần chết được tính cho boss vừa thấy trong **30 giây** gần nhất. Bấm −1 thì trừ đúng boss đó.

### Các mục

| Mục | Ý nghĩa |
|---|---|
| Ghi nhận boss khi chết | Bật/tắt theo dõi boss cho game này. |
| Đang đánh | Tên boss app đang thấy. Đánh boss mà dòng này vẫn trống thì xem mục 10. |
| Vùng tên boss | "Tự động" (dải trên + dưới), "Theo mẫu game" (ví dụ Sekiro: góc trên trái) hoặc vùng bạn tự khoanh. |
| Thống kê | Danh sách boss và số lần chết. OCR đọc lệch thì dùng **Đổi tên**, **Gộp vào...** (gộp hai tên của cùng một boss), **Xóa**. |
| Bảng Top 3 | Bảng kiểu speedrun: hạng 1 màu vàng, boss đang đánh tô xanh. Boss đang đánh ngoài top 3 thì hiện thêm một dòng riêng. |

### Hiện bảng Top 3

- **Trên màn hình**: tick **Hiện bảng nổi trên màn hình**, chỉnh vị trí giống overlay.
- **Trong OBS**: Copy URL dạng `http://127.0.0.1:17321/bosses` → OBS *Browser* source, Width 600, Height 300.

## 8. Phím tắt

| Phím | Tác dụng |
|---|---|
| F9 | +1. Nếu màn hình hiện tại chưa có trong ảnh mẫu thì lưu làm mẫu mới. |
| F10 | −1 (sửa khi đếm nhầm). |
| F11 | Chỉ chụp ảnh mẫu, không đếm. |
| F8 | Bật/tắt tự nhận diện. |

Phím tắt hoạt động cả khi đang ở trong game. Game sẽ không nhận được các phím này.
Đổi phím: mở `config.json` trong thư mục dữ liệu, sửa mục `hotkeys` (ví dụ `"add": "Ctrl+F9"`), rồi mở lại app.

## 9. Dữ liệu được lưu ở đâu

Mặc định `C:\Users\<tên>\AppData\Roaming\DeathCounter\`, hoặc thư mục bạn chọn ở tab Cài đặt (nút **Mở thư mục dữ liệu**):

| File | Nội dung |
|---|---|
| `config.json` | Cài đặt chung: phím tắt, kiểu chữ, vị trí overlay... |
| `profiles\<game>\profile.json` | Số đếm, từ khóa, thống kê boss của từng game. |
| `profiles\<game>\templates\` | Ảnh mẫu màn chết. |
| `history.csv` | Nhật ký từng lần +1/−1, mở bằng Excel được. |
| `obs_deaths.txt` | File text cho OBS (nếu bật). |

Sao lưu: copy cả thư mục này. Đổi chỗ lưu: xem [mục 6](#dữ-liệu).

## 10. Xử lý sự cố

### Chết mà không tự đếm
- Xem **Trạng thái OCR** lúc màn chết hiện.
  - "Không thấy chữ" hoặc đọc ra chữ lạ: chữ cách điệu. Dùng F9 + Khoanh vùng (mục 4).
  - Thấy đúng chữ nhưng "không phải từ khóa": thêm chữ đó vào ô **Từ khóa**.
- Game đang chạy Exclusive Fullscreen: chuyển sang Borderless.
- Có nhiều màn hình: kiểm tra **Màn hình game**.

### Đếm nhầm
- Bấm F10 để trừ.
- Xem tab Nhận diện: xóa ảnh mẫu chụp sai, tăng **Ngưỡng**, khoanh **Vùng nhận diện** sát hơn.

### Phím tắt không chạy
Dòng trạng thái báo "Cảnh báo phím tắt" nghĩa là phần mềm khác đang giữ phím đó. Đổi phím trong `config.json` (mục 8).

### Overlay không hiện trên game
Game đang ở Exclusive Fullscreen. Chuyển sang Borderless.

### OCR không đọc được game tiếng Trung/Nhật/Việt
1. Windows *Settings → Time & language → Language & region → Add a language*, thêm ngôn ngữ của game.
2. Mở lại app, chọn ngôn ngữ đó ở **Ngôn ngữ game** (tab Nhận diện).
3. Thêm từ khóa màn chết bằng ngôn ngữ đó vào ô **Từ khóa**.

### Boss không được ghi nhận
- Xem dòng **Đang đánh** trong lúc đánh boss.
- Trống: tên boss có thể nằm ngoài vùng tự động hoặc chữ quá nhỏ. Bấm **Khoanh vùng...** ở tab Boss và khoanh quanh tên boss.
- Ra sai tên (bắt nhầm chữ khác): khoanh vùng hẹp hơn, sửa tên trong **Thống kê**.

### OBS không hiện Browser Source
- App phải đang mở.
- Kiểm tra URL: cổng có thể khác 17321 nếu cổng đó bận; luôn copy URL từ app.
