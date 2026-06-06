# Trimia

Trimia removes silence and filler words from videos. It extracts audio with ffmpeg, transcribes the audio with Deepgram, builds clean speech segments, and renders a new video that keeps only the selected segments.

## Requirements

- Go 1.25.1 or newer
- `ffmpeg` and `ffprobe` installed and available in `PATH`
- A Deepgram API key

On macOS, ffmpeg can be installed with Homebrew:

```sh
brew install ffmpeg
```

## Setup

### Install Released Binary

Install the latest release on Linux or macOS:

```sh
curl -fsSL https://raw.githubusercontent.com/romerramos/trimia/main/scripts/install.sh | sh
```

Install a specific release tag:

```sh
curl -fsSL https://raw.githubusercontent.com/romerramos/trimia/main/scripts/install.sh | VERSION=v0.1.0 sh
```

Install to a custom directory:

```sh
curl -fsSL https://raw.githubusercontent.com/romerramos/trimia/main/scripts/install.sh | INSTALL_DIR="$HOME/.local/bin" sh
```

Run the same latest-release command again to update Trimia to the newest GitHub release.

The installer downloads from GitHub Releases, verifies the archive against `checksums.txt`, and installs the `trimia` binary. It supports Linux and macOS on `amd64` and `arm64`.

### Build From Source

Install Go dependencies:

```sh
go mod download
```

Save your Deepgram API key in the OS credential store:

```sh
trimia connect
```

You can create a Deepgram account and get your first $200 in credits for free at <https://deepgram.com/>.

Trimia checks `DEEPGRAM_API_KEY` first, then checks the OS credential store. If neither exists during an interactive run, Trimia prompts for the key and saves it.

You can still provide the key through your shell environment:

```sh
export DEEPGRAM_API_KEY=your_deepgram_api_key_here
```

When building from source, you can also create a `.env` file in the project root:

```sh
DEEPGRAM_API_KEY=your_deepgram_api_key_here
```

## Build

Build the binary into `dist/trimia`:

```sh
make build
```

Or run the Go build command directly:

```sh
mkdir -p dist
go build -o dist/trimia ./cmd/trimia
```

## Run

Run with the default input path, `inputs/test.mov`:

```sh
make run
```

Run with a custom input:

```sh
make run INPUT=inputs/demo.mov
```

Run the built binary directly:

```sh
./dist/trimia --input inputs/demo.mov
```

By default, Trimia writes to `outputs/<input-name>_trimia.mp4`. You can choose an explicit output path:

```sh
./dist/trimia --input inputs/demo.mov --output outputs/demo_trimmed.mp4
```

## CLI Options

Common options:

```sh
./dist/trimia \
  --input inputs/demo.mov \
  --output outputs/demo_trimmed.mp4 \
  --language en \
  --pre-roll 0.03 \
  --post-roll 0.06 \
  --merge-gap 0.12
```

Available flags:

- `--input`: input video path. Defaults to `inputs/test.mov`.
- `--output`: output video path. Defaults to `outputs/<input-name>_trimia.mp4`.
- `--overwrite`: overwrite the output file. Defaults to `true`.
- `--language`: Deepgram language code. Leave empty to use language detection.
- `--detect-language`: ask Deepgram to detect the spoken language. Defaults to `true`.
- `--pre-roll`: seconds to keep before each speech segment. Defaults to `0.03`.
- `--post-roll`: seconds to keep after each speech segment. Defaults to `0.06`.
- `--merge-gap`: merge speech segments separated by this many seconds. Defaults to `0.12`.
- `--keep-temp-files`: keep the intermediate extracted audio file. Defaults to `false`.
- `--log-dir`: write a timestamped run log to the given directory.
- `--render-mode`: video render mode, either `segments` or `filter`. Defaults to `segments`.
- `--preset`: x264 preset for rendering. Defaults to `veryfast`.
- `--crf`: x264 CRF quality. Lower means higher quality. Defaults to `18`.
- `--audio-rate`: output audio bitrate. Defaults to `320k`.

## Logs

To write a run log:

```sh
./dist/trimia --input inputs/demo.mov --log-dir tmp
```

Trimia prints the generated log path and a `tail -f` command when logging is enabled.

## Development

Run tests:

```sh
make test
```

Or:

```sh
go test ./...
```

Remove build artifacts:

```sh
make clean
```

## Releases

Releases are built by GitHub Actions with GoReleaser when a `v*` tag is pushed. The workflow verifies that the tagged commit is reachable from `main` before publishing.

Create a release from `main`:

```sh
git checkout main
git pull origin main
git tag v0.1.0
git push origin v0.1.0
```

GoReleaser publishes archives for Linux, macOS, and Windows on `amd64` and `arm64`, plus a `checksums.txt` file used by the installer.
