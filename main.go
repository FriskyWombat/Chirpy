package main

import (
	"fmt"
	"net/http"
)

func main() {
	serverMux := http.NewServeMux()
	server := http.Server{
		Addr:    "localhost:8080",
		Handler: serverMux,
	}
	err := server.ListenAndServe()
	if err != nil {
		fmt.Println(err.Error())
	}
}
