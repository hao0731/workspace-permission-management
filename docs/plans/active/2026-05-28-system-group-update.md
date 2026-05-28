# System Group Update Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement `PUT /api/v1/systems/:system_id/groups/:group_id` so system group updates diff relationship projections, write one mixed permission API request, and persist the actual accepted state.

**Architecture:** Keep the public HTTP contract in `internal/group-service/transport`, validation in `internal/domain/group`, permission relationship diffing and partial-result merge logic in `internal/group-service/services`, and MongoDB reads/transactions in `internal/group-service/repositories`. The handler remains thin and maps service errors to `400`, `404`, `502`, `500`, `200`, or `206`.

**Tech Stack:** Go 1.25, Echo v5, MongoDB Go driver v2, `log/slog`, existing shared permission client, existing group-service test style.

---

## Source Designs

- `docs/designs/system-group-api-design.md`
- `docs/designs/group-service.md`
- `docs/policies/backend-architecture-principle.md`
- `docs/policies/design-and-plan-docs-policy.md`

## Policy Notes

- Backend policy: handlers parse and map only; transport owns DTOs; services own use-case workflow and external permission side effects; repositories isolate MongoDB mechanics; API and database schemas are explicit contracts with tests.
- Design/plan docs policy: this plan starts under `docs/plans/active/` and links back to the source design documents.

## File Structure

- Modify `internal/domain/group/system_group.go`: add `SystemGroupUpdateInput` plus normalize support.
- Modify `internal/domain/group/system_group_validation.go`: share create/update validation and add `group_id` validation.
- Modify `internal/domain/group/system_group_test.go`: add update normalization and validation tests.
- Modify `internal/group-service/transport/system_group_request.go`: add `SystemGroupUpdateRequest`, decoder, shared rule-to-domain mapper, and update `ToDomain`.
- Modify `internal/group-service/transport/system_group_request_test.go`: add update request decode and validation-boundary tests.
- Modify `internal/group-service/transport/system_group_response.go`: add update success and partial response constructors.
- Modify `internal/group-service/transport/system_group_response_test.go`: add update response tests.
- Modify `internal/group-service/repositories/mongo_system_group_repository.go`: add current group/projection read, projection document domain mapping, and update transaction.
- Modify `internal/group-service/repositories/mongo_system_group_repository_test.go`: add mapping, not-found, and integration tests for read/update.
- Modify `internal/group-service/services/system_group_service.go`: extend repository interface, add `UpdateSystemGroup`, and orchestrate update permission writes.
- Modify `internal/group-service/services/system_group_relationship_builder.go`: add update diff and partial merge helpers.
- Modify `internal/group-service/services/system_group_service_test.go`: add update service workflow tests.
- Modify `internal/group-service/handlers/group_handler.go`: add service interface method, route registration, handler, and error mapping.
- Modify `internal/group-service/handlers/group_handler_test.go`: add route/status tests and fake service fields.
- Modify `examples/api/system-groups.http`: add update success, partial, and not-found examples.

## Tasks

### Task 1: Domain Update Input And Validation

**Files:**
- Modify: `internal/domain/group/system_group.go`
- Modify: `internal/domain/group/system_group_validation.go`
- Test: `internal/domain/group/system_group_test.go`

- [ ] **Step 1: Write failing domain tests**

Add these tests to `internal/domain/group/system_group_test.go`:

```go
func validSystemGroupUpdateInput() SystemGroupUpdateInput {
	return SystemGroupUpdateInput{
		SystemID: " system-a ",
		GroupID:  " group-1 ",
		Name:     " System Admins Updated ",
		GroupingRules: []SystemGroupRule{
			{AttributeKey: GroupAttributeOrganization, Operator: OperatorEq, Multi: true, Value: []string{" ORG-300 ", "ORG-100"}},
			{AttributeKey: GroupAttributeJobType, Operator: OperatorEq, Multi: false, Value: " IDL "},
		},
	}
}

func TestSystemGroupUpdateInputNormalize(t *testing.T) {
	input := validSystemGroupUpdateInput().Normalize()

	if input.SystemID != "system-a" {
		t.Fatalf("SystemID = %q, want system-a", input.SystemID)
	}
	if input.GroupID != "group-1" {
		t.Fatalf("GroupID = %q, want group-1", input.GroupID)
	}
	if input.Name != "System Admins Updated" {
		t.Fatalf("Name = %q, want System Admins Updated", input.Name)
	}
	values := input.GroupingRules[0].Value.([]string)
	if values[0] != "ORG-300" {
		t.Fatalf("first org value = %q, want ORG-300", values[0])
	}
	if input.GroupingRules[1].Value.(string) != "IDL" {
		t.Fatalf("job type = %q, want IDL", input.GroupingRules[1].Value.(string))
	}
}

func TestSystemGroupUpdateInputValidateAcceptsValidAndEmptyRules(t *testing.T) {
	if err := validSystemGroupUpdateInput().Normalize().Validate(); err != nil {
		t.Fatalf("Validate error = %v, want nil", err)
	}

	emptyRules := SystemGroupUpdateInput{
		SystemID:      "system-a",
		GroupID:       "group-1",
		Name:          "Everyone",
		GroupingRules: []SystemGroupRule{},
	}
	if err := emptyRules.Normalize().Validate(); err != nil {
		t.Fatalf("Validate empty rules error = %v, want nil", err)
	}
}

func TestSystemGroupUpdateInputValidateRejectsInvalidIdentityAndName(t *testing.T) {
	tests := []struct {
		name   string
		input  SystemGroupUpdateInput
		reason string
	}{
		{name: "empty system id", input: SystemGroupUpdateInput{SystemID: " ", GroupID: "group-1", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "system id is required"},
		{name: "empty group id", input: SystemGroupUpdateInput{SystemID: "system-a", GroupID: " ", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "group id is required"},
		{name: "group id has whitespace", input: SystemGroupUpdateInput{SystemID: "system-a", GroupID: "group 1", Name: "Group", GroupingRules: []SystemGroupRule{}}, reason: "group id must be a single token"},
		{name: "empty name", input: SystemGroupUpdateInput{SystemID: "system-a", GroupID: "group-1", Name: " ", GroupingRules: []SystemGroupRule{}}, reason: "name is required"},
		{name: "nil grouping rules", input: SystemGroupUpdateInput{SystemID: "system-a", GroupID: "group-1", Name: "Group", GroupingRules: nil}, reason: "grouping_rules is required"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			requireSystemGroupInvalidInput(t, tt.input.Normalize().Validate(), tt.reason)
		})
	}
}
```

- [ ] **Step 2: Run the failing domain tests**

Run:

```bash
go test ./internal/domain/group -run 'TestSystemGroup(Update|Create)' -count=1
```

Expected: FAIL with undefined `SystemGroupUpdateInput`.

- [ ] **Step 3: Add update input and normalization**

Modify `internal/domain/group/system_group.go`:

```go
type SystemGroupUpdateInput struct {
	SystemID      string
	GroupID       string
	Name          string
	GroupingRules []SystemGroupRule
}

func (input SystemGroupUpdateInput) Normalize() SystemGroupUpdateInput {
	input.SystemID = strings.TrimSpace(input.SystemID)
	input.GroupID = strings.TrimSpace(input.GroupID)
	input.Name = strings.TrimSpace(input.Name)
	for i := range input.GroupingRules {
		input.GroupingRules[i] = input.GroupingRules[i].Normalize()
	}
	return input
}
```

- [ ] **Step 4: Share system group mutation validation**

Modify `internal/domain/group/system_group_validation.go` so create and update use the same rule validation:

```go
func (input SystemGroupCreateInput) Validate() error {
	return validateSystemGroupMutation(input.SystemID, input.Name, input.GroupingRules)
}

func (input SystemGroupUpdateInput) Validate() error {
	if err := validateSystemGroupID(input.GroupID); err != nil {
		return err
	}
	return validateSystemGroupMutation(input.SystemID, input.Name, input.GroupingRules)
}

func validateSystemGroupMutation(systemID string, name string, rules []SystemGroupRule) error {
	if err := validateSystemID(systemID); err != nil {
		return err
	}
	if strings.TrimSpace(name) == "" {
		return invalidInput("name is required")
	}
	if rules == nil {
		return invalidInput("grouping_rules is required")
	}
	jobTypeCount := 0
	for _, rule := range rules {
		if rule.AttributeKey == GroupAttributeJobType {
			jobTypeCount++
			if jobTypeCount > 1 {
				return invalidInput("only one job_type rule is allowed")
			}
		}
		if err := rule.Validate(); err != nil {
			return err
		}
	}
	return nil
}

func validateSystemGroupID(groupID string) error {
	trimmed := strings.TrimSpace(groupID)
	if trimmed == "" {
		return invalidInput("group id is required")
	}
	if strings.ContainsAny(trimmed, " \t\n\r") {
		return invalidInput("group id must be a single token")
	}
	return nil
}
```

- [ ] **Step 5: Run domain tests**

Run:

```bash
go test ./internal/domain/group -run 'TestSystemGroup(Update|Create|List)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit domain changes**

```bash
git add internal/domain/group/system_group.go internal/domain/group/system_group_validation.go internal/domain/group/system_group_test.go
git commit -m "feat: add system group update domain input"
```

### Task 2: Transport Update DTOs And Responses

**Files:**
- Modify: `internal/group-service/transport/system_group_request.go`
- Modify: `internal/group-service/transport/system_group_request_test.go`
- Modify: `internal/group-service/transport/system_group_response.go`
- Modify: `internal/group-service/transport/system_group_response_test.go`

- [ ] **Step 1: Write failing transport request tests**

Add these tests to `internal/group-service/transport/system_group_request_test.go`:

```go
func TestDecodeSystemGroupUpdateRequestToDomain(t *testing.T) {
	request, err := DecodeSystemGroupUpdateRequest(strings.NewReader(`{
		"name": " System Admins Updated ",
		"grouping_rules": [
			{"attribute_key": "organization", "operator": "eq", "multi": true, "value": [" ORG-300 ", "ORG-100"]},
			{"attribute_key": "job_type", "operator": "eq", "multi": false, "value": " IDL "}
		]
	}`))
	if err != nil {
		t.Fatalf("DecodeSystemGroupUpdateRequest error = %v, want nil", err)
	}

	input, err := request.ToDomain("system-a", "group-1")
	if err != nil {
		t.Fatalf("ToDomain error = %v, want nil", err)
	}
	if input.SystemID != "system-a" || input.GroupID != "group-1" || input.Name != " System Admins Updated " {
		t.Fatalf("input identity/name = %+v, want original transport values", input)
	}
	if len(input.GroupingRules) != 2 {
		t.Fatalf("rules len = %d, want 2", len(input.GroupingRules))
	}
	values, ok := input.GroupingRules[0].Value.([]string)
	if !ok || values[0] != " ORG-300 " {
		t.Fatalf("organization values = %#v, want string slice preserving transport value", input.GroupingRules[0].Value)
	}
}

func TestDecodeSystemGroupUpdateRequestRejectsMalformedJSON(t *testing.T) {
	_, err := DecodeSystemGroupUpdateRequest(strings.NewReader(`{"name":`))
	if err == nil {
		t.Fatal("DecodeSystemGroupUpdateRequest error = nil, want error")
	}
}

func TestSystemGroupUpdateRequestToDomainRejectsMissingGroupingRules(t *testing.T) {
	request := SystemGroupUpdateRequest{Name: "System Admins"}
	_, err := request.ToDomain("system-a", "group-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}

func TestSystemGroupUpdateRequestToDomainRejectsMissingMulti(t *testing.T) {
	request := SystemGroupUpdateRequest{
		Name: "System Admins",
		GroupingRules: []SystemGroupRuleRequest{{
			AttributeKey: "organization",
			Operator:     "eq",
			Value:        []string{"ORG-100"},
		}},
	}
	_, err := request.ToDomain("system-a", "group-1")
	if !errors.Is(err, group.ErrInvalidInput) {
		t.Fatalf("ToDomain error = %v, want ErrInvalidInput", err)
	}
}
```

- [ ] **Step 2: Write failing transport response tests**

Add these tests to `internal/group-service/transport/system_group_response_test.go`:

```go
func TestNewSystemGroupUpdateResponse(t *testing.T) {
	response := NewSystemGroupUpdateResponse(transportSystemGroupModel())
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var body map[string]any
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	groupBody, ok := body["group"].(map[string]any)
	if !ok {
		t.Fatalf("group = %#v, want object", body["group"])
	}
	if groupBody["name"] != "System Admins" {
		t.Fatalf("name = %v, want System Admins", groupBody["name"])
	}
	if _, ok := groupBody["system_id"]; ok {
		t.Fatal("system_id present, want omitted")
	}
}

func TestNewSystemGroupUpdatePartialResponse(t *testing.T) {
	response := NewSystemGroupUpdatePartialResponse(transportSystemGroupModel(), []string{"delete rejected"})
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("Marshal error = %v, want nil", err)
	}
	var body struct {
		Group  map[string]any `json:"group"`
		Errors []string       `json:"errors"`
	}
	if err := json.Unmarshal(data, &body); err != nil {
		t.Fatalf("Unmarshal error = %v, want nil", err)
	}
	if body.Group["id"] != "group-1" {
		t.Fatalf("group id = %v, want group-1", body.Group["id"])
	}
	if len(body.Errors) != 1 || body.Errors[0] != "delete rejected" {
		t.Fatalf("errors = %#v, want delete rejected", body.Errors)
	}
}
```

- [ ] **Step 3: Run failing transport tests**

Run:

```bash
go test ./internal/group-service/transport -run 'SystemGroup(Update|Create)' -count=1
```

Expected: FAIL with undefined `DecodeSystemGroupUpdateRequest`, `SystemGroupUpdateRequest`, and update response constructors.

- [ ] **Step 4: Add update request DTO and shared rule mapper**

Modify `internal/group-service/transport/system_group_request.go`:

```go
type SystemGroupUpdateRequest struct {
	Name          string                   `json:"name"`
	GroupingRules []SystemGroupRuleRequest `json:"grouping_rules"`
}

func DecodeSystemGroupUpdateRequest(body io.Reader) (SystemGroupUpdateRequest, error) {
	var request SystemGroupUpdateRequest
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(&request); err != nil {
		return SystemGroupUpdateRequest{}, fmt.Errorf("decode system group update request: %w", err)
	}
	return request, nil
}

func (request SystemGroupCreateRequest) ToDomain(systemID string) (group.SystemGroupCreateInput, error) {
	rules, err := systemGroupRulesToDomain(request.GroupingRules)
	if err != nil {
		return group.SystemGroupCreateInput{}, err
	}
	return group.SystemGroupCreateInput{
		SystemID:      systemID,
		Name:          request.Name,
		GroupingRules: rules,
	}, nil
}

func (request SystemGroupUpdateRequest) ToDomain(systemID string, groupID string) (group.SystemGroupUpdateInput, error) {
	rules, err := systemGroupRulesToDomain(request.GroupingRules)
	if err != nil {
		return group.SystemGroupUpdateInput{}, err
	}
	return group.SystemGroupUpdateInput{
		SystemID:      systemID,
		GroupID:       groupID,
		Name:          request.Name,
		GroupingRules: rules,
	}, nil
}

func systemGroupRulesToDomain(requestRules []SystemGroupRuleRequest) ([]group.SystemGroupRule, error) {
	if requestRules == nil {
		return nil, invalidGroupRequest("grouping_rules is required")
	}
	rules := make([]group.SystemGroupRule, 0, len(requestRules))
	for _, rule := range requestRules {
		if rule.Multi == nil {
			return nil, invalidGroupRequest("rule multi is required")
		}
		value, err := systemGroupRuleValue(rule.Value, *rule.Multi)
		if err != nil {
			return nil, err
		}
		rules = append(rules, group.SystemGroupRule{
			AttributeKey: group.GroupAttributeKey(rule.AttributeKey),
			Operator:     group.Operator(rule.Operator),
			Multi:        *rule.Multi,
			Value:        value,
		})
	}
	return rules, nil
}
```

- [ ] **Step 5: Add update response DTOs**

Modify `internal/group-service/transport/system_group_response.go`:

```go
type SystemGroupUpdateResponse struct {
	Group SystemGroupResponse `json:"group"`
}

type SystemGroupUpdatePartialResponse struct {
	Group  SystemGroupResponse `json:"group"`
	Errors []string            `json:"errors"`
}

func NewSystemGroupUpdateResponse(model group.SystemGroup) SystemGroupUpdateResponse {
	return SystemGroupUpdateResponse{Group: newSystemGroupResponse(model)}
}

func NewSystemGroupUpdatePartialResponse(model group.SystemGroup, errors []string) SystemGroupUpdatePartialResponse {
	return SystemGroupUpdatePartialResponse{
		Group:  newSystemGroupResponse(model),
		Errors: append([]string(nil), errors...),
	}
}
```

- [ ] **Step 6: Run transport tests**

Run:

```bash
go test ./internal/group-service/transport -run 'SystemGroup(Update|Create|List)' -count=1
```

Expected: PASS.

- [ ] **Step 7: Commit transport changes**

```bash
git add internal/group-service/transport/system_group_request.go internal/group-service/transport/system_group_request_test.go internal/group-service/transport/system_group_response.go internal/group-service/transport/system_group_response_test.go
git commit -m "feat: add system group update transport contract"
```

### Task 3: Repository Current Read And Update Transaction

**Files:**
- Modify: `internal/group-service/repositories/mongo_system_group_repository.go`
- Test: `internal/group-service/repositories/mongo_system_group_repository_test.go`

- [ ] **Step 1: Write failing repository unit tests**

Add these tests to `internal/group-service/repositories/mongo_system_group_repository_test.go`:

```go
func TestSystemGroupRelationshipDocumentToDomain(t *testing.T) {
	projection := repositorySystemGroupProjection()
	doc, err := newSystemGroupRelationshipDocument(projection)
	if err != nil {
		t.Fatalf("newSystemGroupRelationshipDocument error = %v, want nil", err)
	}

	model := doc.toDomain()
	if model.SystemID != "system-a" || model.GroupID != "group-1" {
		t.Fatalf("model = %+v, want projection identity", model)
	}
	if len(model.Relationships) != 1 || model.Relationships[0].Checksum != "checksum-1" {
		t.Fatalf("relationships = %+v, want checksum-1", model.Relationships)
	}
}

func TestNewSystemGroupRelationshipInfoDocuments(t *testing.T) {
	docs, err := newSystemGroupRelationshipInfoDocuments(repositorySystemGroupProjection().Relationships)
	if err != nil {
		t.Fatalf("newSystemGroupRelationshipInfoDocuments error = %v, want nil", err)
	}
	if len(docs) != 1 || docs[0].Checksum != "checksum-1" {
		t.Fatalf("docs = %+v, want checksum-1", docs)
	}
}

func TestBuildSystemGroupIdentityFilter(t *testing.T) {
	filter := buildSystemGroupIdentityFilter("system-a", "group-1")
	want := bson.M{"system_id": "system-a", "_id": "group-1"}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}

func TestBuildSystemGroupRelationshipIdentityFilter(t *testing.T) {
	filter := buildSystemGroupRelationshipIdentityFilter("system-a", "group-1")
	want := bson.M{"system_id": "system-a", "group_id": "group-1"}
	if !reflect.DeepEqual(filter, want) {
		t.Fatalf("filter = %#v, want %#v", filter, want)
	}
}
```

- [ ] **Step 2: Write failing repository integration test**

Add this test to `internal/group-service/repositories/mongo_system_group_repository_test.go`:

```go
func TestMongoGroupRepositoryGetAndUpdateSystemGroupIntegration(t *testing.T) {
	ctx := context.Background()
	repository := newIntegrationRepository(t)
	if err := repository.EnsureIndexes(ctx); err != nil {
		t.Fatalf("EnsureIndexes error = %v", err)
	}

	created := repositorySystemGroup()
	projection := repositorySystemGroupProjection()
	if _, err := repository.CreateSystemGroup(ctx, created, projection); err != nil {
		t.Fatalf("CreateSystemGroup error = %v", err)
	}

	gotGroup, gotProjection, err := repository.GetSystemGroupWithRelationships(ctx, "system-a", "group-1")
	if err != nil {
		t.Fatalf("GetSystemGroupWithRelationships error = %v", err)
	}
	if gotGroup.ID != "group-1" || gotProjection.GroupID != "group-1" {
		t.Fatalf("got group/projection = %+v/%+v, want group-1", gotGroup, gotProjection)
	}

	updated := gotGroup
	updated.Name = "Updated Admins"
	updated.GroupingRules = []group.SystemGroupRule{{
		AttributeKey: group.GroupAttributeJobType,
		Operator:     group.OperatorEq,
		Multi:        false,
		Value:        group.SystemGroupJobTypeIDL,
	}}
	updated.UpdatedAt = gotGroup.UpdatedAt.Add(time.Hour)
	updatedProjection := gotProjection
	updatedProjection.Relationships = []group.RelationshipInfo{{
		Relationship: map[string]any{"relation": "checked_member"},
		Checksum:     "checksum-2",
	}}
	updatedProjection.UpdatedAt = updated.UpdatedAt

	saved, err := repository.UpdateSystemGroup(ctx, updated, updatedProjection)
	if err != nil {
		t.Fatalf("UpdateSystemGroup error = %v", err)
	}
	if saved.Name != "Updated Admins" || !saved.CreatedAt.Equal(created.CreatedAt) {
		t.Fatalf("saved = %+v, want updated name and preserved created_at", saved)
	}

	gotGroup, gotProjection, err = repository.GetSystemGroupWithRelationships(ctx, "system-a", "group-1")
	if err != nil {
		t.Fatalf("Get after update error = %v", err)
	}
	if gotGroup.Name != "Updated Admins" {
		t.Fatalf("group name = %q, want Updated Admins", gotGroup.Name)
	}
	if len(gotProjection.Relationships) != 1 || gotProjection.Relationships[0].Checksum != "checksum-2" {
		t.Fatalf("projection = %+v, want checksum-2", gotProjection)
	}
}

func TestMongoGroupRepositoryGetSystemGroupWithRelationshipsNotFound(t *testing.T) {
	ctx := context.Background()
	repository := newIntegrationRepository(t)

	_, _, err := repository.GetSystemGroupWithRelationships(ctx, "system-a", "missing")
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("GetSystemGroupWithRelationships error = %v, want ErrNotFound", err)
	}
}
```

Add `errors` to the test imports.

- [ ] **Step 3: Run failing repository tests**

Run:

```bash
go test ./internal/group-service/repositories -run 'SystemGroup(RelationshipDocumentToDomain|RelationshipInfoDocuments|IdentityFilter|RelationshipIdentityFilter)|GetAndUpdateSystemGroup|GetSystemGroupWithRelationships' -count=1
```

Expected: FAIL with undefined repository methods and helpers.

- [ ] **Step 4: Add projection document mapping and filters**

Modify `internal/group-service/repositories/mongo_system_group_repository.go`:

```go
func (d systemGroupRelationshipDocument) toDomain() group.SystemGroupRelationshipProjection {
	relationships := make([]group.RelationshipInfo, 0, len(d.Relationships))
	for _, relationship := range d.Relationships {
		relationships = append(relationships, group.RelationshipInfo{
			Relationship: relationship.Relationship,
			Checksum:     relationship.Checksum,
		})
	}
	return group.SystemGroupRelationshipProjection{
		SystemID:      d.SystemID,
		GroupID:       d.GroupID,
		Relationships: relationships,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
	}
}

func buildSystemGroupIdentityFilter(systemID string, groupID string) bson.M {
	return bson.M{"system_id": systemID, "_id": groupID}
}

func buildSystemGroupRelationshipIdentityFilter(systemID string, groupID string) bson.M {
	return bson.M{"system_id": systemID, "group_id": groupID}
}
```

- [ ] **Step 5: Extract relationship info document mapping**

Modify `newSystemGroupRelationshipDocument` in `internal/group-service/repositories/mongo_system_group_repository.go` to use this helper:

```go
func newSystemGroupRelationshipDocument(model group.SystemGroupRelationshipProjection) (systemGroupRelationshipDocument, error) {
	relationships, err := newSystemGroupRelationshipInfoDocuments(model.Relationships)
	if err != nil {
		return systemGroupRelationshipDocument{}, err
	}
	return systemGroupRelationshipDocument{
		SystemID:      model.SystemID,
		GroupID:       model.GroupID,
		Relationships: relationships,
		CreatedAt:     model.CreatedAt,
		UpdatedAt:     model.UpdatedAt,
	}, nil
}

func newSystemGroupRelationshipInfoDocuments(infos []group.RelationshipInfo) ([]systemGroupRelationshipInfoDocument, error) {
	relationships := make([]systemGroupRelationshipInfoDocument, 0, len(infos))
	for _, relationship := range infos {
		relationshipDoc, err := newSystemGroupRelationshipValueDocument(relationship.Relationship)
		if err != nil {
			return nil, err
		}
		relationships = append(relationships, systemGroupRelationshipInfoDocument{
			Relationship: relationshipDoc,
			Checksum:     relationship.Checksum,
		})
	}
	return relationships, nil
}
```

- [ ] **Step 6: Add repository read and update methods**

Modify `internal/group-service/repositories/mongo_system_group_repository.go`:

```go
func (r *MongoGroupRepository) GetSystemGroupWithRelationships(ctx context.Context, systemID string, groupID string) (group.SystemGroup, group.SystemGroupRelationshipProjection, error) {
	var groupDoc systemGroupDocument
	if err := r.systemGroups.FindOne(ctx, buildSystemGroupIdentityFilter(systemID, groupID)).Decode(&groupDoc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, group.ErrNotFound
		}
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, fmt.Errorf("find system group: %w", err)
	}

	var relationshipDoc systemGroupRelationshipDocument
	if err := r.systemGroupRelationships.FindOne(ctx, buildSystemGroupRelationshipIdentityFilter(systemID, groupID)).Decode(&relationshipDoc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, group.ErrNotFound
		}
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, fmt.Errorf("find system group relationships: %w", err)
	}

	return groupDoc.toDomain(), relationshipDoc.toDomain(), nil
}

func (r *MongoGroupRepository) UpdateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error) {
	relationships, err := newSystemGroupRelationshipInfoDocuments(projection.Relationships)
	if err != nil {
		return group.SystemGroup{}, err
	}

	session, err := r.client.StartSession()
	if err != nil {
		return group.SystemGroup{}, fmt.Errorf("start system group update session: %w", err)
	}
	defer session.EndSession(ctx)

	if _, err := session.WithTransaction(ctx, func(sessionCtx context.Context) (any, error) {
		groupResult, updateErr := r.systemGroups.UpdateOne(
			sessionCtx,
			buildSystemGroupIdentityFilter(model.SystemID, model.ID),
			bson.M{"$set": bson.M{
				"name":           model.Name,
				"grouping_rules": newSystemGroupDocument(model).GroupingRules,
				"updated_at":     model.UpdatedAt,
			}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("update system group: %w", updateErr)
		}
		if groupResult.MatchedCount == 0 {
			return nil, group.ErrNotFound
		}

		relationshipResult, updateErr := r.systemGroupRelationships.UpdateOne(
			sessionCtx,
			buildSystemGroupRelationshipIdentityFilter(projection.SystemID, projection.GroupID),
			bson.M{"$set": bson.M{
				"relationship": relationships,
				"updated_at":   projection.UpdatedAt,
			}},
		)
		if updateErr != nil {
			return nil, fmt.Errorf("update system group relationships: %w", updateErr)
		}
		if relationshipResult.MatchedCount == 0 {
			return nil, group.ErrNotFound
		}
		return nil, nil
	}); err != nil {
		return group.SystemGroup{}, err
	}
	return model, nil
}
```

Add `errors` to the production imports.

- [ ] **Step 7: Run repository tests**

Run:

```bash
go test ./internal/group-service/repositories -run 'SystemGroup' -count=1
```

Expected: PASS when local MongoDB integration prerequisites are available; if MongoDB is unavailable, unit tests pass and integration tests fail with the existing repository integration setup error.

- [ ] **Step 8: Commit repository changes**

```bash
git add internal/group-service/repositories/mongo_system_group_repository.go internal/group-service/repositories/mongo_system_group_repository_test.go
git commit -m "feat: add system group update repository workflow"
```

### Task 4: Service Diff, Permission Write, And Partial Merge

**Files:**
- Modify: `internal/group-service/services/system_group_service.go`
- Modify: `internal/group-service/services/system_group_relationship_builder.go`
- Test: `internal/group-service/services/system_group_service_test.go`

- [ ] **Step 1: Extend service test fakes**

Modify `fakeSystemGroupRepository` in `internal/group-service/services/system_group_service_test.go`:

```go
type fakeSystemGroupRepository struct {
	createGroup       group.SystemGroup
	createProjection  group.SystemGroupRelationshipProjection
	currentGroup      group.SystemGroup
	currentProjection group.SystemGroupRelationshipProjection
	updateGroup       group.SystemGroup
	updateProjection  group.SystemGroupRelationshipProjection
	listQuery         group.SystemGroupListQuery
	page              group.SystemGroupPage
	err               error
	createCalls       int
	getCalls          int
	updateCalls       int
	listCalls         int
}

func (f *fakeSystemGroupRepository) GetSystemGroupWithRelationships(ctx context.Context, systemID string, groupID string) (group.SystemGroup, group.SystemGroupRelationshipProjection, error) {
	f.getCalls++
	if f.err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, f.err
	}
	if f.currentGroup.ID == "" {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, group.ErrNotFound
	}
	return f.currentGroup, f.currentProjection, nil
}

func (f *fakeSystemGroupRepository) UpdateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error) {
	f.updateCalls++
	f.updateGroup = model
	f.updateProjection = projection
	if f.err != nil {
		return group.SystemGroup{}, f.err
	}
	return model, nil
}
```

Update `fakeSystemGroupPermissionClient.WriteRelationships` so call ordering covers update:

```go
if f.repo != nil && f.repo.createCalls == 0 && f.repo.updateCalls == 0 {
	f.calledBeforeRepository = true
}
```

- [ ] **Step 2: Add service test helpers**

Add these helpers to `internal/group-service/services/system_group_service_test.go`:

```go
func existingServiceSystemGroup() group.SystemGroup {
	return group.SystemGroup{
		ID:       "group-1",
		SystemID: "system-a",
		Name:     "System Admins",
		GroupingRules: []group.SystemGroupRule{{
			AttributeKey: group.GroupAttributeOrganization,
			Operator:     group.OperatorEq,
			Multi:        true,
			Value:        []string{"ORG-100"},
		}},
		CreatedAt: fixedSystemGroupNow().Add(-time.Hour),
		UpdatedAt: fixedSystemGroupNow().Add(-time.Hour),
	}
}

func existingServiceProjection(t *testing.T) group.SystemGroupRelationshipProjection {
	t.Helper()
	projection, err := buildSystemGroupRelationshipProjection(
		"system-a",
		"group-1",
		existingServiceSystemGroup().GroupingRules,
		fixedSystemGroupNow().Add(-time.Hour),
	)
	if err != nil {
		t.Fatalf("build current projection: %v", err)
	}
	return projection
}

func validServiceSystemGroupUpdateInput() group.SystemGroupUpdateInput {
	return group.SystemGroupUpdateInput{
		SystemID: "system-a",
		GroupID:  "group-1",
		Name:     "Updated Admins",
		GroupingRules: []group.SystemGroupRule{
			{AttributeKey: group.GroupAttributeOrganization, Operator: group.OperatorEq, Multi: true, Value: []string{"ORG-100", "ORG-300"}},
			{AttributeKey: group.GroupAttributeJobType, Operator: group.OperatorEq, Multi: false, Value: group.SystemGroupJobTypeIDL},
		},
	}
}
```

- [ ] **Step 3: Write failing service update tests**

Add these tests to `internal/group-service/services/system_group_service_test.go`:

```go
func TestSystemGroupServiceUpdateSystemGroupWritesRelationshipDiff(t *testing.T) {
	repository := &fakeSystemGroupRepository{
		currentGroup:      existingServiceSystemGroup(),
		currentProjection: existingServiceProjection(t),
	}
	permissionClient := &fakeSystemGroupPermissionClient{repo: repository}
	service := NewSystemGroupService(repository,
		WithSystemGroupPermissionClient(permissionClient),
		WithSystemGroupClock(fixedSystemGroupNow),
	)

	model, permissionErrors, err := service.UpdateSystemGroup(context.Background(), validServiceSystemGroupUpdateInput())
	if err != nil {
		t.Fatalf("UpdateSystemGroup error = %v, want nil", err)
	}
	if len(permissionErrors) != 0 {
		t.Fatalf("permission errors = %#v, want empty", permissionErrors)
	}
	if repository.getCalls != 1 || repository.updateCalls != 1 {
		t.Fatalf("repository get/update calls = %d/%d, want 1/1", repository.getCalls, repository.updateCalls)
	}
	if permissionClient.calls != 1 {
		t.Fatalf("permission calls = %d, want 1", permissionClient.calls)
	}
	if !permissionClient.calledBeforeRepository {
		t.Fatal("permission client was not called before repository update")
	}
	if len(permissionClient.parameter.Tasks) == 0 {
		t.Fatal("permission tasks empty, want relationship diff")
	}
	hasCreate := false
	hasDelete := false
	for _, task := range permissionClient.parameter.Tasks {
		hasCreate = hasCreate || task.Operator == permission.RelationshipOperationCreate
		hasDelete = hasDelete || task.Operator == permission.RelationshipOperationDelete
	}
	if !hasCreate || !hasDelete {
		t.Fatalf("tasks = %+v, want both create and delete operations", permissionClient.parameter.Tasks)
	}
	if model.Name != "Updated Admins" || !model.CreatedAt.Equal(existingServiceSystemGroup().CreatedAt) || !model.UpdatedAt.Equal(fixedSystemGroupNow()) {
		t.Fatalf("model = %+v, want requested name, preserved created_at, fixed updated_at", model)
	}
}

func TestSystemGroupServiceUpdateSystemGroupNoRelationshipDiffSkipsPermissionWrite(t *testing.T) {
	current := existingServiceSystemGroup()
	projection := existingServiceProjection(t)
	repository := &fakeSystemGroupRepository{currentGroup: current, currentProjection: projection}
	permissionClient := &fakeSystemGroupPermissionClient{}
	service := NewSystemGroupService(repository,
		WithSystemGroupPermissionClient(permissionClient),
		WithSystemGroupClock(fixedSystemGroupNow),
	)

	input := group.SystemGroupUpdateInput{
		SystemID:      "system-a",
		GroupID:       "group-1",
		Name:          "Renamed Admins",
		GroupingRules: current.GroupingRules,
	}
	model, permissionErrors, err := service.UpdateSystemGroup(context.Background(), input)
	if err != nil {
		t.Fatalf("UpdateSystemGroup error = %v, want nil", err)
	}
	if len(permissionErrors) != 0 {
		t.Fatalf("permission errors = %#v, want empty", permissionErrors)
	}
	if permissionClient.calls != 0 {
		t.Fatalf("permission calls = %d, want 0", permissionClient.calls)
	}
	if repository.updateCalls != 1 || model.Name != "Renamed Admins" {
		t.Fatalf("repository update calls/model = %d/%+v, want rename persisted", repository.updateCalls, model)
	}
}

func TestSystemGroupServiceUpdateSystemGroupNotFoundSkipsPermissionWrite(t *testing.T) {
	repository := &fakeSystemGroupRepository{err: group.ErrNotFound}
	permissionClient := &fakeSystemGroupPermissionClient{}
	service := NewSystemGroupService(repository, WithSystemGroupPermissionClient(permissionClient))

	_, _, err := service.UpdateSystemGroup(context.Background(), validServiceSystemGroupUpdateInput())
	if !errors.Is(err, group.ErrNotFound) {
		t.Fatalf("UpdateSystemGroup error = %v, want ErrNotFound", err)
	}
	if permissionClient.calls != 0 || repository.updateCalls != 0 {
		t.Fatalf("permission/update calls = %d/%d, want 0/0", permissionClient.calls, repository.updateCalls)
	}
}

func TestSystemGroupServiceUpdateSystemGroupPermissionFailureSkipsRepositoryUpdate(t *testing.T) {
	repository := &fakeSystemGroupRepository{
		currentGroup:      existingServiceSystemGroup(),
		currentProjection: existingServiceProjection(t),
	}
	permissionClient := &fakeSystemGroupPermissionClient{err: errors.New("permission unavailable")}
	service := NewSystemGroupService(repository,
		WithSystemGroupPermissionClient(permissionClient),
		WithSystemGroupClock(fixedSystemGroupNow),
	)

	_, _, err := service.UpdateSystemGroup(context.Background(), validServiceSystemGroupUpdateInput())
	if !errors.Is(err, ErrSystemGroupPermissionWriteFailed) {
		t.Fatalf("UpdateSystemGroup error = %v, want ErrSystemGroupPermissionWriteFailed", err)
	}
	if repository.updateCalls != 0 {
		t.Fatalf("repository update calls = %d, want 0", repository.updateCalls)
	}
}
```

- [ ] **Step 4: Write failing partial merge service test**

Add this test:

```go
func TestSystemGroupServiceUpdateSystemGroupPartialFailureMergesFinalProjection(t *testing.T) {
	repository := &fakeSystemGroupRepository{
		currentGroup:      existingServiceSystemGroup(),
		currentProjection: existingServiceProjection(t),
	}
	permissionClient := &fakeSystemGroupPermissionClient{
		resultFunc: func(parameter permission.WriteRelationshipsParameter) permission.WriteRelationshipsResult {
			var failedCreate permission.RelationshipTask
			var failedDelete permission.RelationshipTask
			for _, task := range parameter.Tasks {
				if task.Operator == permission.RelationshipOperationCreate && failedCreate.Relationship.Relation == "" {
					failedCreate = task
				}
				if task.Operator == permission.RelationshipOperationDelete && failedDelete.Relationship.Relation == "" {
					failedDelete = task
				}
			}
			return permission.WriteRelationshipsResult{
				FailedTasks: []permission.FailedRelationshipTask{
					{RelationshipTask: failedCreate, Error: "create rejected"},
					{RelationshipTask: failedDelete, Error: "delete rejected"},
				},
			}
		},
	}
	var logBuffer bytes.Buffer
	service := NewSystemGroupService(repository,
		WithSystemGroupPermissionClient(permissionClient),
		WithSystemGroupClock(fixedSystemGroupNow),
		WithSystemGroupLogger(slog.New(slog.NewTextHandler(&logBuffer, nil))),
	)

	model, permissionErrors, err := service.UpdateSystemGroup(context.Background(), validServiceSystemGroupUpdateInput())
	if err != nil {
		t.Fatalf("UpdateSystemGroup error = %v, want nil", err)
	}
	if len(permissionErrors) != 2 {
		t.Fatalf("permission errors = %#v, want two errors", permissionErrors)
	}
	if repository.updateCalls != 1 {
		t.Fatalf("repository update calls = %d, want 1", repository.updateCalls)
	}
	if len(repository.updateProjection.Relationships) == 0 {
		t.Fatal("saved relationships empty, want failed delete relationship retained")
	}
	orgValues := systemGroupRuleValues(model.GroupingRules, group.GroupAttributeOrganization)
	if len(orgValues) == 0 || orgValues[0] != "ORG-100" {
		t.Fatalf("organization values = %#v, want retained current organization", orgValues)
	}
	if model.Name != "Updated Admins" {
		t.Fatalf("name = %q, want requested name preserved", model.Name)
	}
	output := logBuffer.String()
	if !strings.Contains(output, "permission API relationship update partially failed") {
		t.Fatalf("log output = %q, want update partial failure warning", output)
	}
	if !strings.Contains(output, "failed_task_count=2") {
		t.Fatalf("log output = %q, want failed_task_count=2", output)
	}
}
```

- [ ] **Step 5: Run failing service tests**

Run:

```bash
go test ./internal/group-service/services -run 'TestSystemGroupServiceUpdateSystemGroup' -count=1
```

Expected: FAIL with undefined `UpdateSystemGroup` and repository interface methods.

- [ ] **Step 6: Extend service repository interface**

Modify `internal/group-service/services/system_group_service.go`:

```go
type SystemGroupRepository interface {
	CreateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error)
	GetSystemGroupWithRelationships(ctx context.Context, systemID string, groupID string) (group.SystemGroup, group.SystemGroupRelationshipProjection, error)
	UpdateSystemGroup(ctx context.Context, model group.SystemGroup, projection group.SystemGroupRelationshipProjection) (group.SystemGroup, error)
	ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error)
}
```

- [ ] **Step 7: Add relationship update helpers**

Modify `internal/group-service/services/system_group_relationship_builder.go`:

```go
func newSystemGroupRelationshipUpdateTasks(current group.SystemGroupRelationshipProjection, desired group.SystemGroupRelationshipProjection) ([]permission.RelationshipTask, error) {
	currentByChecksum := relationshipInfoByChecksum(current.Relationships)
	desiredByChecksum := relationshipInfoByChecksum(desired.Relationships)

	tasks := make([]permission.RelationshipTask, 0)
	for _, info := range desired.Relationships {
		if _, exists := currentByChecksum[info.Checksum]; exists {
			continue
		}
		relationship, err := permissionRelationshipValue(info.Relationship)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, permission.RelationshipTask{
			Operator:     permission.RelationshipOperationCreate,
			Relationship: relationship,
		})
	}
	for _, info := range current.Relationships {
		if _, exists := desiredByChecksum[info.Checksum]; exists {
			continue
		}
		relationship, err := permissionRelationshipValue(info.Relationship)
		if err != nil {
			return nil, err
		}
		tasks = append(tasks, permission.RelationshipTask{
			Operator:     permission.RelationshipOperationDelete,
			Relationship: relationship,
		})
	}
	return tasks, nil
}

func applyFailedSystemGroupRelationshipUpdateTasks(current group.SystemGroupRelationshipProjection, desired group.SystemGroupRelationshipProjection, failedTasks []permission.FailedRelationshipTask) (group.SystemGroupRelationshipProjection, []string, error) {
	failedCreates := map[string]struct{}{}
	failedDeletes := map[string]struct{}{}
	permissionErrors := make([]string, 0, len(failedTasks))
	for _, task := range failedTasks {
		checksum, err := relationshipChecksum(task.Relationship)
		if err != nil {
			return group.SystemGroupRelationshipProjection{}, nil, err
		}
		switch task.Operator {
		case permission.RelationshipOperationCreate:
			failedCreates[checksum] = struct{}{}
		case permission.RelationshipOperationDelete:
			failedDeletes[checksum] = struct{}{}
		}
		permissionErrors = append(permissionErrors, task.Error)
	}

	desiredByChecksum := relationshipInfoByChecksum(desired.Relationships)
	finalRelationships := make([]group.RelationshipInfo, 0, len(desired.Relationships)+len(failedDeletes))
	for _, info := range desired.Relationships {
		if _, failed := failedCreates[info.Checksum]; failed {
			continue
		}
		finalRelationships = append(finalRelationships, info)
	}
	for _, info := range current.Relationships {
		if _, stillDesired := desiredByChecksum[info.Checksum]; stillDesired {
			continue
		}
		if _, failed := failedDeletes[info.Checksum]; failed {
			finalRelationships = append(finalRelationships, info)
		}
	}

	finalProjection := desired
	finalProjection.Relationships = finalRelationships
	return finalProjection, permissionErrors, nil
}

func relationshipInfoByChecksum(infos []group.RelationshipInfo) map[string]group.RelationshipInfo {
	out := make(map[string]group.RelationshipInfo, len(infos))
	for _, info := range infos {
		out[info.Checksum] = info
	}
	return out
}
```

Add the permission import to `system_group_relationship_builder.go`:

```go
permission "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission"
```

- [ ] **Step 8: Add update service method**

Modify `internal/group-service/services/system_group_service.go`:

```go
func (s *SystemGroupService) UpdateSystemGroup(ctx context.Context, input group.SystemGroupUpdateInput) (group.SystemGroup, []string, error) {
	now := s.now().UTC()
	input = input.Normalize()
	if err := input.Validate(); err != nil {
		return group.SystemGroup{}, nil, err
	}

	current, currentProjection, err := s.repository.GetSystemGroupWithRelationships(ctx, input.SystemID, input.GroupID)
	if err != nil {
		return group.SystemGroup{}, nil, fmt.Errorf("get system group for update: %w", err)
	}

	model := group.SystemGroup{
		ID:            current.ID,
		SystemID:      current.SystemID,
		Name:          input.Name,
		GroupingRules: input.GroupingRules,
		CreatedAt:     current.CreatedAt,
		UpdatedAt:     now,
	}
	desiredProjection, err := buildSystemGroupRelationshipProjection(model.SystemID, model.ID, model.GroupingRules, now)
	if err != nil {
		return group.SystemGroup{}, nil, err
	}
	desiredProjection.CreatedAt = currentProjection.CreatedAt
	desiredProjection.UpdatedAt = now

	model, desiredProjection, permissionErrors, err := s.writeSystemGroupRelationshipUpdates(ctx, model, currentProjection, desiredProjection)
	if err != nil {
		return group.SystemGroup{}, nil, err
	}

	saved, err := s.repository.UpdateSystemGroup(ctx, model, desiredProjection)
	if err != nil {
		return group.SystemGroup{}, nil, fmt.Errorf("update system group: %w", err)
	}
	return saved, permissionErrors, nil
}

func (s *SystemGroupService) writeSystemGroupRelationshipUpdates(ctx context.Context, model group.SystemGroup, currentProjection group.SystemGroupRelationshipProjection, desiredProjection group.SystemGroupRelationshipProjection) (group.SystemGroup, group.SystemGroupRelationshipProjection, []string, error) {
	tasks, err := newSystemGroupRelationshipUpdateTasks(currentProjection, desiredProjection)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	if len(tasks) == 0 {
		return model, desiredProjection, nil, nil
	}
	if s.permissionClient == nil {
		err := errors.New("permission client is not configured")
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, fmt.Errorf("%w: %w", ErrSystemGroupPermissionWriteFailed, err)
	}
	result, err := s.permissionClient.WriteRelationships(ctx, permission.WriteRelationshipsParameter{Tasks: tasks})
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, fmt.Errorf("%w: %w", ErrSystemGroupPermissionWriteFailed, err)
	}
	if len(result.FailedTasks) == 0 {
		return model, desiredProjection, nil, nil
	}

	finalProjection, permissionErrors, err := applyFailedSystemGroupRelationshipUpdateTasks(currentProjection, desiredProjection, result.FailedTasks)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	adjustedModel, err := rebuildSystemGroupFromRelationshipProjection(model, finalProjection)
	if err != nil {
		return group.SystemGroup{}, group.SystemGroupRelationshipProjection{}, nil, err
	}
	adjustedModel.Name = model.Name
	adjustedModel.CreatedAt = model.CreatedAt
	adjustedModel.UpdatedAt = model.UpdatedAt

	s.logger.WarnContext(ctx, "permission API relationship update partially failed",
		"system_id", model.SystemID,
		"group_id", model.ID,
		"failed_task_count", len(result.FailedTasks),
		"errors", permissionErrors,
	)
	return adjustedModel, finalProjection, permissionErrors, nil
}
```

- [ ] **Step 9: Run service tests**

Run:

```bash
go test ./internal/group-service/services -run 'TestSystemGroupService(Update|Create)|TestBuildSystemGroupRelationshipProjection|TestRelationshipChecksum' -count=1
```

Expected: PASS.

- [ ] **Step 10: Commit service changes**

```bash
git add internal/group-service/services/system_group_service.go internal/group-service/services/system_group_relationship_builder.go internal/group-service/services/system_group_service_test.go
git commit -m "feat: update system group relationship projection"
```

### Task 5: Handler Route And HTTP Status Mapping

**Files:**
- Modify: `internal/group-service/handlers/group_handler.go`
- Test: `internal/group-service/handlers/group_handler_test.go`

- [ ] **Step 1: Write failing handler tests**

Modify `fakeHTTPGroupService` in `internal/group-service/handlers/group_handler_test.go`:

```go
systemGroupUpdateInput group.SystemGroupUpdateInput
systemUpdateCalls      int
```

Add this method:

```go
func (f *fakeHTTPGroupService) UpdateSystemGroup(ctx context.Context, input group.SystemGroupUpdateInput) (group.SystemGroup, []string, error) {
	f.systemUpdateCalls++
	f.systemGroupUpdateInput = input
	if f.err != nil {
		return group.SystemGroup{}, nil, f.err
	}
	return f.systemGroupModel, f.systemGroupErrors, nil
}
```

Add these tests:

```go
func TestGroupHandlerUpdateSystemGroup(t *testing.T) {
	service := &fakeHTTPGroupService{systemGroupModel: systemGroupHandlerModel()}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/systems/system-a/groups/group-1", strings.NewReader(validSystemGroupRequestBody()))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if service.systemUpdateCalls != 1 || service.systemGroupUpdateInput.SystemID != "system-a" || service.systemGroupUpdateInput.GroupID != "group-1" {
		t.Fatalf("service calls/input = %d/%+v, want system update", service.systemUpdateCalls, service.systemGroupUpdateInput)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if _, ok := body["group"]; !ok {
		t.Fatal("response missing group")
	}
}

func TestGroupHandlerUpdateSystemGroupPartialPermissionFailure(t *testing.T) {
	service := &fakeHTTPGroupService{
		systemGroupModel:  systemGroupHandlerModel(),
		systemGroupErrors: []string{"delete rejected"},
	}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/systems/system-a/groups/group-1", strings.NewReader(validSystemGroupRequestBody()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Fatalf("status = %d, want 206", rec.Code)
	}
	var body struct {
		Group  map[string]any `json:"group"`
		Errors []string       `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Group["id"] != "group-1" {
		t.Fatalf("group id = %v, want group-1", body.Group["id"])
	}
	if len(body.Errors) != 1 || body.Errors[0] != "delete rejected" {
		t.Fatalf("errors = %#v, want delete rejected", body.Errors)
	}
}

func TestGroupHandlerUpdateSystemGroupNotFound(t *testing.T) {
	service := &fakeHTTPGroupService{err: group.ErrNotFound}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/systems/system-a/groups/missing", strings.NewReader(validSystemGroupRequestBody()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	var body map[string]map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body["error"]["code"] != "not_found" {
		t.Fatalf("error code = %v, want not_found", body["error"]["code"])
	}
}

func TestGroupHandlerUpdateSystemGroupPermissionWriteFailure(t *testing.T) {
	service := &fakeHTTPGroupService{err: services.ErrSystemGroupPermissionWriteFailed}
	e := echo.New()
	RegisterRoutes(e, NewGroupHandler(service, newTestLogger(), pagination.New()))

	req := httptest.NewRequestWithContext(context.Background(), http.MethodPut, "/api/v1/systems/system-a/groups/group-1", strings.NewReader(validSystemGroupRequestBody()))
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want 502", rec.Code)
	}
}
```

- [ ] **Step 2: Run failing handler tests**

Run:

```bash
go test ./internal/group-service/handlers -run 'TestGroupHandlerUpdateSystemGroup' -count=1
```

Expected: FAIL because the route and interface method are not implemented.

- [ ] **Step 3: Add handler route and interface method**

Modify `internal/group-service/handlers/group_handler.go`:

```go
type HTTPGroupService interface {
	CreateGroup(ctx context.Context, input group.CreateInput) (group.Group, error)
	GetGroup(ctx context.Context, query group.GetQuery) (*group.Group, error)
	DeleteGroup(ctx context.Context, input group.DeleteInput) error
	UpdateGroupingRule(ctx context.Context, input group.UpdateGroupingRuleInput) error
	ListIndividualMembers(ctx context.Context, query group.ListIndividualMembersQuery) (group.IndividualMemberPage, error)
	AddIndividualMembers(ctx context.Context, input group.AddIndividualMembersInput) ([]group.IndividualMember, error)
	UpdateIndividualMemberExpiration(ctx context.Context, input group.UpdateIndividualMemberExpirationInput) error
	DeleteIndividualMember(ctx context.Context, input group.DeleteIndividualMemberInput) error
	CreateSystemGroup(ctx context.Context, input group.SystemGroupCreateInput) (group.SystemGroup, []string, error)
	UpdateSystemGroup(ctx context.Context, input group.SystemGroupUpdateInput) (group.SystemGroup, []string, error)
	ListSystemGroups(ctx context.Context, query group.SystemGroupListQuery) (group.SystemGroupPage, error)
}
```

Register the route:

```go
e.PUT("/api/v1/systems/:system_id/groups/:group_id", handler.UpdateSystemGroup)
```

- [ ] **Step 4: Add update handler method**

Add this method to `internal/group-service/handlers/group_handler.go`:

```go
func (h *GroupHandler) UpdateSystemGroup(c *echo.Context) error {
	systemID := c.Param("system_id")
	groupID := c.Param("group_id")
	request, err := transport.DecodeSystemGroupUpdateRequest(c.Request().Body)
	if err != nil {
		return c.JSON(http.StatusBadRequest, validationError("request body must be valid JSON"))
	}
	input, err := request.ToDomain(systemID, groupID)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		return c.JSON(http.StatusBadRequest, validationError("request body is invalid"))
	}

	model, permissionErrors, err := h.service.UpdateSystemGroup(c.Request().Context(), input)
	if err != nil {
		if errors.Is(err, group.ErrInvalidInput) {
			return c.JSON(http.StatusBadRequest, validationError(err.Error()))
		}
		if errors.Is(err, group.ErrNotFound) {
			return c.JSON(http.StatusNotFound, exception.WrapResponse(exception.New("not_found", "System group not found", exception.WithDetails(map[string]any{}))))
		}
		if errors.Is(err, services.ErrSystemGroupPermissionWriteFailed) {
			h.logger.Warn("failed to update system group permission relationships", "err", err, "system_id", systemID, "group_id", groupID)
			return c.JSON(http.StatusBadGateway, exception.WrapResponse(exception.New("permission_write_failed", "Failed to write permission relationships")))
		}
		h.logger.Warn("failed to update system group", "err", err, "system_id", systemID, "group_id", groupID)
		return c.JSON(http.StatusInternalServerError, exception.WrapResponse(exception.New("internal_error", "Internal server error")))
	}
	if len(permissionErrors) > 0 {
		return c.JSON(http.StatusPartialContent, transport.NewSystemGroupUpdatePartialResponse(model, permissionErrors))
	}
	return c.JSON(http.StatusOK, transport.NewSystemGroupUpdateResponse(model))
}
```

- [ ] **Step 5: Run handler tests**

Run:

```bash
go test ./internal/group-service/handlers -run 'TestGroupHandler(UpdateSystemGroup|CreateSystemGroup|ListSystemGroups)' -count=1
```

Expected: PASS.

- [ ] **Step 6: Commit handler changes**

```bash
git add internal/group-service/handlers/group_handler.go internal/group-service/handlers/group_handler_test.go
git commit -m "feat: expose system group update endpoint"
```

### Task 6: REST Examples And Full Verification

**Files:**
- Modify: `examples/api/system-groups.http`
- Verify: `internal/domain/group`, `internal/group-service/...`, `cmd/group-service`, full repository

- [ ] **Step 1: Add REST Client examples**

Append these examples to `examples/api/system-groups.http` after the create examples:

```http
@groupId = replace-with-system-group-id

### Update system group
PUT {{baseUrl}}/api/v1/systems/{{systemId}}/groups/{{groupId}}
Content-Type: application/json

{
  "name": "System Admins Updated",
  "grouping_rules": [
    {
      "attribute_key": "organization",
      "operator": "eq",
      "multi": true,
      "value": ["ORG-100", "ORG-300"]
    },
    {
      "attribute_key": "job_type",
      "operator": "eq",
      "multi": false,
      "value": "IDL"
    }
  ]
}

### Update system group may return 206 when permission relationship writes partially fail
PUT {{baseUrl}}/api/v1/systems/{{systemId}}/groups/{{groupId}}
Content-Type: application/json

{
  "name": "Partial Permission Update",
  "grouping_rules": [
    {
      "attribute_key": "organization",
      "operator": "eq",
      "multi": true,
      "value": ["ORG-100", "ORG-400"]
    }
  ]
}

### Update system group not found
PUT {{baseUrl}}/api/v1/systems/{{systemId}}/groups/missing-system-group
Content-Type: application/json

{
  "name": "Missing System Group",
  "grouping_rules": []
}
```

- [ ] **Step 2: Run focused verification**

Run:

```bash
go test ./internal/domain/group ./internal/group-service/transport ./internal/group-service/services ./internal/group-service/handlers -count=1
```

Expected: PASS.

- [ ] **Step 3: Run repository verification**

Run:

```bash
go test ./internal/group-service/repositories -run 'SystemGroup' -count=1
```

Expected: PASS when the local MongoDB integration test dependency is available. If it fails because MongoDB is unavailable, record the exact failure and run the unit-only package tests that do not require MongoDB.

- [ ] **Step 4: Run broad group-service verification**

Run:

```bash
go test ./internal/group-service/... ./cmd/group-service -count=1
```

Expected: PASS.

- [ ] **Step 5: Run full repository verification**

Run:

```bash
go test ./... -count=1
```

Expected: PASS. If integration infrastructure is missing, record the failing package, command output summary, and residual risk.

- [ ] **Step 6: Commit examples**

```bash
git add examples/api/system-groups.http
git commit -m "docs: add system group update API examples"
```

## Final Review Checklist

- [ ] `PUT /api/v1/systems/:system_id/groups/:group_id` is registered.
- [ ] `SystemGroupUpdateRequest` reuses `SystemGroupRuleRequest`.
- [ ] Update validates `system_id`, `group_id`, `name`, and `grouping_rules`.
- [ ] Service reads current group and relationship projection before permission writes.
- [ ] Missing current projection returns `group.ErrNotFound` and maps to `404 not_found`.
- [ ] Checksum diff creates one mixed permission write request containing `create` and `delete` tasks.
- [ ] No relationship diff skips the permission API but still updates `name`, `grouping_rules`, and `updated_at`.
- [ ] Permission request-level failure maps to `502 permission_write_failed` and skips MongoDB update.
- [ ] Partial failed creates are excluded from saved projection.
- [ ] Partial failed deletes remain in saved projection.
- [ ] Partial response returns `206` with saved group and `errors`.
- [ ] MongoDB update modifies `system_groups` and `system_group_relationships` in one transaction.
- [ ] `created_at` is preserved and `updated_at` is replaced.
- [ ] `examples/api/system-groups.http` includes success, partial, and not-found update examples.
- [ ] `go test ./... -count=1` result is recorded before claiming implementation complete.
