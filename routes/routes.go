package routes

import (
	"net/http"

	"component-service/handlers"
)

// RegisterComponentRoutes sets up all the routes for the component service.
func RegisterComponentRoutes() {
	// Handles GET /components (list all) and POST /components (create new)
	http.HandleFunc("/components", func(w http.ResponseWriter, r *http.Request) {
		// This handler is specifically for the "/components" path.
		// Requests for "/components/" (with a trailing slash and potentially an ID)
		// will be routed to handlers.SingleComponentHandler by the rule below.
		if r.URL.Path == "/components" {
			switch r.Method {
			case http.MethodGet:
				handlers.GetComponentsHandler(w, r)
			case http.MethodPost:
				handlers.CreateComponentHandler(w, r)
			default:
				http.Error(w, "Method not allowed for /components", http.StatusMethodNotAllowed)
			}
		} else {
			// This 'else' branch would be hit if this handler was registered for a broader path
			// and the request path was not exactly "/components" but also not caught by
			// the more specific "/components/" handler. For example, a request to "/components-something".
			// Given the specific registration for "/components" and "/components/",
			// this specific http.NotFound might not be strictly necessary as DefaultServeMux
			// would yield a 404 anyway if no other handler matches.
			// However, it clearly defines the scope of this handler.
			http.NotFound(w, r)
		}
	})

	// Handles GET /components/{id}, PUT /components/{id}, DELETE /components/{id}
	// The trailing slash in the pattern is key for DefaultServeMux:
	// it makes this a prefix match for any path starting with "/components/".
	// handlers.SingleComponentHandler is responsible for parsing the ID from the path
	// and dispatching based on the HTTP method (GET, PUT, DELETE).
	http.HandleFunc("/components/", handlers.SingleComponentHandler)
}
