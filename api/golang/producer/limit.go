package producer

import (
	"errors"
	"fmt"
)

// DefaultMaxMessageBytes is the default limit of one message payload (256 KiB). Kafka rejects a
// record above its message.max.bytes (about 1 MB by default) and the bus's gRPC server a request
// above 4 MiB, so a larger payload could never be delivered and, in the outbox, would hold back its
// topic.
const DefaultMaxMessageBytes = 256 * 1024

// ErrMessageTooLarge matches every *MessageTooLargeError with errors.Is.
var ErrMessageTooLarge = errors.New("redbus: message is too large")

// MessageTooLargeError reports a payload above the limit. The message was neither sent to the bus
// nor written to the outbox; nothing was truncated. Size is len(message) of the payload passed to
// produce (for a protobuf message, its serialized bytes): exactly what the bus writes to Kafka as
// the record value, without topic, key or headers.
type MessageTooLargeError struct {
	Topic string
	Size  int
	Limit int
}

func (e *MessageTooLargeError) Error() string {
	return fmt.Sprintf("redbus: message to topic %s is %d bytes, above the limit of %d bytes; it was not sent",
		e.Topic, e.Size, e.Limit)
}

func (e *MessageTooLargeError) Is(target error) bool {
	return target == ErrMessageTooLarge
}

// CheckMessageSize returns a *MessageTooLargeError when message is longer than limit; a limit of
// zero or less means DefaultMaxMessageBytes.
func CheckMessageSize(topic string, message []byte, limit int) error {
	if limit <= 0 {
		limit = DefaultMaxMessageBytes
	}
	if len(message) > limit {
		return &MessageTooLargeError{Topic: topic, Size: len(message), Limit: limit}
	}
	return nil
}
