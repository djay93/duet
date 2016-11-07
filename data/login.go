package data

import (
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/ant0ine/go-json-rest/rest"
	"github.com/dgrijalva/jwt-go"
)

type usernameAndPassword struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type DuetClaims struct {
	jwt.StandardClaims
}

var tokenSecret []byte = []byte("someSecret")

func ServeLogin(w rest.ResponseWriter, r *rest.Request) {
	userAndPass := usernameAndPassword{}
	err := r.DecodeJsonPayload(&userAndPass)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// TODO verify password
	log.Printf("Username: %s, Password: %s\n", userAndPass.Username, userAndPass.Password)

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, DuetClaims{
		jwt.StandardClaims{
			Subject:  userAndPass.Username,
			Issuer:   "Duet",
			Audience: "https://api.helloduet.com",
		},
	})

	tokenString, err := token.SignedString(tokenSecret)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteJson(map[string]string{
		"token": tokenString,
	})
}

func VerifyToken(tokenString string) (*DuetClaims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &DuetClaims{}, func(token *jwt.Token) (interface{}, error) {
		if token.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, fmt.Errorf("Unexpected signing method: %v", token.Header["alg"])
		}
		return tokenSecret, nil
	})

	log.Printf("Verifying token %s\n", tokenString)

	if err != nil {
		return nil, err
	}

	if claims, ok := token.Claims.(*DuetClaims); ok && token.Valid {
		return claims, nil
	} else {
		return nil, fmt.Errorf("Token could not be parsed")
	}
}

func ServeVerifyToken(w rest.ResponseWriter, r *rest.Request) {
	authorization := r.Header.Get("Authorization")
	if !strings.HasPrefix(authorization, "Bearer ") {
		rest.Error(w, "Invalid authentication method", http.StatusUnauthorized)
		return
	}

	tokenString := strings.TrimPrefix(authorization, "Bearer ")
	claims, err := VerifyToken(tokenString)
	if err != nil {
		rest.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	if claims == nil {
		rest.Error(w, "Claims are nil", http.StatusInternalServerError)
		return
	}
	w.WriteJson(claims)
}