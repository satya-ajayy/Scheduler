package mongodb

import (
	// Go Internal Packages
	"context"

	// Local Packages
	smodels "scheduler/models"
	helpers "scheduler/utils/helpers"

	// External Packages
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type SchedulerRepository struct {
	client     *mongo.Client
	collection string
}

func NewSchedulerRepository(client *mongo.Client) *SchedulerRepository {
	return &SchedulerRepository{client: client, collection: "scheduling_tasks"}
}

func (r *SchedulerRepository) GetOne(ctx context.Context, taskID string) (smodels.TaskModel, error) {
	collection := r.client.Database("mybase").Collection(r.collection)
	filter := bson.M{"_id": taskID}
	var result smodels.TaskModel
	err := collection.FindOne(ctx, filter).Decode(&result)
	return result, err
}

func (r *SchedulerRepository) GetActive(ctx context.Context, curUnix helpers.Unix) ([]smodels.TaskModel, error) {
	collection := r.client.Database("mybase").Collection(r.collection)
	filter := bson.M{
		"enable":  true,
		"endUnix": bson.M{"$gte": curUnix},
		"$or": []bson.M{
			{"isRecurEnabled": true},
			{"status.lastExecutedAt": ""},
		},
	}
	cursor, err := collection.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	var result []smodels.TaskModel
	for cursor.Next(ctx) {
		var task smodels.TaskModel
		if err = cursor.Decode(&task); err != nil {
			return nil, err
		}
		result = append(result, task)
	}
	return result, nil
}

func (r *SchedulerRepository) Insert(ctx context.Context, task smodels.TaskModel) error {
	collection := r.client.Database("mybase").Collection(r.collection)
	_, err := collection.InsertOne(ctx, task)
	if err != nil {
		return err
	}
	return nil
}

func (r *SchedulerRepository) UpdateEnable(ctx context.Context, taskID string, enable bool) error {
	collection := r.client.Database("mybase").Collection(r.collection)
	filter := bson.M{"_id": taskID}
	currTime := helpers.GetCurrentDateTime()
	updatedFields := bson.M{"$set": bson.M{"enable": enable, "updatedAt": currTime}}
	_, err := collection.UpdateOne(ctx, filter, updatedFields)
	if err != nil {
		return err
	}
	return nil
}

func (r *SchedulerRepository) Delete(ctx context.Context, taskID string) error {
	collection := r.client.Database("mybase").Collection(r.collection)
	filter := bson.M{"_id": taskID}
	res, err := collection.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *SchedulerRepository) UpdateTaskStatus(ctx context.Context, taskID, exceptionMsg string, isComplete bool) error {
	collection := r.client.Database("mybase").Collection(r.collection)
	filter := bson.M{"_id": taskID}
	updateData := bson.M{
		"$set": bson.M{
			"status.lastExecutedAt":   helpers.GetCurrentDateTime(),
			"status.isComplete":       isComplete,
			"status.exceptionMessage": exceptionMsg,
		},
	}
	_, err := collection.UpdateOne(ctx, filter, updateData)
	return err
}
