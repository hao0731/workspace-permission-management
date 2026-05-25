package permission

import (
	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	permissionrelationship "github.com/hao0731/workspace-permission-management/internal/shared/interactions/permission/relationship"
)

const dynamicContextCondition = "enable_dynamic_context"

type RelationshipOperation string

const (
	RelationshipOperationCreate RelationshipOperation = "create"
	RelationshipOperationDelete RelationshipOperation = "delete"
)

type RegisterResourceAttributesRelationRequest struct {
	ResourceAttribute resource.ResourceAttribute `json:"resAttr"`
	Condition         string                     `json:"condition"`
	IsPublic          bool                       `json:"isPublic"`
}

type RegisterResourceAttributesRequest struct {
	Definition string                                      `json:"definition"`
	Relations  []RegisterResourceAttributesRelationRequest `json:"relations"`
}

func newRegisterResourceAttributesRequest(systemID string, resourceAttributes []resource.ResourceAttribute) RegisterResourceAttributesRequest {
	relations := make([]RegisterResourceAttributesRelationRequest, 0, len(resourceAttributes))
	for _, attribute := range resourceAttributes {
		relations = append(relations, RegisterResourceAttributesRelationRequest{
			ResourceAttribute: attribute,
			Condition:         dynamicContextCondition,
			IsPublic:          false,
		})
	}
	return RegisterResourceAttributesRequest{
		Definition: systemID,
		Relations:  relations,
	}
}

type RelationshipTask struct {
	Operator     RelationshipOperation
	Relationship permissionrelationship.Relationship
}

type WriteRelationshipsParameter struct {
	Tasks []RelationshipTask
}

type FailedRelationshipTask struct {
	RelationshipTask
	Error string
}

type WriteRelationshipsResult struct {
	SuccessTasks []RelationshipTask
	FailedTasks  []FailedRelationshipTask
}

type RelationshipUpdateTask struct {
	Operation    RelationshipOperation               `json:"operation"`
	Relationship permissionrelationship.Relationship `json:"relationship"`
}

type WriteRelationshipsRequest struct {
	Updates []RelationshipUpdateTask `json:"updates"`
}

type UpdatedRelationshipTask struct {
	Error        string                              `json:"error"`
	Relationship permissionrelationship.Relationship `json:"relationship"`
	Success      bool                                `json:"success"`
}

type WriteRelationshipsResponse struct {
	Deletes []UpdatedRelationshipTask `json:"deletes"`
	Writes  []UpdatedRelationshipTask `json:"writes"`
}

func newWriteRelationshipsRequest(parameter WriteRelationshipsParameter) WriteRelationshipsRequest {
	updates := make([]RelationshipUpdateTask, 0, len(parameter.Tasks))
	for _, task := range parameter.Tasks {
		updates = append(updates, RelationshipUpdateTask{
			Operation:    task.Operator,
			Relationship: task.Relationship,
		})
	}
	return WriteRelationshipsRequest{Updates: updates}
}

func newWriteRelationshipsResult(response WriteRelationshipsResponse) WriteRelationshipsResult {
	writeSuccess, writeFailed := relationshipUpdateTasksToResult(RelationshipOperationCreate, response.Writes)
	deleteSuccess, deleteFailed := relationshipUpdateTasksToResult(RelationshipOperationDelete, response.Deletes)

	successTasks := make([]RelationshipTask, 0, len(writeSuccess)+len(deleteSuccess))
	successTasks = append(successTasks, writeSuccess...)
	successTasks = append(successTasks, deleteSuccess...)

	failedTasks := make([]FailedRelationshipTask, 0, len(writeFailed)+len(deleteFailed))
	failedTasks = append(failedTasks, writeFailed...)
	failedTasks = append(failedTasks, deleteFailed...)

	return WriteRelationshipsResult{
		SuccessTasks: successTasks,
		FailedTasks:  failedTasks,
	}
}

func relationshipUpdateTasksToResult(operation RelationshipOperation, tasks []UpdatedRelationshipTask) ([]RelationshipTask, []FailedRelationshipTask) {
	successTasks := make([]RelationshipTask, 0, len(tasks))
	failedTasks := make([]FailedRelationshipTask, 0)
	for _, task := range tasks {
		resultTask := RelationshipTask{
			Operator:     operation,
			Relationship: task.Relationship,
		}
		if task.Success {
			successTasks = append(successTasks, resultTask)
			continue
		}
		failedTasks = append(failedTasks, FailedRelationshipTask{
			RelationshipTask: resultTask,
			Error:            task.Error,
		})
	}
	return successTasks, failedTasks
}
