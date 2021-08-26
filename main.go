// Copyright 2021 New Relic Corporation. All rights reserved.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"log"
	"net/http"
)

func main() {
	log.Printf("Starting")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)

		if _, err := w.Write([]byte("ok")); err != nil {
			log.Printf("Writing response: %v", err)
		}
	})
	log.Fatal(http.ListenAndServe(":8443", nil))
}
