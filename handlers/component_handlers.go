package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings" // Required for path parsing

	"component-service/db"
	"component-service/models"
	// "github.com/gorilla/mux" // REMOVED
)

// Base path for components, used for ID extraction in SingleComponentHandler
const componentsBasePath = "/components/"

func CreateComponentHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Only POST method is allowed for "+r.URL.Path, http.StatusMethodNotAllowed)
		return
	}

	var component models.Component
	if err := json.NewDecoder(r.Body).Decode(&component); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if component.Name == "" {
		http.Error(w, "Component name cannot be empty", http.StatusBadRequest)
		return
	}
	if component.ID != 0 {
		http.Error(w, "Component ID cannot be set on create", http.StatusBadRequest)
		return
	}

	newID, err := db.CreateComponent(&component)
	if err != nil {
		log.Printf("Error creating component: %v", err)
		http.Error(w, "Failed to create component", http.StatusInternalServerError)
		return
	}
	component.ID = newID

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	if err := json.NewEncoder(w).Encode(component); err != nil {
		log.Printf("Error encoding response: %v", err)
	}
}

func GetComponentsHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Only GET method is allowed for "+r.URL.Path, http.StatusMethodNotAllowed)
		return
	}
    // This handler is for the root /components or /components/ path (when not specifying an ID).
    // It should not handle /components/some_id
    if strings.TrimPrefix(r.URL.Path, componentsBasePath) != "" && r.URL.Path != "/components" {
         // If there's anything after componentsBasePath, it's not for this handler.
         // Or if the path is exactly /components
        http.NotFound(w, r)
        return
    }

	components, err := db.GetTopLevelComponents()
	if err != nil {
		log.Printf("Error getting top-level components: %v", err)
		http.Error(w, "Failed to retrieve components", http.StatusInternalServerError)
		return
	}

	if components == nil {
		components = []models.Component{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(components); err != nil {
		log.Printf("Error encoding components response: %v", err)
	}
}

// SingleComponentHandler manages GET, PUT, DELETE for /components/{id}
func SingleComponentHandler(w http.ResponseWriter, r *http.Request) {
    // Extract ID from path: /components/{id}
    idStr := strings.TrimPrefix(r.URL.Path, componentsBasePath)
    // Further trim if there's a trailing slash, e.g. /components/123/
    idStr = strings.TrimSuffix(idStr, "/")

    if idStr == "" {
        // This case should ideally be caught by routing /components/ to this
        // and /components to GetComponentsHandler. If it still occurs,
        // it means the path was literally "/components/" without an ID.
        http.Error(w, "Component ID missing in URL path after "+componentsBasePath, http.StatusBadRequest)
        return
    }

    id, err := strconv.Atoi(idStr)
    if err != nil {
        http.Error(w, "Invalid component ID format in URL path: '"+idStr+"'", http.StatusBadRequest)
        return
    }

    switch r.Method {
    case http.MethodGet:
        getComponent(w, r, id)
    case http.MethodPut:
        updateComponent(w, r, id)
    case http.MethodDelete:
        deleteComponent(w, r, id)
    default:
        http.Error(w, "Method not allowed for "+r.URL.Path, http.StatusMethodNotAllowed)
    }
}


func getComponent(w http.ResponseWriter, r *http.Request, id int) {
	component, err := db.GetComponent(id)
	if err != nil {
		if strings.Contains(err.Error(), "no component found") {
			http.Error(w, fmt.Sprintf("Component with ID %d not found", id), http.StatusNotFound)
		} else {
			log.Printf("Error getting component %d: %v", id, err)
			http.Error(w, "Failed to retrieve component", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(component); err != nil {
		log.Printf("Error encoding component %d response: %v", id, err)
	}
}

func updateComponent(w http.ResponseWriter, r *http.Request, id int) {
	var componentUpdates models.Component
	if err := json.NewDecoder(r.Body).Decode(&componentUpdates); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	if componentUpdates.Name == "" {
		http.Error(w, "Component name cannot be empty for update", http.StatusBadRequest)
		return
	}
	if componentUpdates.ID != 0 && componentUpdates.ID != id {
		http.Error(w, fmt.Sprintf("Component ID in body (%d) does not match ID in path (%d)", componentUpdates.ID, id), http.StatusBadRequest)
		return
	}

	err := db.UpdateComponent(id, &componentUpdates)
	if err != nil {
		if strings.Contains(err.Error(), "no component found") {
			http.Error(w, fmt.Sprintf("Component with ID %d not found for update", id), http.StatusNotFound)
		} else if strings.Contains(err.Error(), "cannot be its own parent") {
			http.Error(w, fmt.Sprintf("Update failed for component ID %d: %s", id, err.Error()), http.StatusBadRequest)
		} else if strings.Contains(err.Error(), "violates foreign key constraint") {
			http.Error(w, fmt.Sprintf("Update failed for component ID %d: invalid parent_id specified", id), http.StatusBadRequest)
		} else {
			log.Printf("Error updating component %d: %v", id, err)
			http.Error(w, fmt.Sprintf("Failed to update component: %v", err), http.StatusInternalServerError)
		}
		return
	}

	updatedComponent, errGet := db.GetComponent(id)
	if errGet != nil {
		if strings.Contains(errGet.Error(), "no component found") {
			http.Error(w, fmt.Sprintf("Component with ID %d not found after update", id), http.StatusNotFound)
		} else {
			log.Printf("Error getting component %d after update: %v", id, errGet)
			http.Error(w, "Failed to retrieve component after update", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(updatedComponent); err != nil {
		log.Printf("Error encoding updated component %d response: %v", id, err)
	}
}

func deleteComponent(w http.ResponseWriter, r *http.Request, id int) {
	err := db.DeleteComponent(id)
	if err != nil {
		if strings.Contains(err.Error(), "no component found") {
			http.Error(w, fmt.Sprintf("Component with ID %d not found for deletion", id), http.StatusNotFound)
		} else {
			log.Printf("Error deleting component %d: %v", id, err)
			http.Error(w, "Failed to delete component", http.StatusInternalServerError)
		}
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
