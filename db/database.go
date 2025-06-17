package db

import (
	"database/sql"
	"fmt"
	"log"
	// "os" // Commented out as not used yet

	_ "github.com/lib/pq"
	"component-service/models"
)

var DB *sql.DB

// ---- Function Variables for Mocking ----
var CreateComponentFunc func(component *models.Component) (int, error)
var GetComponentFunc func(id int) (*models.Component, error)
var GetTopLevelComponentsFunc func() ([]models.Component, error)
var UpdateComponentFunc func(id int, component *models.Component) error
var DeleteComponentFunc func(id int) error
var fetchChildrenFunc func(parentID int) ([]models.Component, error) // Also make internal helper mockable if needed, or test via public funcs


func init() {
	// Assign internal implementations to the Func variables
	CreateComponentFunc = createComponentInternal
	GetComponentFunc = getComponentInternal
	GetTopLevelComponentsFunc = getTopLevelComponentsInternal
	UpdateComponentFunc = updateComponentInternal
	DeleteComponentFunc = deleteComponentInternal
	fetchChildrenFunc = fetchChildrenInternal // Assign internal fetchChildren
}

// ---- Public Functions (calling Func variables) ----

func CreateComponent(component *models.Component) (int, error) {
    if CreateComponentFunc == nil {
        log.Println("Warning: CreateComponentFunc is nil, falling back to internal implementation.")
        return createComponentInternal(component)
    }
	return CreateComponentFunc(component)
}

func GetComponent(id int) (*models.Component, error) {
    if GetComponentFunc == nil {
        log.Println("Warning: GetComponentFunc is nil, falling back to internal implementation.")
        return getComponentInternal(id)
    }
	return GetComponentFunc(id)
}

func GetTopLevelComponents() ([]models.Component, error) {
    if GetTopLevelComponentsFunc == nil {
        log.Println("Warning: GetTopLevelComponentsFunc is nil, falling back to internal implementation.")
        return getTopLevelComponentsInternal()
    }
	return GetTopLevelComponentsFunc()
}

func UpdateComponent(id int, component *models.Component) error {
    if UpdateComponentFunc == nil {
        log.Println("Warning: UpdateComponentFunc is nil, falling back to internal implementation.")
        return updateComponentInternal(id, component)
    }
	return UpdateComponentFunc(id, component)
}

func DeleteComponent(id int) error {
    if DeleteComponentFunc == nil {
        log.Println("Warning: DeleteComponentFunc is nil, falling back to internal implementation.")
        return deleteComponentInternal(id)
    }
	return DeleteComponentFunc(id)
}


// ---- Internal Implementations ----

func InitDB(dbHost, dbPort, dbUser, dbPassword, dbName, sslmode string) {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		dbHost, dbPort, dbUser, dbPassword, dbName, sslmode)

	var err error
	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatalf("Error opening database: %q", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatalf("Error connecting to database: %q", err)
	}

	log.Println("Successfully connected to database!")
}

func CreateTable() {
	if DB == nil {
		log.Fatalf("DB is nil. InitDB must be called before CreateTable.")
		return
	}
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS components (
		id SERIAL PRIMARY KEY,
		name VARCHAR(255) NOT NULL,
		parent_id INTEGER REFERENCES components(id) ON DELETE CASCADE
	);`

	_, err := DB.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Error creating table: %q", err)
	}
	log.Println("Components table created or already exists.")
}

func createComponentInternal(component *models.Component) (int, error) {
	if DB == nil {
		return 0, fmt.Errorf("CreateComponentInternal: database not initialized")
	}
	sqlStatement := `
	INSERT INTO components (name, parent_id)
	VALUES ($1, $2)
	RETURNING id`

	var id int
	var parentID sql.NullInt64
	if component.ParentID != nil {
		parentID = sql.NullInt64{Int64: int64(*component.ParentID), Valid: true}
	} else {
		parentID = sql.NullInt64{Valid: false}
	}

	err := DB.QueryRow(sqlStatement, component.Name, parentID).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("CreateComponentInternal: %w", err)
	}
	component.ID = id
	return id, nil
}

func fetchChildrenInternal(parentID int) ([]models.Component, error) {
	if DB == nil {
		return nil, fmt.Errorf("fetchChildrenInternal: database not initialized")
	}
	rows, err := DB.Query("SELECT id, name, parent_id FROM components WHERE parent_id = $1", parentID)
	if err != nil {
		if err == sql.ErrNoRows {
			return []models.Component{}, nil
		}
		return nil, fmt.Errorf("fetchChildrenInternal query for parentID %d: %w", parentID, err)
	}
	defer rows.Close()

	var children []models.Component
	for rows.Next() {
		var child models.Component
		var dbParentID sql.NullInt64
		if err := rows.Scan(&child.ID, &child.Name, &dbParentID); err != nil {
			return nil, fmt.Errorf("fetchChildrenInternal scan for parentID %d: %w", parentID, err)
		}
		if dbParentID.Valid {
			pID := int(dbParentID.Int64)
			child.ParentID = &pID
		}

		grandchildren, err := fetchChildrenFunc(child.ID) // Use the Func variable for recursive calls
		if err != nil {
			return nil, fmt.Errorf("fetchChildrenInternal fetching grandchildren for childID %d: %w", child.ID, err)
		}
		child.Children = grandchildren
		children = append(children, child)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("fetchChildrenInternal rows error for parentID %d: %w", parentID, err)
	}
	return children, nil
}

func getComponentInternal(id int) (*models.Component, error) {
	if DB == nil {
		return nil, fmt.Errorf("getComponentInternal: database not initialized")
	}
	var component models.Component
	var parentID sql.NullInt64

	sqlStatement := `SELECT id, name, parent_id FROM components WHERE id = $1`
	row := DB.QueryRow(sqlStatement, id)
	err := row.Scan(&component.ID, &component.Name, &parentID)

	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("getComponentInternal: no component found with id %d", id)
		}
		return nil, fmt.Errorf("getComponentInternal scan for id %d: %w", id, err)
	}

	if parentID.Valid {
		pID := int(parentID.Int64)
		component.ParentID = &pID
	}

	children, err := fetchChildrenFunc(component.ID) // Use the Func variable
	if err != nil {
		return nil, fmt.Errorf("getComponentInternal fetching children for id %d: %w", id, err)
	}
	component.Children = children

	return &component, nil
}

func getTopLevelComponentsInternal() ([]models.Component, error) {
	if DB == nil {
		return nil, fmt.Errorf("getTopLevelComponentsInternal: database not initialized")
	}
	rows, err := DB.Query("SELECT id, name FROM components WHERE parent_id IS NULL")
	if err != nil {
		return nil, fmt.Errorf("GetTopLevelComponentsInternal query: %w", err)
	}
	defer rows.Close()

	var components []models.Component
	for rows.Next() {
		var component models.Component
		if err := rows.Scan(&component.ID, &component.Name); err != nil {
			return nil, fmt.Errorf("GetTopLevelComponentsInternal scan: %w", err)
		}

		children, err := fetchChildrenFunc(component.ID) // Use the Func variable
		if err != nil {
			return nil, fmt.Errorf("GetTopLevelComponentsInternal fetching children for id %d: %w", component.ID, err)
		}
		component.Children = children
		components = append(components, component)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("GetTopLevelComponentsInternal rows error: %w", err)
	}
	if len(components) == 0 {
		return []models.Component{}, nil
	}
	return components, nil
}

func updateComponentInternal(id int, component *models.Component) error {
	if DB == nil {
		return fmt.Errorf("updateComponentInternal: database not initialized")
	}
	sqlStatement := `
	UPDATE components
	SET name = $1, parent_id = $2
	WHERE id = $3`

	var parentID sql.NullInt64
	if component.ParentID != nil {
		if *component.ParentID == id {
			return fmt.Errorf("UpdateComponentInternal: component cannot be its own parent (id %d)", id)
		}
		parentID = sql.NullInt64{Int64: int64(*component.ParentID), Valid: true}
	} else {
		parentID = sql.NullInt64{Valid: false}
	}

	result, err := DB.Exec(sqlStatement, component.Name, parentID, id)
	if err != nil {
		return fmt.Errorf("UpdateComponentInternal exec for id %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("UpdateComponentInternal checking rows affected for id %d: %w", id, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("UpdateComponentInternal: no component found with id %d, or data was the same", id)
	}
	return nil
}

func deleteComponentInternal(id int) error {
	if DB == nil {
		return fmt.Errorf("deleteComponentInternal: database not initialized")
	}
	sqlStatement := `DELETE FROM components WHERE id = $1`

	result, err := DB.Exec(sqlStatement, id)
	if err != nil {
		return fmt.Errorf("DeleteComponentInternal exec for id %d: %w", id, err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("DeleteComponentInternal checking rows affected for id %d: %w", id, err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("DeleteComponentInternal: no component found with id %d", id)
	}
	return nil
}
