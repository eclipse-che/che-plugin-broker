//
// Copyright (c) 2012-2018 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package resttemp

import (
	"net/http"
)

// TODO remove this in favour of the same error in eclipse/che

// APIError represents http error
type APIError struct {
	error
	Code int
}

// ServerError represents http error with 500 code
func ServerError(err error) error {
	return APIError{err, http.StatusInternalServerError}
}
