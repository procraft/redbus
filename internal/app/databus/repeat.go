package databus

import "context"

func (b *DataBus) Repeat(ctx context.Context) error {
	return b.repeater.Repeat(ctx)
}
