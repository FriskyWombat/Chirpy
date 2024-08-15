package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/FriskyWombat/chirpy/internal/database"
)

type apiConfig struct {
	fileserverHits int
	db             *database.DB
}

// NewConfig Default constructor for apiConfig
func newConfig() apiConfig {
	d, err := database.NewDB("database.json")
	fmt.Println(err)
	return apiConfig{
		fileserverHits: 0,
		db:             d,
	}
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	type error struct {
		Error string `json:"error"`
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	e := error{Error: msg}
	dat, err := json.Marshal(e)
	if err != nil {
		return
	}
	w.Write(dat)
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	dat, err := json.Marshal(payload)
	if err != nil {
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(dat)
}

func cleanChirp(body string) string {
	words := strings.Fields(body)
	profaneIndices := []int{}
	for i, word := range words {
		word = strings.ToLower(word)
		if word == "kerfuffle" || word == "sharbert" || word == "fornax" {
			profaneIndices = append(profaneIndices, i)
		}
	}
	if len(profaneIndices) > 0 {

		for _, i := range profaneIndices {
			words[i] = "****"
		}
		return strings.Join(words, " ")
	}
	return body
}
