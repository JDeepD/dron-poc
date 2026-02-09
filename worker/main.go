package main
import (
	"log"
	"flag"
	config "github.com/jdeepd/dron-poc/common"
)

func main() {
	log.SetFlags(log.LstdFlags)

	name := flag.String("name", "", "Name of the worker")
	coordinatorAddress := flag.String("coordinator", config.DefaultCoordinatorAddress, "Address of the coordinator")
	address := flag.String("address", "", "Address of the worker")
	flag.Parse()


	if *name == "" {
		log.Fatal("Worker name is required")
	}
	if *address == "" {
		log.Fatal("Worker address is required")
	}
	if *coordinatorAddress == "" {
		log.Fatal("Coordinator address is required")
	}
	
	log.Printf("Worker %s started. Worker address: %s", *name, *address)
}