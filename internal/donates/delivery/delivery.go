package delivery

import (
	"context"
	"encoding/json"
	"tempproj/internal/donates"
	"tempproj/internal/followers"
	"tempproj/internal/types"
	"tempproj/internal/users"
	"tempproj/pkg/error/svcerror"
	"tempproj/pkg/sessioncontext"
	"tempproj/pkg/utils"

	"github.com/sirupsen/logrus"
)

type Delivery interface {
	MakeDonate(ctx context.Context, rawMessage []byte) (interface{}, error)
	GetUserDonators(ctx context.Context, rawMessage []byte) (interface{}, error)
	GetPostDonators(ctx context.Context, rawMessage []byte) (interface{}, error)
	GetDonatedUsers(ctx context.Context, rawMessage []byte) (interface{}, error)
	GetAmountOfDonations(ctx context.Context, rawMessage []byte) (interface{}, error)
}

type websocket struct {
	log       *logrus.Entry
	donates   donates.UseCase
	users     users.UseCase
	followers followers.UseCase
}

type reqMakeDonate struct {
	User   string `json:"user"`
	Post   string `json:"post"`
	Amount uint64 `json:"amount"`
}

func parseMakeDonate(data []byte) (reqMakeDonate, error) {
	var result reqMakeDonate
	err := json.Unmarshal(data, &result)
	return result, err
}

func (w *websocket) MakeDonate(ctx context.Context, rawMessage []byte) (interface{}, error) {
	req, err := parseMakeDonate(rawMessage)
	if err != nil {
		w.log.Errorf("failed while parsing MakeDonate request: %s, error: %s", rawMessage, err)
		return nil, svcerror.ErrMalformed("can't parse data of client request")
	}
	newDonate := donates.NewDonate(sessioncontext.GetUserID(ctx), req.User, req.Post, req.Amount)
	err = w.donates.MakeDonate(ctx, newDonate)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"id": newDonate.ID}, nil
}

type reqGetDonators struct {
	User string `json:"user"`
	Post string `json:"post"`
}

func parseGetDonators(data []byte) (reqGetDonators, error) {
	var result reqGetDonators
	err := json.Unmarshal(data, &result)
	return result, err
}

func (w *websocket) GetUserDonators(ctx context.Context, rawMessage []byte) (interface{}, error) {
	req, err := parseGetDonators(rawMessage)
	if err != nil {
		w.log.Errorf("failed while parsing request: %s, err: %s", rawMessage, err)
		return nil, svcerror.ErrMalformed("can't parse client request")
	}
	donators, err := w.donates.GetUserDonators(ctx, req.User)
	if err != nil {
		return nil, err
	}
	return w.returnUserProfiles(ctx, donators)
}

func (w *websocket) GetPostDonators(ctx context.Context, rawMessage []byte) (interface{}, error) {
	req, err := parseGetDonators(rawMessage)
	if err != nil {
		w.log.Errorf("failed while parsing request: %s, err: %s", rawMessage, err)
		return nil, svcerror.ErrMalformed("can't parse client request")
	}
	donators, err := w.donates.GetPostDonators(ctx, req.Post)
	if err != nil {
		return nil, err
	}
	return w.returnUserProfiles(ctx, donators)
}

func (w *websocket) GetDonatedUsers(ctx context.Context, rawMessage []byte) (interface{}, error) {
	user := sessioncontext.GetUserID(ctx)
	req, err := parseGetDonators(rawMessage)
	if err != nil {
		w.log.Errorf("failed while parsing request: %s, err: %s", rawMessage, err)
		return nil, svcerror.ErrMalformed("can't parse client request")
	}
	if req.User != "" {
		user = req.User
	}
	donators, err := w.donates.GetUsersReceivedDonations(ctx, user)
	if err != nil {
		return nil, err
	}
	return w.returnUserProfiles(ctx, donators)
}

func (w *websocket) returnUserProfiles(ctx context.Context, userIDs []string) (interface{}, error) {
	userID := sessioncontext.GetUserID(ctx)
	profiles, err := w.getUserProfiles(ctx, userIDs, userID)
	if err != nil {
		return nil, svcerror.ErrInternal("can't load user's profiles: %s", err)
	}
	return map[string]interface{}{"users": profiles}, nil
}

func (w *websocket) getUserProfiles(ctx context.Context, userIDs []string, userID string) (interface{}, error) {
	fullUsers, err := w.users.GetByIDs(ctx, userIDs, types.PageOpt{Limit: 0, Offset: 0})
	if err != nil {
		return nil, err
	}
	uniqFollowers := utils.NewUniqMap([]string{})

	if userID != "" {
		// Check followers
		userFollowers, err := w.followers.CheckList(ctx, userID, userIDs)
		if err != nil {
			return nil, err
		}
		uniqFollowers = utils.NewUniqMap(userFollowers)
	}
	shortUsers := make([]users.Base, 0, len(fullUsers))
	for _, u := range fullUsers {
		b := u.Base()
		if uniqFollowers.HasValue(u.ID) {
			b.Subscribed = true
		}
		shortUsers = append(shortUsers, b)
	}
	return shortUsers, nil
}

func (w *websocket) GetAmountOfDonations(ctx context.Context, rawMessage []byte) (interface{}, error) {
	user := sessioncontext.GetUserID(ctx)
	amount, err := w.donates.GetAmountOfDonations(ctx, user)
	if err != nil {
		return nil, err
	}
	return map[string]interface{}{"amount": amount}, nil
}

func New(log *logrus.Entry, donates donates.UseCase, users users.UseCase, followers followers.UseCase) (Delivery, error) {
	switch {
	case log == nil:
		return nil, svcerror.ErrInternal("logger is empty")
	case donates == nil:
		return nil, svcerror.ErrInternal("donate service is empty")
	case users == nil:
		return nil, svcerror.ErrInternal("users service is empty")
	}
	return &websocket{
		log:       log,
		donates:   donates,
		users:     users,
		followers: followers,
	}, nil
}
