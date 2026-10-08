package main

import (
	"fmt"
	"log"

	"tibidle-bot/pkg/auth"
)

func main() {
	cookies := "did=14d2df158e5f0f5e3df5bdb755bec55100eee1ba2937fc585a65e5c57e8a4a65; sid=9189720e9b70b76e8f2cbf07fdc05ed127b343f7313d13619bf65b48a42358b7"
	ticket, err := auth.FetchTicketViaBrowser(cookies)
	if err != nil {
		log.Fatalf("Error: %v", err)
	}
	fmt.Printf("FRESH_TICKET:%s\n", ticket)
}
