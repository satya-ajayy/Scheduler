package mongodb

import (
	"context"

	task "scheduler/internal/task"
	timex "scheduler/pkg/timex"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

type SchedulerRepository struct {
	col *mongo.Collection
}

func NewSchedulerRepository(client *Client) *SchedulerRepository {
	return &SchedulerRepository{
		col: client.Database("scheduler").Collection("tasks"),
	}
}

func (r *SchedulerRepository) GetOne(ctx context.Context, taskID string) (task.Task, error) {
	filter := bson.M{"_id": taskID}
	var result task.Task
	err := r.col.FindOne(ctx, filter).Decode(&result)
	return result, err
}

func (r *SchedulerRepository) GetActive(ctx context.Context, curUnix timex.Unix) ([]task.Task, error) {
	filter := bson.M{
		"enable":  true,
		"endUnix": bson.M{"$gte": curUnix},
		"$or": []bson.M{
			{"isRecurEnabled": true},
			{"status.lastExecutedAt": ""},
		},
	}
	cursor, err := r.col.Find(ctx, filter)
	if err != nil {
		return nil, err
	}
	defer cursor.Close(ctx)

	var result []task.Task
	for cursor.Next(ctx) {
		var t task.Task
		if err = cursor.Decode(&t); err != nil {
			return nil, err
		}
		result = append(result, t)
	}
	if err = cursor.Err(); err != nil {
		return nil, err
	}
	return result, nil
}

func (r *SchedulerRepository) Insert(ctx context.Context, task task.Task) error {
	_, err := r.col.InsertOne(ctx, task)
	return err
}

func (r *SchedulerRepository) UpdateEnable(ctx context.Context, taskID string, enable bool) (bool, error) {
	filter := bson.M{"_id": taskID, "enable": !enable}
	currTime := timex.GetCurrentDateTime()
	updatedFields := bson.M{"$set": bson.M{"enable": enable, "updatedAt": currTime}}
	res, err := r.col.UpdateOne(ctx, filter, updatedFields)
	if err != nil {
		return false, err
	}
	return res.MatchedCount > 0, nil
}

func (r *SchedulerRepository) Delete(ctx context.Context, taskID string) error {
	filter := bson.M{"_id": taskID}
	res, err := r.col.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	if res.DeletedCount == 0 {
		return mongo.ErrNoDocuments
	}
	return nil
}

func (r *SchedulerRepository) UpdateTaskStatus(ctx context.Context, taskID, exceptionMsg string, isComplete bool) error {
	filter := bson.M{"_id": taskID}
	updateData := bson.M{
		"$set": bson.M{
			"status.lastExecutedAt":   timex.GetCurrentDateTime(),
			"status.isComplete":       isComplete,
			"status.exceptionMessage": exceptionMsg,
		},
	}
	_, err := r.col.UpdateOne(ctx, filter, updateData)
	return err
}
