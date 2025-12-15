package order

import (
	"skyrix/internal/config"
	"skyrix/internal/logger"
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
