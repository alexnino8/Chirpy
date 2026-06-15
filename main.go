package main

import (
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

func main() {

	apiConfig := apiConfig{
		fileserverHits: atomic.Int32{},
	}

	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	wrappedHandler := apiConfig.middlewareMetricsInc(fileServerHandler)

	mux := http.NewServeMux()
	mux.Handle("/app/", wrappedHandler)
	mux.Handle("/assets/logo.png", http.FileServer(http.Dir(".")))

	//

	mux.HandleFunc("GET /api/healthz", readinessEndpoint)
	mux.HandleFunc("GET /admin/metrics", apiConfig.writeRequestsCounter)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetFileserverHits)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Fatal(server.ListenAndServe())

}
