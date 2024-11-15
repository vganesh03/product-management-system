package imageprocessing

import (
	"image"
	"image/jpeg"
	"log"
	"net/http"
	"os"

	"github.com/nfnt/resize"
	"github.com/streadway/amqp"
)

// StartImageProcessing initializes the message queue connection and starts consuming messages
func StartImageProcessing() {
	conn, err := amqp.Dial("amqp://guest:guest@localhost:5672/")
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	channel, err := conn.Channel()
	if err != nil {
		log.Fatal(err)
	}
	defer channel.Close()

	msgs, err := channel.Consume(
		"image_queue", // Queue name
		"",            // Consumer
		true,          // Auto-ack
		false,         // Exclusive
		false,         // No-local
		false,         // No-wait
		nil,           // Arguments
	)
	if err != nil {
		log.Fatal(err)
	}

	for msg := range msgs {
		imageURL := string(msg.Body) // Assuming `msg.Body` contains the image URL

		// Download, compress, and save the image
		err := downloadAndCompressImage(imageURL)
		if err != nil {
			log.Println("Error processing image:", err)
		}
	}
}

// downloadAndCompressImage downloads the image from the given URL, compresses it, and saves it
func downloadAndCompressImage(imageURL string) error {
	response, err := http.Get(imageURL)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	img, _, err := image.Decode(response.Body)
	if err != nil {
		return err
	}

	// Compress the image to a width of 800 pixels, maintaining aspect ratio
	compressedImage := resize.Resize(800, 0, img, resize.Lanczos3)

	file, err := os.Create("compressed.jpg")
	if err != nil {
		return err
	}
	defer file.Close()

	err = jpeg.Encode(file, compressedImage, nil)
	if err != nil {
		return err
	}

	log.Println("Image processed and saved successfully")
	return nil
}
