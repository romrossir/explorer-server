package handlers

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"explorer-server/db"
	"explorer-server/models"

	_ "github.com/lib/pq" // PostgreSQL driver
)

var testDB *db.DB
var compHandler *ComponentHandler

// TestMain sets up the database connection and schema for all tests in this package.
func TestMain(m *testing.M) {
	testDatabaseURL := os.Getenv("TEST_DATABASE_URL")
	if testDatabaseURL == "" {
		log.Println("TEST_DATABASE_URL not set, skipping handler tests.")
	}

	var err error
	if testDatabaseURL != "" {
		conn, errSqlOpen := sql.Open("postgres", testDatabaseURL)
		if errSqlOpen != nil {
			log.Fatalf("Failed to open test database connection: %v", errSqlOpen)
		}
		if errPing := conn.Ping(); errPing != nil {
			conn.Close()
			log.Fatalf("Failed to ping test database: %v", errPing)
		}
		testDB = &db.DB{DB: conn} // Correctly initialize the custom DB struct

		schemaPath := filepath.Join("..", "schema.sql")
		schemaBytes, errRead := ioutil.ReadFile(schemaPath)
		if errRead != nil {
			testDB.Close()
			log.Fatalf("Failed to read schema.sql at %s: %v", schemaPath, errRead)
		}
		if _, errExec := testDB.Exec(string(schemaBytes)); errExec != nil {
			testDB.Close()
			log.Fatalf("Failed to apply schema: %v", errExec)
		}
		log.Println("Test database schema applied successfully for handlers.")
		compHandler = NewComponentHandler(testDB)
	}

	exitCode := m.Run()

	if testDB != nil {
		testDB.Close()
		log.Println("Test database connection closed for handlers.")
	}
	os.Exit(exitCode)
}

// clearComponentsTableHandlerTests is a helper to delete all rows from components.
func clearComponentsTableHandlerTests(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test that requires DB.")
		return
	}
	_, err := testDB.Exec("DELETE FROM components")
	if err != nil {
		t.Fatalf("Failed to clear components table: %v", err)
	}
}

// Helper to create a component directly in DB for test setup
func createDBTestComponent(t *testing.T, comp *models.Component) {
	t.Helper()
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, cannot create test component.")
		return
	}
	err := testDB.CreateComponent(comp)
	if err != nil {
		t.Fatalf("Failed to create test component in DB (ID: %s): %v", comp.ID, err)
	}
}


func TestCreateComponentHandler(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}
	clearComponentsTableHandlerTests(t)

	parentID := "parent-handler-create"
	createDBTestComponent(t, &models.Component{ID: parentID, Name: "Parent For Handler Create"})


	tests := []struct {
		name         string
		payload      interface{}
		expectedCode int
		expectedBody *models.Component // Only if successful
	}{
		{
			name: "valid component no parent",
			payload: models.Component{ID: "handler-create-1", Name: "Handler Create 1"},
			expectedCode: http.StatusCreated,
			expectedBody: &models.Component{ID: "handler-create-1", Name: "Handler Create 1"},
		},
		{
			name: "valid component with parent",
			payload: models.Component{ID: "handler-create-2", Name: "Handler Create 2", ParentID: &parentID},
			expectedCode: http.StatusCreated,
			expectedBody: &models.Component{ID: "handler-create-2", Name: "Handler Create 2", ParentID: &parentID},
		},
		{
			name: "invalid JSON payload",
			payload: "not a json",
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "missing component ID",
			payload: models.Component{Name: "Missing ID"},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "missing component Name",
			payload: models.Component{ID: "handler-create-no-name"},
			expectedCode: http.StatusBadRequest,
		},
		{
			name: "duplicate component ID",
			payload: models.Component{ID: "handler-create-1", Name: "Duplicate"}, // handler-create-1 created in first test case
			expectedCode: http.StatusInternalServerError, // Or 409 if db layer returns a specific error
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ensure first component is created for duplicate test, then clear for next test runs if needed
			if tt.name == "duplicate component ID" {
				// No specific setup here, relies on previous successful creation.
				// If tests run in parallel or order changes, this might need explicit setup.
			} else if tt.name != "valid component no parent" { // Avoid double clear if first test
				// For most tests, ensure a clean slate for this specific sub-test,
				// especially if previous sub-tests might have left conflicting data.
				// However, the current duplicate test relies on the state from "valid component no parent".
				// This highlights complexity in ordered sub-tests vs. fully isolated ones.
				// For now, let's proceed, but ideally, each sub-test should set up its own specific preconditions.
			}


			var bodyBytes []byte
			var err error
			if tt.payload != nil {
				bodyBytes, err = json.Marshal(tt.payload)
				if err != nil {
					t.Fatalf("Failed to marshal payload: %v", err)
				}
			}

			req, err := http.NewRequest(http.MethodPost, "/components", bytes.NewBuffer(bodyBytes))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			rr := httptest.NewRecorder()
			compHandler.CreateComponentHandler(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", rr.Code, tt.expectedCode, rr.Body.String())
			}

			if tt.expectedCode == http.StatusCreated {
				if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
					t.Errorf("handler returned wrong content type: got %s want application/json", contentType)
				}
				var actualBody models.Component
				if err := json.NewDecoder(rr.Body).Decode(&actualBody); err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}
				if !reflect.DeepEqual(&actualBody, tt.expectedBody) {
					t.Errorf("handler returned unexpected body: got %v want %v", &actualBody, tt.expectedBody)
				}
			}
		})
	}
}

func TestGetComponentByIDHandler(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}
	clearComponentsTableHandlerTests(t)

	comp := &models.Component{ID: "handler-get-1", Name: "Handler Get 1"}
	createDBTestComponent(t, comp)

	tests := []struct {
		name         string
		id           string
		expectedCode int
		expectedBody *models.Component
	}{
		{"get existing component", comp.ID, http.StatusOK, comp},
		{"get non-existent component", "non-existent-id", http.StatusNotFound, nil},
		{"get component with empty id in path", "", http.StatusBadRequest, nil}, // Test router/handler path parsing robustness
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req, err := http.NewRequest(http.MethodGet, "/components/"+tt.id, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			rr := httptest.NewRecorder()
			compHandler.GetComponentByIDHandler(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", rr.Code, tt.expectedCode, rr.Body.String())
			}

			if tt.expectedCode == http.StatusOK {
				if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
					t.Errorf("handler returned wrong content type: got %s want application/json", contentType)
				}
				var actualBody models.Component
				if err := json.NewDecoder(rr.Body).Decode(&actualBody); err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}
				if !reflect.DeepEqual(&actualBody, tt.expectedBody) {
					t.Errorf("handler returned unexpected body: got %v want %v", &actualBody, tt.expectedBody)
				}
			}
		})
	}
}

func TestGetAllComponentsHandler(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}

	comp1 := &models.Component{ID: "handler-getall-1", Name: "B Handler GetAll"}
	comp2 := &models.Component{ID: "handler-getall-2", Name: "A Handler GetAll"}

	// Expected order due to DB sort by name
	expectedSorted := []*models.Component{comp2, comp1}
	sort.Slice(expectedSorted, func(i, j int) bool { // Ensure test expectation matches db sort
        return expectedSorted[i].Name < expectedSorted[j].Name
    })


	tests := []struct {
		name         string
		setup        func()
		expectedCode int
		expectedBody []*models.Component
	}{
		{
			name: "get all when components exist",
			setup: func() {
				clearComponentsTableHandlerTests(t)
				createDBTestComponent(t, comp1)
				createDBTestComponent(t, comp2)
			},
			expectedCode: http.StatusOK,
			expectedBody: expectedSorted,
		},
		{
			name: "get all when no components exist",
			setup: func() {
				clearComponentsTableHandlerTests(t)
			},
			expectedCode: http.StatusOK,
			expectedBody: []*models.Component{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.setup()
			req, err := http.NewRequest(http.MethodGet, "/components", nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			rr := httptest.NewRecorder()
			compHandler.GetAllComponentsHandler(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", rr.Code, tt.expectedCode, rr.Body.String())
			}

			if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
				t.Errorf("handler returned wrong content type: got %s want application/json", contentType)
			}

			var actualBody []*models.Component
			// For empty array, Decode might err if body is empty vs "[]"
			// httptest.Recorder.Body is a *bytes.Buffer, which is empty if nothing written.
			// json.NewDecoder will error on empty input if it expects an array/object.
			if rr.Body.Len() > 0 { // Check if there's anything to decode
				if err := json.NewDecoder(rr.Body).Decode(&actualBody); err != nil {
					// If expecting empty array, and body is literally empty (not "[]"), this will fail.
					// The handler should ensure it writes "[]" for empty lists.
					if !(len(tt.expectedBody) == 0 && strings.TrimSpace(rr.Body.String()) == "") {
						t.Fatalf("Failed to decode response body: %v. Body content: '%s'", err, rr.Body.String())
					}
				}
			}
			// If actualBody remained nil (due to empty body and empty expected) and expectedBody is empty slice, it's a pass.
			if actualBody == nil && len(tt.expectedBody) == 0 {
				// This is fine, signifies an empty list was correctly returned as empty or nil slice
			} else if !reflect.DeepEqual(actualBody, tt.expectedBody) {
				t.Errorf("handler returned unexpected body: got %#v want %#v", actualBody, tt.expectedBody)
			}
		})
	}
}


func TestUpdateComponentHandler(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}

	originalCompID := "handler-update-1"
	originalParentID := "parent-handler-update-orig"
	newParentID := "parent-handler-update-new"


	tests := []struct {
		name         string
		idToUpdate   string
		payload      interface{}
		setup        func()
		expectedCode int
		expectedBody *models.Component // Only if successful (200)
	}{
		{
			name:       "valid update name and parent",
			idToUpdate: originalCompID,
			payload:    models.Component{Name: "Updated Name", ParentID: &newParentID},
			setup: func() {
				clearComponentsTableHandlerTests(t)
				createDBTestComponent(t, &models.Component{ID: originalParentID, Name: "Original Parent"})
				createDBTestComponent(t, &models.Component{ID: newParentID, Name: "New Parent"})
				createDBTestComponent(t, &models.Component{ID: originalCompID, Name: "Original Name", ParentID: &originalParentID})
			},
			expectedCode: http.StatusOK,
			expectedBody: &models.Component{ID: originalCompID, Name: "Updated Name", ParentID: &newParentID},
		},
		{
			name:       "update to remove parent",
			idToUpdate: originalCompID,
			payload:    models.Component{Name: "Updated Name No Parent", ParentID: nil},
			setup: func() {
				clearComponentsTableHandlerTests(t)
				createDBTestComponent(t, &models.Component{ID: originalParentID, Name: "Original Parent"})
				createDBTestComponent(t, &models.Component{ID: originalCompID, Name: "Original Name", ParentID: &originalParentID})
			},
			expectedCode: http.StatusOK,
			expectedBody: &models.Component{ID: originalCompID, Name: "Updated Name No Parent", ParentID: nil},
		},
		{
			name:       "update non-existent component",
			idToUpdate: "non-existent-id",
			payload:    models.Component{Name: "Update Non Existent"},
			setup:      clearComponentsTableHandlerTests,
			expectedCode: http.StatusNotFound,
		},
		{
			name:       "invalid JSON payload for update",
			idToUpdate: originalCompID,
			payload:    "not a json",
			setup: func() { // Ensure component exists for this path, though payload is bad
				clearComponentsTableHandlerTests(t)
				createDBTestComponent(t, &models.Component{ID: originalCompID, Name: "Exists for bad JSON test"})
			},
			expectedCode: http.StatusBadRequest,
		},
		{
			name:       "missing name in update payload",
			idToUpdate: originalCompID,
			payload:    models.Component{ParentID: &newParentID}, // Name is empty
			setup: func() {
				clearComponentsTableHandlerTests(t)
				createDBTestComponent(t, &models.Component{ID: newParentID, Name: "New Parent"})
				createDBTestComponent(t, &models.Component{ID: originalCompID, Name: "Exists for missing name test"})
			},
			expectedCode: http.StatusBadRequest,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			var bodyBytes []byte
			var err error
			if tt.payload != nil {
				bodyBytes, err = json.Marshal(tt.payload)
				if err != nil {
					t.Fatalf("Failed to marshal payload: %v", err)
				}
			}

			req, err := http.NewRequest(http.MethodPut, "/components/"+tt.idToUpdate, bytes.NewBuffer(bodyBytes))
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			rr := httptest.NewRecorder()
			compHandler.UpdateComponentHandler(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", rr.Code, tt.expectedCode, rr.Body.String())
			}

			if tt.expectedCode == http.StatusOK {
				if contentType := rr.Header().Get("Content-Type"); contentType != "application/json" {
					t.Errorf("handler returned wrong content type: got %s want application/json", contentType)
				}
				var actualBody models.Component
				if err := json.NewDecoder(rr.Body).Decode(&actualBody); err != nil {
					t.Fatalf("Failed to decode response body: %v", err)
				}
				// Compare with expectedBody, ensuring ID matches idToUpdate
				if tt.expectedBody.ID == "" { // If not set in test struct, fill from path
					tt.expectedBody.ID = tt.idToUpdate
				}
				if !reflect.DeepEqual(&actualBody, tt.expectedBody) {
					t.Errorf("handler returned unexpected body: got %#v want %#v", &actualBody, tt.expectedBody)
				}
			}
		})
	}
}

func TestDeleteComponentHandler(t *testing.T) {
	if testDB == nil {
		t.Skip("TEST_DATABASE_URL not set, skipping test.")
		return
	}

	compID := "handler-delete-1"

	tests := []struct {
		name         string
		idToDelete   string
		setup        func()
		expectedCode int
	}{
		{
			name:       "delete existing component",
			idToDelete: compID,
			setup: func() {
				clearComponentsTableHandlerTests(t)
				createDBTestComponent(t, &models.Component{ID: compID, Name: "To Be Deleted By Handler"})
			},
			expectedCode: http.StatusNoContent,
		},
		{
			name:       "delete non-existent component",
			idToDelete: "non-existent-id",
			setup:      clearComponentsTableHandlerTests,
			expectedCode: http.StatusNotFound,
		},
		{
			name: "delete component with empty id in path",
			idToDelete: "", // Test router/handler path parsing robustness
			setup: clearComponentsTableHandlerTests,
			expectedCode: http.StatusBadRequest, // Or 404 if router handles it as not found
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setup != nil {
				tt.setup()
			}

			req, err := http.NewRequest(http.MethodDelete, "/components/"+tt.idToDelete, nil)
			if err != nil {
				t.Fatalf("Failed to create request: %v", err)
			}
			rr := httptest.NewRecorder()
			compHandler.DeleteComponentHandler(rr, req)

			if rr.Code != tt.expectedCode {
				t.Errorf("handler returned wrong status code: got %v want %v. Body: %s", rr.Code, tt.expectedCode, rr.Body.String())
			}

			if tt.expectedCode == http.StatusNoContent {
				// Verify component is actually deleted from DB
				_, errDb := testDB.GetComponentByID(tt.idToDelete)
				if errDb != sql.ErrNoRows {
					t.Errorf("component %s not actually deleted from DB, err: %v", tt.idToDelete, errDb)
				}
			}
		})
	}
}
