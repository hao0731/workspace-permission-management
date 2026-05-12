#!/usr/bin/env bash
set -euo pipefail

DEFAULT_GROUP_ID="group-1"
DEFAULT_NT_ACCOUNT="user1"
DEFAULT_NATS_SUBJECT="app.todo.group.individual-member.expiry.process"
DEFAULT_NATS_SERVER="nats://workspace-permission-management-nats:4222"
DEFAULT_DOCKER_NETWORK="workspace_permission_management"
DEFAULT_NATS_BOX_IMAGE="natsio/nats-box:0.19.3"

usage() {
  cat <<'USAGE'
Usage:
  send_individual_member_expiry_event.sh [group_id] [nt_account] [expiration_bucket] [task_id]

Arguments:
  group_id            Defaults to group-1
  nt_account          Defaults to user1
  expiration_bucket   Defaults to the current UTC date in yyyy-MM-dd
  task_id             Defaults to a generated UUID

Environment overrides:
  GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT  NATS subject and CloudEvent type
  GROUP_SERVICE_NATS_URL                                  NATS server URL reachable from the nats-box container
  DOCKER_NETWORK                                          Docker network for the nats-box container
  NATS_BOX_IMAGE                                          nats-box image to run
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

group_id="${1:-${DEFAULT_GROUP_ID}}"
nt_account="${2:-${DEFAULT_NT_ACCOUNT}}"
expiration_bucket="${3:-$(date -u +"%Y-%m-%d")}"
task_id="${4:-$(new_uuid)}"
nats_subject="${GROUP_SERVICE_INDIVIDUAL_MEMBER_EXPIRY_COMMAND_SUBJECT:-${DEFAULT_NATS_SUBJECT}}"
nats_server="${GROUP_SERVICE_NATS_URL:-${DEFAULT_NATS_SERVER}}"
docker_network="${DOCKER_NETWORK:-${DEFAULT_DOCKER_NETWORK}}"
nats_box_image="${NATS_BOX_IMAGE:-${DEFAULT_NATS_BOX_IMAGE}}"

event_id="$(new_uuid)"
event_time="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

payload="$(
  jq -n \
    --arg event_type "${nats_subject}" \
    --arg event_source "individual-member-expiry-fixture" \
    --arg task_id "${task_id}" \
    --arg event_id "${event_id}" \
    --arg event_time "${event_time}" \
    --arg group_id "${group_id}" \
    --arg nt_account "${nt_account}" \
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
        group_id: $group_id,
        nt_account: $nt_account,
        expiration_bucket: $expiration_bucket
      }
    }'
)"

printf '%s' "${payload}" | docker run --rm -i \
  --network "${docker_network}" \
  "${nats_box_image}" \
  nats --server "${nats_server}" pub --jetstream --force-stdin "${nats_subject}"

printf 'published individual member expiry event: subject=%s group_id=%s nt_account=%s expiration_bucket=%s task_id=%s event_id=%s\n' \
  "${nats_subject}" \
  "${group_id}" \
  "${nt_account}" \
  "${expiration_bucket}" \
  "${task_id}" \
  "${event_id}"
