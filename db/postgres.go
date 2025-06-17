package db

import (
	"database/sql"
	"fmt"

	"explorer-server/models" // Assuming explorer-server is the module name

	_ "github.com/lib/pq" // PostgreSQL driver
)

// DB holds the database connection.
type DB struct {
	*sql.DB
}

// NewDB establishes a new database connection and returns a DB instance.
func NewDB(dataSourceName string) (*DB, error) {
	conn, err := sql.Open("postgres", dataSourceName)
	if err != nil {
		return nil, fmt.Errorf("failed to open database connection: %w", err)
	}

	if err = conn.Ping(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DB{conn}, nil
}

// CreateComponent inserts a new component into the components table.
// It assumes component.ID is already populated with a UUID string.
func (db *DB) CreateComponent(component *models.Component) error {
	query := `
		INSERT INTO components (id, name, parent_id)
		VALUES ($1, $2, $3)
	`
	var parentID sql.NullString
	if component.ParentID != nil {
		parentID = sql.NullString{String: *component.ParentID, Valid: true}
	} else {
		parentID = sql.NullString{Valid: false}
	}

	_, err := db.Exec(query, component.ID, component.Name, parentID)
	if err != nil {
		return fmt.Errorf("failed to insert component: %w", err)
	}
	return nil
}

// GetComponentByID retrieves a component by its ID.
// It returns sql.ErrNoRows if no component is found.
func (db *DB) GetComponentByID(id string) (*models.Component, error) {
	query := `
		SELECT id, name, parent_id
		FROM components
		WHERE id = $1
	`
	row := db.QueryRow(query, id)

	component := &models.Component{}
	var parentID sql.NullString

	err := row.Scan(&component.ID, &component.Name, &parentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, sql.ErrNoRows
		}
		return nil, fmt.Errorf("failed to scan component: %w", err)
	}

	if parentID.Valid {
		component.ParentID = &parentID.String
	}

	return component, nil
}

// GetAllComponents retrieves all components from the database, ordered by name.
func (db *DB) GetAllComponents() ([]*models.Component, error) {
	query := `
		SELECT id, name, parent_id
		FROM components
		ORDER BY name
	`
	rows, err := db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("failed to query components: %w", err)
	}
	defer rows.Close()

	var components []*models.Component
	for rows.Next() {
		component := &models.Component{}
		var parentID sql.NullString

		err := rows.Scan(&component.ID, &component.Name, &parentID)
		if err != nil {
			return nil, fmt.Errorf("failed to scan component row: %w", err)
		}

		if parentID.Valid {
			component.ParentID = &parentID.String
		}
		components = append(components, component)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error during rows iteration: %w", err)
	}

	return components, nil
}

// UpdateComponent updates an existing component in the database.
// It returns sql.ErrNoRows if no component with the given ID is found.
func (db *DB) UpdateComponent(id string, component *models.Component) error {
	query := `
		UPDATE components
		SET name = $1, parent_id = $2
		WHERE id = $3
	`
	var parentID sql.NullString
	if component.ParentID != nil {
		parentID = sql.NullString{String: *component.ParentID, Valid: true}
	} else {
		parentID = sql.NullString{Valid: false}
	}

	result, err := db.Exec(query, component.Name, parentID, id)
	if err != nil {
		return fmt.Errorf("failed to update component: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows // Or a custom error like fmt.Errorf("component with ID %s not found", id)
	}

	return nil
}

// DeleteComponent removes a component from the database by its ID.
// It returns sql.ErrNoRows if no component with the given ID is found.
func (db *DB) DeleteComponent(id string) error {
	query := `
		DELETE FROM components
		WHERE id = $1
	`
	result, err := db.Exec(query, id)
	if err != nil {
		return fmt.Errorf("failed to delete component: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return sql.ErrNoRows // Or a custom error like fmt.Errorf("component with ID %s not found", id)
	}

	return nil
}
