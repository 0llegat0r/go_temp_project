package storage

import (
	"context"
	"tempproj/internal/donates"
	"tempproj/pkg/error/dberror"
	"tempproj/pkg/error/svcerror"

	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Storage interface {
	Create(ctx context.Context, donate *donates.Donate) error
	GetByUser(ctx context.Context, user string) ([]donates.Donate, error)
	GetByIDs(ctx context.Context, ids []string) ([]donates.Donate, error)
	GetNumber(ctx context.Context, user string) (int64, error)
	GetDonators(ctx context.Context, uniq string, filter map[string]interface{}) ([]string, error)
	GetDonatesSum(ctx context.Context, user string) (int64, error)
	Update(ctx context.Context, donateID string, update map[string]interface{}) (*donates.Donate, error)
}

type storageImpl struct {
	log     *logrus.Entry
	donates *mongo.Collection
}

func (s *storageImpl) Create(ctx context.Context, donate *donates.Donate) error {
	result, err := s.donates.InsertOne(ctx, donate)
	if err != nil {
		return dberror.ErrMongoHandle(err, "mongo.InsertOne err: %s", err)
	}
	if result.InsertedID == nil {
		return dberror.ErrInternal("inserted id is empty")
	}
	return nil
}

func (s *storageImpl) GetByUser(ctx context.Context, user string) ([]donates.Donate, error) {
	cursor, err := s.donates.Find(ctx, bson.M{"to": user})
	if err != nil {
		return nil, dberror.ErrMongoHandle(err, "mongo.Find err: %s", err)
	}
	defer cursor.Close(nil)
	result := make([]donates.Donate, 0)
	err = cursor.All(ctx, &result)
	if err != nil {
		return nil, dberror.ErrInternal("can't get donates from cursor: %s", err)
	}
	return result, nil
}

func (s *storageImpl) GetByIDs(ctx context.Context, ids []string) ([]donates.Donate, error) {
	cursor, err := s.donates.Find(ctx, bson.M{"id": bson.M{"$in": ids}})
	if err != nil {
		return nil, dberror.ErrMongoHandle(err, "mongo.Find err: %s", err)
	}
	defer cursor.Close(nil)
	result := make([]donates.Donate, 0)
	err = cursor.All(ctx, &result)
	if err != nil {
		return nil, dberror.ErrInternal("can't get donates from cursor: %s", err)
	}
	return result, nil
}

func (s *storageImpl) GetNumber(ctx context.Context, user string) (int64, error) {
	result, err := s.donates.CountDocuments(ctx, bson.M{"to": user, "status": 2})
	if err != nil {
		return 0, dberror.ErrMongoHandle(err, "mongo.Count err: %s", err)
	}
	return result, nil
}

func (s *storageImpl) GetDonators(ctx context.Context, uniq string, filter map[string]interface{}) ([]string, error) {
	filter["status"] = 2
	donators, err := s.donates.Distinct(ctx, uniq, filter)
	if err != nil {
		return nil, dberror.ErrMongoHandle(err, "mongo.Distinct err: %s", err)
	}
	result := make([]string, 0, len(donators))
	for _, d := range donators {
		result = append(result, d.(string))
	}
	return result, err
}

func (s *storageImpl) GetDonatesSum(ctx context.Context, user string) (int64, error) {
	pipeline := bson.A{
		bson.M{"$match": bson.M{"to": user, "status": 2}},
		bson.M{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$amount"}}},
	}
	cursor, err := s.donates.Aggregate(ctx, pipeline)
	if err != nil {
		return 0, dberror.ErrMongoHandle(err, "mongo.Aggregate err: %s", err)
	}
	defer cursor.Close(nil)
	result := make([]map[string]interface{}, 0, 2)
	err = cursor.All(ctx, &result)
	if err != nil {
		return 0, dberror.ErrInternal("cursor.All err: %s", err)
	}
	var amount int64
	if len(result) == 1 {
		amount = result[0]["total"].(int64)
	}
	return amount, nil
}

func (s *storageImpl) Update(ctx context.Context, donateID string, update map[string]interface{}) (*donates.Donate, error) {
	when := options.After
	opts := &options.FindOneAndUpdateOptions{ReturnDocument: &when}
	donate := &donates.Donate{}
	err := s.donates.FindOneAndUpdate(ctx, bson.M{"id": donateID}, bson.M{"$set": update}, opts).Decode(donate)
	if err != nil {
		return nil, dberror.ErrMongoHandle(err, "mongo.UpdateOne err: %s", err)
	}
	return donate, nil
}

func New(log *logrus.Entry, client *mongo.Client) (Storage, error) {
	switch {
	case log == nil:
		return nil, svcerror.ErrInternal("logger is empty")
	case client == nil:
		return nil, svcerror.ErrInternal("db client is empty")
	}
	return &storageImpl{
		log:     log,
		donates: client.Database("tempproj").Collection("donates"),
	}, nil
}
