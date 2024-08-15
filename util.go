package main

import (
	"encoding/json"
	"net/http"
)

type apiConfig struct {
	fileserverHits int
	chirps         map[int]string
	chirpIndex     int
}

// NewConfig Default constructor for apiConfig
func newConfig() apiConfig {
	return apiConfig{
		fileserverHits: 0,
		chirps:         make(map[int]string),
		chirpIndex:     0,
	}
}

func (cfg *apiConfig) addChirp(body string) int {
	cfg.chirps[cfg.chirpIndex] = body
	cfg.chirpIndex++
	return cfg.chirpIndex - 1
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
