# Test Installer trên Windows VM — Checklist

Mục đích: xác nhận `veo3-manager-amd64-installer.exe` cài đặt sạch, app chạy được, gỡ cài đặt sạch trên một máy Windows **chưa từng có VEO3 Manager** và lý tưởng là **chưa có WebView2 runtime** (để verify embedded WebView2 bootstrap hoạt động).

## Lựa chọn VM

| Cách | Setup time | Phù hợp khi |
|---|---|---|
| **Hyper-V** (built-in Win 10/11 Pro) | ~10 phút | Bạn dùng Win Pro/Enterprise |
| **VirtualBox** | ~15 phút | Win Home không có Hyper-V |
| **Windows Sandbox** | ~30 giây | **Khuyến nghị** — chỉ Win 10/11 Pro + Enterprise |
| **Máy thật khác** | tùy | Có laptop dự phòng |

**Khuyến nghị: Windows Sandbox** — instant, sạch tinh tươm, đóng cửa là biến mất, không tốn ổ cứng.

### Bật Windows Sandbox (nếu chưa)

PowerShell **as Admin**:

```powershell
Enable-WindowsOptionalFeature -Online -FeatureName "Containers-DisposableClientVM" -All
```

Reboot. Sau đó tìm "Windows Sandbox" trong Start Menu.

## Bước 1 — Chuẩn bị file `.wsb` (Sandbox config)

Sandbox mặc định không có mạng + không thấy file host. Tạo file config để map folder build sang sandbox.

Tạo `c:\Users\Admin\veo3_manager_part_1\test\sandbox.wsb`:

```xml
<Configuration>
  <Networking>Default</Networking>
  <MappedFolders>
    <MappedFolder>
      <HostFolder>C:\Users\Admin\veo3_manager_part_1\build\bin</HostFolder>
      <SandboxFolder>C:\installers</SandboxFolder>
      <ReadOnly>true</ReadOnly>
    </MappedFolder>
  </MappedFolders>
  <LogonCommand>
    <Command>cmd.exe /c start "" "C:\installers"</Command>
  </LogonCommand>
</Configuration>
```

Double-click file `.wsb` → Sandbox khởi động + tự mở thư mục `C:\installers` chứa installer.

## Bước 2 — Smoke test bằng tay (5 phút)

Trong sandbox / VM:

| # | Bước | Pass khi |
|---|---|---|
| 1 | Double-click `veo3-manager-amd64-installer.exe` | Welcome screen hiện ra, **không có cảnh báo "publisher unknown"** (nếu có thì OK — chưa code-sign — bấm "Run anyway") |
| 2 | Quan sát icon installer | Phải là icon mới indigo→violet, **không phải "W" mặc định** |
| 3 | Next → Next → Install | Process chạy ~10s, không có error popup |
| 4 | Properties → Details của installer (`right-click → Properties → Details`) | ProductName="VEO3 Manager", Company="Van Quyen", Version=0.1.0, Copyright đầy đủ |
| 5 | Sau khi cài xong, không tick "Launch" → Finish | Cài hoàn tất sạch |
| 6 | Start Menu → "VEO3 Manager" | Có shortcut + icon đúng |
| 7 | Desktop có shortcut | Có + icon đúng |
| 8 | Click shortcut | App khởi động, cửa sổ frameless hiển thị, **không có lỗi WebView2 runtime not found** |
| 9 | Properties → Details của `C:\Program Files\Van Quyen\VEO3 Manager\veo3-manager.exe` | Metadata đầy đủ giống installer |
| 10 | Settings → Apps → "VEO3 Manager" → Uninstall | Gỡ cài đặt chạy thành công |
| 11 | Sau uninstall, kiểm tra `C:\Program Files\Van Quyen\` | Folder bị xoá; shortcut Start Menu + Desktop bị xoá |

## Bước 3 — Smoke test tự động (script PowerShell)

Chạy `c:\Users\Admin\veo3_manager_part_1\scripts\test-installer.ps1` để verify một phần checklist tự động (chỉ kiểm metadata + signature, không thay được mắt người cho UI).

```powershell
.\scripts\test-installer.ps1 -InstallerPath .\build\bin\veo3-manager-amd64-installer.exe
```

Output mẫu:

```
[PASS] File exists, size 11.47 MB
[PASS] Authenticode: NotSigned (chưa code-sign — chấp nhận được cho v0.1.0)
[PASS] VersionInfo.ProductName     = VEO3 Manager
[PASS] VersionInfo.CompanyName     = Van Quyen
[PASS] VersionInfo.ProductVersion  = 0.1.0
[PASS] VersionInfo.LegalCopyright  = Copyright © 2026 Van Quyen. All rights reserved.

Run installer in a VM/Sandbox to complete UI smoke tests.
```

## Bước 4 — Test trên máy KHÔNG có WebView2 (advanced)

Phần lớn Windows 10/11 hiện đại đã có WebView2 sẵn. Để test embedded bootstrap thật sự hoạt động trên máy "tinh nguyên":

1. Trong sandbox/VM **trước khi cài VEO3 Manager**, gỡ WebView2:
   ```powershell
   Get-AppxPackage *WebView2* | Remove-AppxPackage
   ```
2. Cài VEO3 Manager → installer phải tự cài WebView2 (mất ~30s lần đầu).
3. Mở app → vẫn chạy được = embedded bootstrap OK.

Sandbox luôn cài lại WebView2 mới mỗi lần khởi động → đây là môi trường test ideal.

## Báo lỗi gì nếu fail

Nếu một item nào fail, copy/paste:
- Số bước fail (1-11)
- Screenshot popup error (nếu có)
- Output `Get-EventLog -LogName Application -Newest 10 -EntryType Error` ngay sau khi fail

Tôi sẽ debug.
