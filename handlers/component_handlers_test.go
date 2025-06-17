package handlers

import (
	"bytes"
	"component-service/db"
	"component-service/models"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// Helper function to get a pointer to an int.
func pint(i int) *int {
    return &i
}

// Existing tests for CreateComponentHandler, GetComponentsHandler, SingleComponentHandler (GET part) remain.
func TestCreateComponentHandler_Success(t *testing.T) {
	originalCreateComponentFunc := db.CreateComponentFunc
	db.CreateComponentFunc = func(component *models.Component) (int, error) {
		component.ID = 123 // Simulate ID assignment by DB
		return 123, nil
	}
	defer func() { db.CreateComponentFunc = originalCreateComponentFunc }()

	payload := models.Component{Name: "Test Component"}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "/components", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusCreated)
		t.Errorf("response body: %s", rr.Body.String())
	}

	var createdComponent models.Component
	if err := json.Unmarshal(rr.Body.Bytes(), &createdComponent); err != nil {
		t.Fatalf("could not unmarshal response: %v", err)
	}

	if createdComponent.ID != 123 {
		t.Errorf("handler returned unexpected ID: got %v want %v", createdComponent.ID, 123)
	}
	if createdComponent.Name != payload.Name {
		t.Errorf("handler returned unexpected name: got %v want %v", createdComponent.Name, payload.Name)
	}
}

func TestCreateComponentHandler_InvalidPayload(t *testing.T) {
	req, err := http.NewRequest("POST", "/components", bytes.NewBufferString("invalid json"))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid payload: got %v want %v", status, http.StatusBadRequest)
	}
}

func TestCreateComponentHandler_MissingName(t *testing.T) {
	payload := models.Component{Name: ""}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "/components", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for missing name: got %v want %v", status, http.StatusBadRequest)
		t.Errorf("response body: %s", rr.Body.String())
	}
}

func TestCreateComponentHandler_DBError(t *testing.T) {
	originalCreateComponentFunc := db.CreateComponentFunc
	db.CreateComponentFunc = func(component *models.Component) (int, error) {
		return 0, fmt.Errorf("simulated DB error")
	}
	defer func() { db.CreateComponentFunc = originalCreateComponentFunc }()

	payload := models.Component{Name: "Test DB Error"}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", "/components", bytes.NewBuffer(body))
	if err != nil {
		t.Fatalf("could not create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(CreateComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code for DB error: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestGetComponentsHandler_Success(t *testing.T) {
	originalFunc := db.GetTopLevelComponentsFunc
	mockComponents := []models.Component{
		{ID: 1, Name: "Parent 1", Children: []models.Component{{ID: 2, Name: "Child 1.1", ParentID: pint(1)}}},
		{ID: 3, Name: "Parent 2"},
	}
	db.GetTopLevelComponentsFunc = func() ([]models.Component, error) {
		return mockComponents, nil
	}
	defer func() { db.GetTopLevelComponentsFunc = originalFunc }()

	req, _ := http.NewRequest("GET", "/components", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetComponentsHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Errorf("Response body: %s", rr.Body.String())
	}

	var returnedComponents []models.Component
	if err := json.Unmarshal(rr.Body.Bytes(), &returnedComponents); err != nil {
		t.Fatalf("could not unmarshal response: %v", err)
	}
	if !reflect.DeepEqual(returnedComponents, mockComponents) {
		t.Errorf("handler returned unexpected body: got %v want %v", returnedComponents, mockComponents)
	}
}

func TestGetComponentsHandler_EmptyList(t *testing.T) {
	originalFunc := db.GetTopLevelComponentsFunc
	db.GetTopLevelComponentsFunc = func() ([]models.Component, error) {
		return []models.Component{}, nil
	}
	defer func() { db.GetTopLevelComponentsFunc = originalFunc }()

	req, _ := http.NewRequest("GET", "/components", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetComponentsHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	expected := "[]\n" // Empty JSON array with newline
	if rr.Body.String() != expected {
		t.Errorf("handler returned unexpected body for empty list: got %q want %q", rr.Body.String(), expected)
	}
}

func TestGetComponentsHandler_DBError(t *testing.T) {
	originalFunc := db.GetTopLevelComponentsFunc
	db.GetTopLevelComponentsFunc = func() ([]models.Component, error) {
		return nil, fmt.Errorf("simulated DB error")
	}
	defer func() { db.GetTopLevelComponentsFunc = originalFunc }()

	req, _ := http.NewRequest("GET", "/components", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(GetComponentsHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code for DB error: got %v want %v", status, http.StatusInternalServerError)
	}
}

func TestSingleComponentHandler_Get_Success(t *testing.T) {
	originalFunc := db.GetComponentFunc
	mockComponent := &models.Component{ID: 1, Name: "Test Component", Children: []models.Component{{ID:2, Name: "Child", ParentID: pint(1)}}}
	db.GetComponentFunc = func(id int) (*models.Component, error) {
		if id == 1 {
			return mockComponent, nil
		}
		return nil, fmt.Errorf("component not found with id %d", id)
	}
	defer func() { db.GetComponentFunc = originalFunc }()

	req, _ := http.NewRequest("GET", "/components/1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
		t.Errorf("Response body: %s", rr.Body.String())
	}

	var returnedComponent models.Component
	if err := json.Unmarshal(rr.Body.Bytes(), &returnedComponent); err != nil {
		t.Fatalf("could not unmarshal response: %v", err)
	}
	if !reflect.DeepEqual(&returnedComponent, mockComponent) {
		t.Errorf("handler returned unexpected body: got %+v want %+v", &returnedComponent, mockComponent)
	}
}

func TestSingleComponentHandler_Get_NotFound(t *testing.T) {
	originalFunc := db.GetComponentFunc
	db.GetComponentFunc = func(id int) (*models.Component, error) {
		return nil, fmt.Errorf("GetComponent: no component found with id %d", id)
	}
	defer func() { db.GetComponentFunc = originalFunc }()

	req, _ := http.NewRequest("GET", "/components/999", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("handler returned wrong status code for not found: got %v want %v", status, http.StatusNotFound)
	}
}

func TestSingleComponentHandler_Get_InvalidID(t *testing.T) {
	req, _ := http.NewRequest("GET", "/components/abc", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("handler returned wrong status code for invalid ID: got %v want %v", status, http.StatusBadRequest)
		t.Errorf("Response body: %s", rr.Body.String())
	}
}

func TestSingleComponentHandler_Get_DBError(t *testing.T) {
	originalFunc := db.GetComponentFunc
	db.GetComponentFunc = func(id int) (*models.Component, error) {
		return nil, fmt.Errorf("simulated generic DB error")
	}
	defer func() { db.GetComponentFunc = originalFunc }()

	req, _ := http.NewRequest("GET", "/components/1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("handler returned wrong status code for DB error: got %v want %v", status, http.StatusInternalServerError)
	}
}

// --- Tests for SingleComponentHandler (PUT part) ---
func TestSingleComponentHandler_Put_Success(t *testing.T) {
	originalUpdateFunc := db.UpdateComponentFunc
	originalGetFunc := db.GetComponentFunc

	updatedName := "Updated Component Name"
	componentID := 1
	updatePayload := models.Component{Name: updatedName, ParentID: pint(2)} // Example payload

	db.UpdateComponentFunc = func(id int, component *models.Component) error {
		if id == componentID && component.Name == updatedName && *component.ParentID == 2 {
			return nil // Success
		}
		return fmt.Errorf("mock UpdateComponentFunc error: unexpected args or failure (id: %d, name: %s, parent: %v)", id, component.Name, component.ParentID)
	}
	// This is the component that GetComponent should return *after* the update
	// Setting Children to nil to test omitempty behavior and simplify DeepEqual.
	mockUpdatedComponent := &models.Component{ID: componentID, Name: updatedName, ParentID: pint(2), Children: nil}
	db.GetComponentFunc = func(id int) (*models.Component, error) {
		if id == componentID {
			return mockUpdatedComponent, nil
		}
		return nil, fmt.Errorf("mock GetComponentFunc error: component not found post-update (id %d)", id)
	}

	defer func() {
		db.UpdateComponentFunc = originalUpdateFunc
		db.GetComponentFunc = originalGetFunc
	}()

	body, _ := json.Marshal(updatePayload)
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/components/%d", componentID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("PUT success: wrong status: got %v want %v. Body: %s", status, http.StatusOK, rr.Body.String())
	}

	var returnedComponent models.Component
	if err := json.Unmarshal(rr.Body.Bytes(), &returnedComponent); err != nil {
		t.Fatalf("PUT success: could not unmarshal: %v. Body: %s", err, rr.Body.String())
	}

	// Custom comparison for ParentID due to pointer comparison issues with DeepEqual
	parentIDsMatch := (returnedComponent.ParentID == nil && mockUpdatedComponent.ParentID == nil) ||
		(returnedComponent.ParentID != nil && mockUpdatedComponent.ParentID != nil && *returnedComponent.ParentID == *mockUpdatedComponent.ParentID)

	if returnedComponent.ID != mockUpdatedComponent.ID ||
		returnedComponent.Name != mockUpdatedComponent.Name ||
		!parentIDsMatch ||
		!reflect.DeepEqual(returnedComponent.Children, mockUpdatedComponent.Children) {
		t.Errorf("PUT success: unexpected body:\ngot:  %+v\nwant: %+v", returnedComponent, *mockUpdatedComponent)
	}
}

func TestSingleComponentHandler_Put_NotFound(t *testing.T) {
	originalUpdateFunc := db.UpdateComponentFunc
	db.UpdateComponentFunc = func(id int, component *models.Component) error {
		return fmt.Errorf("UpdateComponent: no component found with id %d", id) // Simulate not found
	}
	defer func() { db.UpdateComponentFunc = originalUpdateFunc }()

	payload := models.Component{Name: "Update attempt on non-existing"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/components/999", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("PUT not found: wrong status: got %v want %v. Body: %s", status, http.StatusNotFound, rr.Body.String())
	}
}

func TestSingleComponentHandler_Put_InvalidID(t *testing.T) {
	payload := models.Component{Name: "Update with invalid path ID"}
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/components/abc", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("PUT invalid ID: wrong status: got %v want %v. Body: %s", status, http.StatusBadRequest, rr.Body.String())
	}
}

func TestSingleComponentHandler_Put_InvalidPayload(t *testing.T) {
	req, _ := http.NewRequest("PUT", "/components/1", bytes.NewBufferString("invalid json"))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("PUT invalid payload: wrong status: got %v want %v. Body: %s", status, http.StatusBadRequest, rr.Body.String())
	}
}

func TestSingleComponentHandler_Put_MissingName(t *testing.T) {
	payload := models.Component{Name: ""} // Empty name
	body, _ := json.Marshal(payload)
	req, _ := http.NewRequest("PUT", "/components/1", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("PUT missing name: wrong status: got %v want %v. Body: %s", status, http.StatusBadRequest, rr.Body.String())
	}
}

func TestSingleComponentHandler_Put_SelfParent(t *testing.T) {
	originalUpdateFunc := db.UpdateComponentFunc
	db.UpdateComponentFunc = func(id int, component *models.Component) error {
		// This error is now directly from the handler based on the DB error message
		return fmt.Errorf("UpdateComponent: component cannot be its own parent (id %d)", id)
	}
	defer func() { db.UpdateComponentFunc = originalUpdateFunc }()

	parentID := 1
	payload := models.Component{Name: "Self Parent Update", ParentID: &parentID}
	body, _ := json.Marshal(payload)
	// The request ID (1) and the ParentID in payload (&parentID which is 1) are the same.
	req, _ := http.NewRequest("PUT", fmt.Sprintf("/components/%d", parentID), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("PUT self-parent: wrong status: got %v want %v. Body: %s", status, http.StatusBadRequest, rr.Body.String())
	}
}


// --- Tests for SingleComponentHandler (DELETE part) ---
func TestSingleComponentHandler_Delete_Success(t *testing.T) {
	originalDeleteFunc := db.DeleteComponentFunc
	db.DeleteComponentFunc = func(id int) error {
		if id == 1 {
			return nil // Success
		}
		return fmt.Errorf("mock DeleteComponentFunc: component not found or unexpected id")
	}
	defer func() { db.DeleteComponentFunc = originalDeleteFunc }()

	req, _ := http.NewRequest("DELETE", "/components/1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNoContent {
		t.Errorf("DELETE success: wrong status: got %v want %v. Body: %s", status, http.StatusNoContent, rr.Body.String())
	}
}

func TestSingleComponentHandler_Delete_NotFound(t *testing.T) {
	originalDeleteFunc := db.DeleteComponentFunc
	db.DeleteComponentFunc = func(id int) error {
		return fmt.Errorf("DeleteComponent: no component found with id %d", id) // Simulate not found
	}
	defer func() { db.DeleteComponentFunc = originalDeleteFunc }()

	req, _ := http.NewRequest("DELETE", "/components/999", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusNotFound {
		t.Errorf("DELETE not found: wrong status: got %v want %v. Body: %s", status, http.StatusNotFound, rr.Body.String())
	}
}

func TestSingleComponentHandler_Delete_InvalidID(t *testing.T) {
	req, _ := http.NewRequest("DELETE", "/components/abc", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("DELETE invalid ID: wrong status: got %v want %v. Body: %s", status, http.StatusBadRequest, rr.Body.String())
	}
}

func TestSingleComponentHandler_Delete_DBError(t *testing.T) {
	originalDeleteFunc := db.DeleteComponentFunc
	db.DeleteComponentFunc = func(id int) error {
		return fmt.Errorf("simulated generic DB error on delete")
	}
	defer func() { db.DeleteComponentFunc = originalDeleteFunc }()

	req, _ := http.NewRequest("DELETE", "/components/1", nil)
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(SingleComponentHandler)
	handler.ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("DELETE DB error: wrong status: got %v want %v. Body: %s", status, http.StatusInternalServerError, rr.Body.String())
	}
}
