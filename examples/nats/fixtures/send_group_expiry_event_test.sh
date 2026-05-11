#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT_UNDER_TEST="${SCRIPT_DIR}/send_group_expiry_event.sh"

TMP_DIR="$(mktemp -d)"
cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

cat > "${TMP_DIR}/docker" <<'FAKE_DOCKER'
#!/usr/bin/env bash
set -euo pipefail

printf '%s\n' "$*" > "${DOCKER_ARGS_FILE}"
cat > "${DOCKER_STDIN_FILE}"
FAKE_DOCKER
chmod +x "${TMP_DIR}/docker"

cat > "${TMP_DIR}/uuidgen" <<'FAKE_UUIDGEN'
#!/usr/bin/env bash
set -euo pipefail

count=0
if [[ -f "${UUID_COUNTER_FILE}" ]]; then
  count="$(cat "${UUID_COUNTER_FILE}")"
fi
count=$((count + 1))
printf '%s\n' "${count}" > "${UUID_COUNTER_FILE}"

case "${count}" in
  1) printf '11111111-1111-4111-8111-111111111111\n' ;;
  2) printf '22222222-2222-4222-8222-222222222222\n' ;;
  3) printf '33333333-3333-4333-8333-333333333333\n' ;;
  4) printf '44444444-4444-4444-8444-444444444444\n' ;;
  5) printf '55555555-5555-4555-8555-555555555555\n' ;;
  6) printf '66666666-6666-4666-8666-666666666666\n' ;;
  *) printf '77777777-7777-4777-8777-777777777777\n' ;;
esac
FAKE_UUIDGEN
chmod +x "${TMP_DIR}/uuidgen"

cat > "${TMP_DIR}/date" <<'FAKE_DATE'
#!/usr/bin/env bash
set -euo pipefail

case "$*" in
  "-u +%Y-%m-%dT%H:%M:%SZ") printf '2026-05-10T00:00:00Z\n' ;;
  "-u +%Y-%m-%d") printf '2026-05-10\n' ;;
  *) /bin/date "$@" ;;
esac
FAKE_DATE
chmod +x "${TMP_DIR}/date"

export PATH="${TMP_DIR}:${PATH}"
export UUID_COUNTER_FILE="${TMP_DIR}/uuid.counter"

run_script() {
  local name="$1"
  shift

  DOCKER_ARGS_FILE="${TMP_DIR}/${name}.args" \
    DOCKER_STDIN_FILE="${TMP_DIR}/${name}.stdin" \
    "${SCRIPT_UNDER_TEST}" "$@" > "${TMP_DIR}/${name}.stdout"
}

assert_docker_args() {
  local args_file="$1"
  local expected_args="run --rm -i --network workspace_permission_management natsio/nats-box:0.19.3 nats --server nats://workspace-permission-management-nats:4222 pub --jetstream --force-stdin app.todo.group.expiry.process"
  local actual_args

  actual_args="$(cat "${args_file}")"
  if [[ "${actual_args}" != "${expected_args}" ]]; then
    printf 'docker args mismatch\nwant: %s\ngot:  %s\n' "${expected_args}" "${actual_args}" >&2
    exit 1
  fi
}

run_script custom workspace-9 group-9 2026-06-01 task-9

assert_docker_args "${TMP_DIR}/custom.args"

jq -e '
  .specversion == "1.0" and
  .type == "app.todo.group.expiry.process" and
  .source == "group-expiry-fixture" and
  .subject == "task-9" and
  .id == "11111111-1111-4111-8111-111111111111" and
  .time == "2026-05-10T00:00:00Z" and
  .datacontenttype == "application/json" and
  .data.task_id == "task-9" and
  .data.workspace_id == "workspace-9" and
  .data.group_id == "group-9" and
  .data.expiration_bucket == "2026-06-01"
' "${TMP_DIR}/custom.stdin" >/dev/null

if ! grep -q "task_id=task-9" "${TMP_DIR}/custom.stdout"; then
  printf 'stdout does not include task_id\n' >&2
  exit 1
fi

run_script default_first
run_script default_second

assert_docker_args "${TMP_DIR}/default_first.args"
assert_docker_args "${TMP_DIR}/default_second.args"

uuid_pattern='^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[1-5][0-9a-fA-F]{3}-[89abAB][0-9a-fA-F]{3}-[0-9a-fA-F]{12}$'

jq -e --arg uuid_pattern "${uuid_pattern}" '
  (.id | test($uuid_pattern)) and
  (.subject | test($uuid_pattern)) and
  .subject == .data.task_id and
  .data.workspace_id == "workspace-1" and
  .data.group_id == "group-1" and
  .data.expiration_bucket == "2026-05-10"
' "${TMP_DIR}/default_first.stdin" >/dev/null

jq -e --arg uuid_pattern "${uuid_pattern}" '
  (.id | test($uuid_pattern)) and
  (.subject | test($uuid_pattern)) and
  .subject == .data.task_id and
  .data.workspace_id == "workspace-1" and
  .data.group_id == "group-1" and
  .data.expiration_bucket == "2026-05-10"
' "${TMP_DIR}/default_second.stdin" >/dev/null

first_task_id="$(jq -r '.data.task_id' "${TMP_DIR}/default_first.stdin")"
second_task_id="$(jq -r '.data.task_id' "${TMP_DIR}/default_second.stdin")"
if [[ "${first_task_id}" == "${second_task_id}" ]]; then
  printf 'task IDs should differ between runs\n' >&2
  exit 1
fi
