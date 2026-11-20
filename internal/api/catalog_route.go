// Copyright 2026 Zyvor AI Labs · https://zyvor.dev
// SPDX-License-Identifier: LicenseRef-Zyvor-Production-1.0

package api

import (
	"net/http"

	"github.com/zyvorai/yard/internal/connectors"
	"github.com/zyvorai/yard/internal/model"
)

func (s *Server) connectorCatalog(w http.ResponseWriter, r *http.Request, _ *model.User) {
	if r.Method != http.MethodGet {
		writeJSON(w, 405, map[string]string{"error": "method"})
		return
	}
	writeJSON(w, 200, connectors.Catalog())
}
