/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package logger

import (
	"net/http"
)

func Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		Debugf("%s %s %s", r.RemoteAddr, r.Method, r.RequestURI)

		next.ServeHTTP(w, r)
	})
}
