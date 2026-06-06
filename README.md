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
curl -fsSL https://raw.githubusercontent.com/romerramos/trimia/main/core/scripts/install.sh | sh
```

By default, the installer places `trimia` in `$HOME/.local/bin` and does not ask for `sudo`. If that directory is not in your `PATH`, the installer prints the shell line to add.

Install a specific release tag:

```sh
curl -fsSL https://raw.githubusercontent.com/romerramos/trimia/main/core/scripts/install.sh | VERSION=v0.1.0 sh
```

Install to a custom directory:

```sh
curl -fsSL https://raw.githubusercontent.com/romerramos/trimia/main/core/scripts/install.sh | INSTALL_DIR="$HOME/.local/bin" sh
```

If you prefer to inspect the installer before running it:

```sh
curl -fsSL https://raw.githubusercontent.com/romerramos/trimia/main/core/scripts/install.sh -o install.sh
less install.sh
sh install.sh
```

Run the same latest-release command again to update Trimia to the newest GitHub release.

The installer downloads from GitHub Releases, verifies the archive against `checksums.txt`, and installs the `trimia` binary. It supports Linux and macOS on `amd64` and `arm64`. It only writes to the selected install directory.

### Build From Source

Install Go dependencies:

```sh
cd core
go mod download
```

Trimia uses Deepgram for transcription. Create a Deepgram account to get an API key, then save it:

```sh
trimia connect
```

You can create a Deepgram account and get your first $200 in credits for free at <https://deepgram.com/>.

Trimia uses the OS secure credential store when available. On Linux this is Secret Service, on macOS this is Keychain, and on Windows this is Credential Manager. If the OS secure store is unavailable, Trimia falls back to a plaintext JSON config file at `~/.trimia/config.json` with directory permissions `0700` and file permissions `0600`. This fallback is useful in WSL, where Secret Service is often not running by default, but it is not encrypted.

Show where Trimia is reading credentials from without printing the key:

```sh
trimia config
```

Remove saved credentials from both the OS secure store and the fallback config file:

```sh
trimia disconnect
```

Trimia checks saved credentials during normal interactive use. If no key is saved yet, Trimia prompts for it and saves it using the OS secure store first, then the fallback config file only if the secure store is unavailable.

For scripts or CI, you can also provide the key through the `DEEPGRAM_API_KEY` environment variable:

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
cd core
go build -o ../dist/trimia ./cmd/trimia
```

## Run

Run with an input video path:

```sh
./dist/trimia demo.mov
```

Or use the explicit flag:

```sh
./dist/trimia --input demo.mov
```

When using `make run`, pass the input path:

```sh
make run INPUT=demo.mov
```

By default, Trimia writes next to the input as `<input-name>_trimia.mp4`. You can choose an explicit output path:

```sh
./dist/trimia demo.mov --output demo_trimmed.mp4
```

## HTTP API

Run the local API server for the future SvelteKit app:

```sh
cd core
go run ./cmd/trimia serve \
  --addr 127.0.0.1:3333 \
  --data-dir ../tmp/api \
  --upload-token-secret "$TRIMIA_UPLOAD_TOKEN_SECRET" \
  --allowed-origin http://localhost:5173
```

The API keeps uploaded videos and rendered outputs on the Go server. Clients should use IDs and URLs instead of filesystem paths.

For browser uploads from the SvelteKit app, set the same `TRIMIA_UPLOAD_TOKEN_SECRET` in `web/.env` and `core/.env`. The protected `/upload` page signs a short-lived JWT after checking Better Auth, then the browser uploads directly to Trimia. This keeps large files out of the SvelteKit request pipeline and allows multi-GB uploads. Uploaded files are stored under `<data-dir>/uploads`. The API defaults to a 5 GiB upload ceiling; override it with `--max-upload-bytes` or `TRIMIA_MAX_UPLOAD_BYTES`.

When started from `core`, the API loads `core/.env`, prints its non-secret upload configuration, and logs each request with status and duration. Upload token failures are logged with a specific reason, such as `invalid jwt signature` or `expired token`. Logs default to compact `human` output; set `TRIMIA_LOG_FORMAT=json` or pass `--log-format json` for structured logs.

Minimal flow:

1. Upload media with `POST /api/media` using multipart form field `file`.
2. Start analysis with `POST /api/jobs`.
3. Poll `GET /api/jobs/{jobId}` until `status` is `awaiting_confirmation`.
4. Read proposed segments with `GET /api/jobs/{jobId}/segments`.
5. Save edited segments with `PUT /api/jobs/{jobId}/segments`.
6. Start rendering with `POST /api/jobs/{jobId}/render`.
7. Poll `GET /api/jobs/{jobId}/render` until `status` is `completed`.
8. Download the final video from `GET /api/jobs/{jobId}/download`.

Create an analysis job:

```json
{
  "mediaId": "med_abc123",
  "options": {
    "removeSilence": true,
    "removeFillerWords": true,
    "language": "",
    "detectLanguage": true,
    "preRoll": 0.03,
    "postRoll": 0.06,
    "mergeGap": 0.12
  }
}
```

Save reviewed segments:

```json
{
  "baseVersion": 1,
  "segments": [
    {
      "id": "seg_001",
      "start": 0.35,
      "end": 8.95,
      "text": "Today we're going to look at the first prototype.",
      "source": "deepgram",
      "included": true
    }
  ]
}
```

Start a render:

```json
{
  "segmentVersion": 2,
  "output": {
    "filename": "demo_trimia.mp4",
    "overwrite": true
  },
  "renderOptions": {
    "renderMode": "segments",
    "preset": "veryfast",
    "crf": 18,
    "audioRate": "320k"
  }
}
```

## CLI Options

Common options:

```sh
./dist/trimia \
  demo.mov \
  --output demo_trimmed.mp4 \
  --language en \
  --pre-roll 0.03 \
  --post-roll 0.06 \
  --merge-gap 0.12
```

Available flags:

- Positional input: input video path, for example `trimia demo.mov`.
- `--input`: input video path, as an alternative to the positional argument.
- `--output`: output video path. Defaults to `<input-name>_trimia.mp4` next to the input.
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
./dist/trimia demo.mov --log-dir tmp
```

Trimia prints the generated log path and a `tail -f` command when logging is enabled.

## Development

Run tests:

```sh
make test
```

Or:

```sh
cd core
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
