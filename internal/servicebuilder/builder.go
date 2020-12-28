package servicebuilder

import (
	"tempproj/internal/donates"
	plconf "tempproj/internal/plapi/config"
	api "tempproj/pkg/apigateway"

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/mongo"
)

type ServiceBuilder struct {
	// ...
	donateService donates.UseCase
	// ...
}

func New(log *logrus.Entry, mongo *mongo.Client, redis *redis.Client, config *plconf.Config, api api.APIGateway) (*ServiceBuilder, error) {
	// ...
	donateService := buildDonateService(log, mongo, redis, paymentService, eventService, notificationService)
	// ....

	return &ServiceBuilder{
		//....
		donateService: donateService,
		// ....
	}, nil
}
