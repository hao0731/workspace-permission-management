package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hao0731/workspace-permission-management/internal/domain/resource"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
)

const (
	systemResourcesCollectionName          = "system_resources"
	systemResourceAttributesCollectionName = "system_resource_attributes"
)

type MongoSystemResourceRepository struct {
	client               *mongo.Client
	resourcesCollection  *mongo.Collection
	attributesCollection *mongo.Collection
}

type systemResourceDocument struct {
	ID          string                          `bson:"_id"`
	SystemID    string                          `bson:"system_id"`
	Type        resource.ResourceDefinitionType `bson:"type"`
	Label       string                          `bson:"label"`
	Key         string                          `bson:"key"`
	Description string                          `bson:"description,omitempty"`
	CreatedAt   time.Time                       `bson:"created_at"`
	UpdatedAt   time.Time                       `bson:"updated_at"`
}

type systemResourceAttributesDocument struct {
	ID                 string    `bson:"_id"`
	SystemID           string    `bson:"system_id"`
	ResourceAttributes []string  `bson:"resource_attributes"`
	CreatedAt          time.Time `bson:"created_at"`
	UpdatedAt          time.Time `bson:"updated_at"`
}

func NewMongoSystemResourceRepository(db *mongo.Database) *MongoSystemResourceRepository {
	return &MongoSystemResourceRepository{
		client:               db.Client(),
		resourcesCollection:  db.Collection(systemResourcesCollectionName),
		attributesCollection: db.Collection(systemResourceAttributesCollectionName),
	}
}

func (r *MongoSystemResourceRepository) EnsureIndexes(ctx context.Context) error {
	indexes := systemResourceIndexModels()
	if _, err := r.resourcesCollection.Indexes().CreateOne(ctx, indexes[0]); err != nil {
		return fmt.Errorf("create system_resources index: %w", err)
	}
	if _, err := r.attributesCollection.Indexes().CreateOne(ctx, indexes[1]); err != nil {
		return fmt.Errorf("create system_resource_attributes index: %w", err)
	}
	return nil
}

func (r *MongoSystemResourceRepository) RunInTransaction(ctx context.Context, fn func(context.Context) error) error {
	session, err := r.client.StartSession()
	if err != nil {
		return fmt.Errorf("start mongo session: %w", err)
	}
	defer session.EndSession(ctx)

	_, err = session.WithTransaction(ctx, func(tx context.Context) (any, error) {
		return nil, fn(tx)
	})
	if err != nil {
		return fmt.Errorf("run mongo transaction: %w", err)
	}
	return nil
}

func (r *MongoSystemResourceRepository) ListResourceDefinitions(ctx context.Context, query resource.ResourceDefinitionsQuery) ([]resource.ResourceDefinition, error) {
	cursor, err := r.resourcesCollection.Find(ctx,
		bson.M{"system_id": query.SystemID},
		options.Find().SetSort(bson.D{{Key: "type", Value: 1}, {Key: "key", Value: 1}}),
	)
	if err != nil {
		return nil, fmt.Errorf("find system resources: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var docs []systemResourceDocument
	if err := cursor.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("decode system resources: %w", err)
	}
	definitions := make([]resource.ResourceDefinition, 0, len(docs))
	for _, doc := range docs {
		definitions = append(definitions, doc.toDomain())
	}
	return definitions, nil
}

func (r *MongoSystemResourceRepository) UpsertResourceDefinitions(ctx context.Context, definitions []resource.ResourceDefinition) ([]resource.ResourceDefinition, error) {
	if len(definitions) == 0 {
		return []resource.ResourceDefinition{}, nil
	}
	docs := make([]systemResourceDocument, 0, len(definitions))
	for _, definition := range definitions {
		docs = append(docs, newSystemResourceDocument(definition))
	}
	if _, err := r.resourcesCollection.BulkWrite(ctx, buildSystemResourceBulkWriteModels(docs)); err != nil {
		return nil, fmt.Errorf("bulk upsert system resources: %w", err)
	}

	cursor, err := r.resourcesCollection.Find(ctx, buildSystemResourceReadbackFilter(definitions))
	if err != nil {
		return nil, fmt.Errorf("find upserted system resources: %w", err)
	}
	defer func() {
		_ = cursor.Close(ctx)
	}()

	var persisted []systemResourceDocument
	if decodeErr := cursor.All(ctx, &persisted); decodeErr != nil {
		return nil, fmt.Errorf("decode upserted system resources: %w", decodeErr)
	}
	ordered, err := orderSystemResourceDefinitionsByRequest(definitions, persisted)
	if err != nil {
		return nil, err
	}
	return ordered, nil
}

func (r *MongoSystemResourceRepository) GetResourceAttributes(ctx context.Context, query resource.ResourceAttributesQuery) (resource.ResourceAttributes, bool, error) {
	var doc systemResourceAttributesDocument
	if err := r.attributesCollection.FindOne(ctx, bson.M{"system_id": query.SystemID}).Decode(&doc); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return resource.ResourceAttributes{}, false, nil
		}
		return resource.ResourceAttributes{}, false, fmt.Errorf("find system resource attributes: %w", err)
	}
	return doc.toDomain(), true, nil
}

func (r *MongoSystemResourceRepository) UpsertResourceAttributes(ctx context.Context, attributes resource.ResourceAttributes) (resource.ResourceAttributes, error) {
	doc := newSystemResourceAttributesDocument(attributes)
	filter := bson.M{"system_id": doc.SystemID}
	if _, err := r.attributesCollection.UpdateOne(ctx, filter, buildSystemResourceAttributesUpdate(doc), options.UpdateOne().SetUpsert(true)); err != nil {
		return resource.ResourceAttributes{}, fmt.Errorf("upsert system resource attributes: %w", err)
	}
	var persisted systemResourceAttributesDocument
	if err := r.attributesCollection.FindOne(ctx, filter).Decode(&persisted); err != nil {
		return resource.ResourceAttributes{}, fmt.Errorf("find upserted system resource attributes: %w", err)
	}
	return persisted.toDomain(), nil
}

func systemResourceIndexModels() []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys:    bson.D{{Key: "system_id", Value: 1}, {Key: "type", Value: 1}, {Key: "key", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.D{{Key: "system_id", Value: 1}},
			Options: options.Index().SetUnique(true),
		},
	}
}

func buildSystemResourceFilter(systemID string, kind resource.ResourceDefinitionType, key string) bson.M {
	return bson.M{"system_id": systemID, "type": kind, "key": key}
}

func buildSystemResourceBulkWriteModels(docs []systemResourceDocument) []mongo.WriteModel {
	models := make([]mongo.WriteModel, 0, len(docs))
	for _, doc := range docs {
		models = append(models, mongo.NewUpdateOneModel().
			SetFilter(buildSystemResourceFilter(doc.SystemID, doc.Type, doc.Key)).
			SetUpdate(buildSystemResourceUpdate(doc)).
			SetUpsert(true),
		)
	}
	return models
}

func buildSystemResourceReadbackFilter(definitions []resource.ResourceDefinition) bson.M {
	filters := make(bson.A, 0, len(definitions))
	for _, definition := range definitions {
		filters = append(filters, buildSystemResourceFilter(definition.SystemID, definition.Type, definition.Key))
	}
	return bson.M{"$or": filters}
}

func buildSystemResourceUpdate(doc systemResourceDocument) bson.M {
	set := bson.M{"label": doc.Label, "updated_at": doc.UpdatedAt}
	update := bson.M{
		"$set": set,
		"$setOnInsert": bson.M{
			"_id":        doc.ID,
			"system_id":  doc.SystemID,
			"type":       doc.Type,
			"key":        doc.Key,
			"created_at": doc.CreatedAt,
		},
	}
	if doc.Description == "" {
		update["$unset"] = bson.M{"description": ""}
	} else {
		set["description"] = doc.Description
	}
	return update
}

func orderSystemResourceDefinitionsByRequest(requested []resource.ResourceDefinition, persisted []systemResourceDocument) ([]resource.ResourceDefinition, error) {
	byIdentity := make(map[string]systemResourceDocument, len(persisted))
	for _, doc := range persisted {
		byIdentity[systemResourceDefinitionIdentity(doc.SystemID, doc.Type, doc.Key)] = doc
	}

	ordered := make([]resource.ResourceDefinition, 0, len(requested))
	for _, definition := range requested {
		identity := systemResourceDefinitionIdentity(definition.SystemID, definition.Type, definition.Key)
		doc, ok := byIdentity[identity]
		if !ok {
			return nil, fmt.Errorf("upserted system resource not found: %s/%s/%s", definition.SystemID, definition.Type, definition.Key)
		}
		ordered = append(ordered, doc.toDomain())
	}
	return ordered, nil
}

func systemResourceDefinitionIdentity(systemID string, kind resource.ResourceDefinitionType, key string) string {
	return systemID + "\x00" + string(kind) + "\x00" + key
}

func buildSystemResourceAttributesUpdate(doc systemResourceAttributesDocument) bson.M {
	return bson.M{
		"$set": bson.M{
			"resource_attributes": append([]string(nil), doc.ResourceAttributes...),
			"updated_at":          doc.UpdatedAt,
		},
		"$setOnInsert": bson.M{
			"_id":        doc.ID,
			"system_id":  doc.SystemID,
			"created_at": doc.CreatedAt,
		},
	}
}

func newSystemResourceDocument(definition resource.ResourceDefinition) systemResourceDocument {
	return systemResourceDocument{
		ID:          definition.ID,
		SystemID:    definition.SystemID,
		Type:        definition.Type,
		Label:       definition.Label,
		Key:         definition.Key,
		Description: definition.Description,
		CreatedAt:   definition.CreatedAt,
		UpdatedAt:   definition.UpdatedAt,
	}
}

func (d systemResourceDocument) toDomain() resource.ResourceDefinition {
	return resource.ResourceDefinition{
		ID:          d.ID,
		SystemID:    d.SystemID,
		Type:        d.Type,
		Label:       d.Label,
		Key:         d.Key,
		Description: d.Description,
		CreatedAt:   d.CreatedAt,
		UpdatedAt:   d.UpdatedAt,
	}
}

func newSystemResourceAttributesDocument(attributes resource.ResourceAttributes) systemResourceAttributesDocument {
	values := make([]string, 0, len(attributes.Values))
	for _, value := range attributes.Values {
		values = append(values, string(value))
	}
	return systemResourceAttributesDocument{
		ID:                 attributes.ID,
		SystemID:           attributes.SystemID,
		ResourceAttributes: values,
		CreatedAt:          attributes.CreatedAt,
		UpdatedAt:          attributes.UpdatedAt,
	}
}

func (d systemResourceAttributesDocument) toDomain() resource.ResourceAttributes {
	values := make([]resource.ResourceAttribute, 0, len(d.ResourceAttributes))
	for _, value := range d.ResourceAttributes {
		values = append(values, resource.ResourceAttribute(value))
	}
	return resource.ResourceAttributes{
		ID:        d.ID,
		SystemID:  d.SystemID,
		Values:    values,
		CreatedAt: d.CreatedAt,
		UpdatedAt: d.UpdatedAt,
	}
}
