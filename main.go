package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/FriskyWombat/chirpy/internal/database"
)

func main() {
	apiCfg := newConfig()
	serverMux := http.NewServeMux()
	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serverMux.Handle("/app/*", apiCfg.middlewareMetricsInc(handler))
	serverMux.HandleFunc("GET /api/healthz", healthzHandleFunc)
	serverMux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandleFunc)
	serverMux.HandleFunc("/api/reset", apiCfg.metricsResetHandleFunc)
	serverMux.HandleFunc("POST /api/chirps", apiCfg.chirpHandleFunc)
	serverMux.HandleFunc("GET /api/chirps", apiCfg.chirpsGetAllHandleFunc)
	serverMux.HandleFunc("GET /api/chirps/{id}", apiCfg.chirpGetHandleFunc)
	serverMux.HandleFunc("POST /api/users", apiCfg.newUserHandleFunc)
	serverMux.HandleFunc("POST /api/login", apiCfg.loginHandleFunc)
	serverMux.HandleFunc("PUT /api/users", apiCfg.updateUserHandleFunc)

	server := http.Server{
		Addr:    ":8080",
		Handler: serverMux,
	}
	server.ListenAndServe()
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits++
		next.ServeHTTP(w, r)
	})
}

func healthzHandleFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8 ")
	w.WriteHeader(200)
	w.Write([]byte("OK"))
}

const metricsHTML = `
<html>

<body>
    <h1>Welcome, Chirpy Admin</h1>
    <p>Chirpy has been visited %d times!</p>
</body>

</html>
`

func (cfg *apiConfig) metricsHandleFunc(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8 ")
	w.WriteHeader(200)
	w.Write([]byte(fmt.Sprintf(metricsHTML, cfg.fileserverHits)))
}

func (cfg *apiConfig) metricsResetHandleFunc(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(200)
	cfg.fileserverHits = 0
}

func (cfg *apiConfig) chirpHandleFunc(w http.ResponseWriter, r *http.Request) {
	type chirpReq struct {
		Body string `json:"body"`
	}
	req := chirpReq{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	if len(req.Body) > 140 {
		respondWithError(w, 400, "Chirp is too long")
		return
	}
	body := cleanChirp(req.Body)
	chirp, err := cfg.db.CreateChirp(body)
	if err != nil {
		fmt.Println(err)
		respondWithError(w, 500, "Unable to access database")
		return
	}
	respondWithJSON(w, 201, chirp)
}

func (cfg *apiConfig) chirpGetHandleFunc(w http.ResponseWriter, r *http.Request) {
	type chirpsListResp []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
	}
	i, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		respondWithError(w, 404, "Invalid ID")
		return
	}
	c, err := cfg.db.GetChirp(i)
	if err != nil {
		respondWithError(w, 404, "That chirp does not exist")
		return
	}
	respondWithJSON(w, 200, c)
}

func (cfg *apiConfig) chirpsGetAllHandleFunc(w http.ResponseWriter, r *http.Request) {
	type chirpsListResp []struct {
		ID   int    `json:"id"`
		Body string `json:"body"`
	}
	c, err := cfg.db.GetChirps()
	if err != nil {
		respondWithError(w, 500, "Unable to access database")
		return
	}
	respondWithJSON(w, 200, c)
}

func (cfg *apiConfig) newUserHandleFunc(w http.ResponseWriter, r *http.Request) {
	type userReq struct {
		Password     string `json:"password"`
		Email        string `json:"email"`
		ExpiresAfter int    `json:"expires_in_seconds"`
	}
	req := userReq{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	user, err := cfg.db.CreateUser(req.Email, req.Password)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}
	u := database.SafeUser{ID: user.ID, Email: user.Email}
	respondWithJSON(w, 201, u)
}

func (cfg *apiConfig) updateUserHandleFunc(w http.ResponseWriter, r *http.Request) {

}

func (cfg *apiConfig) loginHandleFunc(w http.ResponseWriter, r *http.Request) {
	type userReq struct {
		Password string `json:"password"`
		Email    string `json:"email"`
	}
	req := userReq{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	u, ok, err := cfg.db.VerifyCredentials(req.Email, req.Password)
	if err != nil {
		respondWithError(w, 500, "Unable to find user")
		return
	}
	if !ok {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	respondWithJSON(w, 200, u)
}
