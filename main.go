package main

import (
	"product-management-system/api"
	"product-management-system/database"
	"product-management-system/logging"
)

func main() {
	logging.Init()
	database.Connect()

	router := api.SetupRouter()
	router.Run(":8080")
}
