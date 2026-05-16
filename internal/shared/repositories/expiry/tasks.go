package expiry

const (
	GroupTaskCollectionName            = "group_expiry_task"
	IndividualMemberTaskCollectionName = "individual_member_expiry_task"

	GroupTaskActiveGroupUniqueIndexName             = "group_expiry_task_active_workspace_group_unique"
	GroupTaskBucketIndexName                        = "group_expiry_task_bucket_id"
	IndividualMemberTaskActiveMemberUniqueIndexName = "individual_member_expiry_task_active_group_account_unique"
	IndividualMemberTaskBucketIndexName             = "individual_member_expiry_task_bucket_id"
)

type GroupTask struct {
	ID               string
	WorkspaceID      string
	GroupID          string
	ExpirationBucket string
}

type IndividualMemberTask struct {
	ID               string
	GroupID          string
	NTAccount        string
	ExpirationBucket string
}

type Cursor struct {
	ExpirationBucket string
	ID               string
}
