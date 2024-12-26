package utils

import (
	"fmt"
	"net/http"
)

func ResponseErrorJSON(w http.ResponseWriter, code int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write([]byte(fmt.Sprintf(`{"message":"%s"}`, err.Error())))
}
