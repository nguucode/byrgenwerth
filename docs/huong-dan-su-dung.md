# Hướng dẫn sử dụng Byrgenwerth

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
11. [Tab Translate: dịch hội thoại theo thời gian thực](#11-tab-translate-dịch-hội-thoại-theo-thời-gian-thực)

---

## 1. Cài đặt

Mở **PowerShell** (Start menu → gõ "PowerShell") rồi chạy:

```powershell
irm https://raw.githubusercontent.com/nguucode/death-counter/main/install.ps1 | iex
```

- **Lần đầu** script hỏi thư mục cài; bấm Enter để dùng `%LOCALAPPDATA%\Programs\Byrgenwerth`. Thư mục bạn gõ không tên là `Byrgenwerth` thì script tạo thư mục `Byrgenwerth` bên trong. App và toàn bộ dữ liệu nằm trong thư mục này.
- Script thêm **Byrgenwerth** vào Start menu và vào *Apps & features* của Windows, rồi mở app.
- **Cập nhật**: chạy lại đúng lệnh đó. Script đóng app, thay file exe trong cùng thư mục, giữ nguyên dữ liệu. Bản Death Counter đang cài sẽ được đổi tên thành Byrgenwerth, giữ nguyên số đếm và cài đặt. Nếu dùng file text trong OBS thì cập nhật lại đường dẫn.
- Lần đầu mở app, Windows có thể **hỏi quyền mạng**: app mở một trang web nội bộ ở `127.0.0.1` để OBS đọc. Trang này chỉ chạy trong máy. Chọn cho phép. Chỉ tab Translate mới ra internet, và chỉ khi đang bật.

Cài tay: tải `Byrgenwerth.exe` ở trang [Releases](https://github.com/nguucode/death-counter/releases/latest) vào thư mục bạn có quyền ghi (không phải `C:\Program Files`). SmartScreen có thể cảnh báo vì exe chưa ký số: *More info → Run anyway*. Khi đó cập nhật cũng phải làm tay.

Game phải chạy ở chế độ **Borderless** hoặc **Windowed**. Ở Exclusive Fullscreen, Windows không cho app chụp màn hình game và overlay bị che.

### macOS

macOS 14 trở lên, Apple Silicon hoặc Intel. Có đủ hai công cụ như bản Windows. Chạy lệnh này trong Terminal:

```bash
curl -fsSL https://raw.githubusercontent.com/nguucode/death-counter/main/install.sh | sh
```

App được cài vào `~/Applications`, dữ liệu nằm ở `~/Library/Application Support/Byrgenwerth`. Chạy lại lệnh để cập nhật. App không có icon ở Dock: dùng mục **Byrgenwerth** trên thanh menu để mở cửa sổ hoặc thoát.

- Cho phép **Screen Recording** khi macOS hỏi, rồi mở lại app. App chưa được notarize nên macOS có thể hỏi lại sau mỗi lần cập nhật.
- API key được lưu trong Keychain.
- Phím F cần giữ **fn** trên đa số bàn phím Mac, trừ khi đã bật *Use F1, F2 as standard function keys*. F11 cũng là phím Show Desktop mặc định, nên dùng nút **Chụp mẫu ngay** sẽ dễ hơn. Muốn đổi phím tắt thì sửa mục `hotkeys` trong `config.json`.
- Tên boss được đọc khi cửa sổ của app khác chiếm gần hết màn hình. Cửa sổ trình duyệt lớn cũng tính, nên khi đánh boss đừng để nó ở phía trước.
- Bản Mac chưa được thử với game thật. Đa số game Windows chạy trên Mac qua lớp dịch, nên khả năng đọc chữ có thể khác.

## 2. Màn hình chính

Đầu cửa sổ là tên app và phiên bản. Hai nút **Death count** và **Translate** chuyển giữa hai công cụ. Các mục dưới đây thuộc Death count; Translate xem [mục 11](#11-tab-translate-dịch-hội-thoại-theo-thời-gian-thực).

- **Game**: chọn game đang chơi; mỗi game có số đếm, ảnh mẫu, từ khóa và thống kê boss riêng.
- **Số lần chết**: số lớn màu đỏ. Các nút **+1**, **−1** và **Reset** để chỉnh tay.
- **4 tab**: Nhận diện, Hiển thị, Cài đặt, Boss.
- **Cửa sổ app luôn nằm trên**: giữ cửa sổ app nổi trên các cửa sổ khác. Khi một game toàn màn hình (kể cả Borderless) được mở lên trước, cửa sổ app tự thu nhỏ; bấm F7 để gọi lại.
- Dòng cuối là **trạng thái**: app vừa làm gì, lỗi gì nếu có.

## 3. Thêm game

1. Bấm **+ Thêm**, gõ tên game, OK.
2. Nếu tên trùng một trong 100 game có sẵn mẫu (gõ gần đúng là được, ví dụ "elden ring", "wuchang"), app tự nạp từ khóa màn chết và vùng tên boss của game đó.
3. **Đổi tên** / **Xóa** để quản lý. Xóa game sẽ xóa luôn số đếm và ảnh mẫu của game đó.

### Game được hỗ trợ

Game nào chạy trên Windows cũng đếm được: OCR không bắt được màn chết thì bấm **F9** một lần để dạy ảnh mẫu, hoặc dùng [chế độ thanh máu](#game-không-có-màn-chết-màn-hình-tối-rồi-về-điểm-lưu). Khác nhau giữa các game là app tự làm được bao nhiêu. "Đã xác minh" dưới đây nghĩa là đã tìm được chữ màn chết qua wiki và ảnh chụp ([nguồn](death-screens-top100.csv)); chưa phải game nào cũng đã được chạy thử trong app.

**Tự đếm, chữ dễ đọc (27):** Elden Ring, Elden Ring Nightreign, Dark Souls III, Dark Souls Remastered, Bloodborne, Demon's Souls (2020), Wuchang: Fallen Feathers, Stellar Blade, The First Berserker: Khazan, Diablo IV, The Witcher 3: Wild Hunt, Fortnite, Apex Legends, PUBG: Battlegrounds, Overwatch 2, Escape from Tarkov, Team Fortress 2, Rust, DayZ, Left 4 Dead 2, Resident Evil 4 (2023 remake), Assassin's Creed Shadows, Metal Gear Solid Delta: Snake Eater, Minecraft, Subnautica, Lethal Company, Risk of Rain 2.
Vài game chỉ đếm được một phần: Apex Legends và Risk of Rain 2 chỉ đếm khi cả đội hoặc cả run thua; PUBG và Escape from Tarkov đếm ở màn tổng kết, không phải ngay lúc chết; Assassin's Creed Shadows đếm cả khi hỏng nhiệm vụ (DESYNCHRONIZED).

**Tự đếm nhưng chữ cách điệu: OCR có thể trượt, bấm F9 một lần (10):** Sekiro: Shadows Die Twice (cần OCR tiếng Nhật), Hunt: Showdown 1896, Grand Theft Auto V, Red Dead Redemption 2, Resident Evil 2 (2019 remake), Hades, Terraria, Cuphead, Balatro, Fall Guys. Balatro và Fall Guys đếm lần thua run hoặc bị loại, không phải chết.

**Có mẫu nhưng chưa xác minh chữ chết, dùng từ khóa chung (54):** Lies of P, Black Myth: Wukong, Lords of the Fallen (2023), Nioh 2, Mortal Shell, Hollow Knight: Silksong, Monster Hunter Wilds, Monster Hunter: World, Dragon's Dogma 2, Baldur's Gate 3, Path of Exile 2, Cyberpunk 2077, Counter-Strike 2, Valorant, Call of Duty: Warzone, Call of Duty: Black Ops 6, Call of Duty: Black Ops 7, Battlefield 6, Marvel Rivals, Tom Clancy's Rainbow Six Siege, ARC Raiders, Helldivers 2, The Finals, Destiny 2, Deadlock, Delta Force, Deep Rock Galactic, Resident Evil Village, Silent Hill 2 (2024 remake), Dead Space (2023 remake), The Last of Us Part II, God of War Ragnarök, Ghost of Tsushima, Ghost of Yōtei, Marvel's Spider-Man 2, Horizon Forbidden West, Assassin's Creed Valhalla, Kingdom Come: Deliverance II, Death Stranding 2: On the Beach, Ninja Gaiden 2 Black, Devil May Cry 5, Hades II, Dead Cells, The Binding of Isaac: Rebirth, Valheim, 7 Days to Die, Project Zomboid, Palworld, Enshrouded, R.E.P.O., PEAK, Vampire Survivors, Slay the Spire, League of Legends.
Nhóm này có thể tự đếm nếu màn chết trùng từ khóa chung; thường sẽ cần bấm F9 một lần.

**Không có chữ chết hoặc chữ quá nhỏ: dùng F9 + ảnh mẫu hoặc chế độ thanh máu (9):** Hollow Knight, The Last of Us Part I, Phasmophobia, Celeste, Super Meat Boy, Geometry Dash (không có chữ chết); Don't Starve Together, Dead by Daylight, Dota 2 (chữ quá nhỏ).

Với mọi game: chỉ chạy trên Windows, game phải để **Borderless** hoặc **Windowed**. Game console (ví dụ Bloodborne, Demon's Souls) chỉ đếm được khi hình hiện trong một cửa sổ trên PC, như PS Remote Play hoặc cửa sổ xem trước của capture card.

## 4. Tab Nhận diện: tự đếm số lần chết

### Cách app nhận ra bạn vừa chết

App chụp màn hình khoảng 2 lần mỗi giây và dùng hai cách song song:

1. **OCR (đọc chữ)**: tìm từ khóa như "YOU DIED", "WASTED", "DEAD"... Chữ phải **to** và **nằm gần giữa màn hình**; chữ nhỏ trong menu, tên item hay chữ ở góc đều bị bỏ qua.
2. **Ảnh mẫu**: so màn hình với ảnh màn chết đã lưu. Dùng cho game có chữ cách điệu hoặc không có chữ. Khi OCR đã đọc được chữ chết trong phiên chơi này, chỉ khớp ảnh mẫu thôi thì không đếm nữa: ảnh mẫu màn chết tối cũng khớp cảnh hồi sinh tối hay màn hình bị công cụ chụp làm tối.

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

Dữ liệu nằm trong thư mục `ByrgenwerthData` cạnh file `Byrgenwerth.exe`, nên thư mục app chứa tất cả. Nút **Mở** mở thư mục đó trong Explorer.

- Lên từ Death Counter: thư mục `DeathCounterData` được đổi tên ở lần mở đầu tiên. Lên từ bản 0.8 trở về trước (dữ liệu ở `%APPDATA%\DeathCounter` hoặc thư mục bạn tự chọn): lần mở đầu tiên app tự chép sang một lần. Bản cũ giữ nguyên cho tới khi gỡ app.
- Thư mục app không ghi được thì dữ liệu tạm lưu ở `%APPDATA%\Byrgenwerth` và app báo cho bạn. Hãy cài lại vào thư mục bạn có quyền ghi.

### Gỡ cài đặt

Bấm **Gỡ Byrgenwerth...** trong tab này, hoặc vào *Settings → Apps → Byrgenwerth → Uninstall* của Windows. Sau khi bạn xác nhận, app xóa:

- toàn bộ dữ liệu, gồm cả API key: `ByrgenwerthData`, `%APPDATA%\Byrgenwerth`, cùng `%APPDATA%\DeathCounter` và thư mục bạn chọn ở bản cũ (chỉ xóa file của app trong đó, không đụng file khác),
- file `Byrgenwerth.exe`, và thư mục cài nếu không còn gì,
- shortcut trong Start menu và mục trong *Apps & features*.

Không hoàn tác được. Muốn giữ số đếm thì copy `ByrgenwerthData` ra chỗ khác trước.

## 7. Tab Boss: Top 3 boss

### Cách app biết bạn đang đánh boss nào

- App đọc chữ ở **dải trên** và **dải dưới** màn hình, nơi hầu hết game in tên boss trên thanh máu.
- Một dòng chữ phải **đứng yên khoảng 5 giây** mới được coi là tên boss, nên phụ đề bị loại. Chữ đứng liên tục hơn 15 phút bị coi là HUD.
- Khi bạn chết, lần chết được tính cho boss vừa thấy trong **30 giây** gần nhất, hoặc cho boss được xác nhận trong vòng 15 giây sau khi chết (game Souls thường chỉ hiện rõ tên trên màn chết). Bấm −1 thì trừ đúng boss đó.
- App chỉ đọc tên boss khi một game toàn màn hình (kể cả Borderless) đang ở trước, nên chữ trong trình duyệt hay cửa sổ chat bạn chuyển sang bị bỏ qua. Dòng có dấu hai chấm, như dòng nhắc nút bấm ("E : Nghỉ ngơi tại bonfire"), không bao giờ được coi là tên boss.

### Các mục

| Mục | Ý nghĩa |
|---|---|
| Ghi nhận boss khi chết | Bật/tắt theo dõi boss cho game này. |
| Đang đánh | Tên boss app đang thấy. Đánh boss mà dòng này vẫn trống thì xem mục 10. |
| Vùng tên boss | "Tự động" (dải trên + dưới), "Theo mẫu game" (ví dụ Sekiro: góc trên trái) hoặc vùng bạn tự khoanh. |
| Thống kê | Danh sách boss và số lần chết. OCR đọc lệch thì dùng **Đổi tên**, **Gộp vào...** (gộp hai tên của cùng một boss), **Xóa**. |
| Bảng Top 3 | Tên game, một đường kẻ, **Deaths** kèm tổng số lần chết, một đường kẻ, rồi top 3 boss kèm số lần chết. Chỉ có chữ và đường kẻ, theo màu và viền chữ ở tab Hiển thị. ▶ đánh dấu boss đang đánh; boss đó ngoài top 3 thì hiện thêm một dòng. Tên dài bị cắt còn một dòng với "…". |

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
| F7 | Ẩn/hiện cửa sổ app (cũng gọi lại cửa sổ sau khi nó tự thu nhỏ vì game). Nhận diện, overlay và OBS vẫn chạy. |
| Ctrl+Alt+T | Bật/tắt dịch. |
| Ctrl+Alt+R | Khoanh lại vùng hội thoại để dịch. |

Phím tắt hoạt động cả khi đang ở trong game. Game sẽ không nhận được các phím này.
Đổi phím: mở `config.json` trong thư mục dữ liệu, sửa mục `hotkeys` (ví dụ `"add": "Ctrl+F9"`), rồi mở lại app.

## 9. Dữ liệu được lưu ở đâu

Trong thư mục `ByrgenwerthData` cạnh file `Byrgenwerth.exe` (nút **Mở** ở tab Cài đặt):

| File | Nội dung |
|---|---|
| `config.json` | Cài đặt chung: phím tắt, kiểu chữ, vị trí overlay... |
| `profiles\<game>\profile.json` | Số đếm, từ khóa, thống kê boss của từng game. |
| `profiles\<game>\templates\` | Ảnh mẫu màn chết. |
| `history.csv` | Nhật ký từng lần +1/−1, mở bằng Excel được. |
| `obs_deaths.txt` | File text cho OBS (nếu bật). |
| `ocr-debug.log` | Mọi lần OCR đọc lúc game toàn màn hình đang ở trước (chữ, vị trí, cỡ chữ), mọi lần đếm và mọi lần đổi boss. Đầy 4 MB thì chuyển sang `ocr-debug.old.log`. Gửi kèm khi báo lỗi. |
| `translate.key` | API key Anthropic, mã hóa theo tài khoản Windows của bạn. |
| `ocr-misses.log` | Chữ OCR đọc được trong 10 giây trước mỗi lần bấm F9 +1, tức là những lần app bỏ sót. Gửi kèm khi báo lỗi. |

Sao lưu: copy cả thư mục này.

## 10. Xử lý sự cố

### Chết mà không tự đếm
- Bấm F9 ngay: app cộng 1 và ghi những gì OCR đọc được vào `ocr-misses.log` trong thư mục dữ liệu. File này cho biết vì sao bị sót.
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
3. Thêm từ khóa màn chết bằng ngôn ngữ đó vào ô **Từ khóa**. Không cần đúng dấu: "BẠN ĐÃ CHẾT" khớp cả "BAN DA CHET". Mẫu của các game hiện "YOU DIED" đã có sẵn "BẠN ĐÃ CHẾT" cho bản Việt hóa.

### Boss không được ghi nhận
- **Thống kê** chỉ có dữ liệu khi bạn chết: boss hiện trong danh sách sau lần đầu nó giết bạn, không phải lúc thanh máu boss xuất hiện.
- Xem dòng **Đang đánh** trong lúc đánh boss.
- Trống: tên boss có thể nằm ngoài vùng tự động hoặc chữ quá nhỏ. Bấm **Khoanh vùng...** ở tab Boss và khoanh quanh tên boss.
- Ra sai tên (bắt nhầm chữ khác): khoanh vùng hẹp hơn, sửa tên trong **Thống kê**.

### OBS không hiện Browser Source
- App phải đang mở.
- Kiểm tra URL: cổng có thể khác 17321 nếu cổng đó bận; luôn copy URL từ app.

## 11. Tab Translate: dịch hội thoại theo thời gian thực

App đọc vùng hội thoại trong game và hiện bản dịch tiếng Việt trong một khung ngay cạnh vùng đó. Hiện chỉ dịch từ tiếng Anh sang tiếng Việt.

1. Lấy API key ở [console.anthropic.com](https://console.anthropic.com), dán vào ô **API key**. Bạn trả phí cho Anthropic theo lượng dùng; dòng **Chi phí ước tính** cộng dồn từ số token và bảng giá, tính lại từ đầu khi đổi key hoặc đổi game.
2. Mở game, bấm **Ctrl+Alt+R** rồi kéo chuột khoanh khung hội thoại. Khoanh càng hẹp càng đỡ tốn phí và càng ít đọc nhầm. Bấm phím này ngay trong game cũng được, hoặc dùng nút **Khoanh vùng...** trong tab.
3. Tick **Bật dịch** hoặc bấm **Ctrl+Alt+T**. Mỗi lần mở app, phần dịch luôn tắt, nên chưa bật thì không có gì được gửi đi.

Cần biết:

- **Dữ liệu gửi đi**: chữ OCR đọc được trong vùng khoanh, 8 câu trước đó và tên game đang chọn được gửi tới API của Anthropic (`claude-haiku-5-5`). Ảnh màn hình không được gửi, chữ ngoài vùng khoanh cũng không.
- **API key** được mã hóa theo tài khoản Windows của bạn (DPAPI) trong file `translate.key`. Gỡ app sẽ xóa file này.
- App chờ chữ đứng yên rồi mới dịch, nên chữ hiện dần sẽ đợi hiện hết. Câu lặp lại lấy từ cache, không tốn phí.
- Khi lỗi, khung hiện câu tiếng Anh kèm lý do. Vùng có hơn 500 ký tự sẽ không dịch: hãy khoanh hẹp lại.
- Windows cần có gói OCR tiếng Anh: *Settings → Time & language → Language & region*.
- Game phải chạy **Borderless** hoặc **Windowed**, nếu không khung dịch không hiện được phía trên.
- Phần dịch mới thêm và chưa được thử với game thật. Có thể còn lỗi.
