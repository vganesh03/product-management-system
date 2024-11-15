package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"product-management-system/api"
	"product-management-system/cache"
	"product-management-system/database"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp"
	"github.com/stretchr/testify/assert"
)

var ctx = context.Background()

func setupRouter() *gin.Engine {
	// Use the SetupRouter function to set up routes
	return api.SetupRouter()
}

func TestCreateProductInAPI(t *testing.T) {
	router := setupRouter()

	productData := map[string]interface{}{
		"user_id":             1,
		"product_name":        "Test Product",
		"product_description": "This is a test product",
		"product_images":      []string{"http://example.com/image1.jpg", "http://example.com/image2.jpg"},
		"product_price":       29.99,
	}
	productJSON, _ := json.Marshal(productData)

	req, err := http.NewRequest(http.MethodPost, "/products", bytes.NewBuffer(productJSON))
	if err != nil {
		t.Fatalf("Failed to create request: %v", err)
	}

	rr := performRequest(router, req)

	assert.Equal(t, http.StatusCreated, rr.Code)

	var productID int
	query := `SELECT id FROM products WHERE product_name=$1`
	err = database.DB.QueryRow(query, "Test Product").Scan(&productID)
	if err != nil {
		t.Fatalf("Failed to query the database: %v", err)
	}

	assert.Greater(t, productID, 0, "Product ID should be greater than 0")

	cacheClient := cache.Connect()
	cachedProduct, err := cacheClient.Get(ctx, "1").Result()
	if err == redis.Nil {
		t.Fatalf("Product not found in Redis cache")
	}
	assert.NotNil(t, cachedProduct, "Product data should be cached")
}

// Renamed the function to avoid redeclaration conflict
func TestGetProductByIDAfterCreation(t *testing.T) {
	router := setupRouter()

	productData := map[string]interface{}{
		"user_id":             1,
		"product_name":        "Test Product",
		"product_description": "This is a test product",
		"product_images":      []string{"http://example.com/image1.jpg"},
		"product_price":       29.99,
	}
	productJSON, _ := json.Marshal(productData)
	req, _ := http.NewRequest(http.MethodPost, "/products", bytes.NewBuffer(productJSON))

	rr := performRequest(router, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	reqGet, _ := http.NewRequest(http.MethodGet, "/products/1", nil)
	rrGet := performRequest(router, reqGet)

	assert.Equal(t, http.StatusOK, rrGet.Code)

	var product struct {
		UserID             int      `json:"user_id"`
		ProductName        string   `json:"product_name"`
		ProductDescription string   `json:"product_description"`
		ProductImages      []string `json:"product_images"`
		ProductPrice       float64  `json:"product_price"`
	}
	err := json.NewDecoder(rrGet.Body).Decode(&product)
	assert.NoError(t, err)
	assert.Equal(t, "Test Product", product.ProductName)
	assert.Equal(t, 29.99, product.ProductPrice)
}

// Renamed the function to avoid redeclaration conflict
func TestRabbitMQImageProcessing(t *testing.T) {
	router := setupRouter()

	productData := map[string]interface{}{
		"user_id":             1,
		"product_name":        "Test Product",
		"product_description": "Test description",
		"product_images":      []string{"http://example.com/image1.jpg", "http://example.com/image2.jpg"},
		"product_price":       25.99,
	}
	productJSON, _ := json.Marshal(productData)
	req, _ := http.NewRequest(http.MethodPost, "/products", bytes.NewBuffer(productJSON))

	rr := performRequest(router, req)
	assert.Equal(t, http.StatusCreated, rr.Code)

	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		t.Fatalf("Failed to connect to RabbitMQ: %v", err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		t.Fatalf("Failed to open a channel: %v", err)
	}
	defer channel.Close()

	queueName := "image_queue"
	channel.QueueDeclare(queueName, true, false, false, false, nil)

	msgs, err := channel.Consume(queueName, "", true, false, false, false, nil)
	if err != nil {
		t.Fatalf("Failed to consume messages: %v", err)
	}

	select {
	case msg := <-msgs:
		// Use bytes.Equal for comparing slices
		if bytes.Equal(msg.Body, []byte("http://example.com/image1.jpg")) || bytes.Equal(msg.Body, []byte("http://example.com/image2.jpg")) {
			// Test passed
		} else {
			t.Fatalf("Received unexpected message: %s", msg.Body)
		}
	default:
		t.Fatal("No messages received from RabbitMQ")
	}
}

// Helper function to perform HTTP requests and return the response recorder
func performRequest(router http.Handler, req *http.Request) *httptest.ResponseRecorder {
	rr := httptest.NewRecorder()
	router.ServeHTTP(rr, req)
	return rr
}
