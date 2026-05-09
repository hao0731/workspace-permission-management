# Group Individual Members API Design

## Background

This document defines individual-member read behavior for `group-service`. Individual members are explicit users assigned to a group in addition to any dynamic grouping rules.

Entry point and shared service concerns are documented in [Group Service Design](group-service.md). Group-level create, read, soft-delete, and grouping-rule replacement behavior is documented in [Group API Design](group-service-group.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- Member list payloads, cursor tokens, MongoDB queries, and indexes are explicit contracts.
- Handlers parse path and query input, services own use-case validation, and repositories own MongoDB pagination details.
- Request and response DTOs belong in `internal/group-service/transport`.
- Cursor and list models stay in domain or transport packages without leaking MongoDB driver types to handlers.

## Goals

- Expose `GET /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members`.
- Return active individual members for one active group.
- Use cursor-based pagination with `limit` and `next_token`.
- Sort individual members by `created_at DESC, _id DESC`.
- Return `members` and `page_info` in the response.
- Reuse `internal/shared/pagination` for limit parsing and cursor token encode/decode.
- Keep member reads scoped by an active group lookup using both `workspace_id` and `group_id`.

## Non-Goals

- Do not implement individual member add, replace, delete, or restore APIs in this phase.
- Do not evaluate dynamic grouping rules or merge dynamic members into this response.
- Do not filter members by employee attributes, NT account prefix, expiration date, or active permission status.
- Do not validate whether `workspace_id` references an existing workspace.
- Do not publish membership-read or membership-changed events.

## List Individual Members API

Endpoint:

```http
GET /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members
```

Path parameters:

- `workspace_id`: workspace scope for the group.
- `group_id`: group identifier.

Query string:

- `limit`: optional integer. Default `20`, maximum `50`.
- `next_token`: optional opaque cursor token.

Behavior:

- First verify that an active group exists for `_id = group_id`, `workspace_id`, and `deleted_at: null`.
- If no active group exists, return `200 OK` with an empty page.
- List only active individual member documents where `deleted_at` is `null`.
- Do not filter out members whose `expiration_date` is in the past. This endpoint reads configured individual member records; permission evaluation can decide whether an expired member grants access.
- Sort by `created_at DESC, _id DESC`.
- Fetch `limit + 1` records to determine `has_next_page`.

Success response:

```http
HTTP/1.1 200 OK
```

```json
{
  "members": [
    {
      "nt_account": "user1",
      "expiration_date": "2026-06-01T00:00:00Z"
    }
  ],
  "page_info": {
    "has_next_page": true,
    "next_token": "<opaque-token>"
  }
}
```

Empty response:

```http
HTTP/1.1 200 OK
```

```json
{
  "members": [],
  "page_info": {
    "has_next_page": false,
    "next_token": ""
  }
}
```

Field contract:

- `members[].nt_account` is the trimmed account persisted during create.
- `members[].expiration_date` is returned as an RFC3339 timestamp string.
- Member `_id`, `group_id`, `created_at`, `updated_at`, and `deleted_at` are persistence metadata and are not included in the public response.
- `page_info.has_next_page` is `true` only when another page exists after the returned items.
- `page_info.next_token` is an opaque string. It is empty when `has_next_page` is `false`.

## Cursor Contract

Cursor tokens should encode the last returned member's pagination identity:

```json
{
  "created_at": "2026-05-05T07:31:00Z",
  "id": "member-123"
}
```

Rules:

- Tokens are base64url-encoded JSON using `internal/shared/pagination`.
- `created_at` must be an RFC3339 timestamp.
- `id` must be non-empty.
- Invalid, malformed, or semantically incomplete tokens return `400 Bad Request`.
- Empty `next_token` is treated as no cursor when the query parameter is omitted or blank.

MongoDB cursor boundary for descending order:

```txt
created_at < cursor.created_at
OR (created_at == cursor.created_at AND _id < cursor.id)
```

## Request Validation

Transport-level validation should reject:

- Non-integer `limit`.
- `limit < 1`.
- `limit > 50`.
- `next_token` that is not base64url-encoded JSON.
- `next_token.created_at` that is missing or not RFC3339.
- `next_token.id` that is missing or empty.

Domain or service validation should reject:

- Empty `workspace_id`.
- Empty `group_id`.
- Non-positive list limits if a non-HTTP caller bypasses transport parsing.
- Cursors with zero `created_at`.
- Cursors with empty IDs.

## Error Handling

Status mapping:

- `400 Bad Request`: invalid path identity, invalid limit, invalid cursor token, or invalid decoded cursor fields.
- `500 Internal Server Error`: unexpected repository or infrastructure failure.
- `200 OK`: successful read, including missing group, soft-deleted group, or no matching members.

Missing-group member reads intentionally return an empty page. GET group remains the endpoint clients should use when they need explicit group existence information.

## Domain Model

The domain package should model member list behavior without Echo or MongoDB dependencies.

Primary types:

- `IndividualMember`: explicit member record with `nt_account`, expiration date, timestamps, and internal ID for pagination.
- `IndividualMemberCursor`: pagination cursor containing `created_at` and member ID.
- `ListIndividualMembersQuery`: list identity containing `workspace_id`, `group_id`, `limit`, and optional cursor.
- `IndividualMemberPage`: page result containing members, `has_next_page`, and optional next cursor.

The public response should map from `IndividualMemberPage` to transport DTOs and omit internal pagination identity fields from each member.

## group_individual_members Collection

Collection: `group_individual_members`

Document schema:

```ts
{
  "_id": string,
  "group_id": string,
  "nt_account": string,
  "expiration_date": Date,
  "created_at": Date,
  "updated_at": Date,
  "deleted_at": Date | null
}
```

Field notes:

- `_id` is a service-generated UUID.
- `group_id` references `groups._id`.
- `nt_account` is trimmed before validation and persistence.
- `expiration_date` is the explicit membership expiration date.
- `created_at` and `updated_at` are set to the same service-generated `now` during creation.
- `updated_at` changes when the member is soft-deleted by group delete.
- `deleted_at` is `null` for active individual member documents.

Indexes:

```txt
partial unique { group_id: 1, nt_account: 1 } where deleted_at == null
{ group_id: 1, created_at: -1, _id: -1 }
```

Rationale:

- The partial unique index prevents multiple active member rows for the same `group_id + nt_account`.
- The pagination index supports active member reads sorted by `created_at DESC, _id DESC`.
- `group_individual_members` intentionally does not duplicate `workspace_id`; member-list reads confirm group ownership through the active `groups` document first.

## Repository Query

Recommended list flow:

1. Read the active group using `_id`, `workspace_id`, and `deleted_at: null`.
2. If no group exists, return an empty page without querying members.
3. Build the member filter:

```txt
{
  group_id: <group_id>,
  deleted_at: null,
  ...cursor boundary when next_token exists
}
```

4. Sort by `created_at DESC, _id DESC`.
5. Limit to `limit + 1`.
6. If more than `limit` documents are returned, trim the extra item and build the next cursor from the last returned member.
7. Return members and page metadata to the service.

## Service Workflow

1. Handler extracts `workspace_id`, `group_id`, `limit`, and `next_token`.
2. Handler uses `internal/shared/pagination.PaginationHelper` to parse `limit`.
3. Transport decodes `next_token` into an `IndividualMemberCursor` when present.
4. Handler maps values to `group.ListIndividualMembersQuery`.
5. Service validates path identity, limit, and cursor invariants.
6. Repository verifies active group ownership and fetches a member page.
7. Service returns `IndividualMemberPage`.
8. Transport maps members and encodes `next_token`.
9. Handler renders `200 OK`.

## REST Client Examples

`examples/api/groups.http` should include:

- List individual members with default limit.
- List individual members with explicit `limit`.
- List individual members with `limit` and `next_token`.
- Missing or soft-deleted group returning an empty page.
- Invalid `limit` returning `400`.
- Invalid `next_token` returning `400`.

## Testing Strategy

Domain tests:

- List query rejects empty `workspace_id`.
- List query rejects empty `group_id`.
- List query rejects non-positive `limit`.
- Cursor validation rejects zero `created_at`.
- Cursor validation rejects empty ID.

Transport tests:

- No `limit` uses default `20`.
- `limit=50` is accepted.
- `limit=51` returns a validation error.
- Non-integer `limit` returns a validation error.
- Empty or omitted `next_token` means no cursor.
- Invalid `next_token` returns a validation error.
- Valid cursor token decodes into domain cursor fields.
- Response maps `nt_account`, `expiration_date`, and `page_info`.
- Response omits member `_id`, `group_id`, `created_at`, `updated_at`, and `deleted_at`.

Service tests:

- Successful list passes query values to the repository.
- Missing active group returns an empty page.
- Repository failures are wrapped with context.
- Validation failures do not call the repository.

Repository tests:

- List first verifies the active group by `_id`, `workspace_id`, and `deleted_at: null`.
- Missing group returns an empty page.
- Soft-deleted group returns an empty page.
- List filters members by `group_id` and `deleted_at: null`.
- List applies `created_at DESC, _id DESC` sort.
- List applies cursor boundary correctly.
- List fetches `limit + 1` and builds `has_next_page` and next cursor correctly.
- Pagination index is created as `{ group_id: 1, created_at: -1, _id: -1 }`.

Handler tests:

- Successful list returns `200` and the documented response body.
- Missing group returns `200` with an empty page.
- Invalid path identity returns `400`.
- Invalid `limit` returns `400`.
- Invalid `next_token` returns `400`.
- Unexpected service failure returns `500`.
