package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
)

func main() {
	client, err := NewInfraiEmailClient(os.Getenv("INFRAI_API_KEY"))
	if err != nil {
		log.Fatal(err)
	}
	verificationBaseURL := os.Getenv("VERIFICATION_BASE_URL")
	service, err := NewVerificationService(client, verificationBaseURL)
	if err != nil {
		log.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", signupHandler(service))
	server := &http.Server{Addr: ":8080", Handler: mux}
	log.Printf("verification service listening on %s", server.Addr)
	log.Fatal(server.ListenAndServe())
}

func signupHandler(service *VerificationService) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var input SignupRequest
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signup request"})
			return
		}
		result, err := service.Start(r.Context(), input)
		if err != nil {
			var apiErr *InfraiError
			if errors.As(err, &apiErr) && apiErr.HTTPStatus >= 400 && apiErr.HTTPStatus < 500 {
				writeJSON(w, apiErr.HTTPStatus, map[string]string{"error": apiErr.Message})
				return
			}
			writeJSON(w, http.StatusBadGateway, map[string]string{"error": "verification notification was not accepted"})
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
