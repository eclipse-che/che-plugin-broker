//
// Copyright (c) 2018-2019 Red Hat, Inc.
// This program and the accompanying materials are made
// available under the terms of the Eclipse Public License 2.0
// which is available at https://www.eclipse.org/legal/epl-2.0/
//
// SPDX-License-Identifier: EPL-2.0
//
// Contributors:
//   Red Hat, Inc. - initial API and implementation
//

package utils

import (
	"errors"
	"net/http"
	"regexp"
	"testing"

	"github.com/eclipse/che-plugin-broker/utils/mocks"
	"github.com/stretchr/testify/assert"
)

const expectedResponseBody = "Test body"

func TestIoUtil_Fetch(t *testing.T) {
	type args struct {
		URL string
	}
	type want struct {
		expected  []byte
		errRegexp *regexp.Regexp
	}
	type clientMocks struct {
		response *http.Response
		isClosed func() bool
		err      error
	}

	generateMocks := func(body string, status int, err error, failRequest bool) clientMocks {
		var response *http.Response
		var isClosed func() bool
		if failRequest {
			response, isClosed = mocks.GenerateErrorResponse(body, status)
		} else {
			response, isClosed = mocks.GenerateResponse(body, status)
		}
		return clientMocks{
			response: response,
			err:      err,
			isClosed: isClosed,
		}
	}

	tests := []struct {
		name  string
		args  args
		mocks clientMocks
		want  want
	}{
		{
			name: "Returns error when Get returns error",
			args: args{
				URL: "test.url",
			},
			want: want{
				expected:  nil,
				errRegexp: regexp.MustCompile("failed to get data from .*"),
			},
			mocks: generateMocks("testBody", http.StatusForbidden, errors.New("TestError"), false),
		},
		{
			name: "Returns error when http request not status OK",
			args: args{
				URL: "test.url",
			},
			want: want{
				expected:  nil,
				errRegexp: regexp.MustCompile("Downloading .* failed. Status code .*"),
			},
			mocks: generateMocks("testBody", http.StatusForbidden, nil, false),
		},
		{
			name: "Returns error when read body results in error",
			args: args{
				URL: "test.url",
			},
			want: want{
				expected:  nil,
				errRegexp: regexp.MustCompile("failed to read response body: .*"),
			},
			mocks: generateMocks("testBody", http.StatusOK, nil, true),
		},
		{
			name: "Returns data from response",
			args: args{
				URL: "test.url",
			},
			want: want{
				expected:  []byte(expectedResponseBody),
				errRegexp: nil,
			},
			mocks: generateMocks(expectedResponseBody, http.StatusOK, nil, false),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			util := &impl{
				mocks.NewTestHTTPClient(tt.mocks.response, tt.mocks.err),
			}
			actual, err := util.Fetch(tt.args.URL)
			if tt.want.errRegexp != nil {
				assertErrorMatches(t, tt.want.errRegexp, err)
				return
			}
			assert.NoError(t, err)
			if tt.want.expected != nil {
				assert.Equal(t, tt.want.expected, actual)
			}
			if !tt.mocks.isClosed() {
				t.Errorf("Should close response body")
			}
		})
	}
}

func assertErrorMatches(t *testing.T, expected *regexp.Regexp, actual error) {
	if actual == nil {
		t.Errorf("Expected error %s but got nil", expected.String())
	} else if !expected.MatchString(actual.Error()) {
		t.Errorf("Error message does not match. Expected '%s' but got '%s'", expected.String(), actual.Error())
	}
}
