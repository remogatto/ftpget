package main

import (
	"github.com/remogatto/ftpget"
	"log"
	"os"
	"path"
)

func main() {
	log.SetFlags(0)
	ftp.Log = true

	// Synchronous file transfer
	{
		const url = "ftp.worldofspectrum.org/pub/sinclair/games/e/EarthAttack.tap.zip"
		log.Println("Retrieving", url)

		f, err := os.Create(path.Base(url))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		err = ftp.Get(url, f)
		if err != nil {
			log.Fatal(err)
		}

		log.Println("Transfer completed")
	}

	// Asynchronous file transfer
	{
		const url = "ftp.worldofspectrum.org/pub/sinclair/games/e/Eagle.tap.zip"
		log.Println("Retrieving", url)

		f, err := os.Create(path.Base(url))
		if err != nil {
			log.Fatal(err)
		}
		defer f.Close()

		// GetAsync spawns the fetching routine but doesn't wait for
		// the transfer to finish. It returns a Transfer object in
		// order to control the transfer status through channels.
		transfer, err := ftp.GetAsync(url, f)
		if err != nil {
			log.Fatal(err)
		}

		// Control the transfer status and errors.
		// The state diagram of the transfer is:
		// STARTED --> COMPLETED
		//         |
		//         --> ABORTED
		//         |
		//         --> ERROR (in this case you should drain the Error channel)
		//
		switch <-transfer.Status {
		case ftp.STARTED:
			switch <-transfer.Status {
			case ftp.COMPLETED:
				log.Println("Transfer completed")
			case ftp.ABORTED:
				log.Println("Transfer aborted")
			case ftp.ERROR:
				log.Fatal(<-transfer.Error)
			default:
				panic("Unknown status")
			}
		}
	}
}
