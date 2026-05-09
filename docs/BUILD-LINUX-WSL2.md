# Build Linux từ Windows qua WSL2 — Runbook

Wails không cross-compile sang Linux từ Windows được (cần WebKitGTK + GTK3). WSL2 cho bạn một Linux thật chạy ngay trên máy Windows này, build ra binary `.AppImage` chạy được trên mọi distro.

## Bước 1 — Cài WSL2 + Ubuntu (1 lần duy nhất)

Mở **PowerShell as Administrator** rồi chạy:

```powershell
wsl --install -d Ubuntu-22.04
```

→ **Reboot máy** khi nó yêu cầu.

Sau khi khởi động lại, Ubuntu tự mở terminal lần đầu, hỏi tạo user/password Linux. Đặt gì cũng được, nhớ password để `sudo` về sau.

Verify:

```powershell
wsl -l -v
# Phải thấy: Ubuntu-22.04   Running   2
```

## Bước 2 — Cài deps cho Wails Linux (1 lần duy nhất)

Trong terminal Ubuntu (gõ `wsl` từ PowerShell hoặc mở "Ubuntu" từ Start Menu):

```bash
# Update + cài Go, build tools, GTK/WebKit headers, UPX, file utils
sudo apt-get update
sudo apt-get install -y \
  build-essential pkg-config \
  libgtk-3-dev libwebkit2gtk-4.0-dev \
  libayatana-appindicator3-dev libssl-dev libgl1-mesa-dev xvfb \
  upx wget file curl

# Cài Go (Ubuntu 22.04 mặc định Go cũ, lấy 1.25 mới)
GO_VER=1.25.0
wget -q https://go.dev/dl/go${GO_VER}.linux-amd64.tar.gz -O /tmp/go.tgz
sudo rm -rf /usr/local/go && sudo tar -C /usr/local -xzf /tmp/go.tgz
echo 'export PATH=$PATH:/usr/local/go/bin:$HOME/go/bin' >> ~/.bashrc
source ~/.bashrc
go version   # go version go1.25.0 linux/amd64

# Cài Wails CLI
go install github.com/wailsapp/wails/v2/cmd/wails@v2.12.0
wails version

# Cài Node 20 (cho frontend)
curl -fsSL https://deb.nodesource.com/setup_20.x | sudo -E bash -
sudo apt-get install -y nodejs
node -v && npm -v
```

## Bước 3 — Build VEO3 Manager Linux

```bash
# Project nằm trên ổ Windows -> WSL2 truy cập qua /mnt/c/...
cd /mnt/c/Users/Admin/veo3_manager_part_1
bash build/scripts/build-linux.sh 0.1.0 amd64
```

Output bạn sẽ thấy:

```
build/bin/
  veo3-manager                        # binary trần ~8 MB sau UPX
  veo3-manager.AppDir/                # AppImage staging
  VEO3-Manager-0.1.0-amd64.AppImage   # portable installer ~25 MB
```

## Bước 4 — Test thử AppImage

Vẫn trong WSL2 Ubuntu:

```bash
chmod +x build/bin/VEO3-Manager-0.1.0-amd64.AppImage
./build/bin/VEO3-Manager-0.1.0-amd64.AppImage
```

WSL2 trên Win 11 (WSLg) sẽ render UI ra desktop Windows luôn. WSL2 trên Win 10 không có WSLg — chỉ verify binary chạy được bằng `--help` hoặc copy file `.AppImage` sang một máy Linux thật / VM Linux.

## Lỗi hay gặp

| Lỗi | Cách xử lý |
|---|---|
| `Package libwebkit2gtk-4.0-dev has no installation candidate` (Ubuntu 24.04+) | Dùng `libwebkit2gtk-4.1-dev`, sau đó `export PKG_CONFIG_PATH=/usr/lib/x86_64-linux-gnu/pkgconfig` rồi build lại |
| `wails: command not found` sau `go install` | `source ~/.bashrc` hoặc mở terminal mới; `$HOME/go/bin` cần có trong PATH |
| Build chậm khinh khủng vì project trên `/mnt/c/` | Optional: `cp -r /mnt/c/Users/Admin/veo3_manager_part_1 ~/veo3_manager` rồi build trong `~/veo3_manager` (filesystem WSL native nhanh hơn nhiều) |

## Sau khi build xong

Copy file `.AppImage` từ WSL2 ra Windows nếu cần:

```powershell
# Trên PowerShell
Copy-Item \\wsl$\Ubuntu-22.04\mnt\c\Users\Admin\veo3_manager_part_1\build\bin\*.AppImage .
```

Hoặc đơn giản: file đã có sẵn ở `c:\Users\Admin\veo3_manager_part_1\build\bin\` vì project nằm trên ổ Windows.
