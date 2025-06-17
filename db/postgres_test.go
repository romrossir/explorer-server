package db

import (
	"database/sql"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"testing"

	"explorer-server/models"

	_ "github.com/lib/pq" // PostgreSQL driver
)

var testDB *DB

// TestMain sets up the database connection and schema for all tests in this package.
// It's run once before any tests and cleans up after all tests are done.
func TestMain(m *testing.M) {
	testDatabaseURL := os.Getenv("TEST_DATABASE_URL")
	if testDatabaseURL == "" {
		log.Println("TEST_DATABASE_URL not set, skipping database tests.")
		// Exit with success if no DB URL, as we can't run tests.
		// Alternatively, os.Exit(0) if tests shouldn't be "skipped" but just not run.
		// For CI, it's better to explicitly skip or fail.
		// For now, we'll let individual tests handle the nil testDB.
	}

	var err error
	if testDatabaseURL != "" { // Only attempt connection if URL is provided
		conn, err := sql.Open("postgres", testDatabaseURL)
		if err != nil {
			log.Fatalf("Failed to open test database connection: %v", err)
		}
		if err = conn.Ping(); err != nil {
			conn.Close()
			log.Fatalf("Failed to ping test database: %v", err)
		}
		testDB = &DB{conn}

		// Apply schema
		// Assuming schema.sql is in the parent directory relative to this test file's directory.
		schemaPath := filepath.Join("..", "schema.sql")
		schemaBytes, err := ioutil.ReadFile(schemaPath)
		if err != nil {
			testDB.Close()
			log.Fatalf("Failed to read schema.sql at %s: %v", schemaPath, err)
		}
		if _, err := testDB.Exec(string(schemaBytes)); err != nil {
			testDB.Close()
			log.Fatalf("Failed to apply schema: %v", err)
		}
		log.Println("Test database schema applied successfully.")
	}

	// Run tests
	exitCode := m.Run()

	// Clean up: Close database connection if it was opened
	if testDB != nil {
		// Optionally, drop tables here if desired, or clear them.
		// For simplicity, just closing the connection.
		testDB.Close()
		log.Println("Test database connection closed.")
	}
	os.Exit(exitCode)
}

// clearComponentsTable is a helper to delete all rows from components.
func clearComponentsTable(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}
	_, err := testDB.Exec("DELETE FROM components")
	if err != nil {
		t.Fatalf("Failed to clear components table: %v", err)
	}
	// Reset sequences if any (not strictly necessary for UUIDs as strings)
	// _, err = testDB.Exec("ALTER SEQUENCE components_id_seq RESTART WITH 1") // If using serial IDs
}

// Helper to create a component for testing, fails test on error
func createTestComponent(t *testing.T, db *DB, comp *models.Component) {
	t.Helper()
	err := db.CreateComponent(comp)
	if err != nil {
		t.Fatalf("Failed to create test component (ID: %s): %v", comp.ID, err)
	}
}


func TestCreateComponent(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}
	clearComponentsTable(t)

	parentID := "parent-uuid-for-create"
	parentComp := &models.Component{ID: parentID, Name: "Parent For Create"}
	createTestComponent(t, testDB, parentComp)


	comp1 := &models.Component{
		ID:   "test-create-1",
		Name: "Component Create 1",
	}
	comp2 := &models.Component{
		ID:       "test-create-2",
		Name:     "Component Create 2 With Parent",
		ParentID: &parentID,
	}

	tests := []struct {
		name      string
		component *models.Component
		wantErr   bool
	}{
		{"create component without parent", comp1, false},
		{"create component with parent", comp2, false},
		{"create component with duplicate ID", &models.Component{ID: "test-create-1", Name: "Duplicate"}, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := testDB.CreateComponent(tt.component)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateComponent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				retrieved, errGet := testDB.GetComponentByID(tt.component.ID)
				if errGet != nil {
					t.Fatalf("Failed to retrieve created component: %v", errGet)
				}
				if !reflect.DeepEqual(retrieved, tt.component) {
					t.Errorf("Retrieved component = %v, want %v", retrieved, tt.component)
				}
			}
		})
	}
}

func TestGetComponentByID(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}
	clearComponentsTable(t)

	parentID := "parent-uuid-for-get"
	parentComp := &models.Component{ID: parentID, Name: "Parent For Get"}
	createTestComponent(t, testDB, parentComp)

	comp := &models.Component{
		ID:       "test-get-1",
		Name:     "Component Get 1",
		ParentID: &parentID,
	}
	createTestComponent(t, testDB, comp)

	tests := []struct {
		name      string
		id        string
		want      *models.Component
		wantErr   error
	}{
		{"get existing component", comp.ID, comp, nil},
		{"get non-existent component", "non-existent-id", nil, sql.ErrNoRows},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := testDB.GetComponentByID(tt.id)
			if err != tt.wantErr {
				t.Errorf("GetComponentByID() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetComponentByID() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGetAllComponents(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}
	clearComponentsTable(t)

	parentID := "parent-uuid-for-getall"
	parentComp := &models.Component{ID: parentID, Name: "Parent For GetAll"} // Z name to test sorting later
	createTestComponent(t, testDB, parentComp)

	comp1 := &models.Component{ID: "test-getall-1", Name: "B Component"} // B for sorting
	comp2 := &models.Component{ID: "test-getall-2", Name: "A Component With Parent", ParentID: &parentID} // A for sorting

	createTestComponent(t, testDB, comp1) // Create comp1 (B)
	createTestComponent(t, testDB, comp2) // Create comp2 (A)

	// Expected order: comp2 (A), comp1 (B), parentComp (Parent/Z) - based on current db.GetAllComponents ordering by name
	// Corrected expected order: A, B, Parent (as per current implementation)
	expectedComponents := []*models.Component{comp2, comp1, parentComp}
	// Sort expectedComponents by name to match the function's behavior
	sort.Slice(expectedComponents, func(i, j int) bool {
		return expectedComponents[i].Name < expectedComponents[j].Name
	})


	t.Run("get all components when table has entries", func(t *testing.T) {
		got, err := testDB.GetAllComponents()
		if err != nil {
			t.Fatalf("GetAllComponents() error = %v", err)
		}
		if !reflect.DeepEqual(got, expectedComponents) {
			t.Errorf("GetAllComponents() got = %#v, want %#v", got, expectedComponents)
			for i := range got {
				if i < len(expectedComponents) && !reflect.DeepEqual(got[i], expectedComponents[i]) {
					t.Errorf("Mismatch at index %d: got %v, want %v", i, got[i], expectedComponents[i])
				}
			}
		}
	})

	t.Run("get all components when table is empty", func(t *testing.T) {
		clearComponentsTable(t)
		got, err := testDB.GetAllComponents()
		if err != nil {
			t.Fatalf("GetAllComponents() error on empty table = %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAllComponents() on empty table got %v, want empty slice", got)
		}
	})
}

func TestUpdateComponent(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}
	clearComponentsTable(t)

	parent1ID := "parent-update-1"
	parent2ID := "parent-update-2"
	createTestComponent(t, testDB, &models.Component{ID: parent1ID, Name: "Parent Update 1"})
	createTestComponent(t, testDB, &models.Component{ID: parent2ID, Name: "Parent Update 2"})

	compToUpdate := &models.Component{
		ID:       "test-update-1",
		Name:     "Original Name",
		ParentID: &parent1ID,
	}
	createTestComponent(t, testDB, compToUpdate)

	updates := &models.Component{ // Name and ParentID are part of models.Component
		Name:     "Updated Name",
		ParentID: &parent2ID, // Change parent
	}

	updatesNoParent := &models.Component{
		Name:     "Updated Name No Parent",
		ParentID: nil, // Set parent to NULL
	}


	t.Run("update existing component name and parent", func(t *testing.T) {
		err := testDB.UpdateComponent(compToUpdate.ID, updates)
		if err != nil {
			t.Fatalf("UpdateComponent() error = %v", err)
		}
		retrieved, _ := testDB.GetComponentByID(compToUpdate.ID)
		// The ID of 'updates' is not set, and UpdateComponent doesn't update ID.
		// So, the retrieved component's ID will be compToUpdate.ID.
		// We need to compare Name and ParentID.
		if retrieved.Name != updates.Name || (retrieved.ParentID == nil && updates.ParentID != nil) || (retrieved.ParentID != nil && updates.ParentID == nil) || (retrieved.ParentID != nil && updates.ParentID != nil && *retrieved.ParentID != *updates.ParentID) {
			t.Errorf("UpdateComponent() got Name %s, ParentID %v; want Name %s, ParentID %v",
				retrieved.Name, retrieved.ParentID, updates.Name, updates.ParentID)
		}
	})

	// Restore for next test case
	createTestComponent(t, testDB, &models.Component{ID: "test-update-restore", Name: "To Be Restored"}) // Create a dummy to clear table
	clearComponentsTable(t)
	createTestComponent(t, testDB, &models.Component{ID: parent1ID, Name: "Parent Update 1"})
    createTestComponent(t, testDB, compToUpdate) // recreate compToUpdate with original parent1ID

	t.Run("update existing component to have no parent", func(t *testing.T) {
		err := testDB.UpdateComponent(compToUpdate.ID, updatesNoParent)
		if err != nil {
			t.Fatalf("UpdateComponent() error = %v", err)
		}
		retrieved, _ := testDB.GetComponentByID(compToUpdate.ID)
		if retrieved.Name != updatesNoParent.Name || retrieved.ParentID != nil {
			t.Errorf("UpdateComponent() got Name %s, ParentID %v; want Name %s, ParentID nil",
				retrieved.Name, retrieved.ParentID, updatesNoParent.Name)
		}
	})


	t.Run("update non-existent component", func(t *testing.T) {
		err := testDB.UpdateComponent("non-existent-id", updates)
		if err != sql.ErrNoRows {
			t.Errorf("UpdateComponent() on non-existent ID error = %v, want %v", err, sql.ErrNoRows)
		}
	})
}

func TestDeleteComponent(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}
	clearComponentsTable(t)

	compToDelete := &models.Component{ID: "test-delete-1", Name: "To Be Deleted"}
	createTestComponent(t, testDB, compToDelete)

	t.Run("delete existing component", func(t *testing.T) {
		err := testDB.DeleteComponent(compToDelete.ID)
		if err != nil {
			t.Fatalf("DeleteComponent() error = %v", err)
		}
		_, errGet := testDB.GetComponentByID(compToDelete.ID)
		if errGet != sql.ErrNoRows {
			t.Errorf("GetComponentByID() after delete error = %v, want %v", errGet, sql.ErrNoRows)
		}
	})

	t.Run("delete non-existent component", func(t *testing.T) {
		err := testDB.DeleteComponent("non-existent-id")
		if err != sql.ErrNoRows {
			t.Errorf("DeleteComponent() on non-existent ID error = %v, want %v", err, sql.ErrNoRows)
		}
	})

	// Test cascading delete (ON DELETE SET NULL for parent_id)
	parentCompID := "parent-casc-delete"
	childCompID := "child-casc-delete"
	createTestComponent(t, testDB, &models.Component{ID: parentCompID, Name: "Parent For Cascade Delete Test"})
	createTestComponent(t, testDB, &models.Component{ID: childCompID, Name: "Child For Cascade Delete Test", ParentID: &parentCompID})

	t.Run("delete parent component and check child's parent_id is NULL", func(t *testing.T) {
		// Delete the parent
		err := testDB.DeleteComponent(parentCompID)
		if err != nil {
			t.Fatalf("Failed to delete parent component for cascade test: %v", err)
		}

		// Check the child component
		childComp, err := testDB.GetComponentByID(childCompID)
		if err != nil {
			t.Fatalf("Failed to get child component after parent deletion: %v", err)
		}
		if childComp.ParentID != nil {
			t.Errorf("Child component's ParentID is %v, expected nil after parent deletion due to ON DELETE SET NULL", *childComp.ParentID)
		}
	})
}

// Test for ON DELETE SET NULL with multiple children
func TestDeleteComponent_CascadeSetNullMultipleChildren(t *testing.T) {
    if testDB == nil {
        t.Skip("TEST_DATABASE_URL not set, skipping test.")
        return
    }
    clearComponentsTable(t)

    parentID := "parent-multi-child-delete"
    child1ID := "child1-multi-delete"
    child2ID := "child2-multi-delete"
    grandChildID := "grandchild-multi-delete" // Child of child1

    // Create parent and children
    createTestComponent(t, testDB, &models.Component{ID: parentID, Name: "Parent Multi Child"})
    createTestComponent(t, testDB, &models.Component{ID: child1ID, Name: "Child1 Multi", ParentID: &parentID})
    createTestComponent(t, testDB, &models.Component{ID: child2ID, Name: "Child2 Multi", ParentID: &parentID})
    createTestComponent(t, testDB, &models.Component{ID: grandChildID, Name: "GrandChild of Child1", ParentID: &child1ID})


    // Delete the parent
    err := testDB.DeleteComponent(parentID)
    if err != nil {
        t.Fatalf("Failed to delete parent component: %v", err)
    }

    // Check child1
    child1, err := testDB.GetComponentByID(child1ID)
    if err != nil {
        t.Fatalf("Failed to get child1 component: %v", err)
    }
    if child1.ParentID != nil {
        t.Errorf("Child1's ParentID expected to be NULL, got %s", *child1.ParentID)
    }

    // Check child2
    child2, err := testDB.GetComponentByID(child2ID)
    if err != nil {
        t.Fatalf("Failed to get child2 component: %v", err)
    }
    if child2.ParentID != nil {
        t.Errorf("Child2's ParentID expected to be NULL, got %s", *child2.ParentID)
    }

    // Grandchild should still have child1ID as parent, but child1 has no parent.
    // This is correct behavior. The grandchild's parent_id still points to child1's ID.
    // If child1 were also deleted, then grandchild's parent_id would also become NULL if it referenced child1.
    grandChild, err := testDB.GetComponentByID(grandChildID)
    if err != nil {
        t.Fatalf("Failed to get grandchild component: %v", err)
    }
    if grandChild.ParentID == nil || *grandChild.ParentID != child1ID {
        t.Errorf("Grandchild's ParentID expected to be %s, got %v", child1ID, grandChild.ParentID)
    }
}
