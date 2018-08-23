package api

import (
	"encoding/json"
	"testing"
	"github.com/gin-gonic/gin"
	"net/http"
	"net/http/httptest"
	"io/ioutil"
	"github.com/stretchr/testify/assert"
	"github.com/eclipse/che-plugin-broker/model"
)

func TestGetStatusIdle(t *testing.T) {
	router := gin.Default()
	SetUpRouter(router)
	// Create a response recorder
	w := httptest.NewRecorder()

	req, _ := http.NewRequest("GET", "/status", nil)

	// Create the service and process the above request.
	router.ServeHTTP(w, req)

	// Test that the http status code is 200
	if w.Code != http.StatusOK {
		t.Fail()
		return
	}
	p, err := ioutil.ReadAll(w.Body)
	if err != nil {
		t.Fail()
		return
	}
	var status Status
	err = json.Unmarshal(p, &status)
	assert.Equal(t, model.StatusIdle, status.Status)
}

func TestGetResultsOnIdle(t *testing.T) {
	router := gin.Default()
	SetUpRouter(router)
	// Create a response recorder
	w := httptest.NewRecorder()

	req, _ := http.NewRequest("GET", "/", nil)

	// Create the service and process the above request.
	router.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Fail()
		return
	}
	p, err := ioutil.ReadAll(w.Body)
	if err != nil {
		t.Fail()
		return
	}
	

}