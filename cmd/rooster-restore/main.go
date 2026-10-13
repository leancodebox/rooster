package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/leancodebox/rooster/internal/roosterapp"
)

func main() {
	source := flag.String("backup", "", "downloaded standalone SQLite backup")
	destination := flag.String("destination", "", "new data directory (must not already exist)")
	flag.Parse()
	if *source == "" || *destination == "" || flag.NArg() != 0 {
		flag.Usage()
		os.Exit(2)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := roosterapp.RestoreSnapshot(ctx, *source, *destination); err != nil {
		fmt.Fprintln(os.Stderr, "Restore failed:", err)
		os.Exit(1)
	}
	fmt.Println("Restored to", *destination)
	fmt.Println("All tasks are disabled. Log files are not included in the snapshot.")
}
