package models

import "github.com/golang-jwt/jwt/v5"

type JwtCustomClaims struct {
	Login string `json:"login"`
	Admin bool   `json:"admin"`
	jwt.RegisteredClaims
}
