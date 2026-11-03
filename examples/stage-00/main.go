package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"

	"stage-00/github"
)

func main() {
	login := flag.String("user", "leehainuo", "public Github username")
	out := flag.String("out", "result.txt", "output file path")
	flag.Parse()

	client := &http.Client{Timeout: 10 * time.Second}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	profile, err := github.FetchProfile(ctx, client, "https://api.github.com", *login)
	if err != nil {
		log.Fatal(err)
	}

	res := github.Summary(profile)
	if err := github.WriteSummary(*out, res); err != nil {
		log.Fatal(err)
	}

	fmt.Println(res)
	fmt.Println(*out)
}
