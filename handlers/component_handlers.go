package handlers

import (
	"encoding/json"
	"net/http"

	"explorer-server/db"
	"explorer-server/models"
)

// ComponentHandler holds the database context for component operations.
type ComponentHandler struct {
	DB *db.DB
}

// NewComponentHandler creates a new ComponentHandler.
func NewComponentHandler(database *db.DB) *ComponentHandler {
	return &ComponentHandler{DB: database}
}

// CreateComponentHandler handles POST requests to /components.
// It expects a models.Component in the request body, including its ID.
func (h *ComponentHandler) CreateComponentHandler(w http.ResponseWriter, r *http.Request) {
	var component models.Component
	if err := json.NewDecoder(r.Body).Decode(&component); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Basic validation: Check if ID and Name are provided
	if component.ID == "" || component.Name == "" {
		http.Error(w, "Component ID and Name are required", http.StatusBadRequest)
		return
	}

	if err := h.DB.CreateComponent(&component); err != nil {
		// This could be a duplicate ID error or other database constraint violation
		// For simplicity, returning 500, but could be more specific (e.g., 409 Conflict)
		http.Error(w, "Failed to create component: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(component); err != nil {
		// If encoding fails, it's an internal server error, though the component was created.
		// Log this error on the server side.
		// Consider logging the error properly in a real application.
		http.Error(w, "Failed to write response: "+err.Error(), http.StatusInternalServerError)
	}
}

// UpdateComponentHandler handles PUT requests to /components/{id}.
func (h *ComponentHandler) UpdateComponentHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 || pathParts[2] == "" { // Expecting /components/{id}
		http.Error(w, "Component ID is missing in URL path", http.StatusBadRequest)
		return
	}
	id := pathParts[len(pathParts)-1] // Get the last part as ID

	var componentUpdates models.Component
	if err := json.NewDecoder(r.Body).Decode(&componentUpdates); err != nil {
		http.Error(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	// Basic validation: Name is required for an update
	if componentUpdates.Name == "" {
		http.Error(w, "Component Name is required for update", http.StatusBadRequest)
		return
	}

	// The ID for update comes from the URL, not the body.
	// componentUpdates.ID will be ignored by the db.UpdateComponent if it expects the id as a separate param.
	// If db.UpdateComponent used componentUpdates.ID, we'd need to set it here: componentUpdates.ID = id

	err := h.DB.UpdateComponent(id, &componentUpdates)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Component not found to update", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to update component: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Fetch the updated component to return it, as UpdateComponent itself doesn't return the model
	// This is optional; some APIs might just return 200 OK or 204 No Content.
	// For consistency with Create, returning the updated model.
	updatedComponent, err := h.DB.GetComponentByID(id)
	if err != nil {
		// This would be unusual if the update succeeded.
		http.Error(w, "Failed to retrieve updated component: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedComponent); err != nil {
		http.Error(w, "Failed to write response: "+err.Error(), http.StatusInternalServerError)
	}
}

// DeleteComponentHandler handles DELETE requests to /components/{id}.
func (h *ComponentHandler) DeleteComponentHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 || pathParts[2] == "" { // Expecting /components/{id}
		http.Error(w, "Component ID is missing in URL path", http.StatusBadRequest)
		return
	}
	id := pathParts[len(pathParts)-1] // Get the last part as ID

	err := h.DB.DeleteComponent(id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Component not found to delete", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to delete component: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// GetComponentByIDHandler handles GET requests to /components/{id}.
func (h *ComponentHandler) GetComponentByIDHandler(w http.ResponseWriter, r *http.Request) {
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 3 || pathParts[2] == "" { // Expecting /components/{id}
		http.Error(w, "Component ID is missing in URL path", http.StatusBadRequest)
		return
	}
	id := pathParts[len(pathParts)-1] // Get the last part as ID

	component, err := h.DB.GetComponentByID(id)
	if err != nil {
		if err == sql.ErrNoRows {
			http.Error(w, "Component not found", http.StatusNotFound)
		} else {
			http.Error(w, "Failed to retrieve component: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(component); err != nil {
		http.Error(w, "Failed to write response: "+err.Error(), http.StatusInternalServerError)
	}
}

// GetAllComponentsHandler handles GET requests to /components.
func (h *ComponentHandler) GetAllComponentsHandler(w http.ResponseWriter, r *http.Request) {
	components, err := h.DB.GetAllComponents()
	if err != nil {
		http.Error(w, "Failed to retrieve components: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(components); err != nil {
		http.Error(w, "Failed to write response: "+err.Error(), http.StatusInternalServerError)
	}
}
