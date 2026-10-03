package inbox_test

import (
	"context"
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/procraft/redbus/api/golang/inbox"
	"github.com/procraft/redbus/api/golang/internal/pgtest"
)

func TestKeyMatchesScalaSDK(t *testing.T) {
	require.Equal(t, "group|topic|key", inbox.Key("group", "topic", "key"))
}

func TestGuardWithoutClaimAlwaysRuns(t *testing.T) {
	ran, err := inbox.Guard(context.Background(), nil, nil, func(context.Context) error { return errors.New("x") })
	require.True(t, ran)
	require.EqualError(t, err, "x")
}

func TestClaimAndMarks(t *testing.T) {
	db := pgtest.New(t)
	ctx := context.Background()

	processed, err := inbox.IsProcessed(ctx, db, "g", "t", "k")
	require.NoError(t, err)
	require.False(t, processed)

	claimed, err := inbox.Claim(ctx, db, "g", "t", "k")
	require.NoError(t, err)
	require.True(t, claimed)
	claimed, err = inbox.Claim(ctx, db, "g", "t", "k")
	require.NoError(t, err)
	require.False(t, claimed)

	processed, err = inbox.IsProcessed(ctx, db, "g", "t", "k")
	require.NoError(t, err)
	require.True(t, processed)

	// An existing mark is kept.
	require.NoError(t, inbox.SetProcessed(ctx, db, "g", "t", "k"))
	require.NoError(t, inbox.SetProcessed(ctx, db, "g", "t", "k2"))
	processed, err = inbox.IsProcessed(ctx, db, "g", "t", "k2")
	require.NoError(t, err)
	require.True(t, processed)
}

func TestClaimWithoutCreatedAtDefault(t *testing.T) {
	db := pgtest.New(t)
	_, err := db.Exec(`ALTER TABLE public.redbus_inbox ALTER COLUMN created_at DROP DEFAULT`)
	require.NoError(t, err)

	claimed, err := inbox.Claim(context.Background(), db, "g", "t", "k")
	require.NoError(t, err)
	require.True(t, claimed)
	require.NoError(t, inbox.SetProcessed(context.Background(), db, "g", "t", "k2"))
}

func TestGuardInTransaction(t *testing.T) {
	db := pgtest.New(t)
	ctx := context.Background()
	_, err := db.Exec(`CREATE TABLE business (v int)`)
	require.NoError(t, err)

	run := func() bool {
		tx, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		ran, err := inbox.Guard(ctx, tx, inbox.ClaimFor("g", "t", "k"), func(ctx context.Context) error {
			_, err := tx.ExecContext(ctx, `INSERT INTO business VALUES (1)`)
			return err
		})
		require.NoError(t, err)
		require.NoError(t, tx.Commit())
		return ran
	}
	require.True(t, run())
	require.False(t, run())

	var n int
	require.NoError(t, db.QueryRow(`SELECT count(*) FROM business`).Scan(&n))
	require.Equal(t, 1, n)
}

func TestConcurrentClaimWaitsForFirstTransaction(t *testing.T) {
	db := pgtest.New(t)
	ctx := context.Background()

	for _, commit := range []bool{true, false} {
		key := "commit"
		if !commit {
			key = "rollback"
		}
		first, err := db.BeginTx(ctx, nil)
		require.NoError(t, err)
		claimed, err := inbox.Claim(ctx, first, "g", "t", key)
		require.NoError(t, err)
		require.True(t, claimed)

		second := make(chan bool, 1)
		go func() {
			tx, err := db.BeginTx(ctx, nil)
			if err != nil {
				panic(err)
			}
			defer func() { _ = tx.Rollback() }()
			claimed, err := inbox.Claim(ctx, tx, "g", "t", key)
			if err != nil {
				panic(err)
			}
			second <- claimed
		}()

		select {
		case <-second:
			t.Fatal("a concurrent claim must wait for the first transaction")
		case <-time.After(300 * time.Millisecond):
		}
		if commit {
			require.NoError(t, first.Commit())
			require.False(t, <-second, "the key is taken after commit")
		} else {
			require.NoError(t, first.Rollback())
			require.True(t, <-second, "a rollback releases the key")
		}
	}
}

var _ inbox.DB = (*sql.DB)(nil)
var _ inbox.Execer = (*sql.Tx)(nil)
