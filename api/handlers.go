package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"product-management-system/cache"
	"product-management-system/database"
	"product-management-system/logging"

	"github.com/gin-gonic/gin"
	"github.com/go-redis/redis/v8"
	"github.com/streadway/amqp"
)

var ctx = context.Background()

// CreateProduct handles the creation of a new product
func CreateProduct(c *gin.Context) {
	var product struct {
		UserID             int      `json:"user_id"`
		ProductName        string   `json:"product_name"`
		ProductDescription string   `json:"product_description"`
		ProductImages      []string `json:"product_images"`
		ProductPrice       float64  `json:"product_price"`
	}

	if err := c.ShouldBindJSON(&product); err != nil {
		logging.Logger.Error("Invalid product data: ", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid product data"})
		return
	}

	// Insert product data into the database
	query := `INSERT INTO products (user_id, product_name, product_description, product_images, product_price) 
              VALUES ($1, $2, $3, $4, $5) RETURNING id`
	var productID int
	err := database.DB.QueryRow(query, product.UserID, product.ProductName, product.ProductDescription,
		product.ProductImages, product.ProductPrice).Scan(&productID)

	if err != nil {
		logging.Logger.Error("Failed to insert product: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create product"})
		return
	}

	// Enqueue the image URLs for processing asynchronously
	go enqueueImagesForProcessing(product.ProductImages, productID)

	c.JSON(http.StatusCreated, gin.H{"product_id": productID})
}

// GetProductByID retrieves product details by its ID
func GetProductByID(c *gin.Context) {
	productID := c.Param("id")
	cacheClient := cache.Connect()

	// Check if product data is cached
	cachedProduct, err := cacheClient.Get(ctx, productID).Result()
	if err == redis.Nil {
		// Product not found in cache, query from database
		query := `SELECT user_id, product_name, product_description, product_images, 
                  compressed_product_images, product_price FROM products WHERE id=$1`
		var product struct {
			UserID                  int
			ProductName             string
			ProductDescription      string
			ProductImages           []string
			CompressedProductImages []string
			ProductPrice            float64
		}
		err := database.DB.QueryRow(query, productID).Scan(&product.UserID, &product.ProductName,
			&product.ProductDescription, &product.ProductImages, &product.CompressedProductImages,
			&product.ProductPrice)

		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "Product not found"})
			return
		} else if err != nil {
			logging.Logger.Error("Database query error: ", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve product"})
			return
		}

		// Cache the product data
		productJSON, _ := json.Marshal(product)
		cacheClient.Set(ctx, productID, productJSON, 0)

		c.JSON(http.StatusOK, product)
	} else if err != nil {
		logging.Logger.Error("Cache retrieval error: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve product from cache"})
	} else {
		var product struct {
			UserID                  int
			ProductName             string
			ProductDescription      string
			ProductImages           []string
			CompressedProductImages []string
			ProductPrice            float64
		}
		json.Unmarshal([]byte(cachedProduct), &product)
		c.JSON(http.StatusOK, product)
	}
}

// GetProductsByUserID retrieves all products for a specific user with optional filtering
func GetProductsByUserID(c *gin.Context) {
	userID := c.Query("user_id")
	minPrice := c.Query("min_price")
	maxPrice := c.Query("max_price")
	productName := c.Query("product_name")

	query := `SELECT id, product_name, product_description, product_price FROM products WHERE user_id=$1`
	args := []interface{}{userID}

	if minPrice != "" && maxPrice != "" {
		query += " AND product_price BETWEEN $2 AND $3"
		args = append(args, minPrice, maxPrice)
	}

	if productName != "" {
		query += " AND product_name ILIKE $4"
		args = append(args, "%"+productName+"%")
	}

	rows, err := database.DB.Query(query, args...)
	if err != nil {
		logging.Logger.Error("Database query error: ", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to retrieve products"})
		return
	}
	defer rows.Close()

	products := []map[string]interface{}{}
	for rows.Next() {
		var id int
		var name, description string
		var price float64
		rows.Scan(&id, &name, &description, &price)
		products = append(products, gin.H{"id": id, "name": name, "description": description, "price": price})
	}

	c.JSON(http.StatusOK, products)
}

// enqueueImagesForProcessing sends image URLs to a RabbitMQ queue for processing
func enqueueImagesForProcessing(imageURLs []string, productID int) {
	// Connect to RabbitMQ
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		logging.Logger.Error("Failed to connect to RabbitMQ: ", err)
		return
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		logging.Logger.Error("Failed to open a channel: ", err)
		return
	}
	defer channel.Close()

	// Declare a queue
	_, err = channel.QueueDeclare(
		"image_queue", // Queue name
		true,          // Durable
		false,         // Delete when unused
		false,         // Exclusive
		false,         // No-wait
		nil,           // Arguments
	)
	if err != nil {
		logging.Logger.Error("Failed to declare a queue: ", err)
		return
	}

	// Send each image URL to the queue
	for _, imageURL := range imageURLs {
		// Prepare the message to be published
		message := imageURL // You can customize this format if needed

		err = channel.Publish(
			"",            // Exchange
			"image_queue", // Routing key
			false,         // Mandatory
			false,         // Immediate
			amqp.Publishing{
				ContentType: "text/plain",
				Body:        []byte(message),
			},
		)
		if err != nil {
			log.Println("Failed to publish message: ", err)
		} else {
			log.Println("Image URL enqueued successfully: ", imageURL)
		}
	}
}
