package payment

import "context"

type Request struct {
	ID             string
	Amount         int64
	Currency       string
	IdempotencyKey string
}

type Payment struct {
	ID       string
	Amount   int64
	Currency string
}

type PaymentProvider interface {
	Charge(context.Context, Request) (Payment, error)
}
