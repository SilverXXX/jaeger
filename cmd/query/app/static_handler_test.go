// Copyright (c) 2017 Uber Technologies, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package app

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jaegertracing/jaeger/pkg/testutils"
)

func TestStaticAssetsHandler(t *testing.T) {
	r := mux.NewRouter()
	handler, err := NewStaticAssetsHandler("fixture", "")
	require.NoError(t, err)
	handler.RegisterRoutes(r)
	server := httptest.NewServer(r)
	defer server.Close()

	httpClient = &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := httpClient.Get(server.URL)
	assert.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

func TestDefaultStaticAssetsRoot(t *testing.T) {
	handler, err := NewStaticAssetsHandler("", "")
	assert.Nil(t, handler)
	assert.Nil(t, err)
}

func TestNotExistingUiConfig(t *testing.T) {
	handler, err := NewStaticAssetsHandler("/foo/bar", "")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Cannot read UI static assets")
	assert.Nil(t, handler)
}

func TestRegisterStaticHandlerPanic(t *testing.T) {
	logger, buf := testutils.NewLogger()
	assert.Panics(t, func() {
		RegisterStaticHandler(mux.NewRouter(), logger, &QueryOptions{StaticAssets: "/foo/bar"})
	})
	assert.Contains(t, buf.String(), "Could not create static assets handler")
	assert.Contains(t, buf.String(), "Cannot read UI static assets")
}

func TestRegisterStaticHandlerNotCreated(t *testing.T) {
	logger, buf := testutils.NewLogger()
	RegisterStaticHandler(mux.NewRouter(), logger, &QueryOptions{})
	assert.Contains(t, buf.String(), "Static handler is not registered")
}

func TestRegisterStaticHandler(t *testing.T) {
	logger, buf := testutils.NewLogger()
	r := mux.NewRouter()
	RegisterStaticHandler(r, logger, &QueryOptions{StaticAssets: "fixture/"})
	assert.Empty(t, buf.String())

	server := httptest.NewServer(r)
	defer server.Close()
	expectedRespString := "Test Favicon\n"

	httpClient = &http.Client{
		Timeout: 2 * time.Second,
	}

	resp, err := httpClient.Get(fmt.Sprintf("%s/favicon.ico", server.URL))
	require.NoError(t, err)
	defer resp.Body.Close()

	respByteArray, _ := ioutil.ReadAll(resp.Body)
	respString := string(respByteArray)

	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, expectedRespString, respString)
}

func TestNewStaticAssetsHandlerWithConfig(t *testing.T) {
	_, err := NewStaticAssetsHandler("fixture", "fixture/invalid-config")
	assert.Error(t, err)

	handler, err := NewStaticAssetsHandler("fixture", "fixture/ui-config.json")
	require.NoError(t, err)
	require.NotNil(t, handler)
	html := string(handler.indexHTML)
	assert.True(t, strings.Contains(html, `JAEGER_CONFIG = {"x":"y"};`), "actual: %v", html)
}

func TestLoadUIConfig(t *testing.T) {
	type testCase struct {
		configFile    string
		expected      map[string]interface{}
		expectedError string
	}

	run := func(description string, testCase testCase) {
		t.Run(description, func(t *testing.T) {
			config, err := loadUIConfig(testCase.configFile)
			if testCase.expectedError != "" {
				assert.EqualError(t, err, testCase.expectedError)
			} else {
				assert.NoError(t, err)
			}
			assert.EqualValues(t, testCase.expected, config)
		})
	}

	run("no config", testCase{})
	run("invalid config", testCase{
		configFile:    "invalid",
		expectedError: "Cannot read UI config file invalid: open invalid: no such file or directory",
	})
	run("unsupported type", testCase{
		configFile:    "fixture/ui-config.toml",
		expectedError: "Unrecognized UI config file format fixture/ui-config.toml",
	})
	run("malformed", testCase{
		configFile:    "fixture/ui-config-malformed.json",
		expectedError: "Cannot parse UI config file fixture/ui-config-malformed.json: invalid character '=' after object key",
	})
	run("json", testCase{
		configFile: "fixture/ui-config.json",
		expected:   map[string]interface{}{"x": "y"},
	})
	run("json-menu", testCase{
		configFile: "fixture/ui-config-menu.json",
		expected: map[string]interface{}{
			"menu": []interface{}{
				map[string]interface{}{
					"label": "GitHub",
					"url":   "https://github.com/jaegertracing/jaeger",
				},
			},
		},
	})
}
