package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/FriskyWombat/chirpy/internal/database"
	"github.com/joho/godotenv"
)

func main() {
	godotenv.Load()
	apiCfg := newConfig(os.Getenv("JWT_SECRET"), os.Getenv("POLKA_API_KEY"))
	serverMux := http.NewServeMux()
	handler := http.StripPrefix("/app", http.FileServer(http.Dir(".")))
	serverMux.Handle("/app/*", apiCfg.middlewareMetricsInc(handler))
	serverMux.HandleFunc("GET /api/healthz", healthzHandleFunc)
	serverMux.HandleFunc("GET /admin/metrics", apiCfg.metricsHandleFunc)
	serverMux.HandleFunc("/api/reset", apiCfg.metricsResetHandleFunc)
	serverMux.HandleFunc("POST /api/chirps", apiCfg.chirpHandleFunc)
	serverMux.HandleFunc("GET /api/chirps", apiCfg.chirpsGetAllHandleFunc)
	serverMux.HandleFunc("GET /api/chirps/{id}", apiCfg.chirpGetHandleFunc)
	serverMux.HandleFunc("DELETE /api/chirps/{id}", apiCfg.chirpDeleteHandleFunc)
	serverMux.HandleFunc("POST /api/users", apiCfg.newUserHandleFunc)
	serverMux.HandleFunc("POST /api/login", apiCfg.loginHandleFunc)
	serverMux.HandleFunc("PUT /api/users", apiCfg.updateUserHandleFunc)
	serverMux.HandleFunc("POST /api/refresh", apiCfg.refreshHandleFunc)
	serverMux.HandleFunc("POST /api/revoke", apiCfg.revokeHandleFunc)
	serverMux.HandleFunc("POST /api/polka/webhooks", apiCfg.polkaWebhookHandleFunc)

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
	headers := strings.Fields(r.Header.Get("Authorization"))
	if headers[0] != "Bearer" {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	id, err := database.ParseJWTKey(headers[1], cfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	body := cleanChirp(req.Body)
	chirp, err := cfg.db.CreateChirp(id, body)
	if err != nil {
		respondWithError(w, 500, "Unable to access database")
		return
	}
	respondWithJSON(w, 201, chirp)
}

func (cfg *apiConfig) chirpGetHandleFunc(w http.ResponseWriter, r *http.Request) {
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

	chirps, err := cfg.db.GetChirps()
	if err != nil {
		respondWithError(w, 500, "Unable to access database")
		return
	}
	authorID := r.URL.Query().Get("author_id")
	if authorID != "" {
		out := []database.Chirp{}
		for _, chirp := range chirps {
			i, _ := strconv.Atoi(authorID)
			if chirp.AuthorID == i {
				out = append(out, chirp)
			}
		}
		chirps = out
	}
	sortOrder := r.URL.Query().Get("sort")
	sort.Slice(chirps, func(i, j int) bool {
		if sortOrder == "desc" {
			return chirps[i].ID > chirps[j].ID
		}
		return chirps[i].ID < chirps[j].ID
	})
	respondWithJSON(w, 200, chirps)
}

func (cfg *apiConfig) chirpDeleteHandleFunc(w http.ResponseWriter, r *http.Request) {
	i, err := strconv.Atoi(r.PathValue("id"))
	if err != nil {
		respondWithError(w, 404, "Invalid ID")
		return
	}
	headers := strings.Fields(r.Header.Get("Authorization"))
	if headers[0] != "Bearer" {
		respondWithError(w, 403, "Unauthorized")
		return
	}
	err = cfg.db.DeleteChirp(i, headers[1], cfg.jwtSecret)
	if err != nil {
		respondWithError(w, 403, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (cfg *apiConfig) newUserHandleFunc(w http.ResponseWriter, r *http.Request) {
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
	user, err := cfg.db.CreateUser(req.Email, req.Password)
	if err != nil {
		respondWithError(w, 500, err.Error())
		return
	}
	u := database.SafeUser{ID: user.ID, Email: user.Email}
	respondWithJSON(w, 201, u)
}

func (cfg *apiConfig) updateUserHandleFunc(w http.ResponseWriter, r *http.Request) {
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
	headers := strings.Fields(r.Header.Get("Authorization"))
	if headers[0] != "Bearer" {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	id, err := database.ParseJWTKey(headers[1], cfg.jwtSecret)
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	u, err := cfg.db.UpdateUser(id, req.Email, req.Password)
	su := database.SafeUser{ID: u.ID, Email: u.Email, IsPremium: u.IsPremium}
	respondWithJSON(w, 200, su)
}

func (cfg *apiConfig) loginHandleFunc(w http.ResponseWriter, r *http.Request) {
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
	u, ok, err := cfg.db.VerifyCredentials(req.Email, req.Password)
	if err != nil {
		respondWithError(w, 500, "Unable to find user")
		return
	}
	if !ok {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	jwt := database.CreateJWTKey(u, cfg.jwtSecret)
	su := database.SignedUser{ID: u.ID, Email: u.Email, Token: jwt, RefreshToken: u.RefreshToken, IsPremium: u.IsPremium}
	respondWithJSON(w, 200, su)
}

func (cfg *apiConfig) refreshHandleFunc(w http.ResponseWriter, r *http.Request) {
	headers := strings.Fields(r.Header.Get("Authorization"))
	if headers[0] != "Bearer" {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	u, err := cfg.db.GetUserFromRefreshToken(headers[1])
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	jwt := database.CreateJWTKey(u, cfg.jwtSecret)
	type tokenResp struct {
		Token string `json:"token"`
	}
	t := tokenResp{jwt}
	respondWithJSON(w, 200, t)
}

func (cfg *apiConfig) revokeHandleFunc(w http.ResponseWriter, r *http.Request) {
	headers := strings.Fields(r.Header.Get("Authorization"))
	if headers[0] != "Bearer" {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	err := cfg.db.RevokeRefreshToken(headers[1])
	if err != nil {
		respondWithError(w, 401, err.Error())
		return
	}
	w.WriteHeader(204)
}

func (cfg *apiConfig) polkaWebhookHandleFunc(w http.ResponseWriter, r *http.Request) {
	headers := strings.Fields(r.Header.Get("Authorization"))
	if len(headers) < 2 || headers[0] != "ApiKey" || headers[1] != cfg.polkaApiKey {
		respondWithError(w, 401, "Unauthorized")
		return
	}
	type webhoookReq struct {
		Event string `json:"event"`
		Data  struct {
			UserID int `json:"user_id"`
		} `json:"data"`
	}
	req := webhoookReq{}
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&req)
	if err != nil {
		respondWithError(w, 500, "Something went wrong")
		return
	}
	if req.Event != "user.upgraded" {
		respondWithError(w, 204, "Invalid Event")
		return
	}
	err = cfg.db.UpgradeUserToPremium(req.Data.UserID)
	if err != nil {
		respondWithError(w, 404, "Failed to upgrade user")
		return
	}
	w.WriteHeader(204)
}
