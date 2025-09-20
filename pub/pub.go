package pub

import (
	"github.com/IsaacDSC/broker/broker"
)

type Broker struct {
	*broker.Client
}

func NewBroker(conn *broker.Client) *Broker {
	return &Broker{
		Client: conn,
	}
}
