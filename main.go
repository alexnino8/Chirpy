package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync/atomic"
	"time"

	"github.com/alexnino8/Chirpy/internal/chirp"
	"github.com/alexnino8/Chirpy/internal/database"
	"github.com/google/uuid"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	fileserverHits atomic.Int32
	dbQueries      *database.Queries
}

// user struct
type User struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	Email     string    `json:"email"`
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

func (cfg *apiConfig) createUser(w http.ResponseWriter, r *http.Request) {
	//define the struct to decode the request into
	type parameters struct {
		Email string `json:"email"`
	}

	// decode the request in the new instance of the parameters struct
	decoder := json.NewDecoder(r.Body)
	params := parameters{}
	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 400, fmt.Sprintf("Error decoding request: %s", err))
		return
	}

	// send the query to create the new user -> get a database.User
	user, err := cfg.dbQueries.CreateUser(r.Context(), params.Email)
	if err != nil {
		respondWithError(w, 500, fmt.Sprintf("Couldn't create user: %s", err))
	}

	// respond with the details of the new user created (using User struct)
	respondWithJson(w, 201, User{
		ID:        user.ID,
		CreatedAt: user.CreatedAt,
		UpdatedAt: user.UpdatedAt,
		Email:     user.Email,
	})

}

func respondWithJson(w http.ResponseWriter, code int, payload any) {
	dat, err := json.Marshal(payload)
	if err != nil {
		log.Printf("Error marshaling JSON: %s", err)
		w.WriteHeader(500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type errorResponse struct {
		Error string `json:"error"`
	}

	respondWithJson(w, code, errorResponse{Error: msg})
}

func validateChirp(w http.ResponseWriter, r *http.Request) {
	type parameters struct {
		Body string `json:"body"`
	}

	type cleanedResponse struct {
		CleanedBody string `json:"cleaned_body"`
	}

	type errorResponse struct {
		Error string `json:"error"`
	}

	decoder := json.NewDecoder(r.Body)
	params := parameters{}

	err := decoder.Decode(&params)
	if err != nil {
		respondWithError(w, 400, fmt.Sprintf("Error decoding request: %s", err))
		return
	}

	if !chirp.ValidateLength(params.Body) {
		respondWithError(w, 400, "Chirp is too long")
		return
	}

	res := cleanedResponse{CleanedBody: chirp.RedactProfaneWords(params.Body)}

	respondWithJson(w, 200, res)
}

func main() {
	godotenv.Load()
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatal(err)
	}
	dbQueries := database.New(db)

	apiConfig := apiConfig{
		fileserverHits: atomic.Int32{},
		dbQueries:      dbQueries,
	}

	fileServerHandler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	wrappedHandler := apiConfig.middlewareMetricsInc(fileServerHandler)

	mux := http.NewServeMux()
	mux.Handle("/app/", wrappedHandler)
	mux.Handle("/assets/logo.png", http.FileServer(http.Dir(".")))

	//api endpoints
	mux.HandleFunc("GET /api/healthz", readinessEndpoint)
	mux.HandleFunc("POST /api/validate_chirp", validateChirp)

	// create user endpoint
	mux.HandleFunc("POST /api/users", apiConfig.createUser)

	// admin endpoints
	mux.HandleFunc("GET /admin/metrics", apiConfig.writeRequestsCounter)
	mux.HandleFunc("POST /admin/reset", apiConfig.resetFileserverHits)

	server := &http.Server{
		Addr:    ":8080",
		Handler: mux,
	}

	log.Fatal(server.ListenAndServe())

}
