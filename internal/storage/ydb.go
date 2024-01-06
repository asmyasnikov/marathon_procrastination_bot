package storage

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
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
		ydb.WithPositionalArgs(),
	)
	if err != nil {
		return nil, err
	}
	return &storage{
		db: sql.OpenDB(connector),
	}, initSchema(nativeDriver)
}

func initSchema(d *ydb.Driver) error {
	connector, err := ydb.Connector(d,
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
	db *sql.DB
}

func (s *storage) UsersForRotate(ctx context.Context, hour int32) (ids []int64, err error) {
	err = retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		ids = ids[:0]
		rows, err := cc.QueryContext(ctx, `
			SELECT user_id 
			FROM users 
			WHERE 
			    hour_to_rotate_stats=? AND last_stats_rotate_ts<?;
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

func (s *storage) RotateUserStats(ctx context.Context, userID int64) error {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("user %d not exists", userID)
	}
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE activities SET total=0
			WHERE user_id=? AND current=0;
			`, userID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE activities SET total=total+current, current=0
			WHERE user_id=? AND current>0;
			`, userID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET last_stats_rotate_ts=?
			WHERE user_id=?;
			`, time.Now().UTC(), userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) SetUserRotateHour(ctx context.Context, userID int64, hour int32) error {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("user %d not exists", userID)
	}
	return retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		_, err := cc.ExecContext(ctx, `
			UPDATE users 
			SET hour_to_rotate_stats=?
			WHERE user_id=?;
			`, hour, userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) UserStats(ctx context.Context, userID int64, activity string) (total uint64, current uint64, _ error) {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return total, current, err
	} else if !exists {
		return total, current, fmt.Errorf("user %d not exists", userID)
	}
	err := retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		row := cc.QueryRowContext(ctx, `
			SELECT total, current 
			FROM activities 
			WHERE user_id=? AND activity=?;`,
			userID, activity,
		)
		if err := row.Scan(&total, &current); err != nil {
			return err
		}
		return row.Err()
	})
	return total, current, err
}

func (s *storage) AddUser(ctx context.Context, userID int64) error {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return err
	} else if exists {
		return fmt.Errorf("user %d already exists", userID)
	}
	return retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		_, err := cc.ExecContext(ctx, `
			INSERT INTO users (
				user_id, hour_to_rotate_stats
			) VALUES (
				?, 12
			);`, userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) RemoveUser(ctx context.Context, userID int64) error {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("user %d not found", userID)
	}
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			DELETE FROM users 
			WHERE user_id=?;`,
			userID,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			DELETE FROM activities 
			WHERE user_id=?;`,
			userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) UserActivities(ctx context.Context, userID int64) (activities []string, _ error) {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return nil, err
	} else if !exists {
		return nil, fmt.Errorf("user %d not found", userID)
	}
	err := retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		activities = activities[:0]
		rows, err := cc.QueryContext(ctx,
			`SELECT activity 
					FROM activities 
					WHERE user_id=? ORDER BY activity;`,
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

func (s *storage) AppendUserActivity(ctx context.Context, userID int64, activity string) error {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("user %d not found", userID)
	}
	return retry.DoTx(ctx, s.db, func(ctx context.Context, tx *sql.Tx) error {
		_, err := tx.ExecContext(ctx, `
			UPDATE activities 
			SET current=current+1
            WHERE user_id=? AND activity=?;`,
			userID,
			activity,
		)
		if err != nil {
			return err
		}
		_, err = tx.ExecContext(ctx, `
			UPDATE users SET last_post_ts=?
			WHERE user_id=?;
			`, time.Now().UTC(), userID,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) NewUserActivity(ctx context.Context, userID int64, activity string) error {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("user %d not found", userID)
	}
	return retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		_, err := cc.ExecContext(ctx, `
			INSERT INTO activities (
				user_id, activity, total, current
			) VALUES (
				?, ?, 0, 0
			);`, userID, activity,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) DeleteUserActivity(ctx context.Context, userID int64, activity string) error {
	if exists, err := s.isUserExists(ctx, userID); err != nil {
		return err
	} else if !exists {
		return fmt.Errorf("user %d not found", userID)
	}
	return retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		_, err := cc.ExecContext(ctx, `
			DELETE FROM activities 
			WHERE user_id=? AND activity=?;`,
			userID,
			activity,
		)
		if err != nil {
			return err
		}
		return nil
	})
}

func (s *storage) isUserExists(ctx context.Context, userID int64) (_ bool, err error) {
	var count uint64
	err = retry.Do(ctx, s.db, func(ctx context.Context, cc *sql.Conn) error {
		row := cc.QueryRowContext(ctx, `
			SELECT COUNT(*)
			FROM users
			WHERE user_id=?;
		`, userID)
		if err := row.Scan(&count); err != nil {
			return err
		}
		return row.Err()
	})
	return count > 0, err
}
