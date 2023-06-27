/*
 *  Copyright (c) 2023 Juice Technologies, Inc. All Rights Reserved.
 */
package prometheus

import (
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/gorilla/mux"

	"github.com/Juice-Labs/Juice-Labs/pkg/server"
)

func InitializeEndpoints(server *server.Server) {
	server.AddCreateEndpoint(getMetrics)
}

func getMetrics(router *mux.Router) error {
	router.Methods("GET").Path("/v1/prometheus/metrics").Handler(
		promhttp.Handler())

	return nil
}
