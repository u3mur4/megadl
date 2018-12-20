package main

import (
	"io"
	"log"
	"os"

	"github.com/u3mur4/megadl"
	pb "gopkg.in/cheggaaa/pb.v1"
)

func exitIfErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func main() {
	if len(os.Args) != 2 {
		return
	}

	// create reader
	reader, info, err := megadl.Download(os.Args[1])
	exitIfErr(err)
	defer reader.Close()

	// create progress bar
	bar := pb.New(info.Size).SetUnits(pb.U_BYTES).Prefix(info.Name + " ")
	bar.Start()
	reader = bar.NewProxyReader(reader)

	// create writer
	outFile, err := os.Create(info.Name)
	exitIfErr(err)
	defer outFile.Close()

	// go
	if _, err := io.Copy(outFile, reader); err != nil {
		exitIfErr(err)
	}

	bar.Finish()
}
