package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/farewatch/farewatch/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
}

func New(ctx context.Context, databaseURL string) (*Store, error) {
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	s := &Store{pool: pool}
	if err := s.Migrate(ctx); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() { s.pool.Close() }

func (s *Store) Migrate(ctx context.Context) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL UNIQUE,
			name TEXT NOT NULL DEFAULT '',
			password_hash TEXT NOT NULL,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS routes (
			id UUID PRIMARY KEY,
			origin TEXT NOT NULL,
			destination TEXT NOT NULL,
			depart_date DATE NOT NULL,
			return_date DATE,
			cabin TEXT NOT NULL DEFAULT 'economy',
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_routes_unique
			ON routes (origin, destination, depart_date, COALESCE(return_date, '1900-01-01'::date), cabin)`,
		`CREATE TABLE IF NOT EXISTS fares (
			id UUID PRIMARY KEY,
			route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
			airline TEXT NOT NULL,
			airline_code TEXT NOT NULL,
			price NUMERIC(12,2) NOT NULL,
			currency TEXT NOT NULL DEFAULT 'USD',
			deep_link TEXT NOT NULL DEFAULT '',
			cached BOOLEAN NOT NULL DEFAULT FALSE,
			observed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS watches (
			id UUID PRIMARY KEY,
			email TEXT NOT NULL,
			route_id UUID NOT NULL REFERENCES routes(id) ON DELETE CASCADE,
			target_price NUMERIC(12,2),
			notify_on_drop BOOLEAN NOT NULL DEFAULT TRUE,
			drop_percent NUMERIC(5,2) NOT NULL DEFAULT 5.0,
			active BOOLEAN NOT NULL DEFAULT TRUE,
			created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS alerts (
			id UUID PRIMARY KEY,
			watch_id UUID NOT NULL REFERENCES watches(id) ON DELETE CASCADE,
			fare_id UUID NOT NULL REFERENCES fares(id) ON DELETE CASCADE,
			old_price NUMERIC(12,2) NOT NULL,
			new_price NUMERIC(12,2) NOT NULL,
			airline TEXT NOT NULL,
			sent_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			delivered_in_ms BIGINT NOT NULL DEFAULT 0
		)`,
		// Additive columns for richer flight offers + auth ownership
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS flight_number TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS origin TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS origin_city TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS destination TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS destination_city TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS depart_at TIMESTAMPTZ`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS arrive_at TIMESTAMPTZ`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS duration_minutes INT NOT NULL DEFAULT 0`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS stops INT NOT NULL DEFAULT 0`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS cabin TEXT NOT NULL DEFAULT 'economy'`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS aircraft TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE fares ADD COLUMN IF NOT EXISTS source TEXT NOT NULL DEFAULT ''`,
		`ALTER TABLE watches ADD COLUMN IF NOT EXISTS user_id UUID REFERENCES users(id) ON DELETE CASCADE`,
		`ALTER TABLE users ADD COLUMN IF NOT EXISTS firebase_uid TEXT`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_users_firebase_uid ON users(firebase_uid) WHERE firebase_uid IS NOT NULL`,
		`CREATE INDEX IF NOT EXISTS idx_fares_route_observed ON fares(route_id, observed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_fares_route_airline ON fares(route_id, airline_code, observed_at DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_watches_route ON watches(route_id) WHERE active = TRUE`,
		`CREATE INDEX IF NOT EXISTS idx_watches_user ON watches(user_id) WHERE active = TRUE`,
		`CREATE INDEX IF NOT EXISTS idx_alerts_sent ON alerts(sent_at DESC)`,
	}
	for _, q := range stmts {
		if _, err := s.pool.Exec(ctx, q); err != nil {
			return fmt.Errorf("migrate: %w\nquery: %s", err, q)
		}
	}
	return nil
}

func (s *Store) CreateUser(ctx context.Context, email, name, passwordHash string) (*models.User, error) {
	id := uuid.NewString()
	var u models.User
	err := s.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, name, password_hash)
		VALUES ($1, LOWER($2), $3, $4)
		RETURNING id, email, name, password_hash, created_at
	`, id, email, name, passwordHash).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, created_at FROM users WHERE email = LOWER($1)
	`, email).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, created_at FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// UpsertFirebaseUser links or creates a local user for a Firebase identity.
func (s *Store) UpsertFirebaseUser(ctx context.Context, firebaseUID, email, name, passwordHash string) (*models.User, error) {
	var u models.User
	err := s.pool.QueryRow(ctx, `
		SELECT id, email, name, password_hash, created_at FROM users WHERE firebase_uid = $1
	`, firebaseUID).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err == nil {
		return &u, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	// Same email already registered with password — attach firebase uid.
	existing, err := s.GetUserByEmail(ctx, email)
	if err == nil {
		_, _ = s.pool.Exec(ctx, `UPDATE users SET firebase_uid = $1, name = COALESCE(NULLIF($2, ''), name) WHERE id = $3`,
			firebaseUID, name, existing.ID)
		existing.Name = name
		return existing, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return nil, err
	}

	id := uuid.NewString()
	err = s.pool.QueryRow(ctx, `
		INSERT INTO users (id, email, name, password_hash, firebase_uid)
		VALUES ($1, LOWER($2), $3, $4, $5)
		RETURNING id, email, name, password_hash, created_at
	`, id, email, name, passwordHash, firebaseUID).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (s *Store) CreateRoute(ctx context.Context, origin, destination, departDate string, returnDate *string, cabin string) (*models.Route, error) {
	if cabin == "" {
		cabin = "economy"
	}
	if existing, err := s.findRoute(ctx, origin, destination, departDate, returnDate, cabin); err == nil {
		_, _ = s.pool.Exec(ctx, `UPDATE routes SET active = TRUE WHERE id = $1`, existing.ID)
		existing.Active = true
		return existing, nil
	}
	id := uuid.NewString()
	var r models.Route
	err := s.pool.QueryRow(ctx, `
		INSERT INTO routes (id, origin, destination, depart_date, return_date, cabin)
		VALUES ($1, UPPER($2), UPPER($3), $4::date, NULLIF($5, '')::date, $6)
		RETURNING id, origin, destination, depart_date::text, NULLIF(return_date::text, ''), cabin, active, created_at
	`, id, origin, destination, departDate, deref(returnDate), cabin).Scan(
		&r.ID, &r.Origin, &r.Destination, &r.DepartDate, &r.ReturnDate, &r.Cabin, &r.Active, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) findRoute(ctx context.Context, origin, destination, departDate string, returnDate *string, cabin string) (*models.Route, error) {
	var r models.Route
	err := s.pool.QueryRow(ctx, `
		SELECT id, origin, destination, depart_date::text, NULLIF(return_date::text, ''), cabin, active, created_at
		FROM routes
		WHERE origin = UPPER($1) AND destination = UPPER($2)
		  AND depart_date = $3::date
		  AND COALESCE(return_date, '1900-01-01'::date) = COALESCE(NULLIF($4, '')::date, '1900-01-01'::date)
		  AND cabin = $5
		LIMIT 1
	`, origin, destination, departDate, deref(returnDate), cabin).Scan(
		&r.ID, &r.Origin, &r.Destination, &r.DepartDate, &r.ReturnDate, &r.Cabin, &r.Active, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) ListRoutes(ctx context.Context, activeOnly bool) ([]models.Route, error) {
	q := `
		SELECT id, origin, destination, depart_date::text, NULLIF(return_date::text, ''), cabin, active, created_at
		FROM routes
	`
	if activeOnly {
		q += ` WHERE active = TRUE`
	}
	q += ` ORDER BY created_at DESC`
	rows, err := s.pool.Query(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Route
	for rows.Next() {
		var r models.Route
		if err := rows.Scan(&r.ID, &r.Origin, &r.Destination, &r.DepartDate, &r.ReturnDate, &r.Cabin, &r.Active, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) ListRoutesWithActiveWatches(ctx context.Context) ([]models.Route, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT DISTINCT r.id, r.origin, r.destination, r.depart_date::text, NULLIF(r.return_date::text, ''), r.cabin, r.active, r.created_at
		FROM routes r
		JOIN watches w ON w.route_id = r.id AND w.active = TRUE
		WHERE r.active = TRUE
		ORDER BY r.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Route
	for rows.Next() {
		var r models.Route
		if err := rows.Scan(&r.ID, &r.Origin, &r.Destination, &r.DepartDate, &r.ReturnDate, &r.Cabin, &r.Active, &r.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

func (s *Store) GetRoute(ctx context.Context, id string) (*models.Route, error) {
	var r models.Route
	err := s.pool.QueryRow(ctx, `
		SELECT id, origin, destination, depart_date::text, NULLIF(return_date::text, ''), cabin, active, created_at
		FROM routes WHERE id = $1
	`, id).Scan(&r.ID, &r.Origin, &r.Destination, &r.DepartDate, &r.ReturnDate, &r.Cabin, &r.Active, &r.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

func (s *Store) InsertFare(ctx context.Context, fare models.Fare) (*models.Fare, error) {
	if fare.ID == "" {
		fare.ID = uuid.NewString()
	}
	if fare.ObservedAt.IsZero() {
		fare.ObservedAt = time.Now().UTC()
	}
	if fare.Currency == "" {
		fare.Currency = "USD"
	}
	var departAt, arriveAt any
	if fare.DepartAt != "" {
		departAt = fare.DepartAt
	}
	if fare.ArriveAt != "" {
		arriveAt = fare.ArriveAt
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO fares (
			id, route_id, airline, airline_code, flight_number,
			origin, origin_city, destination, destination_city,
			depart_at, arrive_at, duration_minutes, stops, cabin, aircraft,
			price, currency, deep_link, source, cached, observed_at
		) VALUES (
			$1,$2,$3,$4,$5,
			$6,$7,$8,$9,
			$10,$11,$12,$13,$14,$15,
			$16,$17,$18,$19,$20,$21
		)
	`, fare.ID, fare.RouteID, fare.Airline, fare.AirlineCode, fare.FlightNumber,
		fare.Origin, fare.OriginCity, fare.Destination, fare.DestinationCity,
		departAt, arriveAt, fare.DurationMinutes, fare.Stops, fare.Cabin, fare.Aircraft,
		fare.Price, fare.Currency, fare.DeepLink, fare.Source, fare.Cached, fare.ObservedAt)
	if err != nil {
		return nil, err
	}
	return &fare, nil
}

func scanFare(row interface {
	Scan(dest ...any) error
}) (*models.Fare, error) {
	var f models.Fare
	var departAt, arriveAt *time.Time
	err := row.Scan(
		&f.ID, &f.RouteID, &f.Airline, &f.AirlineCode, &f.FlightNumber,
		&f.Origin, &f.OriginCity, &f.Destination, &f.DestinationCity,
		&departAt, &arriveAt, &f.DurationMinutes, &f.Stops, &f.Cabin, &f.Aircraft,
		&f.Price, &f.Currency, &f.DeepLink, &f.Source, &f.Cached, &f.ObservedAt,
	)
	if err != nil {
		return nil, err
	}
	if departAt != nil {
		f.DepartAt = departAt.UTC().Format(time.RFC3339)
	}
	if arriveAt != nil {
		f.ArriveAt = arriveAt.UTC().Format(time.RFC3339)
	}
	return &f, nil
}

const fareSelect = `
	SELECT id, route_id, airline, airline_code, flight_number,
	       origin, origin_city, destination, destination_city,
	       depart_at, arrive_at, duration_minutes, stops, cabin, aircraft,
	       price::float8, currency, deep_link, source, cached, observed_at
	FROM fares
`

func (s *Store) ListFares(ctx context.Context, routeID string, limit int) ([]models.Fare, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := s.pool.Query(ctx, fareSelect+` WHERE route_id = $1 ORDER BY observed_at DESC LIMIT $2`, routeID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Fare
	for rows.Next() {
		f, err := scanFare(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *f)
	}
	return out, rows.Err()
}

func (s *Store) LatestFareByAirline(ctx context.Context, routeID, airlineCode string) (*models.Fare, error) {
	return scanFare(s.pool.QueryRow(ctx, fareSelect+`
		WHERE route_id = $1 AND airline_code = $2
		ORDER BY observed_at DESC LIMIT 1
	`, routeID, airlineCode))
}

func (s *Store) LatestFareForRoute(ctx context.Context, routeID string) (*models.Fare, error) {
	return scanFare(s.pool.QueryRow(ctx, fareSelect+`
		WHERE route_id = $1 ORDER BY price ASC, observed_at DESC LIMIT 1
	`, routeID))
}

func (s *Store) Fare24hAgo(ctx context.Context, routeID string) (*models.Fare, error) {
	return scanFare(s.pool.QueryRow(ctx, fareSelect+`
		WHERE route_id = $1 AND observed_at <= NOW() - INTERVAL '20 hours'
		ORDER BY observed_at DESC LIMIT 1
	`, routeID))
}

func (s *Store) CreateWatch(ctx context.Context, userID, email, routeID string, targetPrice *float64, dropPercent float64) (*models.Watch, error) {
	if dropPercent <= 0 {
		dropPercent = 5
	}
	id := uuid.NewString()
	var w models.Watch
	err := s.pool.QueryRow(ctx, `
		INSERT INTO watches (id, user_id, email, route_id, target_price, drop_percent)
		VALUES ($1, $2, LOWER($3), $4, $5, $6)
		RETURNING id, COALESCE(user_id::text, ''), email, route_id, target_price, notify_on_drop, drop_percent::float8, active, created_at
	`, id, nullUUID(userID), email, routeID, targetPrice, dropPercent).Scan(
		&w.ID, &w.UserID, &w.Email, &w.RouteID, &w.TargetPrice, &w.NotifyOnDrop, &w.DropPercent, &w.Active, &w.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &w, nil
}

func nullUUID(id string) any {
	if id == "" {
		return nil
	}
	return id
}

func (s *Store) ListWatchesByUser(ctx context.Context, userID string) ([]models.Watch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT w.id, COALESCE(w.user_id::text, ''), w.email, w.route_id, w.target_price, w.notify_on_drop, w.drop_percent::float8, w.active, w.created_at,
		       r.id, r.origin, r.destination, r.depart_date::text, NULLIF(r.return_date::text, ''), r.cabin, r.active, r.created_at
		FROM watches w
		JOIN routes r ON r.id = w.route_id
		WHERE w.active = TRUE AND w.user_id = $1
		ORDER BY w.created_at DESC
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []models.Watch
	for rows.Next() {
		var w models.Watch
		var r models.Route
		if err := rows.Scan(
			&w.ID, &w.UserID, &w.Email, &w.RouteID, &w.TargetPrice, &w.NotifyOnDrop, &w.DropPercent, &w.Active, &w.CreatedAt,
			&r.ID, &r.Origin, &r.Destination, &r.DepartDate, &r.ReturnDate, &r.Cabin, &r.Active, &r.CreatedAt,
		); err != nil {
			return nil, err
		}
		w.Route = &r
		if fare, err := s.LatestFareForRoute(ctx, r.ID); err == nil {
			w.LatestFare = fare
			if prev, err := s.Fare24hAgo(ctx, r.ID); err == nil && prev.Price > 0 {
				chg := ((fare.Price - prev.Price) / prev.Price) * 100
				w.Change24h = &chg
			}
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) ActiveWatchesForRoute(ctx context.Context, routeID string) ([]models.Watch, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, COALESCE(user_id::text, ''), email, route_id, target_price, notify_on_drop, drop_percent::float8, active, created_at
		FROM watches WHERE route_id = $1 AND active = TRUE
	`, routeID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Watch
	for rows.Next() {
		var w models.Watch
		if err := rows.Scan(&w.ID, &w.UserID, &w.Email, &w.RouteID, &w.TargetPrice, &w.NotifyOnDrop, &w.DropPercent, &w.Active, &w.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

func (s *Store) DeactivateWatch(ctx context.Context, userID, watchID string) error {
	ct, err := s.pool.Exec(ctx, `
		UPDATE watches SET active = FALSE WHERE id = $1 AND user_id = $2
	`, watchID, userID)
	if err != nil {
		return err
	}
	if ct.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (s *Store) InsertAlert(ctx context.Context, a models.Alert) (*models.Alert, error) {
	if a.ID == "" {
		a.ID = uuid.NewString()
	}
	if a.SentAt.IsZero() {
		a.SentAt = time.Now().UTC()
	}
	_, err := s.pool.Exec(ctx, `
		INSERT INTO alerts (id, watch_id, fare_id, old_price, new_price, airline, sent_at, delivered_in_ms)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
	`, a.ID, a.WatchID, a.FareID, a.OldPrice, a.NewPrice, a.Airline, a.SentAt, a.DeliveredIn)
	if err != nil {
		return nil, err
	}
	return &a, nil
}

func (s *Store) ListAlertsForUser(ctx context.Context, userID string, limit int) ([]models.Alert, error) {
	if limit <= 0 {
		limit = 20
	}
	rows, err := s.pool.Query(ctx, `
		SELECT a.id, a.watch_id, a.fare_id, a.old_price::float8, a.new_price::float8, a.airline, a.sent_at, a.delivered_in_ms
		FROM alerts a
		JOIN watches w ON w.id = a.watch_id
		WHERE w.user_id = $1
		ORDER BY a.sent_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []models.Alert
	for rows.Next() {
		var a models.Alert
		if err := rows.Scan(&a.ID, &a.WatchID, &a.FareID, &a.OldPrice, &a.NewPrice, &a.Airline, &a.SentAt, &a.DeliveredIn); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
