package repository

import (
	"strings"
	"testing"

	"github.com/prokraft/redbus/internal/app/model"
)

func TestRepeatFieldsMatchScanDestinations(t *testing.T) {
	fields := strings.Split(repeatFields, ",")
	destinations := repeatScanDest(&model.Repeat{})

	if len(fields) != len(destinations) {
		t.Fatalf("repeat field count must match scan destinations: fields=%d destinations=%d", len(fields), len(destinations))
	}
}
