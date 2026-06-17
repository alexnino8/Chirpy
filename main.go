package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync/atomic"
)

type apiConfig struct {
	fileserverHits atomic.Int32
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits.Add(1)
		next.ServeHTTP(w, r)
	})
}

func (cfg *apiConfig) writeRequestsCounter(w http.ResponseWriter, r *http.Request) {
	currentHits := cfg.fileserverHits.Load()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	out := fmt.Sprintf(`<html>
		<body>
			<h1>Welcome, Chirpy Admin</h1>
			<p>Chirpy has been visited %d times!</p>
		</body>
		</html>`, currentHits)
	fmt.Fprint(w, out)
}

func (cfg *apiConfig) resetFileserverHits(w http.ResponseWriter, _ *http.Request) {
	cfg.fileserverHits.Store(0)
	w.WriteHeader(http.StatusOK)
}

func readinessEndpoint(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func validateChirp(w http.ResponseWriter, r *http.Request) {
	type chirp struct {
		Body string `json:"body"`
	}

	type returnVal struct {
		Valid bool   `json:"valid"`
		Error string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	body := chirp{}
	res := returnVal{}
	err := decoder.Decode(&body)
	if err != nil {
		res.Error = fmt.Sprintf("Error decoding request: %v", err)
	}

	if len(body.Body) > 140 {
		res.Error = "Chirp is too long"
		w.WriteHeader(400)
	} else {
		res.Valid = true
		w.WriteHeader(200)
	}

	dat, err := json.Marshal(res)
	if err != nil {
		log.Printf("Error marshaling JSON: %s", err)
		w.WriteHeader(500)
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(dat)
}

func main() {

	apiConfig := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	wrappedHandler := apiConfig.middlewareMetricsInc(fileServerHandler)

	mux := http.NewServeMux()
	mux.Handle("/app/", wrappedHandler)
	mux.Handle("/assets/logo.png", http.FileServer(http.Dir(".")))

	//api endpoints
	mux.HandleFunc("GET /api/healthz", readinessEndpoint)
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	// admin endpoints
	mux.HandleFunc("GET /admin/metrics", apiConfig.writeRequestsCounter)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetFileserverHits)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Fatal(server.ListenAndServe())

}
