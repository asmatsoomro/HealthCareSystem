package main

import (
    "errors"
    "fmt"
    "net/http"
    "strconv"
)

// RBAC
type Role string

const (
    RoleAdmin     Role = "admin"
    RolePhysician Role = "physician"
    RolePatient   Role = "patient"
)

func readRole(r *http.Request) (Role, error) {
    v := r.Header.Get("X-Role")
    switch Role(v) {
    case RoleAdmin, RolePhysician, RolePatient:
        return Role(v), nil
    default:
        return "", fmt.Errorf("invalid or missing X-Role header")
    }
}

// We use X-User-ID to identify the caller (patient or physician id)
func readUserID(r *http.Request) (int64, error) {
    s := r.Header.Get("X-User-ID")
    if s == "" {
        return 0, errors.New("missing X-User-ID header")
    }
    id, err := strconv.ParseInt(s, 10, 64)
    if err != nil || id <= 0 {
        return 0, errors.New("invalid X-User-ID header")
    }
    return id, nil
}
