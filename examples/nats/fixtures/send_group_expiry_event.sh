#!/usr/bin/env bash
set -euo pipefail

DEFAULT_WORKSPACE_ID="workspace-1"
DEFAULT_GROUP_ID="group-1"
DEFAULT_NATS_SUBJECT="app.todo.group.expiry.process"
DEFAULT_NATS_SERVER="nats://workspace-permission-management-nats:4222"
DEFAULT_DOCKER_NETWORK="workspace_permission_management"
DEFAULT_NATS_BOX_IMAGE="natsio/nats-box:0.19.3"

usage() {
  cat <<'USAGE'
Usage:
  send_group_expiry_event.sh [workspace_id] [group_id] [expiration_bucket] [task_id]

Arguments:
  workspace_id        Defaults to workspace-1
  group_id            Defaults to group-1
  expiration_bucket   Defaults to the current UTC date in yyyy-MM-dd
  task_id             Defaults to a generated UUID

Environment overrides:
  GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT  NATS subject and CloudEvent type
  GROUP_SERVICE_NATS_URL                      NATS server URL reachable from the nats-box container
  DOCKER_NETWORK                              Docker network for the nats-box container
  NATS_BOX_IMAGE                              nats-box image to run
USAGE
}

require_command() {
  local name="$1"

  if ! command -v "${name}" >/dev/null 2>&1; then
    printf 'missing required command: %s\n' "${name}" >&2
    exit 1
  fi
}

new_uuid() {
  uuidgen | tr '[:upper:]' '[:lower:]'
}

if [[ "${1:-}" == "-h" || "${1:-}" == "--help" ]]; then
  usage
  exit 0
fi

if [[ "$#" -gt 4 ]]; then
  usage >&2
  exit 1
fi

require_command docker
require_command jq
require_command uuidgen
require_command date

workspace_id="${1:-${DEFAULT_WORKSPACE_ID}}"
group_id="${2:-${DEFAULT_GROUP_ID}}"
expiration_bucket="${3:-$(date -u +"%Y-%m-%d")}"
task_id="${4:-$(new_uuid)}"
nats_subject="${GROUP_SERVICE_GROUP_EXPIRY_COMMAND_SUBJECT:-${DEFAULT_NATS_SUBJECT}}"
nats_server="${GROUP_SERVICE_NATS_URL:-${DEFAULT_NATS_SERVER}}"
docker_network="${DOCKER_NETWORK:-${DEFAULT_DOCKER_NETWORK}}"
nats_box_image="${NATS_BOX_IMAGE:-${DEFAULT_NATS_BOX_IMAGE}}"

event_id="$(new_uuid)"
event_time="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

payload="$(
  jq -n \
    --arg event_type "${nats_subject}" \
    --arg event_source "group-expiry-fixture" \
    --arg task_id "${task_id}" \
    --arg event_id "${event_id}" \
    --arg event_time "${event_time}" \
    --arg workspace_id "${workspace_id}" \
    --arg group_id "${group_id}" \
    --arg expiration_bucket "${expiration_bucket}" \
    '{
      specversion: "1.0",
      type: $event_type,
      source: $event_source,
      subject: $task_id,
      id: $event_id,
      time: $event_time,
      datacontenttype: "application/json",
      data: {
        task_id: $task_id,
        workspace_id: $workspace_id,
        group_id: $group_id,
        expiration_bucket: $expiration_bucket
      }
    }'
)"

printf '%s' "${payload}" | docker run --rm -i \
  --network "${docker_network}" \
  "${nats_box_image}" \
  nats --server "${nats_server}" pub --jetstream --force-stdin "${nats_subject}"

printf 'published group expiry event: subject=%s workspace_id=%s group_id=%s expiration_bucket=%s task_id=%s event_id=%s\n' \
  "${nats_subject}" \
  "${workspace_id}" \
  "${group_id}" \
  "${expiration_bucket}" \
  "${task_id}" \
  "${event_id}"
