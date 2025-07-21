package main

import (
	"fmt"
	"modular-blockchain/config"
)

func main() {
	config.InitEnv()

	version := config.GetEnvVar("PROJECT_VERSION")

	fmt.Println(version)
}
