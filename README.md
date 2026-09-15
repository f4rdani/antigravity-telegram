# agy-tele: Antigravity CLI Remote Bridge via Telegram

> Remote control Antigravity CLI (`agy`) dari Telegram dengan performa ultra-ringan (Go binary ~6.5MB, Idle RAM ≤ 15MB), streaming respon *real-time*, dan dukungan 100% semua fitur slash command (`/help`, `/plan`, `/usage`, `/model`, `/skills`, dll.).

---

## 🌟 Fitur Utama

- **Dual-Engine Dispatcher**:
  - **Mode A (Fast CLI Inspector)**: Perintah bawaan CLI (`/usage`, `/credits`, `/skills`, `/model`, `/effort`, `/changelog`) dieksekusi instan lewat print mode (<100ms).
  - **Mode B (Stream-JSON Agent Engine)**: Perintah coding, percakapan, dan agent modes (`/plan`, `/goal`, skills) dieksekusi streaming via protokol NDJSON `stream-json` dengan retensi konteks `--conversation <id>`.
- **Throttled Live Streaming**: Teks dialirkan secara bertahap (buffer 1.2 detik) sehingga pengguna melihat respon secara *live* tanpa terkena rate-limit API Telegram (*HTTP 429 Too Many Requests*).
- **User-Configurable Tool Permissions**:
  - `auto`: Menyetujui semua tools/perintah secara otomatis (`--dangerously-skip-permissions`) untuk kenyamanan *hands-free* di smartphone.
  - `ask`: Mengirim tombol konfirmasi interaktif di Telegram sebelum aksi dijalankan.
- **Dynamic Workspace Navigation**:
  - Default workspace di direktori **Root** (`/` di Linux atau `C:\` di Windows).
  - Berpindah direktori kerja kapan saja langsung dari Telegram via `/cwd <path>`, serta melihat isi file via `/ls` dan `/pwd`.
- **Single-Tenant Security**:
  - Whitelist ketat `allowed_user_ids`. Pesan dari User ID yang tidak terdaftar akan otomatis ditolak.
- **Zero-Bloat Single Binary**:
  - Tidak membutuhkan Node.js, Python, atau runtime tambahan. Cukup satu file biner native Go.
  - Memory footprint: **Idle ≤ 15 MB**, **Active ≤ 35 MB**, **CPU idle ≈ 0%**.

---

## 📋 Daftar Perintah Telegram (Cheatsheet)

### 🤖 Agent & Coding (Mode B)
- **Kirim teks biasa**: Menjalankan prompt percakapan / tugas coding interaktif.
- `/plan <task>`: Menjalankan mode perencanaan mendalam (`--mode plan`).
- `/goal <task>`: Menjalankan task autonomous jangka panjang.
- `/continue`: Melanjutkan sesi obrolan terakhir (`--continue`).
- `/cancel` atau `/stop`: Menghentikan proses `agy` yang sedang berjalan di server.

### 📊 Status & Kuota CLI (Mode A)
- `/usage` atau `/quota`: Menampilkan tabel limit kuota 5 jam & mingguan (Gemini & Claude/GPT).
- `/credits`: Menampilkan sisa kredit G1.
- `/model [nama]`: Melihat atau memilih model aktif lewat tombol interaktif.
- `/effort [low|medium|high]`: Mengatur reasoning effort.
- `/skills`: Menampilkan daftar semua skills Antigravity yang terpasang.
- `/agents`: Menampilkan daftar subagents kustom.
- `/changelog`: Menampilkan catatan rilis terbaru.

### 📂 Workspace, Sesi & Pengaturan
- `/cwd [path]`: Menampilkan atau mengubah direktori kerja aktif.
- `/pwd`: Menampilkan direktori kerja saat ini.
- `/ls [path]`: Menampilkan daftar file dan folder di server.
- `/new [path]`: Mereset sesi percakapan (memulai percakapan baru).
- `/sessions`: Melihat riwayat sesi percakapan sebelumnya.
- `/switch <id>`: Berpindah ke ID percakapan tertentu.
- `/permission [auto|ask]`: Mengatur mode persetujuan tools.
- `/status`: Menampilkan status runtime bot, workspace, model, dan sesi aktif.
- `/file <rel_path>`: Mengunduh file dari server langsung ke chat Telegram.

---

## ⚙️ Konfigurasi (`config.json`)

Salin file `config.example.json` menjadi `config.json`:

```json
{
  "telegram": {
    "bot_token": "123456789:ABCdefGhIJKlmNoPQRsTUVwxyZ",
    "allowed_user_ids": [
      123456789
    ],
    "stream_edit_interval_ms": 1200
  },
  "agy": {
    "binary_path": "agy",
    "default_workspace": "auto",
    "default_model": "",
    "default_effort": "",
    "permission_mode": "auto"
  },
  "storage": {
    "session_file": "./sessions.json"
  }
}
```

> **Catatan**:
> - Dapatkan `bot_token` dari [@BotFather](https://t.me/botfather).
> - Dapatkan Telegram ID Anda melalui bot seperti [@userinfobot](https://t.me/userinfobot) dan masukkan ke dalam array `allowed_user_ids`.
> - Jika `default_workspace` diisi `"auto"`, bot otomatis menggunakan root `/` di Linux dan `C:\` di Windows.

---

## 🧪 Menjalankan untuk Pengujian di Windows

Biner Windows sudah terkompilasi di `bin/agy-tele.exe`.

1. Buat file `config.json` dan isi `bot_token` serta `allowed_user_ids`.
2. Jalankan di PowerShell atau Command Prompt:
   ```powershell
   .\bin\agy-tele.exe -config config.json
   ```
3. Buka bot Anda di Telegram dan kirim `/help` atau `/status`!

---

## 🚀 Deploy ke Linux Server (Produksi)

Biner Linux 64-bit sudah dikompilasi di: **`bin/agy-tele-linux-amd64`** (hanya ~6.5MB).

### Langkah 1: Upload ke Server Linux
Upload biner dan file konfigurasi ke server:
```bash
# Contoh menggunakan scp
scp bin/agy-tele-linux-amd64 user@your-server-ip:/opt/agy-tele/agy-tele
scp config.json user@your-server-ip:/opt/agy-tele/config.json
scp service/agy-tele.service user@your-server-ip:/etc/systemd/system/agy-tele.service
```

### Langkah 2: Berikan Permission Eksekusi
Login ke server Linux via SSH:
```bash
sudo chmod +x /opt/agy-tele/agy-tele
```

### Langkah 3: Aktifkan Service Latar Belakang (Systemd)
```bash
sudo systemctl daemon-reload
sudo systemctl enable agy-tele
sudo systemctl start agy-tele
```

### Langkah 4: Cek Status Service
```bash
sudo systemctl status agy-tele
# Melihat log realtime:
journalctl -u agy-tele -f
```

---

## 🔨 Build Ulang dari Source Code

Jika Anda melakukan modifikasi kode dan ingin mengompilasi ulang:

```bash
# Build untuk Windows:
go build -ldflags="-s -w" -o bin/agy-tele.exe ./cmd/agy-tele

# Cross-compile untuk Linux Server (AMD64):
# Di CMD / Bash:
set GOOS=linux&& set GOARCH=amd64&& set CGO_ENABLED=0&& go build -ldflags="-s -w" -o bin/agy-tele-linux-amd64 ./cmd/agy-tele
```
