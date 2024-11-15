package tests

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"product-management-system/api"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCreateProduct(t *testing.T) {
	router := api.SetupRouter()

	product := `{
		"user_id": 1,
		"product_name": "Test Product",
		"product_description": "This is a test product.",
		"product_images": ["http://example.com/image1.jpg"],
		"product_price": 99.99
	}`

	req, _ := http.NewRequest("POST", "/products", bytes.NewBufferString(product))
	req.Header.Set("Content-Type", "application/json")

	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusCreated, resp.Code)
}

func TestGetProductByID(t *testing.T) {
	router := api.SetupRouter()

	req, _ := http.NewRequest("GET", "/products/1", nil)
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)

	assert.Equal(t, http.StatusOK, resp.Code)
}
