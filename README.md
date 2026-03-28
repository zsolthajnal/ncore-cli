# ncore-cli

A command-line tool for searching and downloading torrent files from [ncore.pro](https://ncore.pro).

## Installation

<details>
<summary><strong>macOS (Apple Silicon)</strong></summary>

```sh
curl -L https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_darwin_arm64 -o /usr/local/bin/ncore-cli
chmod +x /usr/local/bin/ncore-cli
```

</details>

<details>
<summary><strong>macOS (Intel)</strong></summary>

```sh
curl -L https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_darwin_amd64 -o /usr/local/bin/ncore-cli
chmod +x /usr/local/bin/ncore-cli
```

</details>

<details>
<summary><strong>Linux (x86_64)</strong></summary>

```sh
curl -L https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_linux_amd64 -o /usr/local/bin/ncore-cli
chmod +x /usr/local/bin/ncore-cli
```

</details>

<details>
<summary><strong>Windows (x86_64)</strong></summary>

```powershell
curl -L https://github.com/zsolthajnal/ncore-cli/releases/latest/download/ncore-cli_windows_amd64.exe -o ncore-cli.exe
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
```

Category defaults to `all_own` (all categories). Examples:

```sh
# Search across all categories
ncore-cli search "inception"

# Search only in Hungarian HD films
ncore-cli search "inception" hd_hun

# Search in English HD films
ncore-cli search "the matrix" hd
```

Output:

```
Found 25 result(s) for "inception":

ID        NAME                                                     SIZE        SEEDS  LEECHES  TYPE
────────  ───────────────────────────────────────────────────────  ──────────  ─────  ───────  ────────────
3995642   Inception.2010.1080p.UHD.BluRay.DDP.5.1.DoVi-HDR...    18.53 GiB   78     0        HD/HU
3969041   Inception.2010.READ.NFO.2160p.UHD.BluRay.REMUX.DTS...   66.97 GiB   56     10       HD/HU
...
```

### Download

```sh
ncore-cli download <id> [output_dir]
```

`id` is the torrent ID shown in search results. Output directory defaults to the current working directory.

```sh
# Download to current directory
ncore-cli download 3995642

# Download to a specific folder
ncore-cli download 3995642 ~/Downloads
```

### Categories

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
