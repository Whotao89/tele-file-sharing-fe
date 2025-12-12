# Environment Configuration

Dự án này sử dụng [SOPS](https://github.com/getsops/sops) để mã hóa các biến môi trường nhạy cảm.

## Cấu trúc thư mục

-   `example.env` - File mẫu cấu hình (không chứa giá trị thực)
-   `dev.enc` - File cấu hình dev đã mã hóa 
-   `keys.txt` - File chứa age private key để decrypt 

## Cài đặt

### 1. Cài đặt SOPS

```bash
# Windows (sử dụng chocolatey)
choco install sops

# Hoặc download từ: https://github.com/getsops/sops/releases
```

### 2. Cài đặt age

```bash
# Windows (sử dụng chocolatey)
choco install age

# Hoặc download từ: https://github.com/FiloSottile/age/releases
```

## Sử dụng

### Tạo file dev.env cho lần đầu tiên

Khi clone project về lần đầu, bạn cần giải mã file `dev.enc` để tạo file `dev.env`:

```powershell
# 1. Nhận file keys.txt từ team leader và lưu vào env/keys.txt

# 2. Giải mã và tạo file dev.env tự động (UTF-8 không BOM)
$env:SOPS_AGE_KEY_FILE="env/keys.txt"
$yaml = sops -d --input-type yaml --output-type yaml env/dev.enc
$lines = $yaml -split "`n"
$result = ""
foreach($line in $lines) {
    $line = $line.Trim()
    if($line -match '^([A-Z_]+):\s*(.+)$') {
        $result += "$($matches[1])=$($matches[2] -replace '^\"|\"$','')`n"
    } elseif($line -match '^#') {
        $result += "$line`n"
    }
}
$utf8NoBOM = New-Object System.Text.UTF8Encoding $false
[System.IO.File]::WriteAllText("$PWD\env\dev.env", $result.TrimEnd("`n") + "`n", $utf8NoBOM)
```

Sau khi có file `dev.env`, bạn có thể chạy project bình thường.

### Giải mã file để sử dụng local

```powershell
# Set environment variable để SOPS biết dùng key nào
$env:SOPS_AGE_KEY_FILE="env/keys.txt"

# Giải mã file dev để xem
sops -d --input-type yaml --output-type yaml env/dev.enc

# Hoặc edit trực tiếp (cần có editor như notepad)
$env:EDITOR="notepad"
sops --input-type yaml --output-type yaml env/dev.enc
```

### Mã hóa file sau khi chỉnh sửa

```powershell
# Set environment variable
$env:SOPS_AGE_KEY_FILE="env/keys.txt"
$env:EDITOR="notepad"

# Chỉnh sửa file đã mã hóa (recommended)
sops --input-type yaml --output-type yaml env/dev.enc

# Commit file đã mã hóa
git add env/dev.enc
git commit -m "Update dev environment config"
```

### Chỉnh sửa trực tiếp file đã mã hóa

```powershell
# Set environment variable
$env:SOPS_AGE_KEY_FILE="env/keys.txt"
$env:EDITOR="notepad"

# SOPS sẽ mở file, tự động giải mã, cho bạn chỉnh sửa, và mã hóa lại khi lưu
sops --input-type yaml --output-type yaml env/dev.enc
```

## Lưu ý quan trọng

⚠️ **KHÔNG BAO GIỜ commit các file sau:**

-   `*.env` (file cấu hình gốc chưa mã hóa)
-   `keys.txt` (private key)
-   Bất kỳ file nào chứa secrets/tokens thực

✅ **CHỈ commit:**

-   `*.enc` (file đã mã hóa)
-   `example.env` (file mẫu không chứa giá trị thực)

## Chia sẻ với team

Để member khác trong team có thể decrypt files:

1. Chia sẻ nội dung file `env/keys.txt` qua kênh an toàn (Slack DM, 1Password, etc.)
2. Member mới tạo file `env/keys.txt` và paste private key vào
3. Member có thể xem: `sops -d --input-type yaml --output-type yaml env/dev.enc`

## Age Public Key

Public key được dùng trong project này (có thể public):

```
age1k0w3mqjfk7jy5wvvz04ec39hsr9fg5evvpm4j6my5lr0eslnasvq6mekf4
```

Private key được lưu trong `env/keys.txt` (KHÔNG được public).
