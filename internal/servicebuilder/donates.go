package servicebuilder

import (
	"tempproj/internal/donates"
	donateStorage "tempproj/internal/donates/storage"
	donateUseCase "tempproj/internal/donates/usecase"
	"tempproj/internal/events"
	"tempproj/pkg/notification"
	"tempproj/pkg/payment"

	// ...

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

func buildDonateService(
	log *logrus.Entry,
	mongo *mongo.Client,
	client *redis.Client,
	payments payment.Payments,
	events events.UseCase,
	notifications notification.UseCase,
) donates.UseCase {
	storage, err := donateStorage.New(log, mongo)
	if err != nil {
		log.Fatalf("failed while creating donates storage: %s", err)
	}
	donates, err := donateUseCase.New(log, storage, client, payments, events, notifications)
	if err != nil {
		log.Fatalf("failed while creating donates service: %s", err)
	}
	return donates
}

func (b *ServiceBuilder) GetDonateService() donates.UseCase {
	return b.donateService
}
