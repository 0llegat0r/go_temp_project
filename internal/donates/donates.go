package donates

import (
	"context"
	"time"

	"github.com/rs/xid"
)

type UseCase interface {
	MakeDonate(ctx context.Context, donate *Donate) error
	GetDonatesNumber(ctx context.Context, userID string) (int64, error)
	GetUserDonators(ctx context.Context, user string) ([]string, error)
	GetPostDonators(ctx context.Context, post string) ([]string, error)
	GetUsersReceivedDonations(ctx context.Context, user string) ([]string, error)
	GetAmountOfDonations(ctx context.Context, user string) (int64, error)
	GetDonatesByIDs(ctx context.Context, ids []string) ([]Short, error)
}

type Status int

const (
	New       Status = iota // newly created donate
	Pending                 // set after payment provider initialized new payout, waiting client's actions
	Confirmed               // set after successful payment
	Failed                  // failed payments, don't show to users
)

type Donate struct {
	ID        string    `bson:"id"`
	From      string    `bson:"from"`
	To        string    `bson:"to"`
	Amount    uint64    `bson:"amount"`
	Status    Status    `bson:"status"`
	Post      string    `bson:"post"`
	CreatedAt time.Time `bson:"created"`
	UpdatedAt time.Time `bson:"updated"`
}

type Short struct {
	ID     string
	Amount uint64
}

func (d *Donate) Short() *Short {
	return &Short{
		ID:     d.ID,
		Amount: d.Amount,
	}
}

func NewDonate(from, to, post string, amount uint64) *Donate {
	now := time.Now()
	return &Donate{
		ID:        xid.New().String(),
		From:      from,
		To:        to,
		Amount:    amount,
		Status:    New,
		Post:      post,
		CreatedAt: now,
		UpdatedAt: now,
	}
}
