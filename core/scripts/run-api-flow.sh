#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CORE_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
REPO_DIR="$(cd "$CORE_DIR/.." && pwd)"

TRIMIA_BIN="${TRIMIA_BIN:-$REPO_DIR/dist/trimia}"
INPUT_PATH="${INPUT_PATH:-$REPO_DIR/test.mov}"
DATA_DIR="${DATA_DIR:-$REPO_DIR/tmp/api-flow}"
OUTPUT_PATH="${OUTPUT_PATH:-$REPO_DIR/tmp/api-flow/test_api_trimia.mp4}"
ADDR="${ADDR:-127.0.0.1:3333}"
BASE_URL="http://$ADDR"

load_env_file() {
  local env_file=""

  if [[ -n "${ENV_FILE:-}" ]]; then
    env_file="$ENV_FILE"
  elif [[ -f "$REPO_DIR/.env" ]]; then
    env_file="$REPO_DIR/.env"
  elif [[ -f "$CORE_DIR/.env" ]]; then
    env_file="$CORE_DIR/.env"
  fi

  if [[ -z "$env_file" ]]; then
    return 0
  fi

  if [[ ! -f "$env_file" ]]; then
    echo "ENV_FILE does not exist: $env_file" >&2
    exit 1
  fi

  echo "Loading environment from $env_file"
  set -a
  # shellcheck disable=SC1090
  source "$env_file"
  set +a
}

json_get() {
  python3 -c 'import json,sys; value=json.load(sys.stdin); [value := value[p] for p in sys.argv[1].split(".") if p]; print(value)' "$1"
}

wait_for_server() {
  for _ in {1..50}; do
    if curl -fsS "$BASE_URL/api/health" >/dev/null 2>&1; then
      return 0
    fi
    sleep 0.1
  done

  echo "Trimia API did not become ready at $BASE_URL" >&2
  return 1
}

poll_job() {
  local job_id="$1"
  local response status phase

  for _ in {1..300}; do
    response="$(curl -fsS "$BASE_URL/api/jobs/$job_id")"
    status="$(printf '%s' "$response" | json_get status)"
    phase="$(printf '%s' "$response" | json_get phase)"
    printf 'analysis: status=%s phase=%s\n' "$status" "$phase"

    case "$status" in
    awaiting_confirmation) return 0 ;;
    failed)
      printf '%s\n' "$response" >&2
      return 1
      ;;
    esac

    sleep 1
  done

  echo "Timed out waiting for analysis" >&2
  return 1
}

poll_render() {
  local job_id="$1"
  local response status phase

  for _ in {1..300}; do
    response="$(curl -fsS "$BASE_URL/api/jobs/$job_id/render")"
    status="$(printf '%s' "$response" | json_get status)"
    phase="$(printf '%s' "$response" | json_get phase)"
    printf 'render: status=%s phase=%s\n' "$status" "$phase"

    case "$status" in
    completed) return 0 ;;
    failed)
      printf '%s\n' "$response" >&2
      return 1
      ;;
    esac

    sleep 1
  done

  echo "Timed out waiting for render" >&2
  return 1
}

if [[ ! -x "$TRIMIA_BIN" ]]; then
  echo "Trimia binary not found at $TRIMIA_BIN. Run: make build" >&2
  exit 1
fi

if [[ ! -f "$INPUT_PATH" ]]; then
  echo "Input video not found at $INPUT_PATH" >&2
  exit 1
fi

load_env_file

if [[ -z "${DEEPGRAM_API_KEY:-}" ]]; then
  echo "DEEPGRAM_API_KEY is not set. Add it to $REPO_DIR/.env, $CORE_DIR/.env, or export it before running." >&2
  exit 1
fi

mkdir -p "$DATA_DIR" "$(dirname "$OUTPUT_PATH")"

"$TRIMIA_BIN" serve --addr "$ADDR" --data-dir "$DATA_DIR" &
SERVER_PID="$!"
trap 'kill "$SERVER_PID" >/dev/null 2>&1 || true' EXIT

wait_for_server

echo "Uploading $INPUT_PATH"
media_response="$(curl -fsS -X POST "$BASE_URL/api/media" -F "file=@$INPUT_PATH")"
media_id="$(printf '%s' "$media_response" | json_get mediaId)"
echo "mediaId=$media_id"

echo "Starting analysis"
job_response="$(curl -fsS -X POST "$BASE_URL/api/jobs" \
  -H 'Content-Type: application/json' \
  --data "{\"mediaId\":\"$media_id\",\"options\":{\"removeSilence\":true,\"removeFillerWords\":true,\"language\":\"\",\"detectLanguage\":true,\"preRoll\":0.03,\"postRoll\":0.06,\"mergeGap\":0.12}}")"
job_id="$(printf '%s' "$job_response" | json_get jobId)"
echo "jobId=$job_id"

poll_job "$job_id"

segments_response="$(curl -fsS "$BASE_URL/api/jobs/$job_id/segments")"
segment_version="$(printf '%s' "$segments_response" | json_get version)"
echo "segmentVersion=$segment_version"

echo "Confirming proposed segments"
confirmed_payload="$(printf '%s' "$segments_response" | python3 -c 'import json,sys; data=json.load(sys.stdin); print(json.dumps({"baseVersion": data["version"], "segments": data["segments"]}))')"
curl -fsS -X PUT "$BASE_URL/api/jobs/$job_id/segments" \
  -H 'Content-Type: application/json' \
  --data "$confirmed_payload" >/dev/null

updated_segments_response="$(curl -fsS "$BASE_URL/api/jobs/$job_id/segments")"
updated_segment_version="$(printf '%s' "$updated_segments_response" | json_get version)"
echo "updatedSegmentVersion=$updated_segment_version"

echo "Starting render"
curl -fsS -X POST "$BASE_URL/api/jobs/$job_id/render" \
  -H 'Content-Type: application/json' \
  --data "{\"segmentVersion\":$updated_segment_version,\"output\":{\"filename\":\"$(basename "$OUTPUT_PATH")\",\"overwrite\":true},\"renderOptions\":{\"renderMode\":\"segments\",\"preset\":\"veryfast\",\"crf\":18,\"audioRate\":\"320k\"}}" >/dev/null

poll_render "$job_id"

echo "Downloading output to $OUTPUT_PATH"
curl -fsS "$BASE_URL/api/jobs/$job_id/download" -o "$OUTPUT_PATH"
echo "Done: $OUTPUT_PATH"
