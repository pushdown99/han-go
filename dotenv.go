package main

import (
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
)

func goDotEnvVariable(key string) string {
   err := godotenv.Load(".env")
   if err != nil {
      log.Fatalf("Error loading .env file")
   }
   return os.Getenv(key)
}

func main() {
   dotenv := goDotEnvVariable("STRONGEST_AVENGER")
   fmt.Printf("godotenv : %s = %s \n", "STRONGEST_AVENGER", dotenv)
}