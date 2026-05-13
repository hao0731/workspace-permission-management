#!/usr/bin/env bash
set -euo pipefail

DEFAULT_WORKSPACE_ID="workspace-1"
DEFAULT_FUNCTION_KEY="todo"
DEFAULT_NATS_SERVER="nats://workspace-permission-management-nats:4222"
DEFAULT_DOCKER_NETWORK="workspace_permission_management"
DEFAULT_NATS_BOX_IMAGE="natsio/nats-box:0.19.3"

usage() {
  cat <<'USAGE'
Usage:
  create_function_resource.sh [workspace_id] [function_key]

Environment overrides:
  FUNCTION_SERVICE_NATS_URL           NATS server URL reachable from the nats-box container
  DOCKER_NETWORK                      Docker network for the nats-box container
  NATS_BOX_IMAGE                      nats-box image to run
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

if [[ "$#" -gt 2 ]]; then
  usage >&2
  exit 1
fi

require_command docker
require_command jq
require_command uuidgen
require_command date

workspace_id="${1:-${DEFAULT_WORKSPACE_ID}}"
function_key="${2:-${DEFAULT_FUNCTION_KEY}}"
nats_subject="app.${function_key}.resource.upserted"
nats_server="${FUNCTION_SERVICE_NATS_URL:-${DEFAULT_NATS_SERVER}}"
docker_network="${DOCKER_NETWORK:-${DEFAULT_DOCKER_NETWORK}}"
nats_box_image="${NATS_BOX_IMAGE:-${DEFAULT_NATS_BOX_IMAGE}}"

resource_id="$(new_uuid)"
event_id="$(new_uuid)"
event_time="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

payload="$(
  jq -n \
    --arg event_type "${nats_subject}" \
    --arg event_source "function-resource-fixture" \
    --arg resource_id "${resource_id}" \
    --arg event_id "${event_id}" \
    --arg event_time "${event_time}" \
    --arg workspace_id "${workspace_id}" \
    --arg function_key "${function_key}" \
    '{
      specversion: "1.0",
      type: $event_type,
      source: $event_source,
      subject: $resource_id,
      id: $event_id,
      time: $event_time,
      datacontenttype: "application/json",
      data: {
        resource_id: $resource_id,
        display_name: "Fixture Function Resource",
        resource_type: "document",
        resource_tags: ["section_1"],
        function_key: $function_key,
        workspace_id: $workspace_id
      }
    }'
)"

printf '%s' "${payload}" | docker run --rm -i \
  --network "${docker_network}" \
  "${nats_box_image}" \
  nats --server "${nats_server}" pub --jetstream --force-stdin "${nats_subject}"

printf 'published resource upsert event: subject=%s workspace_id=%s function_key=%s resource_id=%s event_id=%s\n' \
  "${nats_subject}" \
  "${workspace_id}" \
  "${function_key}" \
  "${resource_id}" \
  "${event_id}"
