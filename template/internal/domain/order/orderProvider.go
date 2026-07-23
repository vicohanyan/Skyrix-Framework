package order

import (
	"gitlab.com/skyrix-lib/framework/logging"
	"gitlab.com/skyrix-lib/template/internal/config"
)

type Order struct {
	OrderRepository string
	OrderService    string
	// Add other Handlers here
}

func ProvideOrder(
	logger logger.Interface,
	cfg *config.Config,
) *Order {
	return &Order{
		OrderRepository: "",
		OrderService:    "",
		// Add other Handlers here
	}
}
