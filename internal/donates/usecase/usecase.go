package usecase

import (
	"context"
	"tempproj/internal/donates"
	"tempproj/internal/donates/storage"
	"tempproj/internal/events"
	"tempproj/pkg/error/svcerror"
	"tempproj/pkg/event"
	"tempproj/pkg/messagequeue"
	redismq "tempproj/pkg/messagequeue/redis"
	"tempproj/pkg/notification"
	"tempproj/pkg/payment"

	"github.com/go-redis/redis"
	"github.com/sirupsen/logrus"
)

const minDonateValue = 50 * 100

type useCaseImpl struct {
	log           *logrus.Entry
	storage       storage.Storage
	payments      payment.Payments
	events        events.UseCase
	notifications notification.UseCase
	mq            messagequeue.MessageQueue
}

func (u *useCaseImpl) MakeDonate(ctx context.Context, donate *donates.Donate) error {
	switch {
	case ctx == nil:
		return svcerror.ErrInternal("ctx is empty")
	case donate == nil:
		return svcerror.ErrInvalidParams("donate is empty")
	case donate.To == "":
		return svcerror.ErrInvalidParams("author is empty")
	case donate.Amount < minDonateValue:
		return svcerror.ErrInvalidParams("amount is less than minimum available value")
	}
	// Create new donate with "new" status
	err := u.storage.Create(ctx, donate)
	if err != nil {
		return svcerror.HandleError(err, "can't create new donate: %s", err)
	}
	// Create new payment
	newPayment := payment.NewPayment(donate.ID, donate.From, donate.Amount)
	paymentEvent, err := payment.PackPaymentEvent(newPayment)
	if err != nil {
		return svcerror.ErrInternal("can't pack donate event: %s", err)
	}
	err = u.mq.Pub(messagequeue.PAYMENT_TO, paymentEvent)
	if err != nil {
		return svcerror.ErrInternal("can't send donate event to mq: %s", err)
	}
	return nil
}

// Update donate status from payment status (payment.OrderID == donate.ID)
func (u *useCaseImpl) UpdateDonate(ctx context.Context, donateID string, update map[string]interface{}) (*donates.Donate, error) {
	switch {
	case ctx == nil:
		return nil, svcerror.ErrInternal("ctx is empty")
	case update == nil:
		return nil, svcerror.ErrInternal("update is empty")
	}
	donate, err := u.storage.Update(ctx, donateID, update)
	if err != nil {
		return nil, svcerror.HandleError(err, "can't update donate info: %s", err)
	}
	return donate, nil
}

func (u *useCaseImpl) GetDonatesNumber(ctx context.Context, userID string) (int64, error) {
	switch {
	case ctx == nil:
		return 0, svcerror.ErrInternal("ctx is empty")
	case userID == "":
		return 0, svcerror.ErrInvalidParams("userID is empty")
	}
	num, err := u.storage.GetNumber(ctx, userID)
	if err != nil {
		return 0, svcerror.HandleError(err, "can't get number of donates: %s", err)
	}
	return num, nil
}

// Return list of uniq userIDs that make donation to the user
func (u *useCaseImpl) GetUserDonators(ctx context.Context, user string) ([]string, error) {
	switch {
	case ctx == nil:
		return nil, svcerror.ErrInternal("ctx is empty")
	case user == "":
		return nil, svcerror.ErrInvalidParams("user is empty")
	}
	users, err := u.storage.GetDonators(ctx, "from", map[string]interface{}{"to": user})
	if err != nil {
		return nil, svcerror.HandleError(err, "can't get donators: %s", err)
	}
	return users, nil
}

// Return list of uniq userIDs that make donation to the post
func (u *useCaseImpl) GetPostDonators(ctx context.Context, post string) ([]string, error) {
	switch {
	case ctx == nil:
		return nil, svcerror.ErrInternal("ctx is empty")
	case post == "":
		return nil, svcerror.ErrInvalidParams("post is empty")
	}
	users, err := u.storage.GetDonators(ctx, "from", map[string]interface{}{"post": post})
	if err != nil {
		return nil, svcerror.HandleError(err, "can't get donators: %s", err)
	}
	return users, nil
}

// Return list of uniq userIDs to whom user donated
func (u *useCaseImpl) GetUsersReceivedDonations(ctx context.Context, user string) ([]string, error) {
	switch {
	case ctx == nil:
		return nil, svcerror.ErrInternal("ctx is empty")
	case user == "":
		return nil, svcerror.ErrInvalidParams("user is empty")
	}
	users, err := u.storage.GetDonators(ctx, "to", map[string]interface{}{"from": user})
	if err != nil {
		return nil, svcerror.HandleError(err, "can't get donators: %s", err)
	}
	return users, nil
}

func (u *useCaseImpl) GetAmountOfDonations(ctx context.Context, user string) (int64, error) {
	switch {
	case ctx == nil:
		return 0, svcerror.ErrInternal("ctx is empty")
	case user == "":
		return 0, svcerror.ErrInvalidParams("user is emtpy")
	}
	sumDonates, err := u.storage.GetDonatesSum(ctx, user)
	if err != nil {
		return 0, svcerror.HandleError(err, "can't get sum of donates: %s", err)
	}
	return sumDonates, nil
}

func (u *useCaseImpl) GetDonatesByIDs(ctx context.Context, ids []string) ([]donates.Short, error) {
	switch {
	case ctx == nil:
		return nil, svcerror.ErrInternal("ctx is empty")
	case ids == nil:
		return nil, svcerror.ErrInvalidParams("ids is empty")
	case len(ids) == 0:
		return []donates.Short{}, nil
	}
	allDonates, err := u.storage.GetByIDs(ctx, ids)
	if err != nil {
		return nil, svcerror.HandleError(err, "can't get donates by ids: %s", err)
	}
	result := make([]donates.Short, 0, len(allDonates))
	for _, donate := range allDonates {
		result = append(result, *donate.Short())
	}
	return result, nil
}

func (u *useCaseImpl) handler() {
	ctx := context.TODO()
	events, err := u.mq.Sub(ctx, messagequeue.PAYMENT_FROM)
	if err != nil {
		u.log.Fatalf("can't subscribe on payment events mq: %s", err)
	}
	for {
		select {
		case evt := <-events:
			paymentUpdate, err := payment.UnpackPaymentEvent(evt)
			if err != nil {
				u.log.Printf("can't unpack payment update: %s", err)
				continue
			}
			donate, err := u.UpdateDonate(
				ctx,
				paymentUpdate.OrderID,
				map[string]interface{}{
					"status": paymentUpdate.Status,
				})
			if err != nil {
				u.log.Printf("can't update donate: %s", err)
				continue
			}
			payload := map[string]interface{}{
				"id": donate.ID,
			}
			switch paymentUpdate.Status {
			case payment.Processing:
				// send url with payment form
				u.log.Printf("send url to user: %s", donate.From)
				payload["status"] = "processing"
				payload["url"] = paymentUpdate.Url

			case payment.Confirmed:
				// send donate to events service
				err := u.events.DonateUser(ctx, donate.From, donate.To, donate.Short())
				if err != nil {
					u.log.Printf("can't save confirmed donate event: %s", err)
				}
				payload["status"] = "confirmed"

			case payment.Failed:
				// send failed status to user
				payload["status"] = "failed"
			default:
				u.log.Printf("Unhandled payment status: %d", paymentUpdate.Status)
				continue
			}
			err = u.notifications.Notify(ctx, event.PaymentUpdate, payload, donate.From)
			if err != nil {
				u.log.Printf("can't send notification to user: %s", err)
			}
		}
	}
}

func New(
	log *logrus.Entry,
	storage storage.Storage,
	redis *redis.Client,
	payments payment.Payments,
	events events.UseCase,
	notifications notification.UseCase,
) (
	donates.UseCase,
	error,
) {
	switch {
	case log == nil:
		return nil, svcerror.ErrInternal("logger is empty")
	case storage == nil:
		return nil, svcerror.ErrInternal("storage is empty")
	case redis == nil:
		return nil, svcerror.ErrInternal("redis is empty")
	case payments == nil:
		return nil, svcerror.ErrInternal("payments is empty")
	case events == nil:
		return nil, svcerror.ErrInternal("events is empty")
	case notifications == nil:
		return nil, svcerror.ErrInternal("notifications is empty")
	}
	mq, err := redismq.New(redis, 1024)
	if err != nil {
		return nil, svcerror.ErrInternal("can't create message queue: %s", err)
	}
	s := &useCaseImpl{
		log:           log,
		storage:       storage,
		payments:      payments,
		events:        events,
		notifications: notifications,
		mq:            mq,
	}
	go s.handler()
	return s, nil
}
