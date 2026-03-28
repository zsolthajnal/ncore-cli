# ncore-cli

A command-line tool for searching and downloading torrent files from [ncore.pro](https://ncore.pro), with an optional series watcher that automatically finds and downloads new episodes.

## Installation

<details>
<summary><strong>macOS — Homebrew (recommended)</strong></summary>

```sh
brew install zsolthajnal/tap/ncore-cli
```

To upgrade later:

```sh
brew upgrade ncore-cli
```

</details>

<details>
<summary><strong>Linux — .deb (Debian / Ubuntu)</strong></summary>

```sh
curl -LO https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_linux_amd64.deb
sudo dpkg -i ncore-cli_linux_amd64.deb
```

ARM64:

```sh
curl -LO https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_linux_arm64.deb
sudo dpkg -i ncore-cli_linux_arm64.deb
```

</details>

<details>
<summary><strong>Linux — .rpm (Fedora / RHEL)</strong></summary>

```sh
curl -LO https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_linux_amd64.rpm
sudo rpm -i ncore-cli_linux_amd64.rpm
```

</details>

<details>
<summary><strong>Linux — binary</strong></summary>

```sh
curl -L https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_linux_amd64.tar.gz | tar -xz
sudo mv ncore-cli /usr/local/bin/
```

</details>

<details>
<summary><strong>macOS — binary</strong></summary>

Apple Silicon:

```sh
curl -L https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_darwin_arm64.tar.gz | tar -xz
sudo mv ncore-cli /usr/local/bin/
```

Intel:

```sh
curl -L https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_darwin_amd64.tar.gz | tar -xz
sudo mv ncore-cli /usr/local/bin/
```

</details>

<details>
<summary><strong>Windows (x86_64)</strong></summary>

```powershell
curl -L https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_windows_amd64.zip -o ncore-cli.zip
Expand-Archive ncore-cli.zip
```

Place `ncore-cli.exe` somewhere on your `PATH`.

</details>

<details>
<summary><strong>Build from source</strong></summary>

Requires [Go 1.22+](https://go.dev/dl/).

```sh
git clone https://github.com/zsolthajnal/ncore-cli.git
cd ncore-cli
go build -o ncore-cli .
```

</details>

## Configuration

Set your ncore.pro credentials as environment variables:

```sh
export NCORE_USER=your_username
export NCORE_PASS=your_password
```

Using [direnv](https://direnv.net/)? Add them to a local `.envrc`:

```sh
export NCORE_USER=your_username
export NCORE_PASS=your_password
```

> **Note:** Never commit your credentials to version control.

## Usage

### Search

```sh
ncore-cli search <query> [category]
# short: ncore-cli s <query> [category]
```

Category defaults to `all_own` (all categories). Examples:

```sh
ncore-cli search "inception"
ncore-cli search "inception" hd_hun
ncore-cli search "the matrix" hd
```

Output:

```
Found 25 result(s) for "inception":

ID        NAME                                                     SIZE        SEEDS  LEECHES  TYPE
────────  ───────────────────────────────────────────────────────  ──────────  ─────  ───────  ────────────
3995642   Inception.2010.1080p.UHD.BluRay.DDP.5.1.DoVi-HDR...    18.53 GiB   78     0        HD/HU
3969041   Inception.2010.READ.NFO.2160p.UHD.BluRay.REMUX.DTS...   66.97 GiB   56     10       HD/HU
```

### Download

```sh
ncore-cli download <id> [output_dir]
# short: ncore-cli d <id> [output_dir]
```

`id` is the torrent ID shown in search results. Output directory defaults to the current working directory.

```sh
ncore-cli download 3995642
ncore-cli download 3995642 ~/Downloads
```

### Scan

List all series and their known episodes found in a media directory:

```sh
ncore-cli scan --media-dir /path/to/series
# short: ncore-cli ls --media-dir /path/to/series
```

### Watch

Automatically check for new episodes of every series found in a media directory and download them via [webtorrent-cli](https://github.com/webtorrent/webtorrent-cli):

```sh
ncore-cli watch \
  --media-dir /srv/data1/plexmediaserver/series \
  --torrent-dir /tmp/torrents \
  --interval 1h
# short: ncore-cli w ...
```

<details>
<summary><strong>Watch flags</strong></summary>

| Flag | Default | Description |
|------|---------|-------------|
| `--media-dir` | *(required)* | Directory containing series folders |
| `--torrent-dir` | `<media-dir>/torrent` | Where to store `.torrent` files temporarily |
| `--state` | `/var/lib/ncore-cli/state.json` | Path to episode state file |
| `--interval` | `1h` | How often to check for new episodes |
| `--download-cmd` | `webtorrent download {torrent} --out {out_dir}` | Command template to run after download |
| `--require` | | Comma-separated terms that must appear in the torrent name (e.g. `1080p`) |
| `--exclude` | | Comma-separated terms to reject (e.g. `2160p,4K,UHD`) |
| `--once` | `false` | Run once and exit instead of looping |

</details>

Quality filtering example — only 1080p, no 4K:

```sh
ncore-cli watch \
  --media-dir /srv/data1/plexmediaserver/series \
  --require 1080p \
  --exclude 2160p,4K,UHD
```

### Install as a systemd service

```sh
sudo ncore-cli install \
  --media-dir /srv/data1/plexmediaserver/series \
  --require 1080p \
  --exclude 2160p,4K,UHD
# short: sudo ncore-cli i ...
```

Edit credentials after install:

```sh
sudo nano /etc/ncore-cli/env
sudo systemctl restart ncore-cli
```

Useful commands:

```sh
systemctl status ncore-cli
journalctl -fu ncore-cli
```

### Uninstall systemd service

```sh
sudo ncore-cli uninstall
```

Credentials (`/etc/ncore-cli/env`) and state (`/var/lib/ncore-cli/state.json`) are kept — remove manually if no longer needed.

## Shell completion

<details>
<summary><strong>bash</strong></summary>

```sh
# Load for current session
source <(ncore-cli completion bash)

# Install permanently
ncore-cli completion bash | sudo tee /etc/bash_completion.d/ncore-cli
```

</details>

<details>
<summary><strong>zsh</strong></summary>

```sh
# Load for current session
source <(ncore-cli completion zsh)

# Install permanently
ncore-cli completion zsh > "${fpath[1]}/_ncore-cli"
```

</details>

<details>
<summary><strong>fish</strong></summary>

```sh
ncore-cli completion fish > ~/.config/fish/completions/ncore-cli.fish
```

</details>

## Categories

<details>
<summary>All categories</summary>

| Category | Description |
|----------|-------------|
| `all_own` | All categories (default) |
| `hd_hun` | Film HD / Hungarian |
| `hd` | Film HD / English |
| `xvid_hun` | Film SD / Hungarian |
| `xvid` | Film SD / English |
| `dvd_hun` | Film DVDR / Hungarian |
| `dvd` | Film DVDR / English |
| `dvd9_hun` | Film DVD9 / Hungarian |
| `dvd9` | Film DVD9 / English |
| `hdser_hun` | Series HD / Hungarian |
| `hdser` | Series HD / English |
| `xvidser_hun` | Series SD / Hungarian |
| `xvidser` | Series SD / English |
| `mp3_hun` | MP3 / Hungarian |
| `mp3` | MP3 / English |
| `lossless_hun` | Lossless / Hungarian |
| `lossless` | Lossless / English |
| `ebook_hun` | eBook / Hungarian |
| `ebook` | eBook / English |
| `game_iso` | Game PC / ISO |
| `game_rip` | Game PC / RIP |
| `console` | Console games |
| `xxx_xvid` | XXX SD |
| `xxx_dvd` | XXX DVDR |
| `xxx_hd` | XXX HD |
| `misc` | Apps / RIP |
| `mobil` | Apps / Mobile |

</details>

## License

MIT
