package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"github.com/ydb-platform/ydb-go-sdk/v3/table/types"
	"os"
	"time"

	"github.com/pressly/goose/v3"
	environ "github.com/ydb-platform/ydb-go-sdk-auth-environ"
	"github.com/ydb-platform/ydb-go-sdk/v3"
	"github.com/ydb-platform/ydb-go-sdk/v3/balancers"
	"github.com/ydb-platform/ydb-go-sdk/v3/retry"

	"marathon_procrastination_bot/internal/env"
)

//go:embed migrations/*.sql
var embedMigrations embed.FS

func New(ctx context.Context) (*storage, error) {
	nativeDriver, err := ydb.Open(context.Background(),
		os.Getenv(env.YDB_CONNECTION_STRING),
		environ.WithEnvironCredentials(ctx),
		ydb.WithBalancer(balancers.SingleConn()),
	)
	if err != nil {
		return nil, err
	}
	connector, err := ydb.Connector(nativeDriver,
		ydb.WithTablePathPrefix(nativeDriver.Name()),
		ydb.WithAutoDeclare(),
		ydb.WithNumericArgs(),
	)
	if err != nil {
		return nil, err
	}
	return &storage{
		native: nativeDriver,
		db:     sql.OpenDB(connector),
	}, nil
}

func (s *storage) UpdateSchema() error {
	connector, err := ydb.Connector(s.native,
		ydb.WithDefaultQueryMode(ydb.ScriptingQueryMode),
		ydb.WithFakeTx(ydb.ScriptingQueryMode),
		ydb.WithAutoDeclare(),
		ydb.WithNumericArgs(),
	)
	if err != nil {
		return err
	}
	db := sql.OpenDB(connector)
	defer func() {
		_ = db.Close()
	}()
	goose.SetBaseFS(embedMigrations)
	if err := goose.SetDialect("ydb"); err != nil {
		return err
	}
	if err := goose.Up(db, "migrations"); err != nil {
		return err
	}
	return nil
}

type storage struct {
	native *ydb.Driver
	db     *sql.DB
}

func (s *storage) UsersForRotate(ctx context.Context, hour int32) (ids []int64, err error) {
	err = retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		ids = ids[:0]
		rows, err := cc.QueryContext(ctx, `
			SELECT user_id 
			FROM users 
			WHERE hour_to_rotate_stats=$1 AND last_stats_rotate_ts<$2;
		`, hour, time.Unix(int64(time.Now().UnixMilli()/1000/60/60-23)*60*60, 0).UTC())
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

func (s *storage) UsersForNotification(ctx context.Context) (ids []int64, err error) {
	err = retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		ids = ids[:0]
		rows, err := cc.QueryContext(ctx, `
			SELECT DISTINCT user_id 
			FROM activities 
			WHERE current=0 
				AND COALESCE(last_notificated, CAST(0 AS Timestamp))<CAST($1 AS Timestamp);
			`, time.Now().UTC().Add(-time.Duration(env.FreezeHours())*time.Hour),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

func (s *storage) UsersWithoutActivities(ctx context.Context) (ids []int64, err error) {
	err = retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		ids = ids[:0]
		rows, err := cc.QueryContext(ctx, `
			SELECT user_id 
			FROM users
			WHERE user_id NOT IN(
			    SELECT DISTINCT user_id
				FROM activities 
			)
			`, time.Now().UTC().Add(-time.Duration(env.FreezeHours())*time.Hour),
		)
		if err != nil {
			return err
		}
		defer func() { _ = rows.Close() }()
		for rows.Next() {
			var id int64
			if err := rows.Scan(&id); err != nil {
				return err
			}
			ids = append(ids, id)
		}
		return rows.Err()
	})
	return ids, err
}

func (s *storage) RotateUserStats(ctx context.Context, userID int64) error {
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found")
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE activities SET total=0
			WHERE user_id=$1 AND current=0;
			`, userID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE activities SET total=total+current, current=0
			WHERE user_id=$1 AND current>0;
			`, userID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET last_stats_rotate_ts=$1
			WHERE user_id=$1;
			`, time.Now().UTC(), userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) SetUserRotateHour(ctx context.Context, userID int64, hour int32) error {
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found")
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE users 
			SET hour_to_rotate_stats=$1, last_activity_ts=$3
			WHERE user_id=$2;
			`, hour, userID, time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) UserStats(ctx context.Context, userID int64, activity string) (total uint64, current uint64, _ error) {
	err := retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found")
		}
		row = tx.QueryRowContext(ctx, `
			SELECT total, current 
			FROM activities 
			WHERE user_id=$1 AND activity=$2;`,
			userID, activity,
		)
		if err := row.Scan(&total, &current); err != nil {
			return err
		}
		return row.Err()
	})
	return total, current, err
}

func (s *storage) AddUser(ctx context.Context, userID int64, chatID int64) error {
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPSERT INTO users (
				user_id, hour_to_rotate_stats, registration_chat_id, last_activity_ts
			) VALUES (
				$1, $2, $3, $4
			);`, userID, 0, chatID, time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) RemoveUser(ctx context.Context, userID int64) error {
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found", userID)
		}
		_, err := tx.ExecContext(ctx, `
			DELETE FROM users 
			WHERE user_id=$1;`,
			userID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			DELETE FROM activities 
			WHERE user_id=$1;`,
			userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) UserRegistrationChatID(ctx context.Context, userID int64) (chatID int64, _ error) {
	err := retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found")
		}
		row = tx.QueryRowContext(ctx, `
			SELECT COALESCE(registration_chat_id, $1)
			FROM users
			WHERE user_id=$1;
		`, userID)
		if err := row.Scan(&chatID); err != nil {
			return err
		}
		return row.Err()
	})
	return chatID, err
}

func (s *storage) UserActivities(ctx context.Context, userID int64) (activities []string, _ error) {
	err := retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found", userID)
		}
		activities = activities[:0]
		rows, err := tx.QueryContext(ctx,
			`SELECT activity 
					FROM activities 
					WHERE user_id=$1 ORDER BY activity;`,
			userID,
		)
		if err != nil {
			return err
		}
		for rows.Next() {
			var activity string
			if err := rows.Scan(&activity); err != nil {
				return err
			}
			activities = append(activities, activity)
		}
		return rows.Err()
	})
	return activities, err
}

func (s *storage) UpdateUserActivityLastNotificated(ctx context.Context, userID int64, activities ...string) error {
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPSERT INTO activities (
			    user_id, activity, last_notificated
			) SELECT user_id, activity, last_notificated FROM AS_TABLE($1);`,
			func() types.Value {
				var (
					lastNotificated = time.Now().UTC()
					rows            = make([]types.Value, len(activities))
				)
				for i, activity := range activities {
					rows[i] = types.StructValue(
						types.StructFieldValue("user_id", types.Int64Value(userID)),
						types.StructFieldValue("activity", types.TextValue(activity)),
						types.StructFieldValue("last_notificated", types.TimestampValueFromTime(lastNotificated)),
					)
				}
				return types.ListValue(rows...)
			}(),
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) PostUserActivity(ctx context.Context, userID int64, activity string) error {
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found")
		}
		_, err := tx.ExecContext(ctx, `
			UPDATE activities 
			SET current=current+1, post_ts=$3
            WHERE user_id=$1 AND activity=$2;`,
			userID,
			activity,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET last_post_ts=$1, last_activity_ts=$1
			WHERE user_id=$2;
			`, time.Now().UTC(), userID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPSERT INTO posts (
			    user_id, activity, ts
			) VALUES (
			    $1, $2, $3
			);`,
			userID,
			activity,
			time.Now().UTC(),
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) NewUserActivity(ctx context.Context, userID int64, activity string) error {
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found")
		}
		_, err := tx.ExecContext(ctx, `
			INSERT INTO activities (
				user_id, activity, total, current
			) VALUES (
				$1, $2, 0, 0
			);`, userID, activity,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET last_activity_ts=$1
			WHERE user_id=$2;
			`, time.Now().UTC(), userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) DeleteUserActivity(ctx context.Context, userID int64, activity string) error {
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		row := tx.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=$1;
		`, userID)
		var count uint64
		if err := row.Scan(&count); err != nil {
			return err
		}
		if err := row.Err(); err != nil {
			return err
		}
		if count == 0 {
			return fmt.Errorf("user %d not found")
		}
		_, err := tx.ExecContext(ctx, `
			DELETE FROM activities 
			WHERE user_id=$1 AND activity=$2;`,
			userID,
			activity,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET last_activity_ts=$1
			WHERE user_id=$2;
			`, time.Now().UTC(), userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}
