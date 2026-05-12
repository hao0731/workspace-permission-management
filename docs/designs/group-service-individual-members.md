# Group Individual Members API Design

## Background

This document defines individual-member read and mutation behavior for `group-service`. Individual members are explicit users assigned to a group in addition to any dynamic grouping rules.

Entry point and shared service concerns are documented in [Group Service Design](group-service.md). Group-level create, read, soft-delete, and grouping-rule replacement behavior is documented in [Group API Design](group-service-group.md). Individual-member expiry tasks and command handling are documented in [Group Individual Member Expiry Command Design](group-service-individual-member-expiry-command.md).

## Classification and Policies

This is backend and design documentation work.

Required policies:

- [Backend Architecture Principle](../policies/backend-architecture-principle.md)
- [Design and Plan Docs Policy](../policies/design-and-plan-docs-policy.md)

Policy alignment:

- Member list payloads, mutation payloads, cursor tokens, MongoDB queries, indexes, expiry task side effects, and transaction behavior are explicit contracts.
- Handlers parse path, query, and body input, services own use-case validation, and repositories own MongoDB pagination and mutation details.
- Request and response DTOs belong in `internal/group-service/transport`.
- Cursor, list, and mutation models stay in domain or transport packages without leaking MongoDB driver types to handlers.

## Goals

- Expose `GET /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members`.
- Expose `POST /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members`.
- Expose `PATCH /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account`.
- Expose `DELETE /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account`.
- Return active individual members for one active group.
- Add active individual members to one active group.
- Update one active individual member's expiration date.
- Soft-delete one active individual member idempotently.
- Create and replace matching `individual_member_expiry_task` documents when individual members are created or their expiration date is updated.
- Remove matching `individual_member_expiry_task` documents when individual members are soft-deleted.
- Track asynchronous individual-member expiration with internal `expired_at` metadata.
- Use cursor-based pagination with `limit` and `next_token`.
- Sort individual members by `created_at DESC, _id DESC`.
- Return `members` and `page_info` in list responses.
- Return added `members` in add responses.
- Reuse `internal/shared/pagination` for limit parsing and cursor token encode/decode.
- Keep member reads scoped by an active group lookup using both `workspace_id` and `group_id`.
- Keep member mutations scoped by an active group lookup using both `workspace_id` and `group_id`.

## Non-Goals

- Do not implement individual member restore, hard-delete, history, search, or bulk replacement APIs in this phase.
- Do not evaluate dynamic grouping rules or merge dynamic members into this response.
- Do not filter members by employee attributes, NT account prefix, expiration date, or active permission status.
- Do not validate whether `workspace_id` references an existing workspace.
- Do not publish membership-read or membership-changed events.
- Do not define the individual-member expiry command consumer here; that behavior is documented in [Group Individual Member Expiry Command Design](group-service-individual-member-expiry-command.md).

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
- Do not filter out members whose `expiration_date` is in the past or whose `expired_at` is non-null. This endpoint reads configured individual member records; permission evaluation can decide whether an expired member grants access.
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
- Member `_id`, `group_id`, `created_at`, `updated_at`, `expired_at`, and `deleted_at` are persistence metadata and are not included in the public response.
- `page_info.has_next_page` is `true` only when another page exists after the returned items.
- `page_info.next_token` is an opaque string. It is empty when `has_next_page` is `false`.

## Add Individual Members API

Endpoint:

```http
POST /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members
```

Path parameters:

- `workspace_id`: workspace scope for the group.
- `group_id`: group identifier.

Request body:

```json
{
  "individual_members": [
    {
      "nt_account": "user2",
      "expiration_date": "2026-06-01T00:00:00Z"
    }
  ]
}
```

Success response:

```http
HTTP/1.1 201 Created
```

```json
{
  "members": [
    {
      "nt_account": "user2",
      "expiration_date": "2026-06-01T00:00:00Z"
    }
  ]
}
```

Behavior:

- First verify that an active group exists for `_id = group_id`, `workspace_id`, and `deleted_at: null`.
- If no active group exists, return `404 Not Found`.
- Add every requested member as a new active `group_individual_members` document.
- Use one service-generated `now` for validation, `created_at`, and `updated_at` across the request.
- Create one matching `individual_member_expiry_task` document for each inserted member.
- Use one MongoDB transaction for the active group check, member inserts, and expiry task inserts.
- Return only the members added by this request.

Add field contract:

- `individual_members` is required and must contain at least one item.
- `individual_members` is limited by `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS`, defaulting to 1000 items.
- `individual_members[].nt_account` is trimmed before validation, persistence, uniqueness checks, and response rendering.
- Duplicate `individual_members[].nt_account` values in the same request after trimming return `409 Conflict`.
- An `nt_account` that already has an active member document in the same group returns `409 Conflict`.
- `individual_members[].expiration_date` must be accepted as an RFC3339 timestamp string and must be later than the service's request processing time.
- Member `_id`, `group_id`, `created_at`, `updated_at`, `expired_at`, and `deleted_at` are persistence metadata and are not included in the public response.

## Update Individual Member Expiration API

Endpoint:

```http
PATCH /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account
```

Path parameters:

- `workspace_id`: workspace scope for the group.
- `group_id`: group identifier.
- `nt_account`: individual member account. Clients must URL-encode characters that are not safe in a path segment.

Request body:

```json
{
  "expiration_date": "2026-07-01T00:00:00Z"
}
```

Success response:

```http
HTTP/1.1 204 No Content
```

Behavior:

- First verify that an active group exists for `_id = group_id`, `workspace_id`, and `deleted_at: null`.
- If no active group exists, return `404 Not Found`.
- Update only the active member document matching `group_id`, trimmed `nt_account`, and `deleted_at: null`.
- If no active member exists for that account in the active group, return `404 Not Found`.
- Set `expiration_date` to the request value, set `updated_at` to the service-generated `now`, and reset `expired_at` to `null`.
- Replace the matching `individual_member_expiry_task` with a new task ID and recalculated expiration bucket.
- Do not modify `created_at`, `deleted_at`, or any other member fields.
- Use one MongoDB transaction for the active group check, member update, and expiry task replacement.

Update field contract:

- `expiration_date` is required.
- `expiration_date` must be accepted as an RFC3339 timestamp string and must be later than the service's request processing time.
- The response has no body.

## Delete Individual Member API

Endpoint:

```http
DELETE /api/v1/workspaces/:workspace_id/groups/:group_id/individual-members/:nt_account
```

Path parameters:

- `workspace_id`: workspace scope for the group.
- `group_id`: group identifier.
- `nt_account`: individual member account. Clients must URL-encode characters that are not safe in a path segment.

Success response:

```http
HTTP/1.1 204 No Content
```

Behavior:

- Delete is idempotent and returns `204 No Content` even when the active group or active member is already missing.
- When an active group exists for `_id = group_id`, `workspace_id`, and `deleted_at: null`, soft-delete the active member document matching `group_id`, trimmed `nt_account`, and `deleted_at: null`.
- Set `deleted_at` and `updated_at` to the same service-generated `now`.
- Do not hard-delete member documents.
- Use one MongoDB transaction for the active group check, member soft delete, and expiry task cleanup.

Delete field contract:

- The response has no body.
- Soft-deleted individual members are excluded from list, update, and add uniqueness checks.
- Soft-deleting an individual member deletes any matching `individual_member_expiry_task` for `group_id + nt_account` in the same transaction.
- A later add request may recreate the same `nt_account` as a new active member document because the unique index only applies to active documents.

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
- Malformed JSON bodies for add or update.
- Add requests where `individual_members` is missing, `null`, or empty.
- Add requests where any `individual_members[].expiration_date` is missing or not RFC3339.
- Update requests where `expiration_date` is missing or not RFC3339.

Domain or service validation should reject:

- Empty `workspace_id`.
- Empty `group_id`.
- Empty or whitespace-only `nt_account` after trimming when the endpoint path or request body includes an account.
- Non-positive list limits if a non-HTTP caller bypasses transport parsing.
- Cursors with zero `created_at`.
- Cursors with empty IDs.
- Add requests where `individual_members` exceeds the configured `GROUP_SERVICE_MAX_INDIVIDUAL_MEMBERS` limit.
- Add requests where duplicate `individual_members[].nt_account` values appear after trimming.
- Add or update requests where `expiration_date` is not later than the service's request processing time.

## Error Handling

Status mapping:

- `400 Bad Request`: invalid path identity, invalid limit, invalid cursor token, invalid decoded cursor fields, malformed JSON, invalid request shape, empty account values, configured limit violations, or expiration dates that are not in the future.
- `404 Not Found`: add or update requests where the active group does not exist, or update requests where the active member does not exist.
- `409 Conflict`: add requests with duplicate `nt_account` values in the same request or an already-active member in the same group.
- `500 Internal Server Error`: unexpected repository or infrastructure failure.
- `200 OK`: successful read, including missing group, soft-deleted group, or no matching members.
- `201 Created`: successful member add.
- `204 No Content`: successful update or delete. Delete also returns `204` when the active group or active member is already missing.

Missing-group member reads intentionally return an empty page. GET group remains the endpoint clients should use when they need explicit group existence information.

## Domain Model

The domain package should model member list and mutation behavior without Echo or MongoDB dependencies.

Primary types:

- `IndividualMember`: explicit member record with `nt_account`, expiration date, internal expiration state, timestamps, and internal ID for pagination.
- `IndividualMemberCursor`: pagination cursor containing `created_at` and member ID.
- `ListIndividualMembersQuery`: list identity containing `workspace_id`, `group_id`, `limit`, and optional cursor.
- `IndividualMemberPage`: page result containing members, `has_next_page`, and optional next cursor.
- `AddIndividualMembersInput`: add input containing `workspace_id`, `group_id`, and requested individual members.
- `UpdateIndividualMemberExpirationInput`: update identity containing `workspace_id`, `group_id`, `nt_account`, and the replacement expiration date.
- `DeleteIndividualMemberInput`: delete identity containing `workspace_id`, `group_id`, and `nt_account`.

List responses should map from `IndividualMemberPage` to transport DTOs. Add responses should map from the added `IndividualMember` records. Both response shapes must omit internal pagination identity and persistence metadata fields from each member.

## group_individual_members Collection

Collection: `group_individual_members`

Document schema:

```ts
{
  "_id": string,
  "group_id": string,
  "nt_account": string,
  "expiration_date": Date,
  "expired_at": Date | null,
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
- `expired_at` is `null` until the individual-member expiry command marks the member expired. Legacy documents without this field should be treated as `null`.
- `created_at` and `updated_at` are set to the same service-generated `now` during creation or member add.
- `updated_at` changes when the member's expiration date is updated, when the member is marked expired by command handling, when the member is soft-deleted directly, and when the member is soft-deleted by group delete.
- `deleted_at` is `null` for active individual member documents.

Indexes:

```txt
partial unique { group_id: 1, nt_account: 1 } where deleted_at == null
partial { group_id: 1, _id: 1 } where deleted_at == null and expired_at == null
{ group_id: 1, created_at: -1, _id: -1 }
```

Rationale:

- The partial unique index prevents multiple active member rows for the same `group_id + nt_account` and supports direct update/delete lookup by member identity.
- The partial active-unexpired index supports existence checks used when grouping rules are removed and the service must confirm that at least one effective individual member remains.
- The pagination index supports active member reads sorted by `created_at DESC, _id DESC`.
- `group_individual_members` intentionally does not duplicate `workspace_id`; member APIs confirm group ownership through the active `groups` document first.
- `expired_at` is not part of the unique index. Expired records remain active records until they are soft-deleted; clients can extend an expired member through the expiration update API, which resets `expired_at`.

Related collection:

- `individual_member_expiry_task` stores one active expiry task per active member. Its schema and command contract are documented in [Group Individual Member Expiry Command Design](group-service-individual-member-expiry-command.md#individual_member_expiry_task-collection).

## Repository Flows

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

Recommended add flow:

1. Start a MongoDB transaction.
2. Read the active group using `_id`, `workspace_id`, and `deleted_at: null`.
3. If no group exists, return `group.ErrNotFound`.
4. Insert one `group_individual_members` document for each requested member with generated `_id`, `group_id`, trimmed `nt_account`, `expiration_date`, `expired_at: null`, `created_at`, `updated_at`, and `deleted_at: null`.
5. Insert one `individual_member_expiry_task` document for each requested member with generated task `_id`, `group_id`, `nt_account`, and calculated `expiration_bucket`.
6. Map duplicate-key errors from the partial unique `{ group_id: 1, nt_account: 1 }` index to a stable duplicate-member domain error.
7. Commit the transaction and return the added members to the service.

Recommended update flow:

1. Start a MongoDB transaction.
2. Read the active group using `_id`, `workspace_id`, and `deleted_at: null`.
3. If no group exists, return `group.ErrNotFound`.
4. Update one active member by `group_id`, trimmed `nt_account`, and `deleted_at: null`, setting `expiration_date`, `updated_at`, and `expired_at: null`.
5. If the update matched no active member, return `group.ErrNotFound`.
6. Delete any existing `individual_member_expiry_task` for `group_id + nt_account`.
7. Insert a replacement task with a generated task `_id` and recalculated `expiration_bucket`.
8. Commit the transaction.

Recommended delete flow:

1. Start a MongoDB transaction.
2. Read the active group using `_id`, `workspace_id`, and `deleted_at: null`.
3. If no group exists, commit or abort without mutating members and return success.
4. Soft-delete one active member by `group_id`, trimmed `nt_account`, and `deleted_at: null`, setting `deleted_at` and `updated_at`.
5. Delete any existing `individual_member_expiry_task` for `group_id + nt_account`.
6. If the update matched no active member, return success.
7. Commit the transaction.

## Service Workflows

List workflow:

1. Handler extracts `workspace_id`, `group_id`, `limit`, and `next_token`.
2. Handler uses `internal/shared/pagination.PaginationHelper` to parse `limit`.
3. Transport decodes `next_token` into an `IndividualMemberCursor` when present.
4. Handler maps values to `group.ListIndividualMembersQuery`.
5. Service validates path identity, limit, and cursor invariants.
6. Repository verifies active group ownership and fetches a member page.
7. Service returns `IndividualMemberPage`.
8. Transport maps members and encodes `next_token`.
9. Handler renders `200 OK`.

Add workflow:

1. Handler extracts `workspace_id` and `group_id`, decodes the request, and maps it to `group.AddIndividualMembersInput`.
2. Service normalizes and validates path identity, requested members, request size, duplicate accounts, and future expiration dates.
3. Service generates member IDs, individual-member expiry task IDs, expiration buckets, and one timestamp for the request.
4. Repository verifies active group ownership and inserts the member documents and task documents in one transaction.
5. Repository maps duplicate active members to the duplicate-member domain error.
6. Transport maps the added members to the `members` response.
7. Handler renders `201 Created`.

Update workflow:

1. Handler extracts `workspace_id`, `group_id`, and `nt_account`, decodes the request, and maps it to `group.UpdateIndividualMemberExpirationInput`.
2. Service normalizes and validates path identity, account, and future expiration date.
3. Service generates one timestamp for `updated_at`, resets `expired_at`, and generates a replacement expiry task ID and bucket.
4. Repository verifies active group ownership, updates the active member, and replaces the matching task in one transaction.
5. Handler renders `204 No Content`, or maps missing active group/member to `404 Not Found`.

Delete workflow:

1. Handler extracts `workspace_id`, `group_id`, and `nt_account`.
2. Service normalizes and validates path identity and account.
3. Service generates one timestamp for `deleted_at` and `updated_at`.
4. Repository verifies active group ownership, soft-deletes the active member when present, and removes any matching expiry task in one transaction.
5. Handler renders `204 No Content` for success, missing active group, or missing active member.

## REST Client Examples

`examples/api/groups.http` should include:

- List individual members with default limit.
- List individual members with explicit `limit`.
- List individual members with `limit` and `next_token`.
- Missing or soft-deleted group returning an empty page.
- Invalid `limit` returning `400`.
- Invalid `next_token` returning `400`.
- Add individual members successfully.
- Add individual members with duplicate accounts returning `409`.
- Add individual members with an invalid expiration date returning `400`.
- Update an individual member expiration date successfully.
- Update a missing active member returning `404`.
- Delete an individual member successfully.
- Delete a missing active member returning idempotent `204`.

## Testing Strategy

Domain tests:

- List query rejects empty `workspace_id`.
- List query rejects empty `group_id`.
- List query rejects non-positive `limit`.
- Cursor validation rejects zero `created_at`.
- Cursor validation rejects empty ID.
- Add input rejects empty `workspace_id`.
- Add input rejects empty `group_id`.
- Add input rejects empty `individual_members`.
- Add input rejects empty or whitespace-only `nt_account`.
- Add input rejects duplicate `nt_account` values after trimming.
- Add input rejects expiration dates that are not in the future.
- Update input rejects empty `workspace_id`, `group_id`, or `nt_account`.
- Update input rejects expiration dates that are not in the future.
- Delete input rejects empty `workspace_id`, `group_id`, or `nt_account`.

Transport tests:

- No `limit` uses default `20`.
- `limit=50` is accepted.
- `limit=51` returns a validation error.
- Non-integer `limit` returns a validation error.
- Empty or omitted `next_token` means no cursor.
- Invalid `next_token` returns a validation error.
- Valid cursor token decodes into domain cursor fields.
- Response maps `nt_account`, `expiration_date`, and `page_info`.
- Response omits member `_id`, `group_id`, `created_at`, `updated_at`, `expired_at`, and `deleted_at`.
- Add request maps `individual_members` to domain members.
- Add response maps added `members` and omits persistence metadata.
- Update request maps `expiration_date` to a domain update input.
- Malformed add or update JSON returns a validation error.
- Missing or invalid add/update `expiration_date` returns a validation error.

Service tests:

- Successful list passes query values to the repository.
- Missing active group returns an empty page.
- Successful add validates, generates member IDs, task IDs, buckets, timestamps, and passes members and tasks to the repository.
- Add duplicate-member errors are preserved for handler conflict mapping.
- Successful update validates, resets `expired_at`, generates a replacement task, and passes the replacement expiration date to the repository.
- Update missing active group/member errors are preserved for handler not-found mapping.
- Successful delete validates and passes the member identity to the repository.
- Delete missing active group/member results still return success.
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
- Add verifies active group ownership before inserting members.
- Add inserts all requested members and matching `individual_member_expiry_task` documents in one transaction.
- Add maps duplicate-key errors on active `group_id + nt_account` to the duplicate-member domain error.
- Update verifies active group ownership before updating a member.
- Update changes `expiration_date`, `updated_at`, and `expired_at` on the active member.
- Update replaces the matching `individual_member_expiry_task` in the same transaction.
- Update returns not found when the active group or active member is missing.
- Delete verifies active group ownership before soft-deleting a member.
- Delete sets `deleted_at` and `updated_at` to the same timestamp.
- Delete removes the matching `individual_member_expiry_task` in the same transaction.
- Delete returns success when the active group or active member is missing.
- Active-unexpired existence index is created as partial `{ group_id: 1, _id: 1 }` where `deleted_at == null and expired_at == null`.
- Pagination index is created as `{ group_id: 1, created_at: -1, _id: -1 }`.
- Individual-member expiry task indexes are created as `unique { group_id: 1, nt_account: 1 }` and `{ expiration_bucket: 1, _id: 1 }`.

Handler tests:

- Successful list returns `200` and the documented response body.
- Missing group returns `200` with an empty page.
- Successful add returns `201` and the documented response body.
- Add duplicate member returns `409`.
- Add missing active group returns `404`.
- Successful update returns `204`.
- Update missing active group/member returns `404`.
- Successful delete returns `204`.
- Delete missing active group/member returns `204`.
- Invalid path identity returns `400`.
- Invalid `limit` returns `400`.
- Invalid `next_token` returns `400`.
- Invalid add/update body returns `400`.
- Unexpected service failure returns `500`.
